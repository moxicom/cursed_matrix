//go:build integration

package httphandler_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

type planView struct {
	Plan   string `json:"plan"`
	Prices []struct {
		Language string `json:"language"`
		Currency string `json:"currency"`
		Amount   int64  `json:"amount"`
	} `json:"prices"`
	ActiveTaskLimit *int `json:"activeTaskLimit"`
	TaskLinkLimit   *int `json:"taskLinkLimit"`
}

type subscriptionView struct {
	Plan      string  `json:"plan"`
	ExpiresAt *string `json:"expiresAt"`
	Expired   bool    `json:"expired"`
}

func TestPlansAreePublic(t *testing.T) {
	// No cookies at all: the pricing page is one of the two that does not
	// need an account.
	anonymous := &client{handler: server(t), cookies: map[string]string{}}

	response := anonymous.do(t, http.MethodGet, "/plans", "")
	if response.Code != http.StatusOK {
		t.Fatalf("plans = %d: %s", response.Code, response.Body)
	}

	var body struct {
		BillingEnabled bool       `json:"billingEnabled"`
		Plans          []planView `json:"plans"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("plans: %v", err)
	}

	if body.BillingEnabled {
		t.Error("billingEnabled is true, but no provider is wired")
	}
	if len(body.Plans) != 2 {
		t.Fatalf("%d plans, want free and paid", len(body.Plans))
	}

	byPlan := map[string]planView{}
	for _, plan := range body.Plans {
		byPlan[plan.Plan] = plan
	}

	free := byPlan["FREE"]
	if free.ActiveTaskLimit == nil || *free.ActiveTaskLimit != 35 {
		t.Errorf("free activeTaskLimit = %v, want the 35 the server enforces", free.ActiveTaskLimit)
	}
	if free.TaskLinkLimit == nil || *free.TaskLinkLimit != 25 {
		t.Errorf("free taskLinkLimit = %v, want 25", free.TaskLinkLimit)
	}

	paid := byPlan["PRO"]
	// Unlimited is the absence of a limit, not a large number: the client
	// renders no counter for null.
	if paid.ActiveTaskLimit != nil || paid.TaskLinkLimit != nil {
		t.Errorf("paid limits = %v / %v, want none", paid.ActiveTaskLimit, paid.TaskLinkLimit)
	}
	if len(paid.Prices) == 0 {
		t.Fatal("the paid plan has no price")
	}
	if paid.Prices[0].Currency != "USD" || paid.Prices[0].Amount != 500 {
		t.Errorf("price = %+v, want minor units in the market's own currency", paid.Prices[0])
	}
}

func TestCheckoutGrantsThePlanWhileBillingIsOff(t *testing.T) {
	handler := server(t)
	username := "buy_" + randomSuffix()
	c := registerAs(t, handler, username)

	t.Run("a new account is on a trial that says when it ends", func(t *testing.T) {
		response := c.do(t, http.MethodGet, "/me", "")
		var me struct {
			Plan          string  `json:"plan"`
			PlanExpiresAt *string `json:"planExpiresAt"`
			PlanExpired   bool    `json:"planExpired"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &me); err != nil {
			t.Fatalf("me: %v", err)
		}
		if me.Plan != "FREE" || me.PlanExpired {
			t.Errorf("plan = %q, expired = %v, want a live free trial", me.Plan, me.PlanExpired)
		}
		// Without the date the client can say the trial is live but not how
		// long is left, which is the one thing the screen needs to show.
		if me.PlanExpiresAt == nil {
			t.Fatal("the trial does not say when it ends")
		}
		until, err := time.Parse(time.RFC3339, *me.PlanExpiresAt)
		if err != nil {
			t.Fatalf("planExpiresAt: %v", err)
		}
		if days := time.Until(until).Hours() / 24; days < 13 || days > 15 {
			t.Errorf("trial runs %.1f days, want about fourteen", days)
		}
	})

	var granted subscriptionView
	t.Run("buying grants a month there and then", func(t *testing.T) {
		response := c.do(t, http.MethodPost, "/billing/checkout", "")
		if response.Code != http.StatusOK {
			t.Fatalf("checkout = %d: %s", response.Code, response.Body)
		}
		if err := json.Unmarshal(response.Body.Bytes(), &granted); err != nil {
			t.Fatalf("checkout: %v", err)
		}

		if granted.Plan != "PRO" || granted.Expired {
			t.Fatalf("subscription = %+v", granted)
		}
		if granted.ExpiresAt == nil {
			t.Fatal("the granted plan has no expiry")
		}

		until, err := time.Parse(time.RFC3339, *granted.ExpiresAt)
		if err != nil {
			t.Fatalf("expiresAt: %v", err)
		}
		if days := time.Until(until).Hours() / 24; days < 29 || days > 31 {
			t.Errorf("granted %.1f days, want about thirty", days)
		}
	})

	t.Run("the account reports the new plan once the token is refreshed", func(t *testing.T) {
		// The plan rides in the access token, so the old one still says FREE.
		if response := c.do(t, http.MethodPost, "/auth/refresh", ""); response.Code != http.StatusNoContent {
			t.Fatalf("refresh = %d: %s", response.Code, response.Body)
		}

		response := c.do(t, http.MethodGet, "/me", "")
		var me struct {
			Plan string `json:"plan"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &me); err != nil {
			t.Fatalf("me: %v", err)
		}
		if me.Plan != "PRO" {
			t.Errorf("plan = %q, want PRO", me.Plan)
		}
	})

	t.Run("the paid plan lifts the quotas", func(t *testing.T) {
		// Thirty-six tasks: one past what the free plan allows.
		for i := range 36 {
			body := `{"title":"past the free cap","quadrant":"NOT_IMPORTANT_NOT_URGENT"}`
			if response := c.do(t, http.MethodPost, "/tasks", body); response.Code != http.StatusCreated {
				t.Fatalf("task %d = %d: %s", i, response.Code, response.Body)
			}
		}
	})

	t.Run("buying again does not add more free time", func(t *testing.T) {
		// Nothing was paid, so asking repeatedly must not accumulate: a
		// client in a loop would otherwise grant itself years.
		first, err := time.Parse(time.RFC3339, *granted.ExpiresAt)
		if err != nil {
			t.Fatalf("expiresAt: %v", err)
		}

		for range 3 {
			response := c.do(t, http.MethodPost, "/billing/checkout", "")
			if response.Code != http.StatusOK {
				t.Fatalf("checkout = %d: %s", response.Code, response.Body)
			}

			var again subscriptionView
			if err := json.Unmarshal(response.Body.Bytes(), &again); err != nil {
				t.Fatalf("checkout: %v", err)
			}
			second, err := time.Parse(time.RFC3339, *again.ExpiresAt)
			if err != nil {
				t.Fatalf("expiresAt: %v", err)
			}
			if second.Sub(first) > time.Minute {
				t.Fatalf("expiry moved from %v to %v; a grant must not stack", first, second)
			}
		}
	})
}
