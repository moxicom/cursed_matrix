// Package preferences reads the account settings that other use cases depend
// on, without making each of them know how they are cached.
package preferences

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/app/cache"
	"github.com/moxicom/cursed_matrix/back/internal/app/port"
	"github.com/moxicom/cursed_matrix/back/internal/domain/user"
)

// TTL bounds how long a read may use stale preferences if the invalidation on
// the settings write is ever missed.
const TTL = time.Minute

type Reader struct {
	users port.UserRepository
	cache port.Cache
}

// NewReader wires the lookup. The cache is optional: without it every read
// resolves from the database.
func NewReader(users port.UserRepository, cached port.Cache) *Reader {
	return &Reader{users: users, cache: cached}
}

// Settings resolves the preferences a request depends on.
//
// Only the preferences are cached, never the account: the password hash has no
// business in a second store.
func (r *Reader) Settings(ctx context.Context, userID uuid.UUID) (user.Settings, error) {
	key := cache.Key{Scope: cache.UserScope(userID), Name: "settings"}

	if r.cache != nil {
		var cached user.Settings
		// A cache that is down must not take the request with it.
		if hit, err := r.cache.GetInto(ctx, key, &cached); err == nil && hit {
			return cached, nil
		}
	}

	account, err := r.users.ByID(ctx, userID)
	if err != nil {
		return user.Settings{}, err
	}

	if r.cache != nil {
		// A failed write costs a round-trip next time and nothing else.
		_ = r.cache.Set(ctx, key, account.Settings, TTL)
	}
	return account.Settings, nil
}
