//go:build integration

package redis_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	redisadapter "github.com/moxicom/cursed_matrix/back/internal/adapter/redis"
	"github.com/moxicom/cursed_matrix/back/internal/app/session"
)

func newRefreshStore(t *testing.T) *redisadapter.RefreshStore {
	t.Helper()
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		t.Skip("TEST_REDIS_URL is not set")
	}
	options, err := redis.ParseURL(url)
	if err != nil {
		t.Fatalf("redis url: %v", err)
	}
	client := redis.NewClient(options)
	t.Cleanup(func() { _ = client.Close() })
	return redisadapter.NewRefreshStore(client)
}

// What a second presentation of a refresh token finds decides whether an
// account keeps its sessions, so each way a token can have left the store is
// spelled out.
func TestRefreshStoreSecondPresentation(t *testing.T) {
	const grace = 150 * time.Millisecond

	tests := []struct {
		name string
		// first is what happened to the token before it is presented again.
		first func(t *testing.T, store *redisadapter.RefreshStore, userID uuid.UUID, token string)
		wait  time.Duration
		grace time.Duration
		want  session.RefreshOutcome
	}{
		{
			name:  "never spent",
			first: func(*testing.T, *redisadapter.RefreshStore, uuid.UUID, string) {},
			grace: grace,
			want:  session.RefreshValid,
		},
		{
			name:  "spent a moment ago",
			first: consume(grace),
			grace: grace,
			want:  session.RefreshJustRotated,
		},
		{
			name:  "spent, and the grace has passed",
			first: consume(grace),
			wait:  2 * grace,
			grace: grace,
			want:  session.RefreshUnknown,
		},
		{
			name:  "spent with no grace at all",
			first: consume(0),
			grace: 0,
			want:  session.RefreshUnknown,
		},
		{
			name: "signed out",
			first: func(t *testing.T, store *redisadapter.RefreshStore, userID uuid.UUID, token string) {
				t.Helper()
				if err := store.Drop(context.Background(), userID, token); err != nil {
					t.Fatalf("Drop: %v", err)
				}
			},
			grace: grace,
			want:  session.RefreshUnknown,
		},
		{
			name: "spent, then every session revoked",
			first: func(t *testing.T, store *redisadapter.RefreshStore, userID uuid.UUID, token string) {
				t.Helper()
				consume(grace)(t, store, userID, token)
				if err := store.RevokeAll(context.Background(), userID); err != nil {
					t.Fatalf("RevokeAll: %v", err)
				}
			},
			grace: grace,
			want:  session.RefreshUnknown,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := newRefreshStore(t)
			userID, token := uuid.New(), uuid.NewString()
			if err := store.Save(context.Background(), userID, token, time.Minute); err != nil {
				t.Fatalf("Save: %v", err)
			}

			tc.first(t, store, userID, token)
			time.Sleep(tc.wait)

			got, err := store.Consume(context.Background(), userID, token, tc.grace)
			if err != nil {
				t.Fatalf("Consume: %v", err)
			}
			if got != tc.want {
				t.Errorf("outcome = %d, want %d", got, tc.want)
			}
		})
	}
}

func consume(grace time.Duration) func(*testing.T, *redisadapter.RefreshStore, uuid.UUID, string) {
	return func(t *testing.T, store *redisadapter.RefreshStore, userID uuid.UUID, token string) {
		t.Helper()
		got, err := store.Consume(context.Background(), userID, token, grace)
		if err != nil {
			t.Fatalf("first Consume: %v", err)
		}
		if got != session.RefreshValid {
			t.Fatalf("first Consume = %d, want a live token", got)
		}
	}
}
