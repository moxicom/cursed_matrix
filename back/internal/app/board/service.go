// Package board holds the read side of the board: one query, every task the
// user is allowed to see.
package board

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/app/cache"
	"github.com/moxicom/cursed_matrix/back/internal/app/port"
	"github.com/moxicom/cursed_matrix/back/internal/domain/progression"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/task"
	"github.com/moxicom/cursed_matrix/back/internal/domain/user"
)

// settingsTTL bounds how long a board read may use a stale timezone if the
// invalidation on the settings write is ever missed.
const settingsTTL = time.Minute

type Service struct {
	tasks  port.TaskRepository
	users  port.UserRepository
	cache  port.Cache
	tx     port.TxManager
	ledger port.XPLedger
	xp     progression.Config
	clock  shared.Clock
}

// NewService wires the board to its ports. The cache is optional: without it
// every read resolves the timezone from the database.
func NewService(
	tasks port.TaskRepository,
	users port.UserRepository,
	cached port.Cache,
	tx port.TxManager,
	ledger port.XPLedger,
	xp progression.Config,
	clock shared.Clock,
) *Service {
	return &Service{
		tasks:  tasks,
		users:  users,
		cache:  cached,
		tx:     tx,
		ledger: ledger,
		xp:     xp,
		clock:  clock,
	}
}

// settings resolves the preferences a board read depends on.
//
// This is the hottest endpoint in the product — the board and the graph both
// poll it — and the only thing it needs from the account is the timezone, so
// it is worth keeping out of the database. Only the preferences are cached,
// never the account: the password hash has no business in a second store.
func (s *Service) settings(ctx context.Context, userID uuid.UUID) (user.Settings, error) {
	key := cache.Key{Scope: cache.UserScope(userID), Name: "settings"}

	if s.cache != nil {
		var cached user.Settings
		// A cache that is down must not take the board with it.
		if hit, err := s.cache.GetInto(ctx, key, &cached); err == nil && hit {
			return cached, nil
		}
	}

	account, err := s.users.ByID(ctx, userID)
	if err != nil {
		return user.Settings{}, err
	}

	if s.cache != nil {
		// A failed write costs a round-trip next time and nothing else.
		_ = s.cache.Set(ctx, key, account.Settings, settingsTTL)
	}
	return account.Settings, nil
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
	settings, err := s.settings(ctx, userID)
	if err != nil {
		return nil, err
	}

	filter := task.DefaultFilter(userID)
	filter.Now = s.clock.Now().UTC()
	filter.Location = settings.Location()
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
