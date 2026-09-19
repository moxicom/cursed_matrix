package redis

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/moxicom/cursed_matrix/back/internal/app/port"
)

// RateLimiter counts attempts in a fixed window.
//
// Fixed rather than sliding on purpose: it is two commands and no stored
// history, and the worst it allows is a burst across a window boundary —
// acceptable for slowing password guessing, where the point is that an attacker
// gets tens of attempts per hour rather than millions.
type RateLimiter struct {
	client *redis.Client
	log    *slog.Logger
}

// NewRateLimiter builds the limiter over an existing client.
func NewRateLimiter(client *redis.Client, log *slog.Logger) *RateLimiter {
	return &RateLimiter{client: client, log: log}
}

// Allow counts one attempt. A Redis failure allows the request: a limiter that
// closes the door when its own store is down turns a cache outage into an
// outage of the product.
func (l *RateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, time.Duration, error) {
	full := fmt.Sprintf("%s:ratelimit:%s", keyPrefix, key)

	count, err := l.client.Incr(ctx, full).Result()
	if err != nil {
		l.log.WarnContext(ctx, "rate limiting is not counting", "err", err)
		return true, 0, nil
	}

	// The window starts with the first attempt in it. Every later call checks
	// that the key still has an expiry: an EXPIRE that failed once would
	// otherwise leave a counter that never resets, turning a momentary Redis
	// hiccup into a permanent refusal for that address or account.
	if err := l.ensureWindow(ctx, full, window, count == 1); err != nil {
		l.log.WarnContext(ctx, "rate limit window not set", "err", err)
		return true, 0, nil
	}

	if count <= int64(limit) {
		return true, 0, nil
	}

	retryAfter, err := l.client.TTL(ctx, full).Result()
	if err != nil || retryAfter < 0 {
		retryAfter = window
	}
	return false, retryAfter, nil
}

func (l *RateLimiter) ensureWindow(ctx context.Context, key string, window time.Duration, first bool) error {
	if first {
		return l.client.Expire(ctx, key, window).Err()
	}

	ttl, err := l.client.TTL(ctx, key).Result()
	if err != nil {
		return err
	}
	if ttl < 0 {
		return l.client.Expire(ctx, key, window).Err()
	}
	return nil
}

var _ port.RateLimiter = (*RateLimiter)(nil)
