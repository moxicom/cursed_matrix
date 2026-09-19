package board_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/app/board"
	"github.com/moxicom/cursed_matrix/back/internal/app/cache"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/task"
	"github.com/moxicom/cursed_matrix/back/internal/domain/user"
)

type stubTasks struct {
	seen  task.Filter
	calls int
	tasks []task.Task
	err   error
}

func (s *stubTasks) ListBoard(_ context.Context, filter task.Filter) ([]task.Task, error) {
	s.seen = filter
	s.calls++
	return s.tasks, s.err
}

type stubUsers struct {
	account *user.User
	err     error
	calls   int
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

			service := board.NewService(tasks, users, nil, &fixedClock{at: now})
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
				service = board.NewService(tasks, users, cached, &fixedClock{at: now})
			} else {
				service = board.NewService(tasks, users, nil, &fixedClock{at: now})
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
			service := board.NewService(tasks, users, nil, &fixedClock{at: now})

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
