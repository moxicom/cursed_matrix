package graph_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/app/graph"
	"github.com/moxicom/cursed_matrix/back/internal/domain/activity"
	"github.com/moxicom/cursed_matrix/back/internal/domain/link"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/task"
	"github.com/moxicom/cursed_matrix/back/internal/domain/user"
)

type stubLinks struct {
	links   []link.Link
	created []link.Link
}

func (l *stubLinks) Create(_ context.Context, item *link.Link) error {
	l.created = append(l.created, *item)
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
func (l *stubLinks) Update(context.Context, *link.Link) error             { return nil }
func (l *stubLinks) Remove(context.Context, uuid.UUID, uuid.UUID) error   { return nil }
func (l *stubLinks) Count(context.Context, uuid.UUID) (int, error)        { return len(l.links), nil }

func (l *stubLinks) RemoveForTasks(context.Context, uuid.UUID, []uuid.UUID) error {
	return nil
}

type stubTasks struct{ tasks []task.Task }

func (s *stubTasks) ByID(_ context.Context, _, taskID uuid.UUID) (*task.Task, error) {
	for i := range s.tasks {
		if s.tasks[i].ID == taskID {
			found := s.tasks[i]
			return &found, nil
		}
	}
	return nil, shared.NewError(shared.CodeTaskNotFound, nil)
}

func (s *stubTasks) ByIDForUpdate(ctx context.Context, userID, taskID uuid.UUID) (*task.Task, error) {
	return s.ByID(ctx, userID, taskID)
}
func (s *stubTasks) ListBoard(context.Context, task.Filter) ([]task.Task, error) { return nil, nil }
func (s *stubTasks) Create(context.Context, *task.Task) error                    { return nil }
func (s *stubTasks) Update(context.Context, *task.Task) error                    { return nil }
func (s *stubTasks) SoftDelete(context.Context, uuid.UUID, []uuid.UUID, time.Time) error {
	return nil
}

func (s *stubTasks) ScopeTasks(context.Context, uuid.UUID, shared.Quadrant) ([]task.Task, error) {
	return nil, nil
}
func (s *stubTasks) Reposition(context.Context, uuid.UUID, []task.Placement) error { return nil }
func (s *stubTasks) CountActive(context.Context, uuid.UUID) (int, error)           { return 0, nil }
func (s *stubTasks) Search(context.Context, uuid.UUID, string, int) ([]task.Hit, error) {
	return nil, nil
}

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

func (s *stubTasks) Subtasks(context.Context, uuid.UUID, uuid.UUID) ([]task.Task, error) {
	return nil, nil
}

type stubUsers struct{ locks int }

func (u *stubUsers) ByID(_ context.Context, id uuid.UUID) (*user.User, error) {
	return &user.User{ID: id}, nil
}
func (u *stubUsers) ByUsername(context.Context, string) (*user.User, error) { return nil, nil }
func (u *stubUsers) Create(context.Context, *user.User) error               { return nil }
func (u *stubUsers) UpdateSettings(context.Context, uuid.UUID, user.Settings) error {
	return nil
}
func (u *stubUsers) TouchLogin(context.Context, uuid.UUID, time.Time) error { return nil }
func (u *stubUsers) ApplyStats(context.Context, uuid.UUID, user.StatsDelta) (user.Stats, error) {
	return user.Stats{}, nil
}
func (u *stubUsers) SetLevel(context.Context, uuid.UUID, int32) error { return nil }

func (u *stubUsers) LockAccount(context.Context, uuid.UUID) error {
	u.locks++
	return nil
}

func (u *stubUsers) TouchStreak(context.Context, uuid.UUID, time.Time) (user.StreakChange, error) {
	return user.StreakChange{}, nil
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

type stubTx struct{}

func (*stubTx) Do(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

type fixedClock struct{ at time.Time }

func (c *fixedClock) Now() time.Time { return c.at }

func TestCreateLink(t *testing.T) {
	userID := uuid.New()
	parentID, childID, otherID := uuid.New(), uuid.New(), uuid.New()
	quadrant := shared.QuadrantImportantUrgent

	tasks := &stubTasks{tasks: []task.Task{
		{ID: parentID, UserID: userID, Quadrant: &quadrant},
		{ID: childID, UserID: userID, ParentID: &parentID},
		{ID: otherID, UserID: userID, Quadrant: &quadrant},
	}}

	tests := []struct {
		name     string
		source   uuid.UUID
		target   uuid.UUID
		linkType shared.LinkType
		held     int
		wantErr  shared.ErrorCode
		wantType shared.LinkType
	}{
		{name: "two unrelated tasks", source: parentID, target: otherID, wantType: shared.LinkRelated},
		{
			name: "a parent and its own subtask are already related",
			// The graph draws that relation itself; a second edge says it
			// twice.
			source:  parentID,
			target:  childID,
			wantErr: shared.CodeDuplicateLink,
		},
		{
			name:    "and the other way round",
			source:  childID,
			target:  parentID,
			wantErr: shared.CodeDuplicateLink,
		},
		{
			name:    "a task cannot be linked to itself",
			source:  parentID,
			target:  parentID,
			wantErr: shared.CodeSelfLink,
		},
		{
			name:    "a task that is not there",
			source:  parentID,
			target:  uuid.New(),
			wantErr: shared.CodeTaskNotFound,
		},
		{
			name:    "at the plan's ceiling",
			source:  parentID,
			target:  otherID,
			held:    user.FreeTaskLinks,
			wantErr: shared.CodeQuotaLimitReached,
		},
		{
			name:   "one below the ceiling still fits",
			source: parentID,
			target: otherID,
			held:   user.FreeTaskLinks - 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			links := &stubLinks{links: make([]link.Link, tt.held)}
			users := &stubUsers{}
			events := &stubEvents{}
			service := graph.NewService(links, tasks, users, events, &stubTx{},
				&fixedClock{at: time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)})

			created, err := service.Create(context.Background(), userID, tt.source, tt.target, tt.linkType)

			if tt.wantErr != "" {
				if shared.CodeOf(err) != tt.wantErr {
					t.Fatalf("err = %v, want %s", err, tt.wantErr)
				}
				if len(links.created) != 0 {
					t.Error("a refused link was written anyway")
				}
				return
			}
			if err != nil {
				t.Fatalf("Create: %v", err)
			}
			if tt.wantType != "" && created.Type != tt.wantType {
				t.Errorf("type = %q, want %q", created.Type, tt.wantType)
			}
			if users.locks != 1 {
				t.Errorf("the account was locked %d times, want once while the quota was counted", users.locks)
			}
			if len(events.recorded) != 1 || events.recorded[0].Type != shared.EventTaskLinked {
				t.Errorf("events = %+v, want one TASK_LINKED", events.recorded)
			}
		})
	}
}
