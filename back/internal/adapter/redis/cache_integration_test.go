//go:build integration

package redis_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	redisadapter "github.com/moxicom/cursed_matrix/back/internal/adapter/redis"
	"github.com/moxicom/cursed_matrix/back/internal/app/cache"
	"github.com/moxicom/cursed_matrix/back/internal/app/port"
)

func newCache(t *testing.T) *redisadapter.Cache {
	t.Helper()
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		t.Skip("TEST_REDIS_URL is not set")
	}
	store, err := redisadapter.NewCache(context.Background(), redisadapter.Options{
		URL:        url,
		DefaultTTL: time.Minute,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func freshKey() cache.Key {
	return cache.Key{
		Scope: cache.Scope("test:" + uuid.NewString()),
		Name:  "board",
		Args:  []string{"active"},
	}
}

type payload struct {
	Title string `json:"title"`
}

func TestCacheHitsAndMisses(t *testing.T) {
	tests := []struct {
		name string
		// written says whether the entry is stored before reading it.
		written bool
		// invalidations happen after the write and before the read.
		invalidations int
		wantHit       bool
	}{
		{name: "never written", written: false, wantHit: false},
		{name: "written, not invalidated", written: true, wantHit: true},
		{name: "written, invalidated once", written: true, invalidations: 1, wantHit: false},
		{name: "written, invalidated twice", written: true, invalidations: 2, wantHit: false},
		{name: "written, invalidated three times", written: true, invalidations: 3, wantHit: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			store := newCache(t)
			key := freshKey()

			if tc.written {
				if err := store.Set(ctx, key, payload{Title: "stored"}, time.Minute); err != nil {
					t.Fatalf("Set: %v", err)
				}
			}
			for i := 0; i < tc.invalidations; i++ {
				if err := store.Invalidate(ctx, key.Scope); err != nil {
					t.Fatalf("Invalidate: %v", err)
				}
			}

			var got payload
			hit, err := store.GetInto(ctx, key, &got)
			if err != nil {
				t.Fatalf("GetInto: %v", err)
			}
			if hit != tc.wantHit {
				t.Fatalf("hit = %v, want %v (value %+v)", hit, tc.wantHit, got)
			}
			if hit && got.Title != "stored" {
				t.Errorf("value = %+v, want the stored one", got)
			}
		})
	}
}

func TestInvalidationIsScopedToOneScope(t *testing.T) {
	ctx := context.Background()
	store := newCache(t)
	mine, theirs := freshKey(), freshKey()

	for _, key := range []cache.Key{mine, theirs} {
		if err := store.Set(ctx, key, payload{Title: string(key.Scope)}, time.Minute); err != nil {
			t.Fatalf("Set: %v", err)
		}
	}
	if err := store.Invalidate(ctx, mine.Scope); err != nil {
		t.Fatalf("Invalidate: %v", err)
	}

	tests := []struct {
		name    string
		key     cache.Key
		wantHit bool
	}{
		{name: "the invalidated scope misses", key: mine, wantHit: false},
		{name: "the untouched scope still hits", key: theirs, wantHit: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got payload
			hit, err := store.GetInto(ctx, tc.key, &got)
			if err != nil {
				t.Fatalf("GetInto: %v", err)
			}
			if hit != tc.wantHit {
				t.Errorf("hit = %v, want %v", hit, tc.wantHit)
			}
		})
	}
}

func TestNoopCacheAlwaysMisses(t *testing.T) {
	ctx := context.Background()
	var store port.Cache = &redisadapter.NoopCache{}
	key := freshKey()

	tests := []struct {
		name string
		call func() error
	}{
		{name: "set", call: func() error { return store.Set(ctx, key, payload{Title: "ignored"}, time.Minute) }},
		{name: "invalidate", call: func() error { return store.Invalidate(ctx, key.Scope) }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); err != nil {
				t.Errorf("%s returned %v, want nil", tc.name, err)
			}
		})
	}

	var got payload
	hit, err := store.GetInto(ctx, key, &got)
	if err != nil || hit {
		t.Fatalf("NoopCache reported hit = %v, err = %v", hit, err)
	}
}
