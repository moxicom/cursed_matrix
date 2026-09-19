package task

import (
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
	}
}

func (f *Filter) Validate() error {
	invalid := func(field, value string) error {
		return shared.NewError(shared.CodeValidationFailed,
			map[string]any{"field": field, "value": value})
	}
	switch {
	case f.UserID == uuid.Nil:
		return invalid("userId", "")
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
