//go:build integration

package httphandler_test

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	httphandler "github.com/moxicom/cursed_matrix/back/internal/adapter/http-handler"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
)

func TestRateLimits(t *testing.T) {
	tests := []struct {
		name string
		// attempts are made against one account from one address; which ceiling
		// bites first is the point of each case.
		limits        httphandler.RateLimits
		attempts      int
		wantRefusedAt int
	}{
		{
			name: "the account ceiling stops guessing at one name",
			limits: httphandler.RateLimits{
				AddressAttempts: 100, AddressWindow: time.Minute,
				AccountAttempts: 3, AccountWindow: time.Minute,
			},
			attempts: 5, wantRefusedAt: 4,
		},
		{
			name: "the address ceiling stops a scan",
			limits: httphandler.RateLimits{
				AddressAttempts: 2, AddressWindow: time.Minute,
				AccountAttempts: 100, AccountWindow: time.Minute,
			},
			attempts: 4, wantRefusedAt: 3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler := serverWithLimits(t, tc.limits)
			username := "t_" + uuid.NewString()[:8]
			// Its own address, so one case cannot spend another's budget. The
			// counters live in Redis and outlive the subtest — and the test
			// run — so the address has to be unlikely to repeat within the
			// window, not merely unique within this run.
			id := uuid.New()
			address := "10." + strconv.Itoa(int(id[0])) + "." +
				strconv.Itoa(int(id[1])) + "." + strconv.Itoa(int(id[2]))

			for attempt := 1; attempt <= tc.attempts; attempt++ {
				recorder := post(t, handler, address, "/auth/login",
					`{"username":"`+username+`","password":"wrong password here"}`)

				if attempt < tc.wantRefusedAt {
					if recorder.Code != http.StatusUnauthorized {
						t.Fatalf("attempt %d = %d, want 401 before the ceiling", attempt, recorder.Code)
					}
					continue
				}

				if recorder.Code != http.StatusTooManyRequests {
					t.Fatalf("attempt %d = %d, want 429 at and after the ceiling", attempt, recorder.Code)
				}
				if retry := recorder.Header().Get("Retry-After"); retry == "" {
					t.Error("no Retry-After header")
				} else if seconds, err := strconv.Atoi(retry); err != nil || seconds <= 0 {
					t.Errorf("Retry-After = %q, want a positive number of seconds", retry)
				}
				if code := errorCode(t, recorder); code != shared.CodeRateLimited {
					t.Errorf("code = %q, want RATE_LIMITED", code)
				}
			}
		})
	}
}

// The username column is CITEXT and the service trims, so these spellings are
// one account. If the counter did not fold the same way, each spelling would
// arrive with its own budget and the account ceiling would be decorative.
func TestAccountCeilingFoldsTheUsername(t *testing.T) {
	limits := httphandler.RateLimits{
		AddressAttempts: 1000, AddressWindow: time.Minute,
		AccountAttempts: 3, AccountWindow: time.Minute,
	}
	handler := serverWithLimits(t, limits)

	base := "t_" + uuid.NewString()[:8]
	spellings := []string{base, strings.ToUpper(base), " " + base + " ", strings.ToTitle(base)}

	for attempt, username := range spellings {
		address := "203.0.113." + strconv.Itoa(attempt+1)
		recorder := post(t, handler, address, "/auth/login",
			`{"username":"`+username+`","password":"wrong password here"}`)

		// Three spellings fit the ceiling; the fourth must find it spent, even
		// though every one came from a different address and a different
		// spelling of the name.
		if attempt < 3 {
			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("%q = %d, want 401", username, recorder.Code)
			}
			continue
		}
		if recorder.Code != http.StatusTooManyRequests {
			t.Fatalf("%q = %d, want 429 — the spelling opened a fresh budget", username, recorder.Code)
		}
	}
}
