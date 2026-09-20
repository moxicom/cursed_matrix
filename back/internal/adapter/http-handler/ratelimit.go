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

// RateLimits are the five ceilings the application enforces for itself. Each
// one answers an attack the others cannot see: an address scanning many
// accounts, many addresses guessing one account, an address manufacturing
// accounts, and a session repeating an expensive read or an endless write.
//
// An account ceiling is also a way to lock a real user out: anyone who knows a
// username can spend its budget from a handful of addresses. That is accepted
// here — credential stuffing across many addresses is the attack this counter
// exists for, and the alternative is not counting it at all — but it is the
// main remaining way to hurt a user rather than guess their password, so a
// challenge instead of a refusal is the upgrade path if it starts happening.
type RateLimits struct {
	AddressAttempts  int
	AddressWindow    time.Duration
	AccountAttempts  int
	AccountWindow    time.Duration
	RegisterAttempts int
	RegisterWindow   time.Duration
	ReadAttempts     int
	ReadWindow       time.Duration
	WriteAttempts    int
	WriteWindow      time.Duration
}

// LimitByAddress refuses a caller that has made too many credential attempts
// from one address.
func LimitByAddress(limiter port.RateLimiter, limits RateLimits) func(http.Handler) http.Handler {
	return limitByAddress(limiter, "address:", limits.AddressAttempts, limits.AddressWindow)
}

// LimitPublicByAddress bounds a route that has no session to bound it by.
//
// Its own prefix, and the read budget rather than the credential one. Sharing
// either with the credential counter would mean that reading the pricing page
// — a page anyone may read, repeatedly, without signing in — spends the budget
// the reader needs to sign in afterwards.
func LimitPublicByAddress(limiter port.RateLimiter, limits RateLimits) func(http.Handler) http.Handler {
	return limitByAddress(limiter, "public:", limits.ReadAttempts, limits.ReadWindow)
}

func limitByAddress(limiter port.RateLimiter, prefix string, attempts int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if attempts <= 0 {
				next.ServeHTTP(w, r)
				return
			}

			allowed, retryAfter, err := limiter.Allow(r.Context(), prefix+ClientAddress(r), attempts, window)
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

// LimitRegistrations bounds how many accounts one address may make.
//
// The credential ceiling above counts registering and signing in together, and
// it has to be loose enough for someone who mistypes a password: twenty
// attempts in five minutes is twenty accounts in five minutes, which is what a
// script wants. This counter is separate and strict, because nobody needs a
// second account this hour and a script needs a thousand.
//
// It counts the attempt, not the account: a refused registration costs a slot
// too, or a caller could probe for free names without limit.
func LimitRegistrations(limiter port.RateLimiter, limits RateLimits) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limits.RegisterAttempts <= 0 {
				next.ServeHTTP(w, r)
				return
			}

			key := "register:" + ClientAddress(r)
			allowed, retryAfter, err := limiter.Allow(r.Context(), key, limits.RegisterAttempts, limits.RegisterWindow)
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

// LimitWrites bounds how often one signed-in account may change something.
//
// The plan quotas bound what an account may hold — thirty-five tasks, twenty-
// five links — and say nothing about how fast it may reach that number and
// leave it again. Creating and deleting in a loop never exceeds a quota and
// writes a transaction, an activity event and a ledger entry every time. This
// is the ceiling on that, and like the read one it sits far above a person
// working: it stops a runaway client, not a fast typist.
//
// Safe methods pass untouched, so a client that has hit the write ceiling can
// still read what it has.
func LimitWrites(limiter port.RateLimiter, limits RateLimits) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := UserFrom(r.Context())
			if !ok || limits.WriteAttempts <= 0 || isSafeMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}

			allowed, retryAfter, err := limiter.Allow(r.Context(),
				"write:"+userID.String(), limits.WriteAttempts, limits.WriteWindow)
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

func isSafeMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

// LimitReads bounds how often one signed-in account may ask for its own data.
//
// The board is unbounded work per call — a filter over every task the account
// ever created — and a session can repeat it as fast as the network allows, so
// a loop in a client, or a bored user with a console, is a load problem for
// every other tenant. The ceiling is set well above what a person browsing can
// produce; it exists to stop a runaway caller, not to pace the UI.
func LimitReads(limiter port.RateLimiter, limits RateLimits) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := UserFrom(r.Context())
			if !ok || limits.ReadAttempts <= 0 {
				next.ServeHTTP(w, r)
				return
			}

			allowed, retryAfter, err := limiter.Allow(r.Context(),
				"read:"+userID.String(), limits.ReadAttempts, limits.ReadWindow)
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
