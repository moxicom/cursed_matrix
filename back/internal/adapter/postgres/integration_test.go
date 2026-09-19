//go:build integration

package postgres_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/adapter/postgres"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/task"
)

var pool *pgxpool.Pool

func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		os.Exit(0)
	}

	ctx := context.Background()

	var err error
	pool, err = postgres.NewPool(ctx, postgres.Options{
		URL:               dsn,
		MaxConns:          4,
		MinConns:          1,
		MaxConnLifetime:   time.Hour,
		MaxConnIdleTime:   15 * time.Minute,
		HealthCheckPeriod: 30 * time.Second,
	}, testLogger())
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	os.Exit(m.Run())
}

func seed(t *testing.T) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	userID := uuid.New()
	suffix := userID.String()[:8]

	_, err := pool.Exec(ctx,
		`INSERT INTO users (id, username, email, password_hash) VALUES ($1, $2, $3, 'x')`,
		userID, "user_"+suffix, "user_"+suffix+"@example.test")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO user_settings (user_id, timezone) VALUES ($1, 'Europe/Moscow')`, userID); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO user_stats (user_id) VALUES ($1)`, userID); err != nil {
		t.Fatalf("seed stats: %v", err)
	}

	critical := uuid.New()
	planned := uuid.New()
	done := uuid.New()
	subtask := uuid.New()

	rows := []struct {
		id       uuid.UUID
		title    string
		quadrant any
		parent   any
		position int
		color    string
	}{
		{critical, "Patch auth token refresh", "IMPORTANT_URGENT", nil, 1024, "ROSE"},
		{planned, "Write the quarterly plan", "IMPORTANT_NOT_URGENT", nil, 1024, "NONE"},
		{subtask, "Reproduce on staging", nil, critical, 1024, "ROSE"},
	}
	for _, row := range rows {
		_, err := pool.Exec(ctx,
			`INSERT INTO tasks (id, user_id, parent_task_id, title, quadrant, position, color, deadline_at)
			 VALUES ($1, $2, $3, $4, $5::quadrant_enum, $6, $7, $8)`,
			row.id, userID, row.parent, row.title, row.quadrant, row.position, row.color,
			time.Now().Add(6*time.Hour))
		if err != nil {
			t.Fatalf("seed task %q: %v", row.title, err)
		}
	}

	_, err = pool.Exec(ctx,
		`INSERT INTO tasks (id, user_id, title, quadrant, position, status, completed_at,
		                    xp_awarded, quadrant_at_completion, completed_via)
		 VALUES ($1, $2, 'Ship the hotfix', 'IMPORTANT_URGENT', 2048, 'COMPLETED', now(),
		         50, 'IMPORTANT_URGENT', 'DIRECT')`, done, userID)
	if err != nil {
		t.Fatalf("seed completed task: %v", err)
	}

	tagID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO tags (id, user_id, name) VALUES ($1, $2, 'backend')`, tagID, userID); err != nil {
		t.Fatalf("seed tag: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO task_tags (task_id, tag_id, user_id) VALUES ($1, $2, $3)`,
		critical, tagID, userID); err != nil {
		t.Fatalf("seed task_tag: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO task_links (user_id, source_task_id, target_task_id, type)
		 VALUES ($1, $2, $3, 'RELATED')`, userID, critical, planned); err != nil {
		t.Fatalf("seed link: %v", err)
	}

	return userID
}

func searchFor(term string) func(task.Filter) task.Filter {
	return func(f task.Filter) task.Filter {
		f.Query = term
		return f
	}
}

func TestListBoard(t *testing.T) {
	ctx := context.Background()
	repo := postgres.NewTaskRepository(pool)
	userID := seed(t)

	tests := []struct {
		name      string
		mutate    func(task.Filter) task.Filter
		wantCount int
		check     func(t *testing.T, tasks []task.Task)
	}{
		{
			name:      "active tasks, subtasks included",
			wantCount: 3,
			check: func(t *testing.T, tasks []task.Task) {
				var tagged, subtasks int
				for _, item := range tasks {
					if len(item.Tags) > 0 {
						tagged++
						if item.Tags[0] != "backend" {
							t.Errorf("tag = %q, want backend", item.Tags[0])
						}
					}
					if item.ParentID != nil {
						subtasks++
						if item.Quadrant != nil {
							t.Error("a subtask came back carrying a quadrant")
						}
					}
				}
				if tagged != 1 {
					t.Errorf("%d tasks carry a tag, want 1", tagged)
				}
				if subtasks != 1 {
					t.Errorf("%d subtasks, want 1", subtasks)
				}
			},
		},
		{
			name: "completed tasks keep their snapshot",
			mutate: func(f task.Filter) task.Filter {
				f.Status = task.StatusFilterCompleted
				return f
			},
			wantCount: 1,
			check: func(t *testing.T, tasks []task.Task) {
				item := tasks[0]
				if item.XPAwarded == nil || *item.XPAwarded != 50 {
					t.Error("the XP snapshot did not survive the round trip")
				}
				if item.QuadrantAtCompletion == nil ||
					*item.QuadrantAtCompletion != shared.QuadrantImportantUrgent {
					t.Error("the quadrant snapshot did not survive the round trip")
				}
			},
		},
		{
			name: "every status returns both",
			mutate: func(f task.Filter) task.Filter {
				f.Status = task.StatusFilterAll
				return f
			},
			wantCount: 4,
		},
		{
			name: "quadrant filter includes that quadrant's subtasks",
			mutate: func(f task.Filter) task.Filter {
				f.Quadrants = []shared.Quadrant{shared.QuadrantImportantUrgent}
				return f
			},
			wantCount: 2,
		},
		{
			name: "tag filter is case-insensitive",
			mutate: func(f task.Filter) task.Filter {
				f.Tags = []string{"BACKEND"}
				return f
			},
			wantCount: 1,
		},
		{
			name: "colour filter",
			mutate: func(f task.Filter) task.Filter {
				f.Colors = []shared.TaskColor{shared.ColorRose}
				return f
			},
			wantCount: 2,
		},
		{
			name: "linked only",
			mutate: func(f task.Filter) task.Filter {
				f.Topology = task.TopologyLinked
				return f
			},
			wantCount: 2,
		},
		{
			name: "unlinked only",
			mutate: func(f task.Filter) task.Filter {
				f.Topology = task.TopologyUnlinked
				return f
			},
			wantCount: 1,
		},
		{
			name:      "search matches a title",
			mutate:    searchFor("auth"),
			wantCount: 1,
		},
		{
			name:      "search matches a tag name",
			mutate:    searchFor("backend"),
			wantCount: 1,
		},
		{name: "hostile term 1 matches nothing", mutate: searchFor(`' OR '1'='1`), wantCount: 0},
		{name: "hostile term 2 matches nothing", mutate: searchFor(`' UNION SELECT password_hash FROM users --`), wantCount: 0},
		{name: "hostile term 3 matches nothing", mutate: searchFor(`%`), wantCount: 0},
		{name: "hostile term 4 matches nothing", mutate: searchFor(`_`), wantCount: 0},
		{
			name: "another user sees nothing",
			mutate: func(f task.Filter) task.Filter {
				f.UserID = uuid.New()
				return f
			},
			wantCount: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			filter := task.DefaultFilter(userID)
			if tc.mutate != nil {
				filter = tc.mutate(filter)
			}

			tasks, err := repo.ListBoard(ctx, filter)
			if err != nil {
				t.Fatalf("ListBoard: %v", err)
			}
			if len(tasks) != tc.wantCount {
				t.Fatalf("got %d tasks, want %d", len(tasks), tc.wantCount)
			}
			if tc.check != nil {
				tc.check(t, tasks)
			}
		})
	}
}

func TestHostileSearchLeavesTheRowsAlone(t *testing.T) {
	ctx := context.Background()
	repo := postgres.NewTaskRepository(pool)
	userID := seed(t)
	before := countTasks(t, userID)

	for _, term := range []string{`' OR '1'='1`, `' UNION SELECT password_hash FROM users --`, `%`, `_`} {
		filter := task.DefaultFilter(userID)
		filter.Query = term
		if _, err := repo.ListBoard(ctx, filter); err != nil {
			t.Fatalf("ListBoard(%q): %v", term, err)
		}
	}

	if after := countTasks(t, userID); after != before {
		t.Fatalf("the task count changed from %d to %d", before, after)
	}
}

func TestUserByID(t *testing.T) {
	ctx := context.Background()
	repo := postgres.NewUserRepository(pool)
	userID := seed(t)

	found, err := repo.ByID(ctx, userID)
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if found.Email == nil {
		t.Error("the seeded account has an address; it did not survive the round trip")
	}

	tests := []struct {
		name string
		got  any
		want any
	}{
		{name: "id", got: found.ID, want: userID},
		{name: "timezone", got: found.Settings.Timezone, want: "Europe/Moscow"},
		{name: "language defaults to EN", got: found.Settings.Language, want: shared.LanguageEN},
		{name: "leaderboard visibility defaults to off", got: found.Settings.ShowInLeaderboard, want: false},
		{name: "level starts at one", got: found.Stats.Level, want: int32(1)},
		{name: "lifetime xp starts at zero", got: found.Stats.LifetimeXP, want: int64(0)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("%s = %v, want %v", tc.name, tc.got, tc.want)
			}
		})
	}

	t.Run("an unknown account is not found", func(t *testing.T) {
		if _, err := repo.ByID(ctx, uuid.New()); err == nil {
			t.Error("ByID found an account that does not exist")
		} else if shared.CodeOf(err) != shared.CodeUserNotFound {
			t.Errorf("error code = %q, want USER_NOT_FOUND", shared.CodeOf(err))
		}
	})
}

func countTasks(t *testing.T, userID uuid.UUID) int {
	t.Helper()
	var count int
	err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM tasks WHERE user_id = $1`, userID).Scan(&count)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	return count
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
