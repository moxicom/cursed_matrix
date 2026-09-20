package httphandler

import (
	"net/http"
	"time"

	gen "github.com/moxicom/cursed_matrix/back/internal/adapter/http-handler/gen"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
)

// ExportAccount answers with everything the account holds.
func (a *API) ExportAccount(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserFrom(r.Context())
	if !ok {
		WriteError(w, r, shared.NewError(shared.CodeSessionExpired, nil))
		return
	}

	export, err := a.profile.ExportAll(r.Context(), userID)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	rendered := gen.AccountExport{
		ExportedAt: export.ExportedAt,
		Account:    a.renderUser(export.Account),
		Tasks:      make([]gen.Task, 0, len(export.Tasks)),
		Links:      make([]gen.TaskLink, 0, len(export.Links)),
		Tags:       make([]gen.Tag, 0, len(export.Tags)),
		Achievements: make([]struct {
			Code       string    `json:"code"`
			UnlockedAt time.Time `json:"unlockedAt"`
		}, 0, len(export.Achievements)),
	}
	for i := range export.Tasks {
		rendered.Tasks = append(rendered.Tasks, renderTask(&export.Tasks[i]))
	}
	for i := range export.Links {
		rendered.Links = append(rendered.Links, renderLink(&export.Links[i]))
	}
	for i := range export.Tags {
		rendered.Tags = append(rendered.Tags, renderTag(&export.Tags[i]))
	}
	for _, unlocked := range export.Achievements {
		rendered.Achievements = append(rendered.Achievements, struct {
			Code       string    `json:"code"`
			UnlockedAt time.Time `json:"unlockedAt"`
		}{Code: unlocked.Code, UnlockedAt: unlocked.UnlockedAt})
	}

	// The client asked for a file to keep, not a page to read.
	w.Header().Set("Content-Disposition", `attachment; filename="cursed-matrix-export.json"`)
	WriteJSON(w, r, http.StatusOK, rendered)
}

// DeleteAccount closes the account after the user proves it is theirs.
func (a *API) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserFrom(r.Context())
	if !ok {
		WriteError(w, r, shared.NewError(shared.CodeSessionExpired, nil))
		return
	}

	var body gen.DeleteAccountJSONRequestBody
	if !decode(w, r, &body) {
		return
	}

	if err := a.auth.DeleteAccount(r.Context(), userID, body.Password); err != nil {
		WriteError(w, r, err)
		return
	}

	// The session goes with the account: leaving the cookies in place would
	// have the client asking for a board that no longer answers.
	a.cookies.Clear(w)
	w.WriteHeader(http.StatusNoContent)
}
