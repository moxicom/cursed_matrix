package profile_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	appachievement "github.com/moxicom/cursed_matrix/back/internal/app/achievement"
	"github.com/moxicom/cursed_matrix/back/internal/app/cache"
	"github.com/moxicom/cursed_matrix/back/internal/app/profile"
	"github.com/moxicom/cursed_matrix/back/internal/domain/achievement"
	"github.com/moxicom/cursed_matrix/back/internal/domain/activity"
	"github.com/moxicom/cursed_matrix/back/internal/domain/leaderboard"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/user"
)

type stubUsers struct {
	settings user.Settings
	touches  int
	days     []time.Time
	change   user.StreakChange
}

func (s *stubUsers) ByID(_ context.Context, id uuid.UUID) (*user.User, error) {
	return &user.User{ID: id, Settings: s.settings}, nil
}

func (s *stubUsers) ByUsername(context.Context, string) (*user.User, error) { return nil, nil }
func (s *stubUsers) Create(context.Context, *user.User) error               { return nil }
func (s *stubUsers) UpdateSettings(context.Context, uuid.UUID, user.Settings) error {
	return nil
}
func (s *stubUsers) TouchLogin(context.Context, uuid.UUID, time.Time) error { return nil }
func (s *stubUsers) ApplyStats(context.Context, uuid.UUID, user.StatsDelta) (user.Stats, error) {
	return user.Stats{}, nil
}
func (s *stubUsers) SetLevel(context.Context, uuid.UUID, int32) error { return nil }
func (s *stubUsers) LockAccount(context.Context, uuid.UUID) error     { return nil }

func (s *stubUsers) TouchStreak(_ context.Context, _ uuid.UUID, at time.Time) (user.StreakChange, error) {
	s.touches++
	s.days = append(s.days, at)
	return s.change, nil
}

type stubEvents struct{ recorded []activity.Event }

func (e *stubEvents) Record(_ context.Context, event *activity.Event) error {
	e.recorded = append(e.recorded, *event)
	return nil
}

func (e *stubEvents) RecordMany(_ context.Context, events []activity.Event) error {
	e.recorded = append(e.recorded, events...)
	return nil
}

func (e *stubEvents) RecordDaily(_ context.Context, event *activity.Event) error {
	e.recorded = append(e.recorded, *event)
	return nil
}

func (e *stubEvents) Heatmap(context.Context, uuid.UUID, time.Time, time.Time) ([]activity.Day, error) {
	return nil, nil
}

func (e *stubEvents) Events(context.Context, uuid.UUID, *activity.Cursor, int) ([]activity.Entry, error) {
	return nil, nil
}

func (e *stubEvents) Stats(context.Context, uuid.UUID, time.Time) (activity.Stats, error) {
	return activity.Stats{}, nil
}

// stubCache holds whatever it is given, because the reader caches settings
// through the same port the day gate uses.
type stubCache struct{ stored map[string]any }

func (c *stubCache) GetInto(_ context.Context, key cache.Key, dst any) (bool, error) {
	value, ok := c.stored[cacheKey(key)]
	if !ok {
		return false, nil
	}
	switch target := dst.(type) {
	case *bool:
		*target = value.(bool)
	case *user.Settings:
		*target = value.(user.Settings)
	default:
		return false, nil
	}
	return true, nil
}

func (c *stubCache) Set(_ context.Context, key cache.Key, value any, _ time.Duration) error {
	if c.stored == nil {
		c.stored = map[string]any{}
	}
	c.stored[cacheKey(key)] = value
	return nil
}

func (c *stubCache) Invalidate(context.Context, cache.Scope) error { return nil }

func cacheKey(key cache.Key) string {
	out := string(key.Scope) + "/" + key.Name
	for _, arg := range key.Args {
		out += "/" + arg
	}
	return out
}

// stubAwards is an empty catalogue: these tests are about the streak, and an
// empty catalogue unlocks nothing.
type stubAwards struct{}

func (*stubAwards) Catalogue(context.Context) ([]achievement.Achievement, error) {
	return nil, nil
}

func (*stubAwards) Metrics(context.Context, uuid.UUID) (achievement.Metrics, error) {
	return achievement.Metrics{}, nil
}

func (*stubAwards) Unlocked(context.Context, uuid.UUID) ([]achievement.Unlock, error) {
	return nil, nil
}

func (*stubAwards) Unlock(context.Context, uuid.UUID, uuid.UUID, time.Time) (bool, error) {
	return false, nil
}

type stubRanking struct{}

func (*stubRanking) Page(context.Context, *time.Time, int, int) ([]leaderboard.Entry, error) {
	return nil, nil
}

func (*stubRanking) Standing(context.Context, uuid.UUID, *time.Time) (leaderboard.Standing, error) {
	return leaderboard.Standing{}, nil
}

type stubTx struct{}

func (*stubTx) Do(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

type movingClock struct{ at time.Time }

func (c *movingClock) Now() time.Time { return c.at }

// TestTouchDayCountsEachDayOnce is the whole streak rule: many visits in one
// day move it once, and a new day always reaches the database however recently
// the last visit was.
func TestTouchDayCountsEachDayOnce(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name        string
		timezone    string
		visits      []time.Time
		wantTouches int
	}{
		{
			name:        "one visit counts once",
			timezone:    "UTC",
			visits:      []time.Time{time.Date(2026, 9, 20, 9, 0, 0, 0, time.UTC)},
			wantTouches: 1,
		},
		{
			name:     "many visits in a day still count once",
			timezone: "UTC",
			visits: []time.Time{
				time.Date(2026, 9, 20, 9, 0, 0, 0, time.UTC),
				time.Date(2026, 9, 20, 9, 0, 30, 0, time.UTC),
				time.Date(2026, 9, 20, 18, 0, 0, 0, time.UTC),
			},
			wantTouches: 1,
		},
		{
			// The case a rolling TTL gets wrong: the second visit is three
			// minutes after the first but on the next day. Miss it and the
			// day is never recorded, and the streak breaks the day after.
			name:     "a visit minutes later but past local midnight is a new day",
			timezone: "UTC",
			visits: []time.Time{
				time.Date(2026, 9, 20, 23, 58, 0, 0, time.UTC),
				time.Date(2026, 9, 21, 0, 1, 0, 0, time.UTC),
			},
			wantTouches: 2,
		},
		{
			// Midnight is the user's, not the server's: in Tokyo the day
			// turns nine hours before it does in UTC.
			name:     "the day turns where the user is",
			timezone: "Asia/Tokyo",
			visits: []time.Time{
				time.Date(2026, 9, 20, 14, 50, 0, 0, time.UTC),
				time.Date(2026, 9, 20, 15, 10, 0, 0, time.UTC),
			},
			wantTouches: 2,
		},
		{
			name:     "the same Tokyo day is one visit",
			timezone: "Asia/Tokyo",
			visits: []time.Time{
				time.Date(2026, 9, 20, 15, 10, 0, 0, time.UTC),
				time.Date(2026, 9, 20, 23, 0, 0, 0, time.UTC),
			},
			wantTouches: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := &stubUsers{
				settings: user.Settings{Timezone: tt.timezone},
				change:   user.StreakChange{Extended: true, Current: 1, Longest: 1},
			}
			clock := &movingClock{}
			service := profile.NewService(users, &stubEvents{}, &stubCache{}, &stubTx{}, awardsFor(users, clock), &stubRanking{}, clock)

			for _, at := range tt.visits {
				clock.at = at
				if _, err := service.TouchDay(context.Background(), userID); err != nil {
					t.Fatalf("TouchDay(%s): %v", at, err)
				}
			}

			if users.touches != tt.wantTouches {
				t.Errorf("the database was asked %d times, want %d", users.touches, tt.wantTouches)
			}
		})
	}
}

func TestTouchDayWithoutACache(t *testing.T) {
	users := &stubUsers{
		settings: user.Settings{Timezone: "UTC"},
		change:   user.StreakChange{Extended: true, Current: 3, Longest: 5},
	}
	events := &stubEvents{}
	clock := &movingClock{at: time.Date(2026, 9, 20, 9, 0, 0, 0, time.UTC)}
	service := profile.NewService(users, events, nil, &stubTx{}, awardsFor(users, clock), &stubRanking{}, clock)

	// No cache means every visit asks; the statement itself is what keeps the
	// day from being counted twice.
	for range 3 {
		if _, err := service.TouchDay(context.Background(), uuid.New()); err != nil {
			t.Fatalf("TouchDay: %v", err)
		}
	}
	if users.touches != 3 {
		t.Errorf("the database was asked %d times, want 3", users.touches)
	}
	if len(events.recorded) != 3 {
		t.Errorf("%d events, want one per extension the database reported", len(events.recorded))
	}
}

func TestTouchDayRecordsOnlyRealExtensions(t *testing.T) {
	tests := []struct {
		name       string
		change     user.StreakChange
		wantEvents int
	}{
		{
			name:       "an extension is worth recording",
			change:     user.StreakChange{Extended: true, Current: 2, Longest: 4},
			wantEvents: 1,
		},
		{
			name:   "a day already counted is not",
			change: user.StreakChange{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := &stubUsers{settings: user.Settings{Timezone: "UTC"}, change: tt.change}
			events := &stubEvents{}
			clock := &movingClock{at: time.Date(2026, 9, 20, 9, 0, 0, 0, time.UTC)}
			service := profile.NewService(users, events, &stubCache{}, &stubTx{}, awardsFor(users, clock), &stubRanking{}, clock)

			change, err := service.TouchDay(context.Background(), uuid.New())
			if err != nil {
				t.Fatalf("TouchDay: %v", err)
			}
			if change.Extended != tt.change.Extended {
				t.Errorf("Extended = %v, want %v", change.Extended, tt.change.Extended)
			}
			if len(events.recorded) != tt.wantEvents {
				t.Fatalf("%d events, want %d", len(events.recorded), tt.wantEvents)
			}
			if tt.wantEvents > 0 && events.recorded[0].Type != shared.EventStreakExtended {
				t.Errorf("event = %s", events.recorded[0].Type)
			}
		})
	}
}

func TestHeatmapIsAFullYearEndingToday(t *testing.T) {
	tests := []struct {
		name      string
		timezone  string
		now       time.Time
		wantLast  string
		wantFirst string
	}{
		{
			name:      "UTC",
			timezone:  "UTC",
			now:       time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC),
			wantFirst: "2025-09-21",
			wantLast:  "2026-09-20",
		},
		{
			// 22:00 UTC is already tomorrow in Tokyo, and the grid has to end
			// on the user's today, not the server's.
			name:      "east of UTC, late in the day",
			timezone:  "Asia/Tokyo",
			now:       time.Date(2026, 9, 20, 22, 0, 0, 0, time.UTC),
			wantFirst: "2025-09-22",
			wantLast:  "2026-09-21",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := &stubUsers{settings: user.Settings{Timezone: tt.timezone}}
			service := profile.NewService(users, &stubEvents{}, nil, &stubTx{}, awardsFor(users, &movingClock{at: tt.now}), &stubRanking{}, &movingClock{at: tt.now})

			days, err := service.Heatmap(context.Background(), uuid.New())
			if err != nil {
				t.Fatalf("Heatmap: %v", err)
			}
			if len(days) != profile.HeatmapDays {
				t.Fatalf("%d days, want %d", len(days), profile.HeatmapDays)
			}
			if got := days[0].Date.Format(time.DateOnly); got != tt.wantFirst {
				t.Errorf("first = %s, want %s", got, tt.wantFirst)
			}
			if got := days[len(days)-1].Date.Format(time.DateOnly); got != tt.wantLast {
				t.Errorf("last = %s, want %s", got, tt.wantLast)
			}
		})
	}
}

func awardsFor(users *stubUsers, clock shared.Clock) *appachievement.Service {
	return appachievement.NewService(&stubAwards{}, users, &stubEvents{}, clock)
}
