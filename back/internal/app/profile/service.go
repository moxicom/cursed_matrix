// Package profile holds what the account has done: its streak, its history and
// the figures the profile screen shows.
package profile

import (
	"context"
	"encoding/base64"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/app/achievement"
	"github.com/moxicom/cursed_matrix/back/internal/app/cache"
	"github.com/moxicom/cursed_matrix/back/internal/app/port"
	"github.com/moxicom/cursed_matrix/back/internal/app/preferences"
	"github.com/moxicom/cursed_matrix/back/internal/domain/activity"
	"github.com/moxicom/cursed_matrix/back/internal/domain/leaderboard"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/user"
)

// dayGateTTL outlives any single calendar day, because the key names the day
// it belongs to and so cannot be mistaken for another one.
const dayGateTTL = 36 * time.Hour

type Service struct {
	users   port.UserRepository
	events  port.ActivityRepository
	cache   port.Cache
	tx      port.TxManager
	awards  *achievement.Service
	ranking port.LeaderboardRepository
	clock   shared.Clock
	prefs   *preferences.Reader
}

func NewService(
	users port.UserRepository,
	events port.ActivityRepository,
	cached port.Cache,
	tx port.TxManager,
	awards *achievement.Service,
	ranking port.LeaderboardRepository,
	clock shared.Clock,
) *Service {
	return &Service{
		users:   users,
		events:  events,
		cache:   cached,
		tx:      tx,
		awards:  awards,
		ranking: ranking,
		clock:   clock,
		prefs:   preferences.NewReader(users, cached),
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
		if err := s.events.Record(ctx, event.
			With("currentStreak", counted.Current).
			With("longestStreak", counted.Longest)); err != nil {
			return err
		}
		_, err = s.awards.Evaluate(ctx, userID)
		return err
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
	return s.tx.Do(ctx, func(ctx context.Context) error {
		// Once per day, whatever the client does: the achievement counts the
		// days a user went looking, not the times they clicked.
		if err := s.events.RecordDaily(ctx, event); err != nil {
			return err
		}
		_, err := s.awards.Evaluate(ctx, userID)
		return err
	})
}

// Ranking is one page of the leaderboard with the asking user's own standing.
type Ranking struct {
	Entries  []leaderboard.Entry
	Me       leaderboard.Standing
	NextPage int
}

// Leaderboard returns the public ranking for a period.
//
// The asking user's standing comes with it even when they are off the page, so
// the screen can show where they are without fetching every page to find out.
func (s *Service) Leaderboard(
	ctx context.Context,
	userID uuid.UUID,
	period leaderboard.Period,
	limit, offset int,
) (*Ranking, error) {
	if !period.Valid() {
		return nil, shared.NewError(shared.CodeValidationFailed,
			map[string]any{"field": "period", "value": string(period)})
	}
	if limit <= 0 || limit > leaderboard.MaxPage {
		limit = leaderboard.MaxPage
	}
	if offset < 0 {
		offset = 0
	}

	var from *time.Time
	if start, bounded := period.Window(s.clock.Now().UTC()); bounded {
		from = &start
	}

	// One row past the page, so the client learns there is more without a
	// second query counting everyone.
	entries, err := s.ranking.Page(ctx, from, limit+1, offset)
	if err != nil {
		return nil, err
	}

	ranking := &Ranking{NextPage: -1}
	if len(entries) > limit {
		entries = entries[:limit]
		ranking.NextPage = offset + limit
	}
	ranking.Entries = entries

	standing, err := s.ranking.Standing(ctx, userID, from)
	if err != nil {
		return nil, err
	}
	ranking.Me = standing
	return ranking, nil
}

// EventPage is a page of the history with the place to continue from.
const (
	eventsPerPage = 30
	maxPerPage    = 100
)

type EventPage struct {
	Entries    []activity.Entry
	NextCursor string
}

// Events lists what the user did, newest first.
func (s *Service) Events(ctx context.Context, userID uuid.UUID, cursor string, limit int) (*EventPage, error) {
	if limit <= 0 {
		limit = eventsPerPage
	}
	if limit > maxPerPage {
		limit = maxPerPage
	}

	after, err := decodeCursor(cursor)
	if err != nil {
		return nil, err
	}

	// One past the page, so the client learns there is more without a second
	// query counting the whole history.
	entries, err := s.events.Events(ctx, userID, after, limit+1)
	if err != nil {
		return nil, err
	}

	page := &EventPage{Entries: entries}
	if len(entries) > limit {
		page.Entries = entries[:limit]
		last := page.Entries[len(page.Entries)-1]
		page.NextCursor = encodeCursor(activity.Cursor{OccurredAt: last.OccurredAt, ID: last.ID})
	}
	return page, nil
}

// Stats are the figures above the heatmap, over the same year it covers.
func (s *Service) Stats(ctx context.Context, userID uuid.UUID) (activity.Stats, error) {
	settings, err := s.prefs.Settings(ctx, userID)
	if err != nil {
		return activity.Stats{}, err
	}

	today := settings.LocalDate(s.clock.Now().UTC())
	from := today.AddDate(0, 0, -(HeatmapDays - 1))
	return s.events.Stats(ctx, userID, from)
}

// encodeCursor hides the ordering from the client, so the page can be keyed on
// something else later without the client having built anything on its shape.
func encodeCursor(cursor activity.Cursor) string {
	raw := cursor.OccurredAt.UTC().Format(time.RFC3339Nano) + " " + cursor.ID.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(value string) (*activity.Cursor, error) {
	if value == "" {
		return nil, nil
	}

	invalid := shared.NewError(shared.CodeValidationFailed,
		map[string]any{"field": "cursor"})

	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, invalid
	}
	at, id, found := strings.Cut(string(raw), " ")
	if !found {
		return nil, invalid
	}

	occurredAt, err := time.Parse(time.RFC3339Nano, at)
	if err != nil {
		return nil, invalid
	}
	parsed, err := uuid.Parse(id)
	if err != nil {
		return nil, invalid
	}
	return &activity.Cursor{OccurredAt: occurredAt, ID: parsed}, nil
}
