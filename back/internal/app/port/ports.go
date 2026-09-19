// Package port declares the interfaces the application needs from adapters.
//
// Interfaces only: the vocabulary they speak lives in the domain, or — where a
// concept is not a domain one, such as a cache key — in its own package beside
// this one. A test in internal/arch enforces that.
package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/app/cache"
	"github.com/moxicom/cursed_matrix/back/internal/domain/link"
	"github.com/moxicom/cursed_matrix/back/internal/domain/progression"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/tag"
	"github.com/moxicom/cursed_matrix/back/internal/domain/task"
	"github.com/moxicom/cursed_matrix/back/internal/domain/user"
)

type TxManager interface {
	Do(ctx context.Context, fn func(context.Context) error) error
}

type TaskRepository interface {
	ListBoard(ctx context.Context, filter task.Filter) ([]task.Task, error)
	ByID(ctx context.Context, userID, taskID uuid.UUID) (*task.Task, error)
	// ByIDForUpdate holds the row until the transaction ends, so a decision
	// taken from what it says cannot be overtaken by a concurrent one.
	ByIDForUpdate(ctx context.Context, userID, taskID uuid.UUID) (*task.Task, error)
	Create(ctx context.Context, item *task.Task) error
	Update(ctx context.Context, item *task.Task) error
	SoftDelete(ctx context.Context, userID uuid.UUID, taskIDs []uuid.UUID, at time.Time) error
	ScopeTasks(ctx context.Context, userID uuid.UUID, quadrant shared.Quadrant) ([]task.Task, error)
	Reposition(ctx context.Context, userID uuid.UUID, placements []task.Placement) error
	CountActive(ctx context.Context, userID uuid.UUID) (int, error)
	Subtasks(ctx context.Context, userID, parentID uuid.UUID) ([]task.Task, error)
}

type UserRepository interface {
	ByID(ctx context.Context, id uuid.UUID) (*user.User, error)
	ByUsername(ctx context.Context, username string) (*user.User, error)
	Create(ctx context.Context, account *user.User) error
	UpdateSettings(ctx context.Context, id uuid.UUID, settings user.Settings) error
	TouchLogin(ctx context.Context, id uuid.UUID, at time.Time) error
	// LockAccount serialises whatever the caller is about to decide from a
	// count of the account's own rows.
	LockAccount(ctx context.Context, id uuid.UUID) error
	// ApplyStats adds the delta and returns the totals it produced. The level
	// is not among its arguments because it follows from the lifetime XP the
	// update itself decides.
	ApplyStats(ctx context.Context, id uuid.UUID, delta user.StatsDelta) (user.Stats, error)
	SetLevel(ctx context.Context, id uuid.UUID, level int32) error
}

type TagRepository interface {
	List(ctx context.Context, userID uuid.UUID) ([]tag.Tag, error)
	Upsert(ctx context.Context, userID uuid.UUID, name string) (*tag.Tag, error)
	Attach(ctx context.Context, userID, taskID, tagID uuid.UUID) error
	Detach(ctx context.Context, userID, taskID, tagID uuid.UUID) error
	DeleteOrphans(ctx context.Context, userID uuid.UUID) error
}

type LinkRepository interface {
	Create(ctx context.Context, item *link.Link) error
	ByID(ctx context.Context, userID, linkID uuid.UUID) (*link.Link, error)
	List(ctx context.Context, userID uuid.UUID) ([]link.Link, error)
	Update(ctx context.Context, item *link.Link) error
	Remove(ctx context.Context, userID, linkID uuid.UUID) error
	Count(ctx context.Context, userID uuid.UUID) (int, error)
}

// XPLedger is the append-only record of every XP movement.
type XPLedger interface {
	Record(ctx context.Context, entry *progression.Entry) error
	GrantCount(ctx context.Context, taskID uuid.UUID, source shared.XPSource) (int, error)
}

type Cache interface {
	GetInto(ctx context.Context, key cache.Key, dst any) (bool, error)
	Set(ctx context.Context, key cache.Key, value any, ttl time.Duration) error
	Invalidate(ctx context.Context, scope cache.Scope) error
}

type TokenIssuer interface {
	Issue(userID uuid.UUID, plan shared.Plan) (token string, expiry time.Time, err error)
	Verify(raw string) (uuid.UUID, shared.Plan, error)
	VerifyExpired(raw string) (uuid.UUID, shared.Plan, error)
	TTL() time.Duration
}

// RateLimiter counts attempts against a key and reports when one is over the
// limit, with how long the caller has to wait.
type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (allowed bool, retryAfter time.Duration, err error)
}

type RefreshStore interface {
	Save(ctx context.Context, userID uuid.UUID, tokenID string, ttl time.Duration) error
	Consume(ctx context.Context, userID uuid.UUID, tokenID string) (bool, error)
	RevokeAll(ctx context.Context, userID uuid.UUID) error
}
