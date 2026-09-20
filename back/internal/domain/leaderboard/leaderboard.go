// Package leaderboard holds the public ranking and the rules for who appears
// in it.
package leaderboard

import (
	"time"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
)

// Period is the window a ranking covers.
type Period string

const (
	PeriodWeek    Period = "WEEK"
	PeriodMonth   Period = "MONTH"
	PeriodAllTime Period = "ALL_TIME"
)

func (p *Period) Valid() bool {
	return *p == PeriodWeek || *p == PeriodMonth || *p == PeriodAllTime
}

// MaxPage is what one request may return.
const MaxPage = 50

// Entry is one row of the ranking.
type Entry struct {
	Rank          int
	UserID        uuid.UUID
	Username      string
	AvatarURL     *string
	XP            int64
	Level         int32
	CurrentStreak int32
}

// Standing is where the asking user sits, which the screen shows even when
// they are off the page.
//
// Visible false means they chose not to appear, and then the rank is not
// merely unknown — publicly it does not exist.
type Standing struct {
	Visible bool
	Rank    int
	XP      int64
}

// Window is the span a period covers, ending at the instant asked about.
//
// Weekly and monthly figures are computed in one shared timezone rather than
// each user's: a ranking where two people's weeks start at different moments
// is not a ranking of the same thing.
func (p *Period) Window(now time.Time) (from time.Time, ok bool) {
	utc := now.UTC()

	switch *p {
	case PeriodWeek:
		// ISO weeks start on Monday; Go counts Sunday as zero.
		offset := (int(utc.Weekday()) + 6) % 7
		day := time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
		return day.AddDate(0, 0, -offset), true
	case PeriodMonth:
		return time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC), true
	case PeriodAllTime:
		return time.Time{}, false
	}
	return time.Time{}, false
}

// ParsePeriod refuses a window nobody defined rather than quietly ranking by
// something else.
func ParsePeriod(value string) (Period, error) {
	period := Period(value)
	if value == "" {
		return PeriodAllTime, nil
	}
	if !period.Valid() {
		return "", shared.NewError(shared.CodeValidationFailed,
			map[string]any{"field": "period", "value": value})
	}
	return period, nil
}
