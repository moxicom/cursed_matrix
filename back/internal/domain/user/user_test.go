package user_test

import (
	"testing"
	"time"

	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/user"
)

func TestSettingsLocation(t *testing.T) {
	tests := []struct {
		name     string
		timezone string
		want     string
	}{
		{name: "a known name resolves", timezone: "Asia/Tokyo", want: "Asia/Tokyo"},
		{name: "empty falls back", timezone: "", want: "UTC"},
		{name: "a name no database knows falls back", timezone: "Mars/Olympus", want: "UTC"},
		{name: "a path traversal attempt falls back", timezone: "../../etc/passwd", want: "UTC"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := user.Settings{Timezone: tt.timezone}
			if got := settings.Location().String(); got != tt.want {
				t.Errorf("Location() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSettingsLocalDate(t *testing.T) {
	// 22:30 UTC: already tomorrow east of the line, still yesterday west of it.
	instant := time.Date(2026, 9, 19, 22, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		timezone string
		wantDay  int
		wantHour int
	}{
		{name: "UTC keeps the day", timezone: "UTC", wantDay: 19},
		{name: "east of UTC is already tomorrow", timezone: "Asia/Tokyo", wantDay: 20},
		{name: "west of UTC is still today", timezone: "America/New_York", wantDay: 19},
		{name: "the far side of the line", timezone: "Pacific/Kiritimati", wantDay: 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := user.Settings{Timezone: tt.timezone}
			got := settings.LocalDate(instant)

			if got.Day() != tt.wantDay {
				t.Errorf("day = %d, want %d (%s)", got.Day(), tt.wantDay, got)
			}
			// A calendar day starts at midnight in its own zone, not at
			// midnight UTC shifted sideways.
			if got.Hour() != 0 || got.Minute() != 0 || got.Second() != 0 {
				t.Errorf("LocalDate = %s, want the start of the day", got)
			}
			if got.Location().String() != tt.timezone {
				t.Errorf("location = %s, want %s", got.Location(), tt.timezone)
			}
		})
	}
}

func TestSubscriptionExpired(t *testing.T) {
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	tests := []struct {
		name      string
		plan      shared.Plan
		expiresAt *time.Time
		want      bool
	}{
		{name: "a paid plan with time left is live", plan: shared.PlanPro, expiresAt: &future},
		{name: "a paid plan past its date is expired", plan: shared.PlanPro, expiresAt: &past, want: true},
		{name: "no expiry never expires", plan: shared.PlanPro},
		{name: "free with no expiry never expires", plan: shared.PlanFree},
		{
			name:      "self-hosted ignores the date entirely",
			plan:      shared.PlanSelfHosted,
			expiresAt: &past,
		},
		{
			name:      "expiry exactly now has not passed yet",
			plan:      shared.PlanPro,
			expiresAt: &now,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subscription := user.Subscription{Plan: tt.plan, ExpiresAt: tt.expiresAt}
			if got := subscription.Expired(now); got != tt.want {
				t.Errorf("Expired() = %v, want %v", got, tt.want)
			}
		})
	}
}
