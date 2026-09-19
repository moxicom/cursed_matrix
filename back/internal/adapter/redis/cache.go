// Package redis implements the cache and token-store ports on Redis.
package redis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/moxicom/cursed_matrix/back/internal/app/cache"
	"github.com/moxicom/cursed_matrix/back/internal/app/port"
)

type Options struct {
	URL        string
	DefaultTTL time.Duration
}

const keyPrefix = "cm"

const versionBeforeFirstInvalidation = 0

// Cache is a version-stamped cache-aside store.
type Cache struct {
	client     *redis.Client
	log        *slog.Logger
	defaultTTL time.Duration
}

func NewCache(ctx context.Context, opts Options, log *slog.Logger) (*Cache, error) {
	clientOpts, err := redis.ParseURL(opts.URL)
	if err != nil {
		return nil, fmt.Errorf("parse REDIS_URL: %w", err)
	}
	client := redis.NewClient(clientOpts)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return &Cache{client: client, log: log, defaultTTL: opts.DefaultTTL}, nil
}

func (c *Cache) Close() error { return c.client.Close() }

// GetInto decodes the current version of an entry into dst and reports a hit.
func (c *Cache) GetInto(ctx context.Context, key cache.Key, dst any) (bool, error) {
	version, err := c.version(ctx, key.Scope)
	if err != nil {
		c.log.Debug("cache version unavailable", "scope", key.Scope, "err", err)
		return false, nil
	}

	raw, err := c.client.Get(ctx, c.dataKey(key, version)).Bytes()
	switch {
	case err == redis.Nil:
		return false, nil
	case err != nil:
		c.log.Debug("cache read failed", "scope", key.Scope, "err", err)
		return false, nil
	}

	if err := json.Unmarshal(raw, dst); err != nil {
		c.log.Debug("cache decode failed", "scope", key.Scope, "err", err)
		return false, nil
	}
	return true, nil
}

// Set stores a value under the scope's current version.
func (c *Cache) Set(ctx context.Context, key cache.Key, value any, ttl time.Duration) error {
	version, err := c.version(ctx, key.Scope)
	if err != nil {
		c.log.Debug("cache version unavailable", "scope", key.Scope, "err", err)
		return nil
	}

	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode cache value: %w", err)
	}
	if ttl <= 0 {
		ttl = c.defaultTTL
	}
	if err := c.client.Set(ctx, c.dataKey(key, version), payload, ttl).Err(); err != nil {
		c.log.Debug("cache write failed", "scope", key.Scope, "err", err)
	}
	return nil
}

// Invalidate retires every entry in the scope.
func (c *Cache) Invalidate(ctx context.Context, scope cache.Scope) error {
	if err := c.client.Incr(ctx, c.versionKey(scope)).Err(); err != nil {
		return fmt.Errorf("invalidate %s: %w", scope, err)
	}
	return nil
}

func (c *Cache) version(ctx context.Context, scope cache.Scope) (uint64, error) {
	raw, err := c.client.Get(ctx, c.versionKey(scope)).Result()
	if err == redis.Nil {
		return versionBeforeFirstInvalidation, nil
	}
	if err != nil {
		return 0, err
	}
	version, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("cache version of %s is not a number: %q", scope, raw)
	}
	return version, nil
}

func (c *Cache) versionKey(scope cache.Scope) string {
	return keyPrefix + ":ver:" + string(scope)
}

func (c *Cache) dataKey(key cache.Key, version uint64) string {
	sum := sha256.Sum256([]byte(strings.Join(key.Args, "\x00")))
	return fmt.Sprintf("%s:%s:%d:%s:%s",
		keyPrefix, key.Scope, version, key.Name, hex.EncodeToString(sum[:8]))
}

// NoopCache satisfies port.Cache when Redis is unavailable.
type NoopCache struct{}

func (*NoopCache) GetInto(context.Context, cache.Key, any) (bool, error) { return false, nil }
func (*NoopCache) Set(context.Context, cache.Key, any, time.Duration) error {
	return nil
}
func (*NoopCache) Invalidate(context.Context, cache.Scope) error { return nil }

var (
	_ port.Cache = (*Cache)(nil)
	_ port.Cache = (*NoopCache)(nil)
)
