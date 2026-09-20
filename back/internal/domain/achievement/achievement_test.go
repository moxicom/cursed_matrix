package achievement_test

import (
	"testing"

	"github.com/moxicom/cursed_matrix/back/internal/domain/achievement"
)

func TestMetricsValue(t *testing.T) {
	metrics := achievement.Metrics{
		TasksCompleted:   7,
		Q1TasksCompleted: 3,
		LifetimeXP:       420,
		Level:            4,
		LongestStreak:    9,
		LinksCreated:     2,
		GraphOpenedDays:  5,
	}

	tests := []struct {
		metric  achievement.Metric
		want    int64
		wantErr bool
	}{
		{metric: achievement.MetricTasksCompleted, want: 7},
		{metric: achievement.MetricQ1TasksCompleted, want: 3},
		{metric: achievement.MetricLifetimeXP, want: 420},
		{metric: achievement.MetricLevel, want: 4},
		{metric: achievement.MetricLongestStreak, want: 9},
		{metric: achievement.MetricLinksCreated, want: 2},
		{metric: achievement.MetricGraphOpenedDays, want: 5},
		{metric: achievement.Metric("TASKS_ABANDONED"), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(string(tt.metric), func(t *testing.T) {
			got, known := metrics.Value(tt.metric)

			if tt.wantErr {
				if known {
					t.Fatalf("Value(%s) = %d, want it reported as unknown", tt.metric, got)
				}
				return
			}
			if !known {
				t.Fatalf("Value(%s) is not known", tt.metric)
			}
			if got != tt.want {
				t.Errorf("= %d, want %d", got, tt.want)
			}
		})
	}
}

func TestProgressAndEarned(t *testing.T) {
	tests := []struct {
		name         string
		threshold    int64
		completed    int64
		wantProgress int64
		wantEarned   bool
	}{
		{name: "nothing done yet", threshold: 100, completed: 0},
		{name: "part way", threshold: 100, completed: 74, wantProgress: 74},
		{name: "exactly the threshold", threshold: 100, completed: 100, wantProgress: 100, wantEarned: true},
		{
			// The client sorts by ratio, so progress never passes the
			// threshold; a value above one would sort above a completed one.
			name:         "well past it",
			threshold:    100,
			completed:    5000,
			wantProgress: 100,
			wantEarned:   true,
		},
		{name: "a threshold of one is earned by the first", threshold: 1, completed: 1, wantProgress: 1, wantEarned: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := achievement.Achievement{
				Metric:    achievement.MetricTasksCompleted,
				Threshold: tt.threshold,
			}
			metrics := achievement.Metrics{TasksCompleted: tt.completed}

			progress, known := entry.Progress(&metrics)
			if !known {
				t.Fatal("the metric is not known")
			}
			if progress != tt.wantProgress {
				t.Errorf("progress = %d, want %d", progress, tt.wantProgress)
			}

			earned, known := entry.Earned(&metrics)
			if !known {
				t.Fatal("the metric is not known")
			}
			if earned != tt.wantEarned {
				t.Errorf("earned = %v, want %v", earned, tt.wantEarned)
			}
		})
	}
}

// TestUnknownMetricIsNeverEarned is the blast radius: this runs inside the
// transaction that completed a task, so a catalogue row nothing can measure
// must cost that achievement and nothing else.
func TestUnknownMetricIsNeverEarned(t *testing.T) {
	entry := achievement.Achievement{
		Code:      "FROM_THE_FUTURE",
		Metric:    achievement.Metric("TASKS_ABANDONED"),
		Threshold: 1,
	}
	metrics := achievement.Metrics{TasksCompleted: 9000}

	progress, known := entry.Progress(&metrics)
	if known {
		t.Error("an unmeasurable metric reported itself known")
	}
	if progress != 0 {
		t.Errorf("progress = %d, want 0", progress)
	}

	earned, known := entry.Earned(&metrics)
	if earned {
		t.Error("an achievement nothing can measure was earned")
	}
	if known {
		t.Error("Earned reported an unknown metric as known")
	}
}
