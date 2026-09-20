//go:build integration

package httphandler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func randomSuffix() string { return uuid.NewString()[:8] }

// database opens a connection of the test's own, to set up states the API
// deliberately offers no way to reach — a lapsed plan is billing's to write,
// and billing does not exist yet.
func database(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func registerAs(t *testing.T, handler http.Handler, username string) *client {
	t.Helper()
	c := &client{handler: handler, cookies: map[string]string{}}
	if response := c.do(t, http.MethodPost, "/auth/register", registerBody(username)); response.Code != http.StatusCreated {
		t.Fatalf("register: %d %s", response.Code, response.Body)
	}
	return c
}

func signInAs(t *testing.T, handler http.Handler, username string) *client {
	t.Helper()
	c := &client{handler: handler, cookies: map[string]string{}}
	body := `{"username":"` + username + `","password":"correct horse battery"}`
	if response := c.do(t, http.MethodPost, "/auth/login", body); response.Code != http.StatusOK {
		t.Fatalf("login: %d %s", response.Code, response.Body)
	}
	return c
}

// lapsePlan puts the account on a paid plan that ran out yesterday.
func lapsePlan(t *testing.T, pool *pgxpool.Pool, username string) {
	t.Helper()
	const update = `UPDATE users SET plan = 'PRO', plan_expires_at = now() - interval '1 day'
		WHERE username = $1`
	if _, err := pool.Exec(context.Background(), update, username); err != nil {
		t.Fatalf("lapse plan: %v", err)
	}
}

func TestSubscriptionGate(t *testing.T) {
	pool := database(t)
	handler := server(t)
	username := "sub_" + randomSuffix()
	c := registerAs(t, handler, username)

	t.Run("the free plan reaches the application", func(t *testing.T) {
		if response := c.do(t, http.MethodGet, "/tasks", ""); response.Code != http.StatusOK {
			t.Fatalf("tasks = %d: %s", response.Code, response.Body)
		}
	})

	lapsePlan(t, pool, username)
	// The token still says what it said; signing in again is what picks the
	// new plan up, which is the same fifteen-minute window everything else
	// about a token has.
	fresh := signInAs(t, handler, username)

	t.Run("a lapsed plan is refused with the payment code", func(t *testing.T) {
		for _, path := range []string{"/tasks", "/graph", "/achievements", "/activity/stats"} {
			response := fresh.do(t, http.MethodGet, path, "")
			if response.Code != http.StatusPaymentRequired {
				t.Errorf("%s = %d, want 402: %s", path, response.Code, response.Body)
				continue
			}
			if code := errorCodeOf(t, response.Body.Bytes()); code != "SUBSCRIPTION_REQUIRED" {
				t.Errorf("%s code = %q", path, code)
			}
		}
	})

	t.Run("the account itself stays reachable", func(t *testing.T) {
		// Someone whose plan ran out still has to see their account, change
		// their settings and sign out.
		if response := fresh.do(t, http.MethodGet, "/me", ""); response.Code != http.StatusOK {
			t.Errorf("me = %d, want 200: %s", response.Code, response.Body)
		}
		if response := fresh.do(t, http.MethodPatch, "/me", `{"language":"RU"}`); response.Code != http.StatusOK {
			t.Errorf("settings = %d, want 200: %s", response.Code, response.Body)
		}
	})

	t.Run("and the account says so", func(t *testing.T) {
		response := fresh.do(t, http.MethodGet, "/me", "")
		var me struct {
			Plan        string `json:"plan"`
			PlanExpired bool   `json:"planExpired"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &me); err != nil {
			t.Fatalf("me: %v", err)
		}
		if me.Plan != "PRO" || !me.PlanExpired {
			t.Errorf("plan = %q, expired = %v", me.Plan, me.PlanExpired)
		}
	})

	t.Run("nothing of theirs was taken away", func(t *testing.T) {
		const count = `SELECT count(*) FROM users WHERE username = $1 AND deleted_at IS NULL`
		var alive int
		if err := pool.QueryRow(context.Background(), count, username).Scan(&alive); err != nil {
			t.Fatalf("count: %v", err)
		}
		if alive != 1 {
			t.Errorf("the account is gone")
		}
	})
}
