package httphandler

import (
	"testing"
	"time"

	"github.com/google/uuid"

	gen "github.com/moxicom/cursed_matrix/back/internal/adapter/http-handler/gen"
	"github.com/moxicom/cursed_matrix/back/internal/app/board"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/task"
)

func TestBoardQuery(t *testing.T) {
	status := gen.StatusFilter("COMPLETED")
	deadline := gen.DeadlineWindow("OVERDUE")
	topology := gen.TopologyFilter("LINKED")
	sort := gen.SortField("DEADLINE")
	direction := gen.SortDirection("DESC")
	search := "matrix"
	quadrants := []gen.Quadrant{"IMPORTANT_URGENT"}
	colors := []gen.TaskColor{"CYAN", "ROSE"}
	tags := []string{"work", "home"}

	tests := []struct {
		name   string
		params gen.ListTasksParams
		assert func(t *testing.T, q board.Query)
	}{
		{
			name:   "empty query leaves every field unset",
			params: gen.ListTasksParams{},
			assert: func(t *testing.T, q board.Query) {
				if q.Status != nil || q.Deadline != nil || q.Topology != nil ||
					q.Sort != nil || q.Direction != nil {
					t.Fatalf("expected no overrides, got %+v", q)
				}
				if q.Search != "" || q.Tags != nil || q.Colors != nil || q.Quadrants != nil {
					t.Fatalf("expected no lists, got %+v", q)
				}
			},
		},
		{
			name: "every parameter crosses the boundary",
			params: gen.ListTasksParams{
				Status: &status, Deadline: &deadline, Topology: &topology,
				Sort: &sort, Direction: &direction, Query: &search,
				Quadrants: &quadrants, Colors: &colors, Tags: &tags,
			},
			assert: func(t *testing.T, q board.Query) {
				if *q.Status != task.StatusFilterCompleted {
					t.Errorf("status = %v", *q.Status)
				}
				if *q.Deadline != task.DeadlineOverdue {
					t.Errorf("deadline = %v", *q.Deadline)
				}
				if *q.Topology != task.TopologyLinked {
					t.Errorf("topology = %v", *q.Topology)
				}
				if *q.Sort != task.SortByDeadline || *q.Direction != task.SortDesc {
					t.Errorf("order = %v %v", *q.Sort, *q.Direction)
				}
				if q.Search != "matrix" {
					t.Errorf("search = %q", q.Search)
				}
				if len(q.Quadrants) != 1 || q.Quadrants[0] != shared.QuadrantImportantUrgent {
					t.Errorf("quadrants = %v", q.Quadrants)
				}
				if len(q.Colors) != 2 || q.Colors[1] != shared.TaskColor("ROSE") {
					t.Errorf("colors = %v", q.Colors)
				}
				if len(q.Tags) != 2 {
					t.Errorf("tags = %v", q.Tags)
				}
			},
		},
		{
			name: "an unknown enum is carried through, not silently dropped",
			params: func() gen.ListTasksParams {
				bogus := gen.StatusFilter("NOPE")
				return gen.ListTasksParams{Status: &bogus}
			}(),
			assert: func(t *testing.T, q board.Query) {
				if q.Status == nil || *q.Status != task.StatusFilter("NOPE") {
					t.Fatalf("status = %v, want the domain to refuse it", q.Status)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.assert(t, boardQuery(tt.params))
		})
	}
}

func TestRenderTask(t *testing.T) {
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	parent := uuid.New()
	quadrant := shared.QuadrantImportantUrgent
	via := shared.CompletionSource("PARENT_CASCADE")
	xp := int32(50)

	tests := []struct {
		name   string
		task   task.Task
		assert func(t *testing.T, got gen.Task)
	}{
		{
			name: "an active task carries no completion snapshot",
			task: task.Task{
				ID: uuid.New(), UserID: uuid.New(),
				Title: "write the handler", Quadrant: &quadrant,
				Position: 1024, Color: shared.ColorNone,
				Status: shared.TaskStatusActive, CreatedAt: now, UpdatedAt: now,
			},
			assert: func(t *testing.T, got gen.Task) {
				if got.CompletedAt != nil || got.XpAwarded != nil ||
					got.QuadrantAtCompletion != nil || got.CompletedVia != nil {
					t.Fatalf("active task rendered a snapshot: %+v", got)
				}
				if *got.Quadrant != gen.Quadrant("IMPORTANT_URGENT") {
					t.Errorf("quadrant = %v", *got.Quadrant)
				}
			},
		},
		{
			name: "no tags renders as an empty list, never null",
			task: task.Task{Status: shared.TaskStatusActive, CreatedAt: now, UpdatedAt: now},
			assert: func(t *testing.T, got gen.Task) {
				if got.Tags == nil || len(got.Tags) != 0 {
					t.Fatalf("tags = %#v", got.Tags)
				}
			},
		},
		{
			name: "a subtask keeps its parent and has no quadrant of its own",
			task: task.Task{
				ParentID: &parent, Title: "a step",
				Status: shared.TaskStatusCompleted, CreatedAt: now, UpdatedAt: now,
				CompletedAt: &now, XPAwarded: &xp,
				QuadrantAtCompletion: &quadrant, CompletedVia: &via,
				Tags: []string{"work"},
			},
			assert: func(t *testing.T, got gen.Task) {
				if got.ParentTaskId == nil || *got.ParentTaskId != parent {
					t.Fatalf("parentTaskId = %v", got.ParentTaskId)
				}
				if got.Quadrant != nil {
					t.Errorf("subtask rendered a quadrant: %v", *got.Quadrant)
				}
				if *got.XpAwarded != 50 || *got.CompletedVia != gen.CompletionSource("PARENT_CASCADE") {
					t.Errorf("snapshot = %v %v", *got.XpAwarded, *got.CompletedVia)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.assert(t, renderTask(&tt.task))
		})
	}
}
