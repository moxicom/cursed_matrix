// Package user holds the account, its settings and progression aggregate.
package user

import (
	"time"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
)

type User struct {
	ID           uuid.UUID
	Username     string
	Email        string
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

// LocalDate returns the calendar day of instant in the user's timezone.
func (s *Settings) LocalDate(instant time.Time) time.Time {
	loc, err := time.LoadLocation(s.Timezone)
	if err != nil {
		loc = time.UTC
	}
	local := instant.In(loc)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
}
