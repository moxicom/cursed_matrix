// Package user holds the account, its settings and progression aggregate.
package user

import (
	"time"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
)

type User struct {
	ID       uuid.UUID
	Username string
	// Email is optional: the account is identified by its username. An address
	// arrives when the user adds one, or from an identity provider.
	Email        *string
	PasswordHash string
	AvatarURL    *string
	CreatedAt    time.Time
	LastLoginAt  *time.Time
	DeletedAt    *time.Time

	Settings     Settings
	Stats        Stats
	Subscription Subscription
}

// Settings are the account preferences, as opposed to per-device ones.
type Settings struct {
	Language             shared.Language
	Timezone             string
	ShowInLeaderboard    bool
	NotificationsEnabled bool
}

type Subscription struct {
	Plan      shared.Plan
	ExpiresAt *time.Time
}

func (s *Subscription) Expired(now time.Time) bool {
	if s.Plan == shared.PlanSelfHosted {
		return false
	}
	return s.ExpiresAt != nil && now.After(*s.ExpiresAt)
}

// Stats is the progression aggregate, recomputable from the ledger and events.
type Stats struct {
	LifetimeXP           int64
	Level                int32
	CurrentStreak        int32
	LongestStreak        int32
	LastStreakDate       *time.Time
	TasksCreated         int32
	TasksCompleted       int32
	SubtasksCompleted    int32
	LinksCreated         int32
	AchievementsUnlocked int32
	UpdatedAt            time.Time
}

// Location resolves the stored timezone, falling back to UTC: an unknown name
// must not stop the account from being read.
func (s *Settings) Location() *time.Location {
	loc, err := time.LoadLocation(s.Timezone)
	if err != nil {
		return time.UTC
	}
	return loc
}

// LocalDate returns the calendar day of instant in the user's timezone.
func (s *Settings) LocalDate(instant time.Time) time.Time {
	local := instant.In(s.Location())
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location())
}

// Trial starts an account's free run.
//
// Zero means no trial and no expiry, which is how every account created before
// trials existed behaves — they are not retroactively given a deadline.
func Trial(period time.Duration, now time.Time) Subscription {
	if period <= 0 {
		return Subscription{Plan: shared.PlanFree}
	}
	until := now.Add(period)
	return Subscription{Plan: shared.PlanFree, ExpiresAt: &until}
}

// Granted is a plan handed out without payment: it runs for one period from
// now, however many times it is asked for.
//
// Not Extend, which adds to what is left. That is right for a purchase — the
// remainder was paid for — but a grant nobody paid for must not accumulate,
// or asking repeatedly would buy years for free.
func Granted(plan shared.Plan, period time.Duration, now time.Time) Subscription {
	until := now.Add(period)
	return Subscription{Plan: plan, ExpiresAt: &until}
}

// Extend grants a plan for another period.
//
// Time already paid for is kept: extending runs from whichever is later, the
// current expiry or now, so buying again before the old one lapses adds to it
// instead of throwing the remainder away.
func (s *Subscription) Extend(plan shared.Plan, period time.Duration, now time.Time) Subscription {
	from := now
	if s.ExpiresAt != nil && s.ExpiresAt.After(now) && s.Plan == plan {
		from = *s.ExpiresAt
	}
	until := from.Add(period)
	return Subscription{Plan: plan, ExpiresAt: &until}
}
