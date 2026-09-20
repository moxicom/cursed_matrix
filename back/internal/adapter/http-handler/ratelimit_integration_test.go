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

// Registering is the one open endpoint that costs a row, an argon2 hash and a
// free trial. The credential ceiling above it has to be loose enough for a
// person who mistypes a password, which makes it far too loose to be the only
// thing standing between one address and a thousand accounts.
func TestRegistrationCeilingStopsAnAccountFactory(t *testing.T) {
	limits := httphandler.RateLimits{
		AddressAttempts: 1000, AddressWindow: time.Minute,
		AccountAttempts: 1000, AccountWindow: time.Minute,
		RegisterAttempts: 2, RegisterWindow: time.Minute,
	}
	handler := serverWithLimits(t, limits)
	address := randomAddress()

	tests := []struct {
		name       string
		address    string
		wantStatus int
	}{
		{name: "the first account", address: address, wantStatus: http.StatusCreated},
		{name: "the second account", address: address, wantStatus: http.StatusCreated},
		{name: "the third is over the ceiling", address: address, wantStatus: http.StatusTooManyRequests},
		// The counter is per address, so it must not have become a global one:
		// a cap that every new user in a country shares is an outage.
		{name: "another address has its own budget", address: randomAddress(), wantStatus: http.StatusCreated},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caller := &client{handler: handler, cookies: map[string]string{}, address: tt.address}
			response := caller.do(t, http.MethodPost, "/auth/register",
				registerBody("f_"+uuid.NewString()[:8]))

			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", response.Code, tt.wantStatus, response.Body)
			}
			if tt.wantStatus != http.StatusTooManyRequests {
				return
			}
			if code := errorCode(t, response); code != shared.CodeRateLimited {
				t.Errorf("code = %q, want RATE_LIMITED", code)
			}
		})
	}
}

// A refused registration has to cost a slot too. If only the successful ones
// counted, a caller could ask for taken names all day at no charge — which is
// both a free way to enumerate accounts and a free argon2 hash each time.
func TestRefusedRegistrationsStillCount(t *testing.T) {
	limits := httphandler.RateLimits{
		AddressAttempts: 1000, AddressWindow: time.Minute,
		AccountAttempts: 1000, AccountWindow: time.Minute,
		RegisterAttempts: 2, RegisterWindow: time.Minute,
	}
	handler := serverWithLimits(t, limits)
	caller := &client{handler: handler, cookies: map[string]string{}, address: randomAddress()}

	// Two attempts that the server refuses on their merits.
	for attempt := 1; attempt <= 2; attempt++ {
		response := caller.do(t, http.MethodPost, "/auth/register",
			`{"username":"x","password":"short","timezone":"UTC"}`)
		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("attempt %d = %d, want 422: %s", attempt, response.Code, response.Body)
		}
	}

	response := caller.do(t, http.MethodPost, "/auth/register",
		registerBody("f_"+uuid.NewString()[:8]))
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("a valid registration after two refused ones = %d, want 429: %s",
			response.Code, response.Body)
	}
}

// The plan quota bounds how much an account may hold. Nothing bounded how fast
// it could churn: create and delete in a loop stays under every quota for ever
// and writes a transaction, an activity event and a ledger entry each time.
func TestWriteCeilingStopsAChurningClient(t *testing.T) {
	handler := serverWithLimits(t, httphandler.RateLimits{
		AddressAttempts: 1000, AddressWindow: time.Minute,
		AccountAttempts: 1000, AccountWindow: time.Minute,
		RegisterAttempts: 1000, RegisterWindow: time.Minute,
		ReadAttempts: 1000, ReadWindow: time.Minute,
		WriteAttempts: 2, WriteWindow: time.Minute,
	})
	caller := &client{handler: handler, cookies: map[string]string{}, address: randomAddress()}

	if response := caller.do(t, http.MethodPost, "/auth/register",
		registerBody("w_"+uuid.NewString()[:8])); response.Code != http.StatusCreated {
		t.Fatalf("register: %d %s", response.Code, response.Body)
	}

	body := `{"title":"node","quadrant":"IMPORTANT_URGENT"}`
	tests := []struct {
		name       string
		wantStatus int
	}{
		{name: "the first write", wantStatus: http.StatusCreated},
		{name: "the second write", wantStatus: http.StatusCreated},
		{name: "the third is over the ceiling", wantStatus: http.StatusTooManyRequests},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := caller.do(t, http.MethodPost, "/tasks", body)
			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", response.Code, tt.wantStatus, response.Body)
			}
		})
	}

	// Reading is a different budget. A client that has spent its writes must
	// still be able to see what it has — and to find out that it has it.
	t.Run("reading survives an exhausted write budget", func(t *testing.T) {
		if response := caller.do(t, http.MethodGet, "/tasks", ""); response.Code != http.StatusOK {
			t.Fatalf("read = %d, want 200: %s", response.Code, response.Body)
		}
	})
}

// The counters live in Redis and outlive the test run, so an address has to be
// unlikely to repeat within a window rather than merely unique within this run.
func randomAddress() string {
	id := uuid.New()
	return "10." + strconv.Itoa(int(id[0])) + "." +
		strconv.Itoa(int(id[1])) + "." + strconv.Itoa(int(id[2]))
}

// Public routes are counted because there is no session to count by, and that
// counter must be its own. Sharing the credential one — the same key, or the
// same budget — means a visitor who reads the prices a few times finds they
// can no longer sign in, which is the pricing page denying service to the
// product it advertises.
func TestReadingThePricesDoesNotSpendTheLoginBudget(t *testing.T) {
	limits := httphandler.RateLimits{
		AddressAttempts: 2, AddressWindow: time.Minute,
		AccountAttempts: 1000, AccountWindow: time.Minute,
		ReadAttempts: 100, ReadWindow: time.Minute,
	}
	handler := serverWithLimits(t, limits)
	address := randomAddress()
	caller := &client{handler: handler, cookies: map[string]string{}, address: address}

	// More reads than the whole credential budget.
	for read := 1; read <= 6; read++ {
		if response := caller.do(t, http.MethodGet, "/plans", ""); response.Code != http.StatusOK {
			t.Fatalf("read %d of the prices = %d, want 200: %s", read, response.Code, response.Body)
		}
	}

	// The credential budget must be untouched: a wrong password still gets to
	// be wrong, rather than being refused as an exhausted budget.
	response := post(t, handler, address, "/auth/login",
		`{"username":"p_`+uuid.NewString()[:8]+`","password":"wrong password here"}`)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("login after reading the prices = %d, want 401: %s", response.Code, response.Body)
	}
}
