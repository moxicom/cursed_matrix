// Package task holds the Task aggregate and its invariants.
package task

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
)

const (
	TitleMaxLen       = 100
	DescriptionMaxLen = 2000

	PositionGap = 1024
)

// Task is both a regular task and a subtask; ParentID tells them apart.
type Task struct {
	ID       uuid.UUID
	UserID   uuid.UUID
	ParentID *uuid.UUID

	Title       string
	Description string

	Quadrant *shared.Quadrant
	Position int32
	Color    shared.TaskColor

	DeadlineAt      *time.Time
	DeadlineHasTime bool

	Status    shared.TaskStatus
	CreatedAt time.Time
	UpdatedAt time.Time

	CompletedAt          *time.Time
	XPAwarded            *int32
	QuadrantAtCompletion *shared.Quadrant
	CompletedVia         *shared.CompletionSource

	DeletedAt *time.Time

	Tags []string
}

func NewRegular(
	id, userID uuid.UUID,
	title string,
	quadrant shared.Quadrant,
	position int32,
	now time.Time,
) (*Task, error) {
	clean, err := NormalizeTitle(title)
	if err != nil {
		return nil, err
	}
	if !quadrant.Valid() {
		return nil, shared.NewError(shared.CodeValidationFailed,
			map[string]any{"field": "quadrant", "value": string(quadrant)})
	}
	q := quadrant
	return &Task{
		ID: id, UserID: userID,
		Title:     clean,
		Quadrant:  &q,
		Position:  position,
		Color:     shared.ColorNone,
		Status:    shared.TaskStatusActive,
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

// NewSubtask creates a subtask under a parent that is not itself a subtask.
func NewSubtask(id uuid.UUID, parent *Task, title string, position int32, now time.Time) (*Task, error) {
	if parent.IsSubtask() {
		return nil, shared.NewError(shared.CodeNestingNotAllowed,
			map[string]any{"taskId": parent.ID.String()})
	}
	clean, err := NormalizeTitle(title)
	if err != nil {
		return nil, err
	}
	parentID := parent.ID
	return &Task{
		ID: id, UserID: parent.UserID,
		ParentID:  &parentID,
		Title:     clean,
		Position:  position,
		Color:     parent.Color,
		Status:    shared.TaskStatusActive,
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (t *Task) IsSubtask() bool { return t.ParentID != nil }

func (t *Task) IsCompleted() bool { return t.Status == shared.TaskStatusCompleted }

// EffectiveQuadrant resolves the quadrant governing XP, false for an orphan.
func (t *Task) EffectiveQuadrant(parent *Task) (shared.Quadrant, bool) {
	if t.Quadrant != nil {
		return *t.Quadrant, true
	}
	if parent != nil && parent.Quadrant != nil {
		return *parent.Quadrant, true
	}
	return "", false
}

// Complete closes the task and freezes its XP snapshot.
func (t *Task) Complete(at time.Time, xp int32, quadrant shared.Quadrant, via shared.CompletionSource) error {
	if t.IsCompleted() {
		return shared.NewError(shared.CodeTaskAlreadyDone, map[string]any{"taskId": t.ID.String()})
	}
	completedAt, awarded, q, source := at, xp, quadrant, via
	t.Status = shared.TaskStatusCompleted
	t.CompletedAt = &completedAt
	t.XPAwarded = &awarded
	t.QuadrantAtCompletion = &q
	t.CompletedVia = &source
	t.UpdatedAt = at
	return nil
}

// Reopen returns the task to work and clears its snapshot.
func (t *Task) Reopen(at time.Time) error {
	if !t.IsCompleted() {
		return shared.NewError(shared.CodeValidationFailed,
			map[string]any{"field": "status", "value": string(t.Status)})
	}
	t.Status = shared.TaskStatusActive
	t.CompletedAt = nil
	t.XPAwarded = nil
	t.QuadrantAtCompletion = nil
	t.CompletedVia = nil
	t.UpdatedAt = at
	return nil
}

func (t *Task) Promote(quadrant shared.Quadrant, position int32, at time.Time) error {
	if !t.IsSubtask() {
		return shared.NewError(shared.CodeNotASubtask, map[string]any{"taskId": t.ID.String()})
	}
	if !quadrant.Valid() {
		return shared.NewError(shared.CodeValidationFailed,
			map[string]any{"field": "quadrant", "value": string(quadrant)})
	}
	q := quadrant
	t.ParentID = nil
	t.Quadrant = &q
	t.Position = position
	t.UpdatedAt = at
	return nil
}

// Validate re-checks the invariants the flat struct is able to violate.
func (t *Task) Validate() error {
	switch {
	case t.ID == uuid.Nil || t.UserID == uuid.Nil:
		return shared.NewError(shared.CodeValidationFailed, map[string]any{"field": "id"})

	case (t.ParentID == nil) != (t.Quadrant != nil):
		return shared.NewError(shared.CodeValidationFailed, map[string]any{"field": "quadrant"})

	case t.ParentID != nil && *t.ParentID == t.ID:
		return shared.NewError(shared.CodeValidationFailed, map[string]any{"field": "parentTaskId"})

	case t.IsCompleted() != (t.CompletedAt != nil):
		return shared.NewError(shared.CodeValidationFailed, map[string]any{"field": "completedAt"})

	case !t.Color.Valid():
		return shared.NewError(shared.CodeValidationFailed, map[string]any{"field": "color"})
	}

	if _, err := NormalizeTitle(t.Title); err != nil {
		return err
	}
	return ValidateDescription(t.Description)
}

// NormalizeTitle trims a title and enforces its length in runes.
func NormalizeTitle(title string) (string, error) {
	clean := strings.TrimSpace(title)
	if clean == "" {
		return "", shared.NewError(shared.CodeValidationFailed,
			map[string]any{"field": "title", "reason": "empty"})
	}
	if utf8.RuneCountInString(clean) > TitleMaxLen {
		return "", shared.NewError(shared.CodeValidationFailed,
			map[string]any{"field": "title", "max": TitleMaxLen})
	}
	return clean, nil
}

func ValidateDescription(description string) error {
	if utf8.RuneCountInString(description) > DescriptionMaxLen {
		return shared.NewError(shared.CodeValidationFailed,
			map[string]any{"field": "description", "max": DescriptionMaxLen})
	}
	return nil
}
