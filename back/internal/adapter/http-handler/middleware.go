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
	"github.com/moxicom/cursed_matrix/back/internal/domain/user"
)

type contextKey string

const (
	userKey         contextKey = "userID"
	subscriptionKey contextKey = "subscription"
)

// SubscriptionFrom returns what the token says the account is entitled to.
func SubscriptionFrom(ctx context.Context) (user.Subscription, bool) {
	subscription, ok := ctx.Value(subscriptionKey).(user.Subscription)
	return subscription, ok
}

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

			userID, subscription, err := tokens.Verify(raw)
			if err != nil {
				WriteError(w, r, err)
				return
			}

			ctx := context.WithValue(r.Context(), userKey, userID)
			ctx = context.WithValue(ctx, subscriptionKey, subscription)
			next.ServeHTTP(w, r.WithContext(ctx))
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

// RequireSubscription refuses the application to an account whose plan has
// lapsed.
//
// The free plan does not lapse — it has limits instead — so this only stops
// someone whose paid time ran out. Nothing they own is touched: the answer
// says to pay, and everything is waiting when they do.
func RequireSubscription(clock shared.Clock) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			subscription, ok := SubscriptionFrom(r.Context())
			if ok && subscription.Expired(clock.Now().UTC()) {
				WriteError(w, r, shared.NewError(shared.CodeSubscriptionRequired,
					map[string]any{"plan": string(subscription.Plan)}))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
