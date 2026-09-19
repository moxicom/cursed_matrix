// Package profile holds what the account has done: its streak, its history and
// the figures the profile screen shows.
package profile

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/app/cache"
	"github.com/moxicom/cursed_matrix/back/internal/app/port"
	"github.com/moxicom/cursed_matrix/back/internal/app/preferences"
	"github.com/moxicom/cursed_matrix/back/internal/domain/activity"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/user"
)

// dayGateTTL outlives any single calendar day, because the key names the day
// it belongs to and so cannot be mistaken for another one.
const dayGateTTL = 36 * time.Hour

type Service struct {
	users  port.UserRepository
	events port.ActivityRepository
	cache  port.Cache
	tx     port.TxManager
	clock  shared.Clock
	prefs  *preferences.Reader
}

func NewService(
	users port.UserRepository,
	events port.ActivityRepository,
	cached port.Cache,
	tx port.TxManager,
	clock shared.Clock,
) *Service {
	return &Service{
		users:  users,
		events: events,
		cache:  cached,
		tx:     tx,
		clock:  clock,
		prefs:  preferences.NewReader(users, cached),
	}
}

// TouchDay counts today towards the streak.
//
// The streak follows from opening the app, not from finishing anything: the
// product rewards turning up.
func (s *Service) TouchDay(ctx context.Context, userID uuid.UUID) (user.StreakChange, error) {
	settings, err := s.prefs.Settings(ctx, userID)
	if err != nil {
		return user.StreakChange{}, err
	}
	today := settings.LocalDate(s.clock.Now().UTC())

	// The gate names the day. A gate that only counted minutes would, for a
	// client polling across local midnight, still be closed on the first
	// request of the new day — and if that were the day's only request, the
	// day would be lost and the streak would break two days later.
	if s.alreadyCounted(ctx, userID, today) {
		return user.StreakChange{}, nil
	}

	var change user.StreakChange
	err = s.tx.Do(ctx, func(ctx context.Context) error {
		counted, err := s.users.TouchStreak(ctx, userID, s.clock.Now().UTC())
		if err != nil {
			return err
		}
		change = counted

		if !counted.Extended {
			return nil
		}
		// In the same transaction as the streak it describes: an event that
		// failed on its own would be lost for good, because the gate would
		// already be shut.
		event, err := activity.New(userID, shared.EventStreakExtended, nil, s.clock.Now().UTC())
		if err != nil {
			return err
		}
		return s.events.Record(ctx, event.
			With("currentStreak", counted.Current).
			With("longestStreak", counted.Longest))
	})
	if err != nil {
		return user.StreakChange{}, err
	}

	s.remember(ctx, userID, today)
	return change, nil
}

func (s *Service) dayKey(userID uuid.UUID, day time.Time) cache.Key {
	return cache.Key{
		Scope: cache.UserScope(userID),
		Name:  "streak-counted",
		Args:  []string{day.Format(time.DateOnly)},
	}
}

func (s *Service) alreadyCounted(ctx context.Context, userID uuid.UUID, day time.Time) bool {
	if s.cache == nil {
		return false
	}
	var counted bool
	// A cache that is down means asking the database, which is correct and
	// only costs a statement.
	hit, err := s.cache.GetInto(ctx, s.dayKey(userID, day), &counted)
	return err == nil && hit
}

func (s *Service) remember(ctx context.Context, userID uuid.UUID, day time.Time) {
	if s.cache == nil {
		return
	}
	_ = s.cache.Set(ctx, s.dayKey(userID, day), true, dayGateTTL)
}

// HeatmapDays is the width of the grid the client renders.
const HeatmapDays = 365

// Heatmap returns one entry per day, including the empty ones.
//
// The grid is fixed, so gaps cannot be left for the client to infer: a missing
// day and a day with nothing done look the same on screen but mean different
// things to the code that lays it out.
func (s *Service) Heatmap(ctx context.Context, userID uuid.UUID) ([]activity.Day, error) {
	settings, err := s.prefs.Settings(ctx, userID)
	if err != nil {
		return nil, err
	}

	today := settings.LocalDate(s.clock.Now().UTC())
	from := today.AddDate(0, 0, -(HeatmapDays - 1))

	recorded, err := s.events.Heatmap(ctx, userID, from, today)
	if err != nil {
		return nil, err
	}

	byDay := make(map[string]activity.Day, len(recorded))
	for _, day := range recorded {
		byDay[day.Date.Format(time.DateOnly)] = day
	}

	days := make([]activity.Day, 0, HeatmapDays)
	for offset := range HeatmapDays {
		date := from.AddDate(0, 0, offset)
		day, ok := byDay[date.Format(time.DateOnly)]
		if !ok {
			day = activity.Day{}
		}
		day.Date = date
		days = append(days, day)
	}
	return days, nil
}

// RecordGraphOpened notes that the user looked at the graph today.
//
// It feeds no heatmap and no XP: the only thing that asks is the exploration
// achievement, which counts the distinct days a user went looking.
func (s *Service) RecordGraphOpened(ctx context.Context, userID uuid.UUID) error {
	event, err := activity.New(userID, shared.EventGraphOpened, nil, s.clock.Now().UTC())
	if err != nil {
		return err
	}
	// Once per day, whatever the client does: the achievement counts the days
	// a user went looking, not the times they clicked.
	return s.events.RecordDaily(ctx, event)
}
