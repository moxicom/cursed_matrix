package user_test

import (
	"testing"
	"time"

	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/user"
)

func TestTrial(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		period    time.Duration
		wantUntil *time.Time
	}{
		{
			name:      "a fortnight",
			period:    14 * 24 * time.Hour,
			wantUntil: new(now.Add(14 * 24 * time.Hour)),
		},
		{
			// Accounts made before trials existed have no expiry, and turning
			// the trial off must reproduce that rather than expire everyone.
			name:   "no trial means no deadline",
			period: 0,
		},
		{name: "a negative period is no trial either", period: -time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subscription := user.Trial(tt.period, now)

			if subscription.Plan != shared.PlanFree {
				t.Errorf("plan = %q, want FREE", subscription.Plan)
			}
			switch {
			case tt.wantUntil == nil && subscription.ExpiresAt != nil:
				t.Errorf("expiresAt = %v, want none", *subscription.ExpiresAt)
			case tt.wantUntil != nil && subscription.ExpiresAt == nil:
				t.Error("expiresAt is missing")
			case tt.wantUntil != nil && !subscription.ExpiresAt.Equal(*tt.wantUntil):
				t.Errorf("expiresAt = %v, want %v", *subscription.ExpiresAt, *tt.wantUntil)
			}
			if subscription.Expired(now) {
				t.Error("a subscription expired the moment it started")
			}
		})
	}
}

func TestExtend(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	month := 30 * 24 * time.Hour

	tests := []struct {
		name    string
		current user.Subscription
		want    time.Time
	}{
		{
			name:    "from nothing, a month from now",
			current: user.Subscription{Plan: shared.PlanFree},
			want:    now.Add(month),
		},
		{
			// The remainder is paid for; throwing it away would charge twice
			// for the same days.
			name: "buying early adds to what is left",
			current: user.Subscription{
				Plan:      shared.PlanPro,
				ExpiresAt: new(now.Add(10 * 24 * time.Hour)),
			},
			want: now.Add(10*24*time.Hour + month),
		},
		{
			name: "a lapsed one starts again from now",
			current: user.Subscription{
				Plan:      shared.PlanPro,
				ExpiresAt: new(now.Add(-24 * time.Hour)),
			},
			want: now.Add(month),
		},
		{
			// A trial's remainder does not carry into a paid plan: they are
			// different things, and the free one had its own limits.
			name: "an unexpired trial does not extend a purchase",
			current: user.Subscription{
				Plan:      shared.PlanFree,
				ExpiresAt: new(now.Add(5 * 24 * time.Hour)),
			},
			want: now.Add(month),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.current.Extend(shared.PlanPro, month, now)

			if got.Plan != shared.PlanPro {
				t.Errorf("plan = %q, want PRO", got.Plan)
			}
			if got.ExpiresAt == nil || !got.ExpiresAt.Equal(tt.want) {
				t.Errorf("expiresAt = %v, want %v", got.ExpiresAt, tt.want)
			}
			if got.Expired(now) {
				t.Error("the new subscription is already expired")
			}
		})
	}
}

//go:fix inline
