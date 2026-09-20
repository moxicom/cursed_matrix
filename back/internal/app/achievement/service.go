// Package achievement decides what the user has earned.
package achievement

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/app/port"
	"github.com/moxicom/cursed_matrix/back/internal/domain/achievement"
	"github.com/moxicom/cursed_matrix/back/internal/domain/activity"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/user"
	"github.com/moxicom/cursed_matrix/back/pkg/utils"
)

type Service struct {
	catalogue port.AchievementRepository
	users     port.UserRepository
	events    port.ActivityRepository
	clock     shared.Clock
}

func NewService(
	catalogue port.AchievementRepository,
	users port.UserRepository,
	events port.ActivityRepository,
	clock shared.Clock,
) *Service {
	return &Service{catalogue: catalogue, users: users, events: events, clock: clock}
}

// View is one row of the achievements screen.
//
// Progress is reported for every entry, earned or not, because the screen
// sorts by how close the user is.
type View struct {
	Code       string
	Category   string
	Threshold  int64
	Progress   int64
	RewardXP   int32
	UnlockedAt *time.Time
}

// List returns the whole catalogue with the user's standing against it.
func (s *Service) List(ctx context.Context, userID uuid.UUID) ([]View, error) {
	catalogue, metrics, held, err := s.state(ctx, userID)
	if err != nil {
		return nil, err
	}

	views := make([]View, 0, len(catalogue))
	for i := range catalogue {
		entry := &catalogue[i]
		progress, known := entry.Progress(&metrics)
		if !known {
			s.warnUnknown(ctx, entry)
		}

		view := View{
			Code:      entry.Code,
			Category:  entry.Category,
			Threshold: entry.Threshold,
			Progress:  progress,
			RewardXP:  entry.RewardXP,
		}
		if at, ok := held[entry.ID]; ok {
			unlockedAt := at
			view.UnlockedAt = &unlockedAt
		}
		views = append(views, view)
	}
	return views, nil
}

// Evaluate unlocks whatever the user has just become entitled to and returns
// the codes, so the response that caused it can say so.
//
// It is called inside the transaction that moved a metric: an achievement
// earned by work that then rolled back would be an award for nothing.
func (s *Service) Evaluate(ctx context.Context, userID uuid.UUID) ([]string, error) {
	catalogue, metrics, held, err := s.state(ctx, userID)
	if err != nil {
		return nil, err
	}

	now := s.clock.Now().UTC()
	var unlocked []string

	for i := range catalogue {
		entry := &catalogue[i]
		// Held is held, repeatable or not: user_achievements is keyed on
		// (user_id, achievement_id), so a second row cannot exist. Honouring
		// the flag here would look like support for repeats while doing
		// nothing; making it real needs the table to change shape first.
		if _, already := held[entry.ID]; already {
			continue
		}

		earned, known := entry.Earned(&metrics)
		if !known {
			s.warnUnknown(ctx, entry)
			continue
		}
		if !earned {
			continue
		}

		fresh, err := s.catalogue.Unlock(ctx, userID, entry.ID, now)
		if err != nil {
			return nil, err
		}
		// Another request may have unlocked it a moment ago; only the one that
		// actually wrote the row announces it.
		if !fresh {
			continue
		}

		event, err := activity.New(userID, shared.EventAchievementUnlocked, nil, now)
		if err != nil {
			return nil, err
		}
		if err := s.events.Record(ctx, event.With("code", entry.Code)); err != nil {
			return nil, err
		}
		unlocked = append(unlocked, entry.Code)
	}

	if len(unlocked) > 0 {
		delta := user.StatsDelta{AchievementsUnlocked: int32(len(unlocked))}
		if _, err := s.users.ApplyStats(ctx, userID, delta); err != nil {
			return nil, err
		}
	}
	return unlocked, nil
}

func (s *Service) state(
	ctx context.Context,
	userID uuid.UUID,
) ([]achievement.Achievement, achievement.Metrics, map[uuid.UUID]time.Time, error) {
	catalogue, err := s.catalogue.Catalogue(ctx)
	if err != nil {
		return nil, achievement.Metrics{}, nil, err
	}

	metrics, err := s.catalogue.Metrics(ctx, userID)
	if err != nil {
		return nil, achievement.Metrics{}, nil, err
	}

	unlocks, err := s.catalogue.Unlocked(ctx, userID)
	if err != nil {
		return nil, achievement.Metrics{}, nil, err
	}

	held := make(map[uuid.UUID]time.Time, len(unlocks))
	for _, unlock := range unlocks {
		held[unlock.AchievementID] = unlock.UnlockedAt
	}
	return catalogue, metrics, held, nil
}

// warnUnknown reports a catalogue row this release cannot measure. It is a
// deployment mistake, not a user's, so it is logged rather than returned.
func (s *Service) warnUnknown(ctx context.Context, entry *achievement.Achievement) {
	utils.LoggerFromContext(ctx).WarnContext(ctx, "achievement watches an unknown metric",
		"code", entry.Code, "metric", string(entry.Metric))
}
