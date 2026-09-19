package postgres

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/task"
)

func boardFilter() task.Filter {
	filter := task.DefaultFilter(uuid.New())
	filter.Now = time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	filter.Location = time.UTC
	return filter
}

func buildBoardSQL(t *testing.T, filter task.Filter) (string, []any) {
	t.Helper()

	query := builder.
		Select(taskColumns...).
		From("tasks t").
		LeftJoin("tasks p ON p.id = t.parent_task_id")
	query = applyStatus(query, filter.Status)
	query = applyQuadrants(query, filter.Quadrants)
	query = applyColors(query, filter.Colors)
	query = applyTags(query, filter.Tags)
	query = applyDeadline(query, filter)
	query = applyTopology(query, filter.Topology)
	query = applySearch(query, filter.Query)

	order, err := orderClause(filter.Sort, filter.Direction)
	if err != nil {
		t.Fatalf("orderClause: %v", err)
	}
	statement, args, err := query.OrderBy(order).ToSql()
	if err != nil {
		t.Fatalf("ToSql: %v", err)
	}
	return statement, args
}

func TestSearchTermNeverReachesTheQueryText(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{name: "statement terminator", payload: `'; DELETE FROM tasks; --`},
		{name: "always true", payload: `" OR 1=1 --`},
		{name: "escaped quote", payload: `\'; UPDATE users SET plan = 'PRO'; --`},
		{name: "union select", payload: `' UNION SELECT password_hash FROM users --`},
		{name: "bare wildcard", payload: `%`},
		{name: "single character wildcard", payload: `_`},
		{name: "percent inside a word", payload: `100%`},
		{name: "control characters", payload: "tab\tand\nnewline"},
		{name: "multi-byte", payload: `Ω≈ç√∫˜µ`},
	}

	baseline := boardFilter()
	baseline.Query = "harmless"
	baselineSQL, _ := buildBoardSQL(t, baseline)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			filter := boardFilter()
			filter.Query = tc.payload
			statement, args := buildBoardSQL(t, filter)

			if statement != baselineSQL {
				t.Fatalf("the search term changed the SQL text:\n%s", statement)
			}

			want := containsPattern(tc.payload)
			found := false
			for _, arg := range args {
				if s, ok := arg.(string); ok && s == want {
					found = true
				}
			}
			if !found {
				t.Fatalf("the escaped term is not among the arguments: %#v", args)
			}
		})
	}
}

func TestContainsPattern(t *testing.T) {
	tests := []struct {
		name string
		term string
		want string
	}{
		{name: "plain", term: "plain", want: "%plain%"},
		{name: "percent", term: "100%", want: `%100\%%`},
		{name: "underscore", term: "snake_case", want: `%snake\_case%`},
		{name: "backslash", term: `back\slash`, want: `%back\\slash%`},
		{name: "all three", term: `a%b_c\d`, want: `%a\%b\_c\\d%`},
		{name: "empty", term: "", want: "%%"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := containsPattern(tc.term); got != tc.want {
				t.Errorf("containsPattern(%q) = %q, want %q", tc.term, got, tc.want)
			}
		})
	}
}

func TestOrderClause(t *testing.T) {
	tests := []struct {
		name      string
		field     task.SortField
		direction task.SortDirection
		want      string
		wantErr   bool
	}{
		{name: "position ascending", field: task.SortByPosition, direction: task.SortAsc, want: "t.position ASC NULLS LAST"},
		{name: "deadline descending", field: task.SortByDeadline, direction: task.SortDesc, want: "t.deadline_at DESC NULLS LAST"},
		{name: "created ascending", field: task.SortByCreated, direction: task.SortAsc, want: "t.created_at ASC NULLS LAST"},
		{name: "title is folded", field: task.SortByTitle, direction: task.SortAsc, want: "lower(t.title) ASC NULLS LAST"},
		{name: "injected field", field: "t.position; TRUNCATE tasks", direction: task.SortAsc, wantErr: true},
		{name: "sub-select as a field", field: "(SELECT password_hash FROM users LIMIT 1)", direction: task.SortAsc, wantErr: true},
		{name: "empty field", field: "", direction: task.SortAsc, wantErr: true},
		{name: "field with trailing space", field: "POSITION ", direction: task.SortAsc, wantErr: true},
		{name: "injected direction", field: task.SortByPosition, direction: "ASC; --", wantErr: true},
		{name: "lowercase direction", field: task.SortByPosition, direction: "asc", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := orderClause(tc.field, tc.direction)
			if (err != nil) != tc.wantErr {
				t.Fatalf("orderClause(%q, %q) error = %v, wantErr %v", tc.field, tc.direction, err, tc.wantErr)
			}
			if !tc.wantErr && got != tc.want {
				t.Errorf("orderClause(%q, %q) = %q, want %q", tc.field, tc.direction, got, tc.want)
			}
		})
	}
}

func TestBoardFilterValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(f *task.Filter)
		wantErr bool
	}{
		{name: "the default filter is valid", mutate: func(*task.Filter) {}},
		{
			name:    "no owner",
			mutate:  func(f *task.Filter) { f.UserID = uuid.Nil },
			wantErr: true,
		},
		{
			name:    "unknown status",
			mutate:  func(f *task.Filter) { f.Status = "ACTIVE OR 1=1" },
			wantErr: true,
		},
		{
			name:    "unknown quadrant",
			mutate:  func(f *task.Filter) { f.Quadrants = []shared.Quadrant{"IMPORTANT_URGENT'; --"} },
			wantErr: true,
		},
		{
			name:    "unknown colour",
			mutate:  func(f *task.Filter) { f.Colors = []shared.TaskColor{"PUCE"} },
			wantErr: true,
		},
		{
			name:    "unknown deadline window",
			mutate:  func(f *task.Filter) { f.Deadline = "SOON" },
			wantErr: true,
		},
		{
			name:    "unknown topology",
			mutate:  func(f *task.Filter) { f.Topology = "TANGLED" },
			wantErr: true,
		},
		{
			name:    "unknown sort field",
			mutate:  func(f *task.Filter) { f.Sort = "password_hash" },
			wantErr: true,
		},
		{
			name:    "unknown direction",
			mutate:  func(f *task.Filter) { f.Direction = "SIDEWAYS" },
			wantErr: true,
		},
		{
			name: "every declared value together",
			mutate: func(f *task.Filter) {
				f.Status = task.StatusFilterAll
				f.Quadrants = shared.Quadrants()
				f.Colors = shared.TaskColors()
				f.Deadline = task.DeadlineWeek
				f.Topology = task.TopologyLinked
				f.Sort = task.SortByTitle
				f.Direction = task.SortDesc
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			filter := boardFilter()
			tc.mutate(&filter)
			if err := filter.Validate(); (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestValuesAreBoundNotInlined(t *testing.T) {
	filter := boardFilter()
	filter.Status = task.StatusFilterAll
	filter.Quadrants = []shared.Quadrant{shared.QuadrantImportantUrgent}
	filter.Colors = []shared.TaskColor{shared.ColorRose}
	filter.Tags = []string{"Backend"}
	filter.Deadline = task.DeadlineToday
	filter.Topology = task.TopologyLinked
	filter.Query = "auth"

	statement, args := buildBoardSQL(t, filter)
	if len(args) == 0 {
		t.Fatal("a filtered query bound no arguments at all")
	}

	tests := []struct {
		name  string
		value string
	}{
		{name: "tag name", value: "Backend"},
		{name: "search term", value: "auth"},
		{name: "colour", value: "ROSE"},
		{name: "unrewritten placeholder", value: "?"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if strings.Contains(statement, tc.value) {
				t.Errorf("%s appears in the SQL text rather than in the arguments:\n%s", tc.name, statement)
			}
		})
	}
}

func TestLocalDayBounds(t *testing.T) {
	moscow, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}
	losAngeles, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}

	// 22:30 UTC is already the next day in Moscow and still the same afternoon
	// in Los Angeles, so one instant belongs to two different calendar days.
	instant := time.Date(2026, 9, 19, 22, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		location *time.Location
		wantDay  int
	}{
		{name: "utc", location: time.UTC, wantDay: 19},
		{name: "ahead of utc", location: moscow, wantDay: 20},
		{name: "behind utc", location: losAngeles, wantDay: 19},
		{name: "nil falls back to utc", location: nil, wantDay: 19},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			start, end := localDayBounds(instant, tc.location)

			location := tc.location
			if location == nil {
				location = time.UTC
			}
			if got := start.In(location).Day(); got != tc.wantDay {
				t.Errorf("the day starts on the %d, want the %d", got, tc.wantDay)
			}
			if end.Sub(start) != 24*time.Hour {
				t.Errorf("the window spans %v, want 24h", end.Sub(start))
			}
			if !start.Before(instant) || !end.After(instant) {
				t.Errorf("the instant %s is outside [%s, %s)", instant, start, end)
			}
		})
	}
}
