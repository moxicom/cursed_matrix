package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/moxicom/cursed_matrix/back/internal/app/port"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
)

// RefreshStore keeps the revocable half of the session in Redis: one key per
// issued token, and a generation counter per account that revokes them all at
// once.
type RefreshStore struct {
	client *redis.Client
}

// NewRefreshStore builds the store over an existing client.
func NewRefreshStore(client *redis.Client) *RefreshStore {
	return &RefreshStore{client: client}
}

// Save records an issued refresh token under the account's current generation.
func (s *RefreshStore) Save(ctx context.Context, userID uuid.UUID, tokenID string, ttl time.Duration) error {
	generation, err := s.generation(ctx, userID)
	if err != nil {
		return err
	}
	if err := s.client.Set(ctx, s.tokenKey(userID, generation, tokenID), "1", ttl).Err(); err != nil {
		return fmt.Errorf("save refresh token: %w", err)
	}
	return nil
}

// Consume deletes the token and reports whether it was still valid. Redis DEL
// returns how many keys it removed, which makes this atomic: two concurrent
// refreshes cannot both see a valid token.
func (s *RefreshStore) Consume(ctx context.Context, userID uuid.UUID, tokenID string) (bool, error) {
	generation, err := s.generation(ctx, userID)
	if err != nil {
		return false, err
	}

	removed, err := s.client.Del(ctx, s.tokenKey(userID, generation, tokenID)).Result()
	if err != nil {
		return false, fmt.Errorf("consume refresh token: %w", err)
	}
	return removed == 1, nil
}

// RevokeAll moves the account to a new generation, which orphans every token
// issued under the old one in a single operation.
func (s *RefreshStore) RevokeAll(ctx context.Context, userID uuid.UUID) error {
	if err := s.client.Incr(ctx, s.generationKey(userID)).Err(); err != nil {
		return fmt.Errorf("revoke refresh tokens: %w", err)
	}
	return nil
}

func (s *RefreshStore) generation(ctx context.Context, userID uuid.UUID) (int64, error) {
	value, err := s.client.Get(ctx, s.generationKey(userID)).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read token generation: %w", err)
	}
	return value, nil
}

func (s *RefreshStore) generationKey(userID uuid.UUID) string {
	return fmt.Sprintf("%s:refresh-generation:%s", keyPrefix, userID)
}

func (s *RefreshStore) tokenKey(userID uuid.UUID, generation int64, tokenID string) string {
	return fmt.Sprintf("%s:refresh:%s:%d:%s", keyPrefix, userID, generation, tokenID)
}

var _ port.RefreshStore = (*RefreshStore)(nil)

// BlockAccess marks an account's access tokens unusable.
//
// A refresh token can be revoked because the store holds it; an access token
// cannot, because nothing holds it — it is believed on its signature alone.
// Closing an account therefore needs somewhere to say so for as long as the
// last issued token could still be presented, which is what this is.
func (s *RefreshStore) BlockAccess(ctx context.Context, userID uuid.UUID, ttl time.Duration) error {
	if err := s.client.Set(ctx, s.blockKey(userID), 1, ttl).Err(); err != nil {
		return shared.WrapError(err, shared.CodeInternal, nil)
	}
	return nil
}

// AccessBlocked reports whether the account's tokens have been disowned.
func (s *RefreshStore) AccessBlocked(ctx context.Context, userID uuid.UUID) (bool, error) {
	found, err := s.client.Exists(ctx, s.blockKey(userID)).Result()
	if err != nil {
		return false, shared.WrapError(err, shared.CodeInternal, nil)
	}
	return found == 1, nil
}

func (s *RefreshStore) blockKey(userID uuid.UUID) string {
	return keyPrefix + ":blocked:" + userID.String()
}
