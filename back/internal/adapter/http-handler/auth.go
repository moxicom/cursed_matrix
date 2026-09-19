package httphandler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	gen "github.com/moxicom/cursed_matrix/back/internal/adapter/http-handler/gen"
	"github.com/moxicom/cursed_matrix/back/internal/app/auth"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/user"
)

// API implements the generated server interface. It decodes, calls the use
// case, and renders — no rule of the product lives here.
type API struct {
	auth       *auth.Service
	cookies    *CookieWriter
	refreshTTL time.Duration
}

// NewAPI wires the handlers to the use cases.
func NewAPI(service *auth.Service, cookies *CookieWriter, refreshTTL time.Duration) *API {
	return &API{auth: service, cookies: cookies, refreshTTL: refreshTTL}
}

// Register creates an account and signs it in.
func (a *API) Register(w http.ResponseWriter, r *http.Request) {
	var body gen.RegisterJSONRequestBody
	if !decode(w, r, &body) {
		return
	}

	creds := auth.Credentials{
		Username: body.Username,
		Password: body.Password,
		Timezone: body.Timezone,
		Email:    emailValue(body.Email),
	}
	if body.Language != nil {
		creds.Language = shared.Language(*body.Language)
	}

	session, err := a.auth.Register(r.Context(), creds)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	a.respondWithSession(w, r, session, http.StatusCreated)
}

// Login verifies the credentials and starts a session.
func (a *API) Login(w http.ResponseWriter, r *http.Request) {
	var body gen.LoginJSONRequestBody
	if !decode(w, r, &body) {
		return
	}

	timezone := ""
	if body.Timezone != nil {
		timezone = *body.Timezone
	}

	session, err := a.auth.Login(r.Context(), body.Username, body.Password, timezone)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	a.respondWithSession(w, r, session, http.StatusOK)
}

// Refresh rotates the token pair.
func (a *API) Refresh(w http.ResponseWriter, r *http.Request) {
	presented := CookieValue(r, RefreshCookie)

	userID, err := auth.AccountFromRefreshToken(presented)
	if err != nil {
		a.cookies.Clear(w)
		WriteError(w, r, err)
		return
	}

	session, err := a.auth.Refresh(r.Context(), userID, presented)
	if err != nil {
		a.cookies.Clear(w)
		WriteError(w, r, err)
		return
	}

	if err := a.cookies.Issue(w, session.AccessToken, session.RefreshToken, session.AccessExpiry, a.refreshTTL); err != nil {
		WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Logout ends the session on this device.
func (a *API) Logout(w http.ResponseWriter, r *http.Request) {
	presented := CookieValue(r, RefreshCookie)

	// Revoking depends on the refresh cookie alone: relying on the access
	// cookie would mean a logout after fifteen idle minutes clears the
	// browser and leaves the session alive in the store.
	if userID, err := auth.AccountFromRefreshToken(presented); err == nil {
		if err := a.auth.Logout(r.Context(), userID, presented); err != nil {
			WriteError(w, r, err)
			return
		}
	}
	// The same answer whether or not there was a session: otherwise the
	// endpoint reports whether a stolen cookie is still live.
	a.cookies.Clear(w)
	w.WriteHeader(http.StatusNoContent)
}

// LogoutAll revokes every refresh token of the account.
func (a *API) LogoutAll(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserFrom(r.Context())
	if !ok {
		WriteError(w, r, shared.NewError(shared.CodeSessionExpired, nil))
		return
	}

	if err := a.auth.LogoutAll(r.Context(), userID); err != nil {
		WriteError(w, r, err)
		return
	}
	a.cookies.Clear(w)
	w.WriteHeader(http.StatusNoContent)
}

// GetMe answers with the account the client boots from.
func (a *API) GetMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserFrom(r.Context())
	if !ok {
		WriteError(w, r, shared.NewError(shared.CodeSessionExpired, nil))
		return
	}

	account, err := a.auth.Account(r.Context(), userID)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, renderUser(account))
}

// UpdateSettings changes the preferences that belong to the account.
func (a *API) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserFrom(r.Context())
	if !ok {
		WriteError(w, r, shared.NewError(shared.CodeSessionExpired, nil))
		return
	}

	var body gen.UpdateSettingsJSONRequestBody
	if !decode(w, r, &body) {
		return
	}

	change := auth.SettingsChange{Timezone: body.Timezone, ShowInLeaderboard: body.ShowInLeaderboard}
	if body.Language != nil {
		language := shared.Language(*body.Language)
		change.Language = &language
	}

	account, err := a.auth.UpdateSettings(r.Context(), userID, change)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, renderUser(account))
}

func (a *API) respondWithSession(w http.ResponseWriter, r *http.Request, session *auth.Session, status int) {
	if err := a.cookies.Issue(w, session.AccessToken, session.RefreshToken, session.AccessExpiry, a.refreshTTL); err != nil {
		WriteError(w, r, err)
		return
	}
	WriteJSON(w, r, status, renderUser(session.User))
}

// maxBodyBytes bounds an unauthenticated request. Without it a single caller
// can feed an arbitrarily long password into argon2, which is 64 MiB of work
// per attempt by design.
const maxBodyBytes = 16 << 10

func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	// An HTML form cannot set this content type, which is what stops a
	// cross-site page from posting a body that happens to parse as JSON and
	// signing the victim's browser into an account the attacker controls.
	if contentType := r.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		WriteError(w, r, shared.NewError(shared.CodeValidationFailed,
			map[string]any{"field": "Content-Type", "value": contentType}))
		return false
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	decoder := json.NewDecoder(r.Body)
	// An unknown field is a client that thinks it is talking to a different
	// API; saying so is friendlier than ignoring half the request.
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		WriteError(w, r, shared.WrapError(err, shared.CodeValidationFailed,
			map[string]any{"reason": "malformed body"}))
		return false
	}
	return true
}

func emailValue(email *openapi_types.Email) *string {
	if email == nil {
		return nil
	}
	value := string(*email)
	return &value
}

func renderUser(account *user.User) gen.User {
	rendered := gen.User{
		Id:                uuid.UUID(account.ID),
		Username:          account.Username,
		Language:          gen.Language(account.Settings.Language),
		Timezone:          account.Settings.Timezone,
		ShowInLeaderboard: account.Settings.ShowInLeaderboard,
		Plan:              gen.Plan(shared.PlanFree),
		PlanExpired:       false,
		CreatedAt:         account.CreatedAt,
		Stats: gen.UserStats{
			LifetimeXp:           account.Stats.LifetimeXP,
			Level:                account.Stats.Level,
			CurrentStreak:        account.Stats.CurrentStreak,
			LongestStreak:        account.Stats.LongestStreak,
			TasksCreated:         account.Stats.TasksCreated,
			TasksCompleted:       account.Stats.TasksCompleted,
			SubtasksCompleted:    account.Stats.SubtasksCompleted,
			LinksCreated:         account.Stats.LinksCreated,
			AchievementsUnlocked: account.Stats.AchievementsUnlocked,
		},
	}
	if account.Email != nil {
		email := openapi_types.Email(*account.Email)
		rendered.Email = &email
	}
	return rendered
}

var _ gen.ServerInterface = (*API)(nil)
