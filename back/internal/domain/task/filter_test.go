package task_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/task"
)

func TestDefaultFilter(t *testing.T) {
	userID := uuid.New()
	filter := task.DefaultFilter(userID)

	if err := filter.Validate(); err != nil {
		t.Fatalf("the default filter does not validate: %v", err)
	}

	tests := []struct {
		name string
		got  any
		want any
	}{
		{name: "scoped to the caller", got: filter.UserID, want: userID},
		{name: "the board shows active work", got: filter.Status, want: task.StatusFilterActive},
		{name: "in the user's own order", got: filter.Sort, want: task.SortByPosition},
		{name: "ascending", got: filter.Direction, want: task.SortAsc},
		{name: "no deadline window", got: filter.Deadline, want: task.DeadlineAny},
		{name: "no topology filter", got: filter.Topology, want: task.TopologyAny},
		{name: "capped", got: filter.Limit, want: task.MaxBoardPage},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("= %v, want %v", tt.got, tt.want)
			}
		})
	}
}

// TestFilterValidate is the gate that keeps an unchecked value out of the
// statement: sort and direction become SQL identifiers, and the rest become
// bound parameters the database would reject as unknown enum labels.
func TestFilterValidate(t *testing.T) {
	valid := func() task.Filter { return task.DefaultFilter(uuid.New()) }

	tests := []struct {
		name      string
		mutate    func(f *task.Filter)
		wantField string
	}{
		{name: "the default filter passes", mutate: func(*task.Filter) {}},
		{
			name:      "an unscoped filter would read another tenant",
			mutate:    func(f *task.Filter) { f.UserID = uuid.Nil },
			wantField: "userId",
		},
		{
			name:      "an unknown status",
			mutate:    func(f *task.Filter) { f.Status = task.StatusFilter("ARCHIVED") },
			wantField: "status",
		},
		{
			name:      "an unknown deadline window",
			mutate:    func(f *task.Filter) { f.Deadline = task.DeadlineWindow("SOON") },
			wantField: "deadline",
		},
		{
			name:      "an unknown topology",
			mutate:    func(f *task.Filter) { f.Topology = task.TopologyFilter("ORPHANED") },
			wantField: "topology",
		},
		{
			name:      "a sort field that is not a column",
			mutate:    func(f *task.Filter) { f.Sort = task.SortField("title; DROP TABLE tasks") },
			wantField: "sort",
		},
		{
			name:      "a direction that is not a direction",
			mutate:    func(f *task.Filter) { f.Direction = task.SortDirection("ASC, (SELECT 1)") },
			wantField: "direction",
		},
		{
			name:      "an unknown colour among valid ones",
			mutate:    func(f *task.Filter) { f.Colors = []shared.TaskColor{"CYAN", "BURNT_SIENNA"} },
			wantField: "colors",
		},
		{
			name:      "an unknown quadrant among valid ones",
			mutate:    func(f *task.Filter) { f.Quadrants = []shared.Quadrant{shared.QuadrantImportantUrgent, "VERY_URGENT"} },
			wantField: "quadrants",
		},
		{
			name:      "a negative limit",
			mutate:    func(f *task.Filter) { f.Limit = -1 },
			wantField: "limit",
		},
		{
			name:      "a limit past the cap",
			mutate:    func(f *task.Filter) { f.Limit = task.MaxBoardPage + 1 },
			wantField: "limit",
		},
		{name: "zero is allowed and means the cap, not no cap", mutate: func(f *task.Filter) { f.Limit = 0 }},
		{name: "empty lists mean no filter", mutate: func(f *task.Filter) { f.Colors, f.Quadrants, f.Tags = nil, nil, nil }},
		{name: "a free-text query is not validated here", mutate: func(f *task.Filter) { f.Query = "100% of _everything_" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := valid()
			tt.mutate(&filter)
			err := filter.Validate()

			if tt.wantField == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}

			domainErr, ok := err.(*shared.Error)
			if !ok {
				t.Fatalf("Validate() = %v, want a domain error", err)
			}
			if domainErr.Code != shared.CodeValidationFailed {
				t.Errorf("code = %q, want %q", domainErr.Code, shared.CodeValidationFailed)
			}
			if domainErr.Params["field"] != tt.wantField {
				t.Errorf("field = %v, want %q", domainErr.Params["field"], tt.wantField)
			}
		})
	}
}

func TestFilterEnumsAcceptEveryDeclaredValue(t *testing.T) {
	tests := []struct {
		name  string
		valid []func() bool
	}{
		{
			name: "status",
			valid: []func() bool{
				func() bool { v := task.StatusFilterActive; return v.Valid() },
				func() bool { v := task.StatusFilterCompleted; return v.Valid() },
				func() bool { v := task.StatusFilterAll; return v.Valid() },
				func() bool { v := task.StatusFilter(""); return v.Valid() },
			},
		},
		{
			name: "deadline",
			valid: []func() bool{
				func() bool { v := task.DeadlineAny; return v.Valid() },
				func() bool { v := task.DeadlineOverdue; return v.Valid() },
				func() bool { v := task.DeadlineToday; return v.Valid() },
				func() bool { v := task.DeadlineWeek; return v.Valid() },
				func() bool { v := task.DeadlineNone; return v.Valid() },
				func() bool { v := task.DeadlineWindow("ANY "); return v.Valid() },
			},
		},
		{
			name: "topology",
			valid: []func() bool{
				func() bool { v := task.TopologyAny; return v.Valid() },
				func() bool { v := task.TopologyLinked; return v.Valid() },
				func() bool { v := task.TopologyUnlinked; return v.Valid() },
				func() bool { v := task.TopologyFilter("linked"); return v.Valid() },
			},
		},
		{
			name: "sort",
			valid: []func() bool{
				func() bool { v := task.SortByPosition; return v.Valid() },
				func() bool { v := task.SortByDeadline; return v.Valid() },
				func() bool { v := task.SortByCreated; return v.Valid() },
				func() bool { v := task.SortByTitle; return v.Valid() },
				func() bool { v := task.SortField("position"); return v.Valid() },
			},
		},
		{
			name: "direction",
			valid: []func() bool{
				func() bool { v := task.SortAsc; return v.Valid() },
				func() bool { v := task.SortDesc; return v.Valid() },
				func() bool { v := task.SortDirection("asc"); return v.Valid() },
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Every declared constant is accepted; the last case of each set is
			// a near miss that must not be.
			for i, check := range tt.valid {
				last := i == len(tt.valid)-1
				if got := check(); got == last {
					t.Errorf("case %d: Valid() = %v", i, got)
				}
			}
		})
	}
}

// TestFilterPageSize is what stops the cap from being optional: a filter built
// by hand, without DefaultFilter, must still be bounded.
func TestFilterPageSize(t *testing.T) {
	tests := []struct {
		name  string
		limit int
		want  int
	}{
		{name: "a hand-built filter asks for nothing and gets the cap", limit: 0, want: task.MaxBoardPage},
		{name: "a smaller page is honoured", limit: 25, want: 25},
		{name: "exactly the cap is honoured", limit: task.MaxBoardPage, want: task.MaxBoardPage},
		{name: "more than the cap is still the cap", limit: task.MaxBoardPage + 1000, want: task.MaxBoardPage},
		{name: "a negative page is the cap", limit: -1, want: task.MaxBoardPage},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := task.Filter{Limit: tt.limit}
			if got := filter.PageSize(); got != tt.want {
				t.Errorf("PageSize() = %d, want %d", got, tt.want)
			}
		})
	}

	// The zero value of the struct itself, which is what a forgetful caller
	// passes to the repository.
	var zero task.Filter
	if zero.PageSize() != task.MaxBoardPage {
		t.Errorf("the zero Filter is unbounded: PageSize() = %d", zero.PageSize())
	}
}
