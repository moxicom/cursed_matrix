package board_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	appachievement "github.com/moxicom/cursed_matrix/back/internal/app/achievement"
	"github.com/moxicom/cursed_matrix/back/internal/app/board"
	"github.com/moxicom/cursed_matrix/back/internal/app/cache"
	"github.com/moxicom/cursed_matrix/back/internal/domain/achievement"
	"github.com/moxicom/cursed_matrix/back/internal/domain/activity"
	"github.com/moxicom/cursed_matrix/back/internal/domain/link"
	"github.com/moxicom/cursed_matrix/back/internal/domain/progression"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/tag"
	"github.com/moxicom/cursed_matrix/back/internal/domain/task"
	"github.com/moxicom/cursed_matrix/back/internal/domain/user"
)

type stubTasks struct {
	seen         task.Filter
	calls        int
	tasks        []task.Task
	err          error
	created      []task.Task
	updated      []task.Task
	deleted      []uuid.UUID
	repositioned []task.Placement
	locked       []uuid.UUID
	nextPosition int32
	active       int
	createErr    error
	countErr     error
}

func (s *stubTasks) ListBoard(_ context.Context, filter task.Filter) ([]task.Task, error) {
	s.seen = filter
	s.calls++
	return s.tasks, s.err
}

func (s *stubTasks) ByID(_ context.Context, _, taskID uuid.UUID) (*task.Task, error) {
	for i := range s.tasks {
		if s.tasks[i].ID == taskID {
			found := s.tasks[i]
			return &found, nil
		}
	}
	return nil, shared.NewError(shared.CodeTaskNotFound, nil)
}

// ByIDForUpdate is the locking read; the stub has no concurrency to guard, so
// it answers exactly as the plain one does.
func (s *stubTasks) ByIDForUpdate(ctx context.Context, userID, taskID uuid.UUID) (*task.Task, error) {
	s.locked = append(s.locked, taskID)
	return s.ByID(ctx, userID, taskID)
}

func (s *stubTasks) Create(_ context.Context, item *task.Task) error {
	if s.createErr != nil {
		return s.createErr
	}
	item.ID = uuid.New()
	s.created = append(s.created, *item)
	return nil
}

func (s *stubTasks) Update(_ context.Context, item *task.Task) error {
	s.updated = append(s.updated, *item)
	return nil
}

func (s *stubTasks) SoftDelete(_ context.Context, _ uuid.UUID, taskIDs []uuid.UUID, _ time.Time) error {
	s.deleted = append(s.deleted, taskIDs...)
	return nil
}

func (s *stubTasks) ScopeTasks(_ context.Context, _ uuid.UUID, quadrant shared.Quadrant) ([]task.Task, error) {
	var scope []task.Task
	for i := range s.tasks {
		item := s.tasks[i]
		if item.ParentID == nil && item.Quadrant != nil && *item.Quadrant == quadrant {
			scope = append(scope, item)
		}
	}
	return scope, nil
}

func (s *stubTasks) Reposition(_ context.Context, _ uuid.UUID, placements []task.Placement) error {
	s.repositioned = append(s.repositioned, placements...)
	return nil
}

func (s *stubTasks) NextPosition(context.Context, uuid.UUID, *shared.Quadrant, *uuid.UUID) (int32, error) {
	return s.nextPosition, nil
}

func (s *stubTasks) CountActive(context.Context, uuid.UUID) (int, error) {
	return s.active, s.countErr
}

func (s *stubTasks) Search(context.Context, uuid.UUID, string, int) ([]task.Hit, error) {
	return nil, nil
}

func (s *stubTasks) All(context.Context, uuid.UUID) ([]task.Task, error) { return s.tasks, nil }

func (s *stubTasks) ListGraph(_ context.Context, filter task.Filter) ([]task.Node, error) {
	nodes := make([]task.Node, 0, len(s.tasks))
	for i := range s.tasks {
		nodes = append(nodes, task.Node{
			ID:        s.tasks[i].ID,
			Title:     s.tasks[i].Title,
			Status:    s.tasks[i].Status,
			ParentID:  s.tasks[i].ParentID,
			IsSubtask: s.tasks[i].ParentID != nil,
		})
	}
	return nodes, nil
}

func (s *stubTasks) Subtasks(_ context.Context, _, parentID uuid.UUID) ([]task.Task, error) {
	var children []task.Task
	for i := range s.tasks {
		if s.tasks[i].ParentID != nil && *s.tasks[i].ParentID == parentID {
			children = append(children, s.tasks[i])
		}
	}
	return children, nil
}

type stubLedger struct {
	entries []progression.Entry
	grants  map[uuid.UUID]int
	err     error
}

func (l *stubLedger) Record(_ context.Context, entry *progression.Entry) error {
	if l.err != nil {
		return l.err
	}
	l.entries = append(l.entries, *entry)
	return nil
}

func (l *stubLedger) GrantCount(_ context.Context, taskID uuid.UUID, _ shared.XPSource) (int, error) {
	return l.grants[taskID], nil
}

type stubTags struct {
	attached []string
	detached int
	pruned   int
}

func (t *stubTags) List(context.Context, uuid.UUID) ([]tag.Tag, error) { return nil, nil }

func (t *stubTags) Upsert(_ context.Context, userID uuid.UUID, name string) (*tag.Tag, error) {
	t.attached = append(t.attached, name)
	return &tag.Tag{ID: uuid.New(), UserID: userID, Name: name}, nil
}

func (t *stubTags) Attach(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error { return nil }

func (t *stubTags) Detach(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	t.detached++
	return nil
}

func (t *stubTags) DeleteOrphans(context.Context, uuid.UUID) error {
	t.pruned++
	return nil
}

type stubEvents struct {
	recorded []activity.Event
}

func (e *stubEvents) Record(_ context.Context, event *activity.Event) error {
	e.recorded = append(e.recorded, *event)
	return nil
}

func (e *stubEvents) RecordMany(_ context.Context, events []activity.Event) error {
	e.recorded = append(e.recorded, events...)
	return nil
}

func (e *stubEvents) RecordDaily(_ context.Context, event *activity.Event) error {
	for _, already := range e.recorded {
		if already.Type == event.Type {
			return nil
		}
	}
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

type stubLinks struct {
	links    []link.Link
	unlinked []uuid.UUID
}

func (l *stubLinks) Create(_ context.Context, item *link.Link) error {
	l.links = append(l.links, *item)
	return nil
}

func (l *stubLinks) ByID(_ context.Context, _, linkID uuid.UUID) (*link.Link, error) {
	for i := range l.links {
		if l.links[i].ID == linkID {
			found := l.links[i]
			return &found, nil
		}
	}
	return nil, shared.NewError(shared.CodeTaskNotFound, nil)
}

func (l *stubLinks) List(context.Context, uuid.UUID) ([]link.Link, error) { return l.links, nil }

func (l *stubLinks) Update(context.Context, *link.Link) error { return nil }

func (l *stubLinks) Remove(context.Context, uuid.UUID, uuid.UUID) error { return nil }

func (l *stubLinks) Count(context.Context, uuid.UUID) (int, error) { return len(l.links), nil }

func (l *stubLinks) RemoveForTasks(_ context.Context, _ uuid.UUID, taskIDs []uuid.UUID) error {
	l.unlinked = append(l.unlinked, taskIDs...)
	return nil
}

// stubAwards is an empty catalogue: these tests are about the work, not about
// what it earns, and an empty catalogue unlocks nothing.
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

type stubTx struct{}

func (*stubTx) Do(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

type stubUsers struct {
	account      *user.User
	err          error
	calls        int
	stats        user.Stats
	applied      []user.StatsDelta
	accountLocks int
	streak       user.StreakChange
}

func (s *stubUsers) ByID(context.Context, uuid.UUID) (*user.User, error) {
	s.calls++
	return s.account, s.err
}

func (s *stubUsers) ByUsername(context.Context, string) (*user.User, error) {
	return s.account, s.err
}
func (s *stubUsers) Create(context.Context, *user.User) error { return nil }
func (s *stubUsers) UpdateSettings(context.Context, uuid.UUID, user.Settings) error {
	return nil
}
func (s *stubUsers) TouchLogin(context.Context, uuid.UUID, time.Time) error { return nil }

func (s *stubUsers) ApplyStats(_ context.Context, _ uuid.UUID, delta user.StatsDelta) (user.Stats, error) {
	s.stats.LifetimeXP += delta.XP
	s.stats.TasksCompleted += delta.TasksCompleted
	s.stats.SubtasksCompleted += delta.SubtasksCompleted
	s.applied = append(s.applied, delta)
	return s.stats, nil
}

func (s *stubUsers) SetLevel(_ context.Context, _ uuid.UUID, level int32) error {
	s.stats.Level = level
	return nil
}

func (s *stubUsers) LockAccount(context.Context, uuid.UUID) error {
	s.accountLocks++
	return nil
}
func (*stubUsers) SoftDelete(context.Context, uuid.UUID, time.Time) error { return nil }

func (*stubUsers) SetSubscription(context.Context, uuid.UUID, user.Subscription) error {
	return nil
}

func (s *stubUsers) TouchStreak(context.Context, uuid.UUID, time.Time) (user.StreakChange, error) {
	return s.streak, nil
}

type stubCache struct {
	stored map[string]user.Settings
	writes int
	getErr error
}

func (c *stubCache) GetInto(_ context.Context, key cache.Key, dst any) (bool, error) {
	if c.getErr != nil {
		return false, c.getErr
	}
	settings, ok := c.stored[string(key.Scope)+"/"+key.Name]
	if !ok {
		return false, nil
	}
	*dst.(*user.Settings) = settings
	return true, nil
}

func (c *stubCache) Set(_ context.Context, key cache.Key, value any, _ time.Duration) error {
	if c.stored == nil {
		c.stored = map[string]user.Settings{}
	}
	c.stored[string(key.Scope)+"/"+key.Name] = value.(user.Settings)
	c.writes++
	return nil
}

func (c *stubCache) Invalidate(context.Context, cache.Scope) error {
	c.stored = nil
	return nil
}

type fixedClock struct{ at time.Time }

func (c *fixedClock) Now() time.Time { return c.at }

func TestServiceList(t *testing.T) {
	userID := uuid.New()
	now := time.Date(2026, 9, 19, 21, 30, 0, 0, time.UTC)
	completed := task.StatusFilterCompleted
	bogus := task.StatusFilter("NOPE")

	account := func(timezone string) *user.User {
		return &user.User{ID: userID, Settings: user.Settings{Timezone: timezone}}
	}

	tests := []struct {
		name     string
		timezone string
		query    board.Query
		users    *stubUsers
		tasks    *stubTasks
		wantErr  shared.ErrorCode
		assert   func(t *testing.T, filter task.Filter)
	}{
		{
			name:     "an empty query asks for the active board in user order",
			timezone: "UTC",
			query:    board.Query{},
			assert: func(t *testing.T, filter task.Filter) {
				if filter.Status != task.StatusFilterActive {
					t.Errorf("status = %v", filter.Status)
				}
				if filter.Sort != task.SortByPosition || filter.Direction != task.SortAsc {
					t.Errorf("order = %v %v", filter.Sort, filter.Direction)
				}
				if filter.UserID != userID {
					t.Errorf("userId = %v", filter.UserID)
				}
			},
		},
		{
			name:     "the deadline window is anchored in the user's timezone",
			timezone: "Asia/Tokyo",
			query:    board.Query{},
			assert: func(t *testing.T, filter task.Filter) {
				if filter.Location.String() != "Asia/Tokyo" {
					t.Fatalf("location = %v", filter.Location)
				}
				// 21:30 UTC is already the next calendar day in Tokyo, which is
				// exactly the case a UTC-only query would get wrong.
				if day := filter.Now.In(filter.Location).Day(); day != 20 {
					t.Errorf("local day = %d, want 20", day)
				}
			},
		},
		{
			name:     "an unknown timezone falls back to UTC rather than failing",
			timezone: "Mars/Olympus",
			query:    board.Query{},
			assert: func(t *testing.T, filter task.Filter) {
				if filter.Location != time.UTC {
					t.Errorf("location = %v", filter.Location)
				}
			},
		},
		{
			name:     "overrides reach the repository",
			timezone: "UTC",
			query: board.Query{
				Status:    &completed,
				Tags:      []string{"work"},
				Quadrants: []shared.Quadrant{shared.QuadrantImportantUrgent},
				Search:    "matrix",
			},
			assert: func(t *testing.T, filter task.Filter) {
				if filter.Status != task.StatusFilterCompleted || filter.Query != "matrix" {
					t.Errorf("filter = %+v", filter)
				}
				if len(filter.Tags) != 1 || len(filter.Quadrants) != 1 {
					t.Errorf("lists = %v %v", filter.Tags, filter.Quadrants)
				}
			},
		},
		{
			name:     "an invalid enum is refused before the query is issued",
			timezone: "UTC",
			query:    board.Query{Status: &bogus},
			wantErr:  shared.CodeValidationFailed,
		},
		{
			name:     "a missing account is not reported as an empty board",
			timezone: "UTC",
			users:    &stubUsers{err: shared.NewError(shared.CodeUserNotFound, nil)},
			wantErr:  shared.CodeUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := tt.users
			if users == nil {
				users = &stubUsers{account: account(tt.timezone)}
			}
			tasks := tt.tasks
			if tasks == nil {
				tasks = &stubTasks{}
			}

			service := board.NewService(board.Deps{
				Tasks: tasks, Users: users, Cache: nil, Tags: &stubTags{}, Links: &stubLinks{},
				Tx: &stubTx{}, Ledger: &stubLedger{}, Events: &stubEvents{},
				Awards: appachievement.NewService(&stubAwards{}, users, &stubEvents{}, &fixedClock{at: now}),
				XP:     progression.DefaultConfig(), Clock: &fixedClock{at: now},
			})
			_, err := service.List(context.Background(), userID, tt.query)

			if tt.wantErr != "" {
				var domainErr *shared.Error
				if !errors.As(err, &domainErr) || domainErr.Code != tt.wantErr {
					t.Fatalf("err = %v, want %s", err, tt.wantErr)
				}
				if tasks.calls != 0 {
					t.Errorf("the repository was queried %d times despite the refusal", tasks.calls)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.assert(t, tasks.seen)
		})
	}
}

func TestServiceCachesThePreferences(t *testing.T) {
	userID := uuid.New()
	now := time.Date(2026, 9, 19, 21, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		cache     *stubCache
		warm      bool
		wantReads int
		wantZone  string
	}{
		{
			name:      "no cache reads the account every time",
			wantReads: 2,
			wantZone:  "Asia/Tokyo",
		},
		{
			name:      "a warm cache spares the database",
			cache:     &stubCache{},
			warm:      true,
			wantReads: 1,
			wantZone:  "Asia/Tokyo",
		},
		{
			name:      "a cache that errors falls back instead of failing",
			cache:     &stubCache{getErr: errors.New("redis is down")},
			wantReads: 2,
			wantZone:  "Asia/Tokyo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := &stubUsers{account: &user.User{
				ID:       userID,
				Settings: user.Settings{Timezone: "Asia/Tokyo"},
			}}
			tasks := &stubTasks{}

			var cached *stubCache
			var service *board.Service
			if tt.cache != nil {
				cached = tt.cache
				service = board.NewService(board.Deps{
					Tasks: tasks, Users: users, Cache: cached, Tags: &stubTags{}, Links: &stubLinks{},
					Tx: &stubTx{}, Ledger: &stubLedger{}, Events: &stubEvents{},
					Awards: appachievement.NewService(&stubAwards{}, users, &stubEvents{}, &fixedClock{at: now}),
					XP:     progression.DefaultConfig(), Clock: &fixedClock{at: now},
				})
			} else {
				service = board.NewService(board.Deps{
					Tasks: tasks, Users: users, Cache: nil, Tags: &stubTags{}, Links: &stubLinks{},
					Tx: &stubTx{}, Ledger: &stubLedger{}, Events: &stubEvents{},
					Awards: appachievement.NewService(&stubAwards{}, users, &stubEvents{}, &fixedClock{at: now}),
					XP:     progression.DefaultConfig(), Clock: &fixedClock{at: now},
				})
			}

			for range 2 {
				if _, err := service.List(context.Background(), userID, board.Query{}); err != nil {
					t.Fatalf("list: %v", err)
				}
			}

			if users.calls != tt.wantReads {
				t.Errorf("account reads = %d, want %d", users.calls, tt.wantReads)
			}
			if tasks.seen.Location.String() != tt.wantZone {
				t.Errorf("location = %v, want %s", tasks.seen.Location, tt.wantZone)
			}
			if tt.warm && cached.writes != 1 {
				t.Errorf("cache writes = %d, want the miss to warm it exactly once", cached.writes)
			}
		})
	}
}

func TestServiceReportsTruncation(t *testing.T) {
	userID := uuid.New()
	now := time.Date(2026, 9, 19, 21, 30, 0, 0, time.UTC)

	rows := func(n int) []task.Task {
		out := make([]task.Task, n)
		for i := range out {
			out[i] = task.Task{ID: uuid.New()}
		}
		return out
	}

	tests := []struct {
		name          string
		returned      int
		wantLen       int
		wantTruncated bool
	}{
		{name: "a short answer is whole", returned: 3, wantLen: 3},
		{name: "exactly the cap is still whole", returned: task.MaxBoardPage, wantLen: task.MaxBoardPage},
		{
			name:          "one row past the cap means there is more",
			returned:      task.MaxBoardPage + 1,
			wantLen:       task.MaxBoardPage,
			wantTruncated: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks := &stubTasks{tasks: rows(tt.returned)}
			users := &stubUsers{account: &user.User{ID: userID}}
			service := board.NewService(board.Deps{
				Tasks: tasks, Users: users, Cache: nil, Tags: &stubTags{}, Links: &stubLinks{},
				Tx: &stubTx{}, Ledger: &stubLedger{}, Events: &stubEvents{},
				Awards: appachievement.NewService(&stubAwards{}, users, &stubEvents{}, &fixedClock{at: now}),
				XP:     progression.DefaultConfig(), Clock: &fixedClock{at: now},
			})

			result, err := service.List(context.Background(), userID, board.Query{})
			if err != nil {
				t.Fatalf("list: %v", err)
			}

			if len(result.Tasks) != tt.wantLen {
				t.Errorf("tasks = %d, want %d", len(result.Tasks), tt.wantLen)
			}
			if result.Truncated != tt.wantTruncated {
				t.Errorf("truncated = %v, want %v", result.Truncated, tt.wantTruncated)
			}
			// The probe row is asked for, never returned.
			if tasks.seen.Limit != task.MaxBoardPage {
				t.Errorf("limit = %d, want %d", tasks.seen.Limit, task.MaxBoardPage)
			}
		})
	}
}
