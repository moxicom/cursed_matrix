package task

import (
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
)

type StatusFilter string

const (
	StatusFilterActive    StatusFilter = "ACTIVE"
	StatusFilterCompleted StatusFilter = "COMPLETED"
	StatusFilterAll       StatusFilter = "ALL"
)

func (s *StatusFilter) Valid() bool {
	return *s == StatusFilterActive || *s == StatusFilterCompleted || *s == StatusFilterAll
}

type DeadlineWindow string

const (
	DeadlineAny     DeadlineWindow = "ANY"
	DeadlineOverdue DeadlineWindow = "OVERDUE"
	DeadlineToday   DeadlineWindow = "TODAY"
	DeadlineWeek    DeadlineWindow = "WEEK"
	DeadlineNone    DeadlineWindow = "NONE"
)

func (d *DeadlineWindow) Valid() bool {
	switch *d {
	case DeadlineAny, DeadlineOverdue, DeadlineToday, DeadlineWeek, DeadlineNone:
		return true
	}
	return false
}

type TopologyFilter string

const (
	TopologyAny      TopologyFilter = "ANY"
	TopologyLinked   TopologyFilter = "LINKED"
	TopologyUnlinked TopologyFilter = "UNLINKED"
)

func (t *TopologyFilter) Valid() bool {
	return *t == TopologyAny || *t == TopologyLinked || *t == TopologyUnlinked
}

type SortField string

const (
	SortByPosition SortField = "POSITION"
	SortByDeadline SortField = "DEADLINE"
	SortByCreated  SortField = "CREATED"
	SortByTitle    SortField = "TITLE"
)

func (s *SortField) Valid() bool {
	switch *s {
	case SortByPosition, SortByDeadline, SortByCreated, SortByTitle:
		return true
	}
	return false
}

type SortDirection string

const (
	SortAsc  SortDirection = "ASC"
	SortDesc SortDirection = "DESC"
)

func (d *SortDirection) Valid() bool { return *d == SortAsc || *d == SortDesc }

// MaxBoardPage bounds a single board read.
//
// The board and the graph both want the whole working set at once, and the
// free plan caps that at 35 active tasks — but completed tasks are never
// deleted, so ALL and COMPLETED grow without end. The cap is what keeps one
// long-lived account from asking for a full scan of its own history on every
// poll; a result that hits it is reported as truncated rather than silently
// cut.
const MaxBoardPage = 500

type Filter struct {
	UserID    uuid.UUID
	Status    StatusFilter
	Tags      []string
	Colors    []shared.TaskColor
	Quadrants []shared.Quadrant
	Deadline  DeadlineWindow
	Topology  TopologyFilter
	Query     string

	Sort      SortField
	Direction SortDirection

	Now      time.Time
	Location *time.Location

	// Limit is the number of rows the caller will accept, capped at
	// MaxBoardPage. Zero means the cap, so a Filter built by hand is bounded
	// like any other.
	Limit int
}

// DefaultFilter returns the query the board issues when the client asks for
// nothing: active tasks, in user order.
func DefaultFilter(userID uuid.UUID) Filter {
	return Filter{
		UserID:    userID,
		Status:    StatusFilterActive,
		Deadline:  DeadlineAny,
		Topology:  TopologyAny,
		Sort:      SortByPosition,
		Direction: SortAsc,
		Now:       time.Now().UTC(),
		Location:  time.UTC,
		Limit:     MaxBoardPage,
	}
}

// PageSize is the row count the query may return, with zero resolved to the
// cap. Reading Limit directly is what would let a hand-built filter through
// unbounded.
func (f *Filter) PageSize() int {
	if f.Limit <= 0 || f.Limit > MaxBoardPage {
		return MaxBoardPage
	}
	return f.Limit
}

func (f *Filter) Validate() error {
	invalid := func(field, value string) error {
		return shared.NewError(shared.CodeValidationFailed,
			map[string]any{"field": field, "value": value})
	}
	switch {
	case f.UserID == uuid.Nil:
		return invalid("userId", "")
	case f.Limit < 0 || f.Limit > MaxBoardPage:
		return invalid("limit", strconv.Itoa(f.Limit))
	case !f.Status.Valid():
		return invalid("status", string(f.Status))
	case !f.Deadline.Valid():
		return invalid("deadline", string(f.Deadline))
	case !f.Topology.Valid():
		return invalid("topology", string(f.Topology))
	case !f.Sort.Valid():
		return invalid("sort", string(f.Sort))
	case !f.Direction.Valid():
		return invalid("direction", string(f.Direction))
	}
	for _, c := range f.Colors {
		if !c.Valid() {
			return invalid("colors", string(c))
		}
	}
	for _, q := range f.Quadrants {
		if !q.Valid() {
			return invalid("quadrants", string(q))
		}
	}
	return nil
}
