package board

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/domain/activity"
	"github.com/moxicom/cursed_matrix/back/internal/domain/progression"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/task"
	"github.com/moxicom/cursed_matrix/back/internal/domain/user"
)

// LevelChange is a level crossing, reported once however many thresholds the
// XP passed at the same time.
type LevelChange struct {
	From int32
	To   int32
}

// Progress is what one completion or reopening changed, so the client can
// reconcile its board without fetching it again.
type Progress struct {
	Tasks     []task.Task
	XPAwarded int32
	LevelUp   *LevelChange
	Unlocked  []string
}

// Complete marks a task done, cascades into its active subtasks and records
// the XP.
//
// All of it is one transaction. The status, the snapshot, the ledger entry and
// the aggregate are one fact about the user's history: a partial write would
// leave XP awarded for a task that is not completed, or a completion that
// nothing paid for.
func (s *Service) Complete(ctx context.Context, userID, taskID uuid.UUID) (*Progress, error) {
	result := &Progress{}

	err := s.tx.Do(ctx, func(ctx context.Context) error {
		// Locked, not just read: two requests for the same task would
		// otherwise both see it active and both pay out.
		item, err := s.tasks.ByIDForUpdate(ctx, userID, taskID)
		if err != nil {
			return err
		}
		if item.IsCompleted() {
			return shared.NewError(shared.CodeTaskAlreadyDone,
				map[string]any{"taskId": taskID.String()})
		}

		quadrant, err := s.effectiveQuadrant(ctx, userID, item)
		if err != nil {
			return err
		}

		now := s.clock.Now().UTC()
		var (
			delta  user.StatsDelta
			events []activity.Event
		)

		if err := s.award(ctx, item, quadrant, shared.CompletionDirect, now, &delta, result, &events); err != nil {
			return err
		}

		// A subtask has no subtasks of its own, so only a parent cascades.
		if !item.IsSubtask() {
			subtasks, err := s.tasks.Subtasks(ctx, userID, item.ID)
			if err != nil {
				return err
			}
			for i := range subtasks {
				child := &subtasks[i]
				if child.IsCompleted() {
					continue
				}
				if err := s.award(ctx, child, quadrant, shared.CompletionParentCascade, now, &delta, result, &events); err != nil {
					return err
				}
			}
		}

		// One statement for the cascade: a parent and its subtasks are one
		// event in the user's day, however many rows that is.
		if err := s.events.RecordMany(ctx, events); err != nil {
			return err
		}
		if err := s.applyProgress(ctx, userID, delta, result); err != nil {
			return err
		}
		// Inside the same transaction as the work that earned it: an award for
		// a completion that then rolled back would be an award for nothing.
		unlocked, err := s.awards.Evaluate(ctx, userID)
		if err != nil {
			return err
		}
		result.Unlocked = unlocked
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Reopen returns a completed task to work and withdraws what it paid.
//
// Subtasks completed by the cascade stay completed: the client asked about one
// task, and silently undoing other completions would take away XP the user has
// no reason to expect to lose.
func (s *Service) Reopen(ctx context.Context, userID, taskID uuid.UUID) (*Progress, error) {
	result := &Progress{}

	err := s.tx.Do(ctx, func(ctx context.Context) error {
		// The same lock as completing, for the same reason: two reopenings of
		// one task would each withdraw the full amount.
		item, err := s.tasks.ByIDForUpdate(ctx, userID, taskID)
		if err != nil {
			return err
		}
		if !item.IsCompleted() {
			return shared.NewError(shared.CodeValidationFailed,
				map[string]any{"field": "status", "value": string(item.Status)})
		}

		granted := int32(0)
		if item.XPAwarded != nil {
			granted = *item.XPAwarded
		}
		wasSubtask := item.IsSubtask()

		now := s.clock.Now().UTC()
		if err := item.Reopen(now); err != nil {
			return err
		}
		if err := s.tasks.Update(ctx, item); err != nil {
			return err
		}

		delta := user.StatsDelta{XP: -int64(granted)}
		if wasSubtask {
			delta.SubtasksCompleted = -1
		} else {
			delta.TasksCompleted = -1
		}

		// Nothing was granted if the task predates the ledger, and an entry of
		// zero is not a fact worth recording. The counters move either way:
		// the task is no longer completed.
		if granted > 0 {
			entry, err := progression.Revoke(userID, item.ID, granted, shared.XPTaskReopened, now)
			if err != nil {
				return err
			}
			if err := s.ledger.Record(ctx, entry); err != nil {
				return err
			}
		}

		result.Tasks = append(result.Tasks, *item)
		result.XPAwarded = -granted
		return s.applyProgress(ctx, userID, delta, result)
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// effectiveQuadrant resolves the quadrant the XP is priced from.
//
// A subtask is priced by its parent's quadrant. If the parent cannot be read,
// the request fails rather than falling back to one: a guessed quadrant would
// pay out at an arbitrary rate and bury the corruption.
func (s *Service) effectiveQuadrant(
	ctx context.Context,
	userID uuid.UUID,
	item *task.Task,
) (shared.Quadrant, error) {
	var parent *task.Task

	if item.IsSubtask() {
		found, err := s.tasks.ByID(ctx, userID, *item.ParentID)
		if err != nil {
			return "", shared.NewError(shared.CodeOrphanedNode,
				map[string]any{"taskId": item.ID.String()})
		}
		parent = found
	}

	quadrant, ok := item.EffectiveQuadrant(parent)
	if !ok {
		return "", shared.NewError(shared.CodeOrphanedNode,
			map[string]any{"taskId": item.ID.String()})
	}
	return quadrant, nil
}

// award completes one task and writes its ledger entry.
func (s *Service) award(
	ctx context.Context,
	item *task.Task,
	quadrant shared.Quadrant,
	via shared.CompletionSource,
	now time.Time,
	delta *user.StatsDelta,
	result *Progress,
	events *[]activity.Event,
) error {
	xp, err := s.xp.Reward(quadrant, item.IsSubtask())
	if err != nil {
		return err
	}
	if err := item.Complete(now, xp, quadrant, via); err != nil {
		return err
	}
	if err := s.tasks.Update(ctx, item); err != nil {
		return err
	}

	source := shared.XPTaskCompleted
	if item.IsSubtask() {
		source = shared.XPSubtaskCompleted
	}
	// Each completion of a task is rewarded once. A task reopened and finished
	// again earns the next sequence number rather than colliding with the
	// entry the first completion left behind.
	granted, err := s.ledger.GrantCount(ctx, item.ID, source)
	if err != nil {
		return err
	}

	entry, err := progression.Grant(item.UserID, item.ID, xp, item.IsSubtask(),
		quadrant, s.xp.Version, granted+1, now)
	if err != nil {
		return err
	}
	if err := s.ledger.Record(ctx, entry); err != nil {
		return err
	}

	delta.XP += int64(xp)
	eventType := shared.EventTaskCompleted
	if item.IsSubtask() {
		delta.SubtasksCompleted++
		eventType = shared.EventSubtaskCompleted
	} else {
		delta.TasksCompleted++
	}

	event, err := activity.New(item.UserID, eventType, &item.ID, now)
	if err != nil {
		return err
	}
	*events = append(*events, *event.With("xp", xp).With("via", string(via)))

	result.Tasks = append(result.Tasks, *item)
	result.XPAwarded += xp
	return nil
}

// applyProgress moves the aggregate and reports a level crossing.
func (s *Service) applyProgress(
	ctx context.Context,
	userID uuid.UUID,
	delta user.StatsDelta,
	result *Progress,
) error {
	if delta.Empty() {
		return nil
	}

	stats, err := s.users.ApplyStats(ctx, userID, delta)
	if err != nil {
		return err
	}

	level := s.xp.LevelForXP(stats.LifetimeXP)
	if level == stats.Level {
		return nil
	}
	if err := s.users.SetLevel(ctx, userID, level); err != nil {
		return err
	}
	// Reported only upwards: losing a level to a reopening is not an event the
	// product celebrates.
	if level <= stats.Level {
		return nil
	}
	result.LevelUp = &LevelChange{From: stats.Level, To: level}

	event, err := activity.New(userID, shared.EventLevelUp, nil, s.clock.Now().UTC())
	if err != nil {
		return err
	}
	return s.events.Record(ctx, event.With("fromLevel", stats.Level).With("toLevel", level))
}
