// Package activity holds the record of what a user did and when.
package activity

import (
	"time"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
)

// Event is one thing that happened.
//
// The day it belongs to is not stored here: it is the user's calendar day,
// which follows from their timezone, and the write resolves it from the
// settings the account already holds.
type Event struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Type       shared.ActivityEventType
	TaskID     *uuid.UUID
	OccurredAt time.Time
	LocalDate  time.Time
	Metadata   map[string]any
}

// New records that something happened to a task.
func New(userID uuid.UUID, eventType shared.ActivityEventType, taskID *uuid.UUID, at time.Time) (*Event, error) {
	if !eventType.Valid() {
		return nil, shared.NewError(shared.CodeValidationFailed,
			map[string]any{"field": "type", "value": string(eventType)})
	}
	if userID == uuid.Nil {
		return nil, shared.NewError(shared.CodeValidationFailed, map[string]any{"field": "userId"})
	}

	var task *uuid.UUID
	if taskID != nil {
		id := *taskID
		task = &id
	}
	return &Event{
		ID: uuid.New(), UserID: userID,
		Type: eventType, TaskID: task,
		OccurredAt: at,
		Metadata:   map[string]any{},
	}, nil
}

// With attaches detail the heatmap does not need but the history does.
func (e *Event) With(key string, value any) *Event {
	e.Metadata[key] = value
	return e
}

// CreationTypes and CompletionTypes are the two halves of the heatmap.
//
// It answers "what did I get done", so creating and completing count and the
// rest — levelling up, unlocking something, opening the graph — do not: they
// follow from work already counted, and counting them again would colour a day
// twice for one action.
func CreationTypes() []string {
	return []string{string(shared.EventTaskCreated)}
}

func CompletionTypes() []string {
	return []string{
		string(shared.EventTaskCompleted),
		string(shared.EventSubtaskCompleted),
	}
}

// Day is one square of the heatmap.
type Day struct {
	Date      time.Time
	Created   int
	Completed int
}

// Total is what the square's colour is chosen from; the breakdown is for the
// detail view.
func (d *Day) Total() int { return d.Created + d.Completed }
