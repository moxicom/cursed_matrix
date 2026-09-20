package billing_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/app/billing"
	"github.com/moxicom/cursed_matrix/back/internal/app/cache"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/user"
)

type stubUsers struct {
	account   *user.User
	written   []user.Subscription
	readErr   error
	writeErr  error
	readCalls int
}

func (s *stubUsers) ByID(_ context.Context, id uuid.UUID) (*user.User, error) {
	s.readCalls++
	if s.readErr != nil {
		return nil, s.readErr
	}
	if s.account == nil {
		return &user.User{ID: id}, nil
	}
	copied := *s.account
	return &copied, nil
}

func (s *stubUsers) SetSubscription(_ context.Context, _ uuid.UUID, subscription user.Subscription) error {
	if s.writeErr != nil {
		return s.writeErr
	}
	s.written = append(s.written, subscription)
	if s.account != nil {
		s.account.Subscription = subscription
	}
	return nil
}

func (s *stubUsers) ByUsername(context.Context, string) (*user.User, error) { return nil, nil }
func (s *stubUsers) Create(context.Context, *user.User) error               { return nil }
func (s *stubUsers) UpdateSettings(context.Context, uuid.UUID, user.Settings) error {
	return nil
}
func (s *stubUsers) TouchLogin(context.Context, uuid.UUID, time.Time) error { return nil }
func (s *stubUsers) ApplyStats(context.Context, uuid.UUID, user.StatsDelta) (user.Stats, error) {
	return user.Stats{}, nil
}
func (s *stubUsers) SetLevel(context.Context, uuid.UUID, int32) error { return nil }
func (s *stubUsers) LockAccount(context.Context, uuid.UUID) error     { return nil }
func (s *stubUsers) TouchStreak(context.Context, uuid.UUID, time.Time) (user.StreakChange, error) {
	return user.StreakChange{}, nil
}
func (s *stubUsers) SoftDelete(context.Context, uuid.UUID, time.Time) error { return nil }

type stubCache struct{ retired []cache.Scope }

func (c *stubCache) GetInto(context.Context, cache.Key, any) (bool, error) { return false, nil }
func (c *stubCache) Set(context.Context, cache.Key, any, time.Duration) error {
	return nil
}

func (c *stubCache) Invalidate(_ context.Context, scope cache.Scope) error {
	c.retired = append(c.retired, scope)
	return nil
}

type fixedClock struct{ at time.Time }

func (c *fixedClock) Now() time.Time { return c.at }

const month = 30 * 24 * time.Hour

func config(enabled bool) billing.Config {
	return billing.Config{
		Enabled:       enabled,
		GrantedPeriod: month,
		Prices:        []billing.Price{{Language: shared.LanguageEN, Currency: "USD", Amount: 500}},
	}
}

// TestCheckoutDoesNotStack is the whole reason a grant is not an extension:
// nothing was paid, so asking again must not buy more time.
func TestCheckoutDoesNotStack(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	userID := uuid.New()

	tests := []struct {
		name    string
		current user.Subscription
	}{
		{name: "from a fresh trial", current: user.Trial(14*24*time.Hour, now)},
		{
			name: "from most of a month already granted",
			current: user.Subscription{
				Plan:      shared.PlanPro,
				ExpiresAt: new(now.Add(29 * 24 * time.Hour)),
			},
		},
		{
			name: "from one that lapsed",
			current: user.Subscription{
				Plan:      shared.PlanPro,
				ExpiresAt: new(now.Add(-time.Hour)),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := &stubUsers{account: &user.User{ID: userID, Subscription: tt.current}}
			service := billing.NewService(users, &stubCache{}, config(false), &fixedClock{at: now})

			// Five times over, as a client in a loop would.
			for range 5 {
				granted, err := service.Checkout(context.Background(), userID)
				if err != nil {
					t.Fatalf("Checkout: %v", err)
				}
				if granted.Plan != shared.PlanPro {
					t.Fatalf("plan = %q", granted.Plan)
				}
				if granted.ExpiresAt == nil || !granted.ExpiresAt.Equal(now.Add(month)) {
					t.Fatalf("expiresAt = %v, want a month from now however often it is asked",
						granted.ExpiresAt)
				}
			}
		})
	}
}

func TestCheckout(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		enabled   bool
		readErr   error
		writeErr  error
		wantErr   shared.ErrorCode
		wantWrite bool
	}{
		{name: "billing off grants the plan", wantWrite: true},
		{
			// Nothing will collect the money, so the instruction is refused
			// rather than accepted.
			name:    "billing on with no provider refuses",
			enabled: true,
			wantErr: shared.CodeBillingUnavailable,
		},
		{
			name:    "an account that is not there",
			readErr: shared.NewError(shared.CodeUserNotFound, nil),
			wantErr: shared.CodeUserNotFound,
		},
		{
			name:     "a failed write is a failed purchase",
			writeErr: shared.NewError(shared.CodeInternal, nil),
			wantErr:  shared.CodeInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := &stubUsers{readErr: tt.readErr, writeErr: tt.writeErr}
			cached := &stubCache{}
			service := billing.NewService(users, cached, config(tt.enabled), &fixedClock{at: now})

			_, err := service.Checkout(context.Background(), uuid.New())

			if tt.wantErr != "" {
				if shared.CodeOf(err) != tt.wantErr {
					t.Fatalf("err = %v, want %s", err, tt.wantErr)
				}
				if len(users.written) != 0 {
					t.Error("a refused purchase was written anyway")
				}
				return
			}
			if err != nil {
				t.Fatalf("Checkout: %v", err)
			}
			if len(users.written) != 1 {
				t.Fatalf("%d writes, want one", len(users.written))
			}
			// The plan rides in the access token, so what the account has
			// cached about itself has to go.
			if len(cached.retired) != 1 {
				t.Errorf("retired %d scopes, want the account's", len(cached.retired))
			}
		})
	}
}

func TestCheckoutWithoutACache(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	users := &stubUsers{}
	service := billing.NewService(users, nil, config(false), &fixedClock{at: now})

	if _, err := service.Checkout(context.Background(), uuid.New()); err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	if len(users.written) != 1 {
		t.Errorf("%d writes, want one", len(users.written))
	}
}

func TestOffers(t *testing.T) {
	service := billing.NewService(&stubUsers{}, nil, config(false), &fixedClock{})

	offers := service.Offers()
	if len(offers) != 2 {
		t.Fatalf("%d offers, want the free and the paid plan", len(offers))
	}

	byPlan := map[shared.Plan]billing.Offer{}
	for _, offer := range offers {
		byPlan[offer.Plan] = offer
	}

	free := byPlan[shared.PlanFree]
	// The page must not promise a number the server does not enforce.
	if free.ActiveTaskLimit != user.FreeActiveTasks || free.TaskLinkLimit != user.FreeTaskLinks {
		t.Errorf("free limits = %d/%d, want the quotas' own", free.ActiveTaskLimit, free.TaskLinkLimit)
	}
	if len(free.Prices) != 0 {
		t.Errorf("the free plan has a price: %+v", free.Prices)
	}

	paid := byPlan[shared.PlanPro]
	if paid.ActiveTaskLimit != user.Unlimited || paid.TaskLinkLimit != user.Unlimited {
		t.Errorf("paid limits = %d/%d, want unlimited", paid.ActiveTaskLimit, paid.TaskLinkLimit)
	}
	if len(paid.Prices) != 1 || paid.Prices[0].Currency != "USD" {
		t.Errorf("prices = %+v", paid.Prices)
	}
}

func TestEnabledIsReported(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		service := billing.NewService(&stubUsers{}, nil, config(enabled), &fixedClock{})
		if service.Enabled() != enabled {
			t.Errorf("Enabled() = %v, want %v", service.Enabled(), enabled)
		}
	}
}
