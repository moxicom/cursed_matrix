package board

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/task"
	"github.com/moxicom/cursed_matrix/back/internal/domain/user"
)

// Draft is a task the user asked for. Position is not among the fields: where
// a task lands is the server's decision, not the client's.
type Draft struct {
	ParentID        *uuid.UUID
	Title           string
	Description     string
	Quadrant        shared.Quadrant
	Color           shared.TaskColor
	DeadlineAt      *time.Time
	DeadlineHasTime bool
}

// Patch is a change to an existing task. A nil field is one the client did not
// mention, which is not the same as one it cleared: DeadlineAt is cleared by
// setting ClearDeadline.
type Patch struct {
	Title           *string
	Description     *string
	Color           *shared.TaskColor
	DeadlineAt      *time.Time
	DeadlineHasTime *bool
	ClearDeadline   bool
}

// Create adds a task or a subtask to the board.
//
// The quota check and the insert share one transaction: counted outside it,
// two requests arriving together would both see room for the last slot.
func (s *Service) Create(ctx context.Context, userID uuid.UUID, draft Draft) (*task.Task, error) {
	var created *task.Task

	err := s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.withinQuota(ctx, userID); err != nil {
			return err
		}

		now := s.clock.Now().UTC()
		item, err := s.draftToTask(ctx, userID, draft, now)
		if err != nil {
			return err
		}

		if err := s.tasks.Create(ctx, item); err != nil {
			return err
		}
		created = item
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (s *Service) draftToTask(
	ctx context.Context,
	userID uuid.UUID,
	draft Draft,
	now time.Time,
) (*task.Task, error) {
	var (
		item *task.Task
		err  error
	)

	if draft.ParentID != nil {
		parent, parentErr := s.tasks.ByID(ctx, userID, *draft.ParentID)
		if parentErr != nil {
			return nil, parentErr
		}

		position, posErr := s.tasks.NextPosition(ctx, userID, nil, draft.ParentID)
		if posErr != nil {
			return nil, posErr
		}
		// NewSubtask refuses a parent that is itself a subtask, which is the
		// one-level rule the schema also enforces with a trigger.
		item, err = task.NewSubtask(uuid.New(), parent, draft.Title, position, now)
	} else {
		position, posErr := s.tasks.NextPosition(ctx, userID, &draft.Quadrant, nil)
		if posErr != nil {
			return nil, posErr
		}
		item, err = task.NewRegular(uuid.New(), userID, draft.Title, draft.Quadrant, position, now)
	}
	if err != nil {
		return nil, err
	}

	if err := applyDraft(item, draft); err != nil {
		return nil, err
	}
	if err := item.Validate(); err != nil {
		return nil, err
	}
	return item, nil
}

// Update changes the fields of a task the user may edit.
func (s *Service) Update(ctx context.Context, userID, taskID uuid.UUID, patch Patch) (*task.Task, error) {
	var updated *task.Task

	err := s.tx.Do(ctx, func(ctx context.Context) error {
		item, err := s.tasks.ByID(ctx, userID, taskID)
		if err != nil {
			return err
		}
		// A completed task is a record of what happened. Editing it would
		// rewrite history the XP ledger already refers to.
		if item.IsCompleted() {
			return shared.NewError(shared.CodeCompletedTaskFrozen,
				map[string]any{"taskId": taskID.String()})
		}

		if err := applyPatch(item, patch); err != nil {
			return err
		}
		if err := item.Validate(); err != nil {
			return err
		}
		if err := s.tasks.Update(ctx, item); err != nil {
			return err
		}
		updated = item
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// Delete removes a task from the user's views without losing the row.
func (s *Service) Delete(ctx context.Context, userID, taskID uuid.UUID) error {
	return s.tx.Do(ctx, func(ctx context.Context) error {
		if _, err := s.tasks.ByID(ctx, userID, taskID); err != nil {
			return err
		}
		return s.tasks.SoftDelete(ctx, userID, taskID, s.clock.Now().UTC())
	})
}

// withinQuota refuses a creation that would take the account past its plan.
func (s *Service) withinQuota(ctx context.Context, userID uuid.UUID) error {
	// Every account is on the free plan until billing exists; when it does,
	// the plan comes from the account rather than from here.
	limit := user.ActiveTaskLimit(shared.PlanFree)
	if limit == user.Unlimited {
		return nil
	}

	active, err := s.tasks.CountActive(ctx, userID)
	if err != nil {
		return err
	}
	if active >= limit {
		return shared.NewError(shared.CodeQuotaLimitReached,
			map[string]any{"limit": limit, "resource": "ACTIVE_TASKS"})
	}
	return nil
}

func applyDraft(item *task.Task, draft Draft) error {
	if err := task.ValidateDescription(draft.Description); err != nil {
		return err
	}
	item.Description = draft.Description

	if draft.Color != "" {
		if !draft.Color.Valid() {
			return shared.NewError(shared.CodeValidationFailed,
				map[string]any{"field": "color", "value": string(draft.Color)})
		}
		item.Color = draft.Color
	}

	item.DeadlineAt = draft.DeadlineAt
	item.DeadlineHasTime = draft.DeadlineAt != nil && draft.DeadlineHasTime
	return nil
}

func applyPatch(item *task.Task, patch Patch) error {
	if patch.Title != nil {
		clean, err := task.NormalizeTitle(*patch.Title)
		if err != nil {
			return err
		}
		item.Title = clean
	}
	if patch.Description != nil {
		if err := task.ValidateDescription(*patch.Description); err != nil {
			return err
		}
		item.Description = *patch.Description
	}
	if patch.Color != nil {
		if !patch.Color.Valid() {
			return shared.NewError(shared.CodeValidationFailed,
				map[string]any{"field": "color", "value": string(*patch.Color)})
		}
		item.Color = *patch.Color
	}

	switch {
	case patch.ClearDeadline:
		item.DeadlineAt = nil
		item.DeadlineHasTime = false
	case patch.DeadlineAt != nil:
		item.DeadlineAt = patch.DeadlineAt
		if patch.DeadlineHasTime != nil {
			item.DeadlineHasTime = *patch.DeadlineHasTime
		}
	case patch.DeadlineHasTime != nil:
		item.DeadlineHasTime = item.DeadlineAt != nil && *patch.DeadlineHasTime
	}
	return nil
}
