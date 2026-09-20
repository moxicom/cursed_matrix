package postgres

import (
	"time"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/task"
	"github.com/moxicom/cursed_matrix/back/internal/domain/user"
)

type taskRow struct {
	ID                   uuid.UUID
	UserID               uuid.UUID
	ParentTaskID         *uuid.UUID
	Title                string
	Description          string
	Quadrant             *string
	Position             int32
	Color                string
	DeadlineAt           *time.Time
	DeadlineHasTime      bool
	Status               string
	CreatedAt            time.Time
	UpdatedAt            time.Time
	CompletedAt          *time.Time
	XPAwarded            *int32
	QuadrantAtCompletion *string
	CompletedVia         *string
	DeletedAt            *time.Time
	Tags                 []string
}

var taskColumns = []string{
	"t.id",
	"t.user_id",
	"t.parent_task_id",
	"t.title",
	"t.description",
	"t.quadrant::text",
	"t.position",
	"t.color",
	"t.deadline_at",
	"t.deadline_has_time",
	"t.status::text",
	"t.created_at",
	"t.updated_at",
	"t.completed_at",
	"t.xp_awarded",
	"t.quadrant_at_completion::text",
	"t.completed_via::text",
	"t.deleted_at",
}

func (r *taskRow) toDomain() (task.Task, error) {
	quadrant, err := optionalQuadrant(r.Quadrant)
	if err != nil {
		return task.Task{}, err
	}
	snapshotQuadrant, err := optionalQuadrant(r.QuadrantAtCompletion)
	if err != nil {
		return task.Task{}, err
	}
	status, err := shared.ParseTaskStatus(r.Status)
	if err != nil {
		return task.Task{}, err
	}
	color, err := shared.ParseTaskColor(r.Color)
	if err != nil {
		return task.Task{}, err
	}

	var completedVia *shared.CompletionSource
	if r.CompletedVia != nil {
		source := shared.CompletionSource(*r.CompletedVia)
		if !source.Valid() {
			return task.Task{}, shared.NewError(shared.CodeValidationFailed,
				map[string]any{"field": "completedVia", "value": *r.CompletedVia})
		}
		completedVia = &source
	}

	tags := r.Tags
	if tags == nil {
		tags = []string{}
	}

	result := task.Task{
		ID:                   r.ID,
		UserID:               r.UserID,
		ParentID:             r.ParentTaskID,
		Title:                r.Title,
		Description:          r.Description,
		Quadrant:             quadrant,
		Position:             r.Position,
		Color:                color,
		DeadlineAt:           r.DeadlineAt,
		DeadlineHasTime:      r.DeadlineHasTime,
		Status:               status,
		CreatedAt:            r.CreatedAt,
		UpdatedAt:            r.UpdatedAt,
		CompletedAt:          r.CompletedAt,
		XPAwarded:            r.XPAwarded,
		QuadrantAtCompletion: snapshotQuadrant,
		CompletedVia:         completedVia,
		DeletedAt:            r.DeletedAt,
		Tags:                 tags,
	}
	if err := result.Validate(); err != nil {
		return task.Task{}, err
	}
	return result, nil
}

func optionalQuadrant(raw *string) (*shared.Quadrant, error) {
	if raw == nil {
		return nil, nil
	}
	quadrant, err := shared.ParseQuadrant(*raw)
	if err != nil {
		return nil, err
	}
	return &quadrant, nil
}

type userRow struct {
	ID           uuid.UUID
	Username     string
	Email        *string
	PasswordHash string
	AvatarURL    *string
	CreatedAt    time.Time
	LastLoginAt  *time.Time
	DeletedAt    *time.Time

	Plan          string
	PlanExpiresAt *time.Time

	Language             *string
	Timezone             *string
	ShowInLeaderboard    *bool
	NotificationsEnabled *bool

	LifetimeXP           *int64
	Level                *int32
	CurrentStreak        *int32
	LongestStreak        *int32
	LastStreakDate       *time.Time
	TasksCreated         *int32
	TasksCompleted       *int32
	SubtasksCompleted    *int32
	LinksCreated         *int32
	AchievementsUnlocked *int32
	StatsUpdatedAt       *time.Time
}

func (r *userRow) toDomain() (user.User, error) {
	subscription := user.Subscription{Plan: shared.Plan(r.Plan), ExpiresAt: r.PlanExpiresAt}

	language := shared.LanguageEN
	if r.Language != nil {
		parsed, err := shared.ParseLanguage(*r.Language)
		if err != nil {
			return user.User{}, err
		}
		language = parsed
	}

	settings := user.Settings{
		Language:             language,
		Timezone:             derefOr(r.Timezone, "UTC"),
		ShowInLeaderboard:    derefOr(r.ShowInLeaderboard, false),
		NotificationsEnabled: derefOr(r.NotificationsEnabled, true),
	}

	stats := user.Stats{
		LifetimeXP:           derefOr(r.LifetimeXP, 0),
		Level:                derefOr(r.Level, 1),
		CurrentStreak:        derefOr(r.CurrentStreak, 0),
		LongestStreak:        derefOr(r.LongestStreak, 0),
		LastStreakDate:       r.LastStreakDate,
		TasksCreated:         derefOr(r.TasksCreated, 0),
		TasksCompleted:       derefOr(r.TasksCompleted, 0),
		SubtasksCompleted:    derefOr(r.SubtasksCompleted, 0),
		LinksCreated:         derefOr(r.LinksCreated, 0),
		AchievementsUnlocked: derefOr(r.AchievementsUnlocked, 0),
		UpdatedAt:            derefOr(r.StatsUpdatedAt, r.CreatedAt),
	}

	return user.User{
		ID:           r.ID,
		Username:     r.Username,
		Email:        r.Email,
		PasswordHash: r.PasswordHash,
		AvatarURL:    r.AvatarURL,
		CreatedAt:    r.CreatedAt,
		LastLoginAt:  r.LastLoginAt,
		DeletedAt:    r.DeletedAt,
		Settings:     settings,
		Stats:        stats,
		Subscription: subscription,
	}, nil
}

func derefOr[T any](value *T, fallback T) T {
	if value == nil {
		return fallback
	}
	return *value
}
