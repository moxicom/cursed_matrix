package httphandler

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/app/port"
	"github.com/moxicom/cursed_matrix/back/internal/app/profile"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
)

type contextKey string

const userKey contextKey = "userID"

// UserFrom returns the account the request is authenticated as.
func UserFrom(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userKey).(uuid.UUID)
	return id, ok
}

// Authenticate verifies the access cookie and puts the account on the context.
// A request without a valid token is refused here, so no handler has to
// remember to check.
func Authenticate(tokens port.TokenIssuer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := CookieValue(r, AccessCookie)
			if raw == "" {
				WriteError(w, r, shared.NewError(shared.CodeSessionExpired, nil))
				return
			}

			userID, _, err := tokens.Verify(raw)
			if err != nil {
				WriteError(w, r, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, userID)))
		})
	}
}

// RequireCSRF compares the header against the cookie on every unsafe method.
// A browser attaches cookies whether or not the page meant to send them; only
// the page itself can read the value and echo it back.
func RequireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}

		cookie := CookieValue(r, CSRFCookie)
		header := r.Header.Get(CSRFHeader)

		if cookie == "" || subtle.ConstantTimeCompare([]byte(cookie), []byte(header)) != 1 {
			WriteError(w, r, shared.NewError(shared.CodeCSRFTokenInvalid, nil))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// TrackStreak counts the visit towards the user's streak.
//
// It never fails a request: the streak is a reward, and a user who cannot
// reach their board because a counter would not move has lost something real
// to protect something decorative.
func TrackStreak(profiles *profile.Service, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if userID, ok := UserFrom(r.Context()); ok {
				if _, err := profiles.TouchDay(r.Context(), userID); err != nil {
					log.WarnContext(r.Context(), "streak not counted", "err", err, "userId", userID)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
