package httphandler

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/moxicom/cursed_matrix/back/internal/app/port"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
)

// An account ceiling is also a way to lock a real user out: anyone who knows a
// username can spend its budget from a handful of addresses. That is accepted
// here — credential stuffing across many addresses is the attack this counter
// exists for, and the alternative is not counting it at all — but it is the
// main remaining way to hurt a user rather than guess their password, so a
// challenge instead of a refusal is the upgrade path if it starts happening.
//
// RateLimits are the two ceilings that protect the credential endpoints. The
// per-address one slows a scan across many accounts; the per-account one slows
// guessing at a single account from many addresses, which the first cannot see.
type RateLimits struct {
	AddressAttempts int
	AddressWindow   time.Duration
	AccountAttempts int
	AccountWindow   time.Duration
}

// LimitByAddress refuses a caller that has made too many attempts from one
// address.
func LimitByAddress(limiter port.RateLimiter, limits RateLimits) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := "address:" + ClientAddress(r)

			allowed, retryAfter, err := limiter.Allow(r.Context(), key, limits.AddressAttempts, limits.AddressWindow)
			if err != nil {
				WriteError(w, r, err)
				return
			}
			if !allowed {
				writeRateLimited(w, r, retryAfter)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// AllowAccount counts an attempt against one account name. It is called after
// the body is decoded, which is the only point at which the name is known.
func AllowAccount(ctx context.Context, limiter port.RateLimiter, limits RateLimits, username string) (bool, time.Duration, error) {
	// Folded exactly the way the lookup folds it: the username column is
	// CITEXT and the service trims, so "Victim", " victim " and "VICTIM" are
	// one account. A key that did not fold would hand each spelling its own
	// budget, which is the same as having no account ceiling at all.
	key := strings.ToLower(strings.TrimSpace(username))
	return limiter.Allow(ctx, "account:"+key, limits.AccountAttempts, limits.AccountWindow)
}

func writeRateLimited(w http.ResponseWriter, r *http.Request, retryAfter time.Duration) {
	w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
	WriteError(w, r, shared.NewError(shared.CodeRateLimited,
		map[string]any{"retryAfterSeconds": int(retryAfter.Seconds())}))
}

// ClientAddress is the caller's address as the reverse proxy reports it.
//
// X-Real-IP is set by nginx from its own connection, overwriting whatever the
// client sent, and the backend is only reachable through nginx — it is on the
// edge network and publishes no port. A deployment that exposes it directly
// must stop trusting this header.
func ClientAddress(r *http.Request) string {
	if forwarded := r.Header.Get("X-Real-IP"); forwarded != "" {
		return forwarded
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
