package task_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/task"
)

var now = time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)

func newParent(t *testing.T) *task.Task {
	t.Helper()
	parent, err := task.NewRegular(uuid.New(), uuid.New(), "parent",
		shared.QuadrantImportantUrgent, 1024, now)
	if err != nil {
		t.Fatalf("NewRegular: %v", err)
	}
	return parent
}

func newSubtask(t *testing.T, parent *task.Task) *task.Task {
	t.Helper()
	subtask, err := task.NewSubtask(uuid.New(), parent, "step", 1024, now)
	if err != nil {
		t.Fatalf("NewSubtask: %v", err)
	}
	return subtask
}

func TestNewRegular(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		quadrant shared.Quadrant
		wantErr  bool
	}{
		{name: "plain title", title: "write the spec", quadrant: shared.QuadrantImportantUrgent},
		{name: "lowest quadrant", title: "someday", quadrant: shared.QuadrantNotImportantNotUrgnt},
		{name: "empty title", title: "", quadrant: shared.QuadrantImportantUrgent, wantErr: true},
		{name: "unknown quadrant", title: "write the spec", quadrant: "SOMEDAY_MAYBE", wantErr: true},
		{name: "empty quadrant", title: "write the spec", quadrant: "", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			created, err := task.NewRegular(uuid.New(), uuid.New(), tc.title, tc.quadrant, 1024, now)
			if (err != nil) != tc.wantErr {
				t.Fatalf("NewRegular error = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if created.IsSubtask() {
				t.Error("a task created without a parent must not be a subtask")
			}
			if created.Quadrant == nil || *created.Quadrant != tc.quadrant {
				t.Errorf("quadrant = %v, want %q", created.Quadrant, tc.quadrant)
			}
			if created.Status != shared.TaskStatusActive {
				t.Errorf("status = %q, want ACTIVE", created.Status)
			}
			if err := created.Validate(); err != nil {
				t.Errorf("a freshly created task fails its own invariants: %v", err)
			}
		})
	}
}

func TestNormalizeTitle(t *testing.T) {
	tests := []struct {
		name    string
		title   string
		want    string
		wantErr bool
	}{
		{name: "plain", title: "write the spec", want: "write the spec"},
		{name: "trimmed", title: "   padded   ", want: "padded"},
		{name: "exactly the limit", title: strings.Repeat("x", 100), want: strings.Repeat("x", 100)},
		{name: "100 cyrillic characters", title: strings.Repeat("я", 100), want: strings.Repeat("я", 100)},
		{name: "empty", title: "", wantErr: true},
		{name: "whitespace only", title: "   \t\n", wantErr: true},
		{name: "one over the limit", title: strings.Repeat("x", 101), wantErr: true},
		{name: "101 cyrillic characters", title: strings.Repeat("я", 101), wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := task.NormalizeTitle(tc.title)
			if (err != nil) != tc.wantErr {
				t.Fatalf("NormalizeTitle(%q) error = %v, wantErr %v", tc.title, err, tc.wantErr)
			}
			if !tc.wantErr && got != tc.want {
				t.Errorf("NormalizeTitle(%q) = %q, want %q", tc.title, got, tc.want)
			}
		})
	}
}

func TestValidateDescription(t *testing.T) {
	tests := []struct {
		name        string
		description string
		wantErr     bool
	}{
		{name: "empty is valid", description: ""},
		{name: "exactly the limit", description: strings.Repeat("x", 2000)},
		{name: "cyrillic at the limit", description: strings.Repeat("я", 2000)},
		{name: "one over the limit", description: strings.Repeat("x", 2001), wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := task.ValidateDescription(tc.description); (err != nil) != tc.wantErr {
				t.Errorf("ValidateDescription(%d chars) error = %v, wantErr %v",
					len([]rune(tc.description)), err, tc.wantErr)
			}
		})
	}
}

func TestNewSubtask(t *testing.T) {
	tests := []struct {
		name     string
		parentOf func(t *testing.T) *task.Task
		title    string
		wantCode shared.ErrorCode
	}{
		{
			name:     "under a regular task",
			parentOf: newParent,
			title:    "step one",
		},
		{
			name:     "under a subtask is refused",
			parentOf: func(t *testing.T) *task.Task { return newSubtask(t, newParent(t)) },
			title:    "step one a",
			wantCode: shared.CodeNestingNotAllowed,
		},
		{
			name:     "empty title is refused",
			parentOf: newParent,
			title:    "",
			wantCode: shared.CodeValidationFailed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parent := tc.parentOf(t)

			created, err := task.NewSubtask(uuid.New(), parent, tc.title, 1024, now)
			if tc.wantCode != "" {
				if err == nil {
					t.Fatalf("NewSubtask accepted %q under %s", tc.title, parent.ID)
				}
				if got := shared.CodeOf(err); got != tc.wantCode {
					t.Errorf("error code = %q, want %q", got, tc.wantCode)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewSubtask: %v", err)
			}
			if !created.IsSubtask() {
				t.Error("a task with a parent must be a subtask")
			}
			if created.Quadrant != nil {
				t.Error("a subtask must not carry a quadrant of its own")
			}
			if created.UserID != parent.UserID {
				t.Error("a subtask must belong to the owner of its parent")
			}
		})
	}
}

func TestEffectiveQuadrant(t *testing.T) {
	tests := []struct {
		name       string
		subject    func(t *testing.T) (*task.Task, *task.Task)
		want       shared.Quadrant
		wantResolv bool
	}{
		{
			name: "a regular task uses its own",
			subject: func(t *testing.T) (*task.Task, *task.Task) {
				return newParent(t), nil
			},
			want: shared.QuadrantImportantUrgent, wantResolv: true,
		},
		{
			name: "a subtask uses the parent's",
			subject: func(t *testing.T) (*task.Task, *task.Task) {
				parent := newParent(t)
				return newSubtask(t, parent), parent
			},
			want: shared.QuadrantImportantUrgent, wantResolv: true,
		},
		{
			name: "an orphaned subtask resolves nothing",
			subject: func(t *testing.T) (*task.Task, *task.Task) {
				return newSubtask(t, newParent(t)), nil
			},
			wantResolv: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			subject, parent := tc.subject(t)
			got, ok := subject.EffectiveQuadrant(parent)
			if ok != tc.wantResolv {
				t.Fatalf("EffectiveQuadrant resolved = %v, want %v", ok, tc.wantResolv)
			}
			if ok && got != tc.want {
				t.Errorf("EffectiveQuadrant = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCompleteAndReopen(t *testing.T) {
	tests := []struct {
		name         string
		completeOnce bool
		operation    string
		wantCode     shared.ErrorCode
	}{
		{name: "complete an active task", operation: "complete"},
		{name: "complete a completed task", completeOnce: true, operation: "complete", wantCode: shared.CodeTaskAlreadyDone},
		{name: "reopen a completed task", completeOnce: true, operation: "reopen"},
		{name: "reopen an active task", operation: "reopen", wantCode: shared.CodeValidationFailed},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			subject := newParent(t)
			if tc.completeOnce {
				if err := subject.Complete(now, 50, shared.QuadrantImportantUrgent, shared.CompletionDirect); err != nil {
					t.Fatalf("setup Complete: %v", err)
				}
			}

			var err error
			switch tc.operation {
			case "complete":
				err = subject.Complete(now.Add(time.Hour), 50, shared.QuadrantImportantUrgent, shared.CompletionDirect)
			case "reopen":
				err = subject.Reopen(now.Add(time.Hour))
			}

			if tc.wantCode != "" {
				if err == nil {
					t.Fatalf("%s was accepted but must be refused", tc.operation)
				}
				if got := shared.CodeOf(err); got != tc.wantCode {
					t.Errorf("error code = %q, want %q", got, tc.wantCode)
				}
				return
			}
			if err != nil {
				t.Fatalf("%s: %v", tc.operation, err)
			}

			snapshotSet := subject.CompletedAt != nil && subject.XPAwarded != nil &&
				subject.QuadrantAtCompletion != nil && subject.CompletedVia != nil
			if snapshotSet != subject.IsCompleted() {
				t.Errorf("the snapshot and the status disagree: completed = %v, snapshot = %v",
					subject.IsCompleted(), snapshotSet)
			}
			if err := subject.Validate(); err != nil {
				t.Errorf("the task fails its own invariants after %s: %v", tc.operation, err)
			}
		})
	}
}

func TestPromote(t *testing.T) {
	tests := []struct {
		name     string
		subject  func(t *testing.T) *task.Task
		quadrant shared.Quadrant
		wantCode shared.ErrorCode
	}{
		{
			name:     "a subtask becomes a regular task",
			subject:  func(t *testing.T) *task.Task { return newSubtask(t, newParent(t)) },
			quadrant: shared.QuadrantNotImportantUrgent,
		},
		{
			name:     "a regular task has nothing to promote",
			subject:  newParent,
			quadrant: shared.QuadrantImportantUrgent,
			wantCode: shared.CodeNotASubtask,
		},
		{
			name:     "an unknown target quadrant is refused",
			subject:  func(t *testing.T) *task.Task { return newSubtask(t, newParent(t)) },
			quadrant: "SOMEDAY_MAYBE",
			wantCode: shared.CodeValidationFailed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			subject := tc.subject(t)

			err := subject.Promote(tc.quadrant, 2048, now)
			if tc.wantCode != "" {
				if err == nil {
					t.Fatal("Promote was accepted but must be refused")
				}
				if got := shared.CodeOf(err); got != tc.wantCode {
					t.Errorf("error code = %q, want %q", got, tc.wantCode)
				}
				return
			}
			if err != nil {
				t.Fatalf("Promote: %v", err)
			}
			if subject.IsSubtask() {
				t.Error("a promoted subtask still has a parent")
			}
			if subject.Quadrant == nil || *subject.Quadrant != tc.quadrant {
				t.Errorf("quadrant = %v, want %q", subject.Quadrant, tc.quadrant)
			}
			if err := subject.Validate(); err != nil {
				t.Errorf("a promoted subtask fails its invariants: %v", err)
			}
		})
	}
}

func TestValidateRejectsIllegalStates(t *testing.T) {
	parentID := uuid.New()
	completedAt := now

	tests := []struct {
		name    string
		corrupt func(item *task.Task)
	}{
		{name: "subtask carrying a quadrant", corrupt: func(item *task.Task) { item.ParentID = &parentID }},
		{name: "regular task without a quadrant", corrupt: func(item *task.Task) { item.Quadrant = nil }},
		{name: "completed without a timestamp", corrupt: func(item *task.Task) { item.Status = shared.TaskStatusCompleted }},
		{name: "timestamp without the status", corrupt: func(item *task.Task) { item.CompletedAt = &completedAt }},
		{name: "unknown colour", corrupt: func(item *task.Task) { item.Color = shared.TaskColor("PUCE") }},
		{name: "missing owner", corrupt: func(item *task.Task) { item.UserID = uuid.Nil }},
		{name: "missing id", corrupt: func(item *task.Task) { item.ID = uuid.Nil }},
		{name: "title over the limit", corrupt: func(item *task.Task) { item.Title = strings.Repeat("x", 101) }},
		{name: "description over the limit", corrupt: func(item *task.Task) { item.Description = strings.Repeat("x", 2001) }},
		{
			name: "its own parent",
			corrupt: func(item *task.Task) {
				item.Quadrant = nil
				item.ParentID = &item.ID
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			item := *newParent(t)
			tc.corrupt(&item)
			if err := item.Validate(); err == nil {
				t.Error("Validate accepted a state the domain forbids")
			}
		})
	}
}
