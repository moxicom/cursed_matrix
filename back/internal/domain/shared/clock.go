package shared

import "time"

type Clock interface {
	Now() time.Time
}

// SystemClock reads the wall clock in UTC.
type SystemClock struct{}

func (*SystemClock) Now() time.Time { return time.Now().UTC() }

type FixedClock struct{ Instant time.Time }

func (c *FixedClock) Now() time.Time { return c.Instant }
