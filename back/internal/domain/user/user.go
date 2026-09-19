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

	Settings Settings
	Stats    Stats
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
