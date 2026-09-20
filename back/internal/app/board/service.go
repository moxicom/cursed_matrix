// Package board holds the read side of the board: one query, every task the
// user is allowed to see.
package board

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/app/achievement"
	"github.com/moxicom/cursed_matrix/back/internal/app/port"
	"github.com/moxicom/cursed_matrix/back/internal/app/preferences"
	"github.com/moxicom/cursed_matrix/back/internal/domain/progression"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/task"
)

type Service struct {
	tasks  port.TaskRepository
	users  port.UserRepository
	cache  port.Cache
	tags   port.TagRepository
	links  port.LinkRepository
	tx     port.TxManager
	ledger port.XPLedger
	events port.ActivityRepository
	awards *achievement.Service
	xp     progression.Config
	clock  shared.Clock
	prefs  *preferences.Reader
}

// Deps are the ports the board needs. A struct rather than nine parameters:
// they are all interfaces, so a wrong order compiles and fails at runtime.
type Deps struct {
	Tasks  port.TaskRepository
	Users  port.UserRepository
	Tags   port.TagRepository
	Links  port.LinkRepository
	Tx     port.TxManager
	Ledger port.XPLedger
	Events port.ActivityRepository
	Awards *achievement.Service
	XP     progression.Config
	Clock  shared.Clock

	// Cache is optional: without it every read resolves the timezone from
	// the database.
	Cache port.Cache
}

func NewService(deps Deps) *Service {
	return &Service{
		tasks:  deps.Tasks,
		users:  deps.Users,
		cache:  deps.Cache,
		tags:   deps.Tags,
		links:  deps.Links,
		tx:     deps.Tx,
		ledger: deps.Ledger,
		events: deps.Events,
		awards: deps.Awards,
		xp:     deps.XP,
		clock:  deps.Clock,
		prefs:  preferences.NewReader(deps.Users, deps.Cache),
	}
}

// Query is what the client may ask for. Every field is optional; a zero value
// means "as the board defaults".
type Query struct {
	Status    *task.StatusFilter
	Quadrants []shared.Quadrant
	Colors    []shared.TaskColor
	Tags      []string
	Deadline  *task.DeadlineWindow
	Topology  *task.TopologyFilter
	Search    string
	Sort      *task.SortField
	Direction *task.SortDirection
}

// Board is one page of the working set. Truncated says the cap was reached,
// so the client can tell an empty tail from a cut one.
type Board struct {
	Tasks     []task.Task
	Truncated bool
}

// List answers with the tasks the filter matches, subtasks included.
//
// The deadline windows are the reason the account is read first: "today" is a
// calendar day in the user's timezone, not a rolling 24 hours, so the query
// cannot be built without knowing where the user is.
func (s *Service) List(ctx context.Context, userID uuid.UUID, query Query) (*Board, error) {
	settings, err := s.prefs.Settings(ctx, userID)
	if err != nil {
		return nil, err
	}

	filter := task.DefaultFilter(userID)
	filter.Now = s.clock.Now().UTC()
	filter.Location = settings.Location()
	applyQuery(&filter, query)

	if err := filter.Validate(); err != nil {
		return nil, err
	}

	found, err := s.tasks.ListBoard(ctx, filter)
	if err != nil {
		return nil, err
	}

	// The repository asks for one row past the cap; that row is the signal,
	// not part of the answer.
	if size := filter.PageSize(); len(found) > size {
		return &Board{Tasks: found[:size], Truncated: true}, nil
	}
	return &Board{Tasks: found}, nil
}

// GraphFilter builds the same query the board runs, for a caller that will
// read it through a different projection.
//
// It lives here because the rule it carries is the board's: the deadline
// windows are the user's calendar days, and only this side knows where the
// user is.
func (s *Service) GraphFilter(ctx context.Context, userID uuid.UUID, query Query) (task.Filter, error) {
	settings, err := s.prefs.Settings(ctx, userID)
	if err != nil {
		return task.Filter{}, err
	}

	filter := task.DefaultFilter(userID)
	filter.Now = s.clock.Now().UTC()
	filter.Location = settings.Location()
	applyQuery(&filter, query)

	if err := filter.Validate(); err != nil {
		return task.Filter{}, err
	}
	return filter, nil
}

// applyQuery overlays what the client asked for onto the board's defaults.
func applyQuery(filter *task.Filter, query Query) {
	filter.Quadrants = query.Quadrants
	filter.Colors = query.Colors
	filter.Tags = query.Tags
	filter.Query = query.Search

	if query.Status != nil {
		filter.Status = *query.Status
	}
	if query.Deadline != nil {
		filter.Deadline = *query.Deadline
	}
	if query.Topology != nil {
		filter.Topology = *query.Topology
	}
	if query.Sort != nil {
		filter.Sort = *query.Sort
	}
	if query.Direction != nil {
		filter.Direction = *query.Direction
	}
}

// SearchLimit bounds the palette. It is not a page: the palette shows what
// fits on screen, and a user who needs more narrows the term.
const SearchLimit = 14

// Search answers the command palette.
func (s *Service) Search(ctx context.Context, userID uuid.UUID, term string, limit int) ([]task.Hit, error) {
	clean := strings.TrimSpace(term)
	if clean == "" {
		return nil, shared.NewError(shared.CodeValidationFailed,
			map[string]any{"field": "query", "reason": "empty"})
	}
	if limit <= 0 || limit > SearchLimit {
		limit = SearchLimit
	}
	return s.tasks.Search(ctx, userID, clean, limit)
}
