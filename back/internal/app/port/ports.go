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
	"github.com/moxicom/cursed_matrix/back/internal/domain/task"
	"github.com/moxicom/cursed_matrix/back/internal/domain/user"
)

type TxManager interface {
	Do(ctx context.Context, fn func(context.Context) error) error
}

type TaskRepository interface {
	ListBoard(ctx context.Context, filter task.Filter) ([]task.Task, error)
}

type UserRepository interface {
	ByID(ctx context.Context, id uuid.UUID) (*user.User, error)
}

type Cache interface {
	GetInto(ctx context.Context, key cache.Key, dst any) (bool, error)
	Set(ctx context.Context, key cache.Key, value any, ttl time.Duration) error
	Invalidate(ctx context.Context, scope cache.Scope) error
}

type RefreshStore interface {
	Save(ctx context.Context, userID uuid.UUID, tokenID string, ttl time.Duration) error
	Consume(ctx context.Context, userID uuid.UUID, tokenID string) (bool, error)
	RevokeAll(ctx context.Context, userID uuid.UUID) error
}
