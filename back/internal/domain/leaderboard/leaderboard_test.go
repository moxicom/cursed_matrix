package leaderboard_test

import (
	"testing"
	"time"

	"github.com/moxicom/cursed_matrix/back/internal/domain/leaderboard"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
)

func TestPeriodWindow(t *testing.T) {
	tests := []struct {
		name     string
		period   leaderboard.Period
		now      time.Time
		wantFrom string
		wantOK   bool
	}{
		{
			name:     "a week starts on Monday",
			period:   leaderboard.PeriodWeek,
			now:      time.Date(2026, 9, 20, 15, 0, 0, 0, time.UTC), // Sunday
			wantFrom: "2026-09-14",
			wantOK:   true,
		},
		{
			name:     "Monday is its own week's first day",
			period:   leaderboard.PeriodWeek,
			now:      time.Date(2026, 9, 14, 0, 30, 0, 0, time.UTC),
			wantFrom: "2026-09-14",
			wantOK:   true,
		},
		{
			name:     "a month starts on the first",
			period:   leaderboard.PeriodMonth,
			now:      time.Date(2026, 9, 20, 15, 0, 0, 0, time.UTC),
			wantFrom: "2026-09-01",
			wantOK:   true,
		},
		{
			name:   "all time has no start",
			period: leaderboard.PeriodAllTime,
			now:    time.Date(2026, 9, 20, 15, 0, 0, 0, time.UTC),
		},
		{
			// The ranking is computed in one shared timezone, so a caller in
			// another one gets the same window.
			name:     "the window is UTC whatever the caller passes",
			period:   leaderboard.PeriodWeek,
			now:      time.Date(2026, 9, 21, 2, 0, 0, 0, time.FixedZone("UTC+9", 9*3600)),
			wantFrom: "2026-09-14",
			wantOK:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			from, ok := tt.period.Window(tt.now)

			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if got := from.Format(time.DateOnly); got != tt.wantFrom {
				t.Errorf("from = %s, want %s", got, tt.wantFrom)
			}
			if from.Location() != time.UTC {
				t.Errorf("location = %v, want UTC", from.Location())
			}
		})
	}
}

func TestParsePeriod(t *testing.T) {
	tests := []struct {
		value   string
		want    leaderboard.Period
		wantErr bool
	}{
		{value: "WEEK", want: leaderboard.PeriodWeek},
		{value: "MONTH", want: leaderboard.PeriodMonth},
		{value: "ALL_TIME", want: leaderboard.PeriodAllTime},
		{value: "", want: leaderboard.PeriodAllTime},
		{value: "YEAR", wantErr: true},
		{value: "week", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got, err := leaderboard.ParsePeriod(tt.value)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParsePeriod(%q) = %q, want an error", tt.value, got)
				}
				if shared.CodeOf(err) != shared.CodeValidationFailed {
					t.Errorf("code = %s", shared.CodeOf(err))
				}
				return
			}
			if err != nil {
				t.Fatalf("ParsePeriod(%q): %v", tt.value, err)
			}
			if got != tt.want {
				t.Errorf("= %q, want %q", got, tt.want)
			}
		})
	}
}
