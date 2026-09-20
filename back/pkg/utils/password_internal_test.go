package utils

import (
	"context"
	"errors"
	"testing"
	"time"
)

// The slot count is what stands between a burst of sign-ins and this process
// asking the machine for 64 MiB per caller at once. A limiter that handed out
// more than it has, or never took one back, would be invisible until the day it
// mattered.
func TestHashLimiter(t *testing.T) {
	tests := []struct {
		name string
		size int
		// take is how many slots are held when the measured call is made.
		take    int
		release int
		wantErr error
	}{
		{name: "a free slot is handed out", size: 2, take: 0, wantErr: nil},
		{name: "the last free slot is handed out", size: 2, take: 1, wantErr: nil},
		{name: "a full limiter refuses rather than queues", size: 2, take: 2, wantErr: ErrHashBusy},
		{name: "a released slot is handed out again", size: 1, take: 1, release: 1, wantErr: nil},
		{name: "one slot means one hash at a time", size: 1, take: 1, wantErr: ErrHashBusy},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Short, because a case that expects a refusal waits the whole
			// budget out before it gets one.
			limiter := newHashLimiter(tt.size, 20*time.Millisecond)

			for held := 0; held < tt.take; held++ {
				if err := limiter.acquire(t.Context()); err != nil {
					t.Fatalf("holding slot %d: %v", held+1, err)
				}
			}
			for returned := 0; returned < tt.release; returned++ {
				limiter.release()
			}

			if err := limiter.acquire(t.Context()); !errors.Is(err, tt.wantErr) {
				t.Fatalf("acquire = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// A caller that goes away while waiting must not be told the server is busy:
// it is the caller that stopped, and the difference decides whether the
// refusal is reported as load or as a cancelled request.
func TestHashLimiterReportsCancellationAsItself(t *testing.T) {
	limiter := newHashLimiter(1, time.Minute)
	if err := limiter.acquire(t.Context()); err != nil {
		t.Fatalf("first acquire: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if err := limiter.acquire(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("acquire = %v, want context.Canceled", err)
	}
}
