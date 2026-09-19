package board

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/domain/progression"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/task"
	"github.com/moxicom/cursed_matrix/back/internal/domain/user"
)

// Move is where the client wants a task, expressed as a neighbour rather than
// a number.
type Move struct {
	Quadrant shared.Quadrant
	Before   *uuid.UUID
	After    *uuid.UUID
}

// MoveTask puts a task in a quadrant, at a place named by its neighbours.
//
// Moving a subtask this way promotes it: it leaves its parent and takes a
// place among the quadrant's own tasks, which is the same outcome as Promote.
func (s *Service) MoveTask(ctx context.Context, userID, taskID uuid.UUID, move Move) ([]task.Task, error) {
	var changed []task.Task

	err := s.tx.Do(ctx, func(ctx context.Context) error {
		item, err := s.tasks.ByIDForUpdate(ctx, userID, taskID)
		if err != nil {
			return err
		}
		// The quadrant a task was completed in is what priced its XP, so it is
		// frozen with the snapshot. Reopening is how a user changes their mind
		// about where finished work belongs.
		if item.IsCompleted() {
			return shared.NewError(shared.CodeCompletedTaskFrozen,
				map[string]any{"taskId": taskID.String()})
		}
		if !move.Quadrant.Valid() {
			return shared.NewError(shared.CodeValidationFailed,
				map[string]any{"field": "targetQuadrant", "value": string(move.Quadrant)})
		}

		moved, err := s.placeInQuadrant(ctx, userID, item, move.Quadrant, move.Before, move.After, &changed)
		if err != nil {
			return err
		}
		changed = append(changed, *moved)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return changed, nil
}

// Promote turns a subtask into a task of its own.
//
// The quadrant defaults to the parent's, which is the one the subtask was
// already being judged by; tags, colour, deadline and its XP snapshot are
// untouched.
func (s *Service) Promote(
	ctx context.Context,
	userID, taskID uuid.UUID,
	quadrant *shared.Quadrant,
) ([]task.Task, error) {
	var changed []task.Task

	err := s.tx.Do(ctx, func(ctx context.Context) error {
		item, err := s.tasks.ByIDForUpdate(ctx, userID, taskID)
		if err != nil {
			return err
		}
		if !item.IsSubtask() {
			return shared.NewError(shared.CodeNotASubtask,
				map[string]any{"taskId": taskID.String()})
		}

		target, err := s.promotionQuadrant(ctx, userID, item, quadrant)
		if err != nil {
			return err
		}

		promoted, err := s.placeInQuadrant(ctx, userID, item, target, nil, nil, &changed)
		if err != nil {
			return err
		}
		changed = append(changed, *promoted)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return changed, nil
}

// placeInQuadrant writes the task into the target scope, rebalancing the scope
// first if the gap it was asked to land in is used up.
func (s *Service) placeInQuadrant(
	ctx context.Context,
	userID uuid.UUID,
	item *task.Task,
	quadrant shared.Quadrant,
	before, after *uuid.UUID,
	changed *[]task.Task,
) (*task.Task, error) {
	scope, err := s.tasks.ScopeTasks(ctx, userID, quadrant)
	if err != nil {
		return nil, err
	}
	// A task already in this quadrant is not its own neighbour.
	scope = without(scope, item.ID)

	position, placements, err := task.Place(scope, before, after)
	if err != nil {
		return nil, err
	}
	if len(placements) > 0 {
		if err := s.tasks.Reposition(ctx, userID, placements); err != nil {
			return nil, err
		}
		for _, placement := range placements {
			*changed = append(*changed, task.Task{ID: placement.ID, Position: placement.Position})
		}
	}

	if item.IsSubtask() {
		if err := item.Promote(quadrant, position, s.clock.Now().UTC()); err != nil {
			return nil, err
		}
	} else {
		target := quadrant
		item.Quadrant = &target
		item.Position = position
	}

	if err := item.Validate(); err != nil {
		return nil, err
	}
	if err := s.tasks.Update(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) promotionQuadrant(
	ctx context.Context,
	userID uuid.UUID,
	item *task.Task,
	asked *shared.Quadrant,
) (shared.Quadrant, error) {
	if asked != nil {
		if !asked.Valid() {
			return "", shared.NewError(shared.CodeValidationFailed,
				map[string]any{"field": "quadrant", "value": string(*asked)})
		}
		return *asked, nil
	}
	return s.effectiveQuadrant(ctx, userID, item)
}

// Delete removes a task and reports what went with it.
//
// A parent with unfinished subtasks is refused rather than quietly taking them
// down: they are the user's open work, and only the user can decide whether to
// finish, promote or drop them. Completed subtasks go with the parent, and any
// XP they were paid is withdrawn by a compensating entry, so the ledger still
// adds up to the user's lifetime total.
func (s *Service) Delete(ctx context.Context, userID, taskID uuid.UUID) ([]uuid.UUID, error) {
	var deleted []uuid.UUID

	err := s.tx.Do(ctx, func(ctx context.Context) error {
		item, err := s.tasks.ByIDForUpdate(ctx, userID, taskID)
		if err != nil {
			return err
		}

		doomed := []*task.Task{item}
		if !item.IsSubtask() {
			subtasks, err := s.tasks.Subtasks(ctx, userID, item.ID)
			if err != nil {
				return err
			}
			for i := range subtasks {
				if !subtasks[i].IsCompleted() {
					return shared.NewError(shared.CodeHasActiveSubtasks,
						map[string]any{"taskId": taskID.String()})
				}
				doomed = append(doomed, &subtasks[i])
			}
		}

		now := s.clock.Now().UTC()
		var delta user.StatsDelta

		for _, victim := range doomed {
			deleted = append(deleted, victim.ID)
			if err := s.withdraw(ctx, userID, victim, now, &delta); err != nil {
				return err
			}
		}

		if err := s.tasks.SoftDelete(ctx, userID, deleted, now); err != nil {
			return err
		}
		return s.applyProgress(ctx, userID, delta, &Progress{})
	})
	if err != nil {
		return nil, err
	}
	return deleted, nil
}

// withdraw takes back what a deleted task counted for.
//
// The counters move for any completed task, whether or not there is XP to
// revoke: a task that predates the ledger still stopped being completed, and
// leaving its count behind overstates the profile for good.
func (s *Service) withdraw(
	ctx context.Context,
	userID uuid.UUID,
	item *task.Task,
	now time.Time,
	delta *user.StatsDelta,
) error {
	if !item.IsCompleted() {
		return nil
	}

	if item.IsSubtask() {
		delta.SubtasksCompleted--
	} else {
		delta.TasksCompleted--
	}

	if item.XPAwarded == nil || *item.XPAwarded <= 0 {
		return nil
	}
	delta.XP -= int64(*item.XPAwarded)

	entry, err := progression.Revoke(userID, item.ID, *item.XPAwarded, shared.XPTaskDeleted, now)
	if err != nil {
		return err
	}
	return s.ledger.Record(ctx, entry)
}

func without(scope []task.Task, id uuid.UUID) []task.Task {
	out := scope[:0]
	for _, item := range scope {
		if item.ID != id {
			out = append(out, item)
		}
	}
	return out
}
