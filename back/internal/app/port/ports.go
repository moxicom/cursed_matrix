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
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/task"
	"github.com/moxicom/cursed_matrix/back/internal/domain/user"
)

type TxManager interface {
	Do(ctx context.Context, fn func(context.Context) error) error
}

type TaskRepository interface {
	ListBoard(ctx context.Context, filter task.Filter) ([]task.Task, error)
	ByID(ctx context.Context, userID, taskID uuid.UUID) (*task.Task, error)
	Create(ctx context.Context, item *task.Task) error
	Update(ctx context.Context, item *task.Task) error
	SoftDelete(ctx context.Context, userID, taskID uuid.UUID, at time.Time) error
	NextPosition(ctx context.Context, userID uuid.UUID, quadrant *shared.Quadrant, parentID *uuid.UUID) (int32, error)
	CountActive(ctx context.Context, userID uuid.UUID) (int, error)
}

type UserRepository interface {
	ByID(ctx context.Context, id uuid.UUID) (*user.User, error)
	ByUsername(ctx context.Context, username string) (*user.User, error)
	Create(ctx context.Context, account *user.User) error
	UpdateSettings(ctx context.Context, id uuid.UUID, settings user.Settings) error
	TouchLogin(ctx context.Context, id uuid.UUID, at time.Time) error
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
