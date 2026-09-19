package board_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/app/board"
	"github.com/moxicom/cursed_matrix/back/internal/domain/progression"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/task"
	"github.com/moxicom/cursed_matrix/back/internal/domain/user"
)

func newWriteService(tasks *stubTasks) *board.Service {
	users := &stubUsers{account: &user.User{Settings: user.Settings{Timezone: "UTC"}}}
	return board.NewService(board.Deps{
		Tasks: tasks, Users: users, Tags: &stubTags{}, Links: &stubLinks{},
		Tx: &stubTx{}, Ledger: &stubLedger{}, Events: &stubEvents{},
		XP:    progression.DefaultConfig(),
		Clock: &fixedClock{at: time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)},
	})
}

func TestCreateRefusesPastTheQuota(t *testing.T) {
	tests := []struct {
		name    string
		active  int
		wantErr shared.ErrorCode
	}{
		{name: "an empty board has room", active: 0},
		{name: "one below the cap still has room", active: user.FreeActiveTasks - 1},
		{name: "at the cap there is none", active: user.FreeActiveTasks, wantErr: shared.CodeQuotaLimitReached},
		{
			name:    "past the cap, however it got there, there is none",
			active:  user.FreeActiveTasks + 10,
			wantErr: shared.CodeQuotaLimitReached,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks := &stubTasks{active: tt.active}
			service := newWriteService(tasks)

			_, err := service.Create(context.Background(), uuid.New(), board.Draft{
				Title:    "a task",
				Quadrant: shared.QuadrantImportantUrgent,
			})

			if tt.wantErr != "" {
				if shared.CodeOf(err) != tt.wantErr {
					t.Fatalf("err = %v, want %s", err, tt.wantErr)
				}
				// Nothing may be written once the quota refuses the request.
				if len(tasks.created) != 0 {
					t.Errorf("%d tasks were created past the quota", len(tasks.created))
				}
				return
			}
			if err != nil {
				t.Fatalf("Create: %v", err)
			}
			if len(tasks.created) != 1 {
				t.Fatalf("created %d tasks, want 1", len(tasks.created))
			}
		})
	}
}

func TestCreateDraft(t *testing.T) {
	userID := uuid.New()
	deadline := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		draft   board.Draft
		wantErr shared.ErrorCode
		assert  func(t *testing.T, created task.Task)
	}{
		{
			name:  "a plain task lands at the end of its quadrant",
			draft: board.Draft{Title: "  a task  ", Quadrant: shared.QuadrantImportantUrgent},
			assert: func(t *testing.T, created task.Task) {
				if created.Title != "a task" {
					t.Errorf("title = %q, want it trimmed", created.Title)
				}
				// First in an empty quadrant: one gap in.
				if created.Position != task.PositionGap {
					t.Errorf("position = %d, want %d", created.Position, task.PositionGap)
				}
				if created.Color != shared.ColorNone {
					t.Errorf("color = %q, want the default", created.Color)
				}
				if created.ID == uuid.Nil {
					t.Error("the task was stored without an identity")
				}
			},
		},
		{
			name: "a deadline without a time is a date",
			draft: board.Draft{
				Title: "a task", Quadrant: shared.QuadrantImportantUrgent,
				DeadlineAt: &deadline,
			},
			assert: func(t *testing.T, created task.Task) {
				if created.DeadlineAt == nil || created.DeadlineHasTime {
					t.Errorf("deadline = %v, hasTime = %v", created.DeadlineAt, created.DeadlineHasTime)
				}
			},
		},
		{
			name: "hasTime without a deadline claims nothing",
			draft: board.Draft{
				Title: "a task", Quadrant: shared.QuadrantImportantUrgent,
				DeadlineHasTime: true,
			},
			assert: func(t *testing.T, created task.Task) {
				if created.DeadlineHasTime {
					t.Error("a task with no deadline says its deadline has a time")
				}
			},
		},
		{
			name:    "an empty title is refused",
			draft:   board.Draft{Title: "   ", Quadrant: shared.QuadrantImportantUrgent},
			wantErr: shared.CodeValidationFailed,
		},
		{
			name:    "an unknown quadrant is refused",
			draft:   board.Draft{Title: "a task", Quadrant: "SOMEWHAT_URGENT"},
			wantErr: shared.CodeValidationFailed,
		},
		{
			name: "an unknown colour is refused",
			draft: board.Draft{
				Title: "a task", Quadrant: shared.QuadrantImportantUrgent,
				Color: "BURNT_SIENNA",
			},
			wantErr: shared.CodeValidationFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks := &stubTasks{}
			service := newWriteService(tasks)

			created, err := service.Create(context.Background(), userID, tt.draft)

			if tt.wantErr != "" {
				if shared.CodeOf(err) != tt.wantErr {
					t.Fatalf("err = %v, want %s", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Create: %v", err)
			}
			tt.assert(t, *created)
		})
	}
}

// TestCreateAppendsAfterWhatIsThere is the ordering rule at creation: a new
// task goes to the end of its quadrant, not on top of what is already there.
func TestCreateAppendsAfterWhatIsThere(t *testing.T) {
	userID := uuid.New()
	quadrant := shared.QuadrantImportantUrgent

	tasks := &stubTasks{tasks: []task.Task{
		{ID: uuid.New(), UserID: userID, Quadrant: &quadrant, Position: 1024, Color: shared.ColorNone},
		{ID: uuid.New(), UserID: userID, Quadrant: &quadrant, Position: 2048, Color: shared.ColorNone},
	}}
	service := newWriteService(tasks)

	created, err := service.Create(context.Background(), userID, board.Draft{
		Title: "third", Quadrant: quadrant,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Position != 3072 {
		t.Errorf("position = %d, want 3072", created.Position)
	}
}

// TestCreateCountsUnderALock is the quota race: the account has to be held
// while the count is taken, or two creations both find the last slot free.
func TestCreateCountsUnderALock(t *testing.T) {
	users := &stubUsers{account: &user.User{Settings: user.Settings{Timezone: "UTC"}}}
	tasks := &stubTasks{}
	service := board.NewService(board.Deps{
		Tasks: tasks, Users: users, Tags: &stubTags{}, Links: &stubLinks{},
		Tx: &stubTx{}, Ledger: &stubLedger{}, Events: &stubEvents{},
		XP:    progression.DefaultConfig(),
		Clock: &fixedClock{at: time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)},
	})

	if _, err := service.Create(context.Background(), uuid.New(), board.Draft{
		Title: "a task", Quadrant: shared.QuadrantImportantUrgent,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if users.accountLocks != 1 {
		t.Errorf("the account was locked %d times, want once", users.accountLocks)
	}
}

func TestUpdateLeavesCompletedTasksAlone(t *testing.T) {
	userID, taskID := uuid.New(), uuid.New()
	title := "renamed"
	quadrant := shared.QuadrantImportantUrgent

	tests := []struct {
		name    string
		status  shared.TaskStatus
		wantErr shared.ErrorCode
	}{
		{name: "an active task can be edited", status: shared.TaskStatusActive},
		{
			name:    "a completed task is a record, not a draft",
			status:  shared.TaskStatusCompleted,
			wantErr: shared.CodeCompletedTaskFrozen,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks := &stubTasks{tasks: []task.Task{{
				ID: taskID, UserID: userID, Title: "original", Status: tt.status,
				Quadrant: &quadrant, Position: 1024,
				Color: shared.ColorNone,
			}}}
			service := newWriteService(tasks)

			_, err := service.Update(context.Background(), userID, taskID, board.Patch{Title: &title})

			if tt.wantErr != "" {
				if shared.CodeOf(err) != tt.wantErr {
					t.Fatalf("err = %v, want %s", err, tt.wantErr)
				}
				if len(tasks.updated) != 0 {
					t.Error("a frozen task was written anyway")
				}
				return
			}
			if err != nil {
				t.Fatalf("Update: %v", err)
			}
			if len(tasks.updated) != 1 || tasks.updated[0].Title != title {
				t.Errorf("updated = %+v", tasks.updated)
			}
		})
	}
}

func TestUpdateDeadlineRules(t *testing.T) {
	userID, taskID := uuid.New(), uuid.New()
	existing := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	replacement := time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC)
	hasTime := true
	quadrant := shared.QuadrantImportantUrgent

	tests := []struct {
		name         string
		patch        board.Patch
		wantDeadline *time.Time
		wantHasTime  bool
	}{
		{name: "a patch that says nothing changes nothing", patch: board.Patch{}, wantDeadline: &existing},
		{
			name:         "a new deadline replaces the old one",
			patch:        board.Patch{DeadlineAt: &replacement, DeadlineHasTime: &hasTime},
			wantDeadline: &replacement,
			wantHasTime:  true,
		},
		{name: "clearing wins over everything", patch: board.Patch{ClearDeadline: true}},
		{
			name:         "clearing wins even when a date is also sent",
			patch:        board.Patch{ClearDeadline: true, DeadlineAt: &replacement},
			wantDeadline: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks := &stubTasks{tasks: []task.Task{{
				ID: taskID, UserID: userID, Title: "a task", Status: shared.TaskStatusActive,
				Quadrant: &quadrant, Position: 1024,
				Color: shared.ColorNone, DeadlineAt: &existing,
			}}}
			service := newWriteService(tasks)

			updated, err := service.Update(context.Background(), userID, taskID, tt.patch)
			if err != nil {
				t.Fatalf("Update: %v", err)
			}

			switch {
			case tt.wantDeadline == nil && updated.DeadlineAt != nil:
				t.Errorf("deadline = %v, want cleared", *updated.DeadlineAt)
			case tt.wantDeadline != nil && updated.DeadlineAt == nil:
				t.Errorf("the deadline was cleared by a patch that did not ask")
			case tt.wantDeadline != nil && !updated.DeadlineAt.Equal(*tt.wantDeadline):
				t.Errorf("deadline = %v, want %v", updated.DeadlineAt, *tt.wantDeadline)
			}
			if updated.DeadlineHasTime != tt.wantHasTime {
				t.Errorf("hasTime = %v, want %v", updated.DeadlineHasTime, tt.wantHasTime)
			}
		})
	}
}

//go:fix inline

// TestDeleteCountsEveryCompletedTask covers the case with no XP to take back:
// a task that predates the ledger still stopped being completed, and leaving
// its count behind overstates the profile for good.
func TestDeleteCountsEveryCompletedTask(t *testing.T) {
	userID := uuid.New()
	quadrant := shared.QuadrantImportantUrgent
	awarded := int32(50)
	completedAt := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)

	tests := []struct {
		name          string
		xp            *int32
		wantXPDelta   int64
		wantEntries   int
		wantCompleted int32
	}{
		{
			name:          "a paid task gives its XP back",
			xp:            &awarded,
			wantXPDelta:   -50,
			wantEntries:   1,
			wantCompleted: -1,
		},
		{
			name: "one with no snapshot still stops counting",
			// No ledger entry: there is nothing to reverse, and an entry of
			// zero is not a fact. The counter moves all the same.
			wantEntries:   0,
			wantCompleted: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskID := uuid.New()
			tasks := &stubTasks{tasks: []task.Task{{
				ID: taskID, UserID: userID, Title: "done", Status: shared.TaskStatusCompleted,
				Quadrant: &quadrant, Position: 1024, Color: shared.ColorNone,
				CompletedAt: &completedAt, XPAwarded: tt.xp,
				QuadrantAtCompletion: &quadrant,
			}}}
			users := &stubUsers{account: &user.User{Settings: user.Settings{Timezone: "UTC"}}}
			ledger := &stubLedger{}
			service := board.NewService(board.Deps{
				Tasks: tasks, Users: users, Tags: &stubTags{}, Links: &stubLinks{},
				Tx: &stubTx{}, Ledger: ledger, Events: &stubEvents{},
				XP:    progression.DefaultConfig(),
				Clock: &fixedClock{at: time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)},
			})

			deleted, err := service.Delete(context.Background(), userID, taskID)
			if err != nil {
				t.Fatalf("Delete: %v", err)
			}
			if len(deleted) != 1 || deleted[0] != taskID {
				t.Fatalf("deleted = %v", deleted)
			}
			if len(ledger.entries) != tt.wantEntries {
				t.Errorf("%d ledger entries, want %d", len(ledger.entries), tt.wantEntries)
			}
			if len(users.applied) != 1 {
				t.Fatalf("the aggregate was touched %d times, want once", len(users.applied))
			}
			if users.applied[0].XP != tt.wantXPDelta {
				t.Errorf("XP delta = %d, want %d", users.applied[0].XP, tt.wantXPDelta)
			}
			if users.applied[0].TasksCompleted != tt.wantCompleted {
				t.Errorf("tasksCompleted delta = %d, want %d",
					users.applied[0].TasksCompleted, tt.wantCompleted)
			}
		})
	}
}
