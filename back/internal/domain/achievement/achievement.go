// Package achievement holds the catalogue and what it takes to earn an entry.
package achievement

import (
	"time"

	"github.com/google/uuid"
)

// Metric is what an achievement watches.
type Metric string

const (
	MetricTasksCompleted   Metric = "TASKS_COMPLETED"
	MetricQ1TasksCompleted Metric = "Q1_TASKS_COMPLETED"
	MetricLifetimeXP       Metric = "LIFETIME_XP"
	MetricLevel            Metric = "LEVEL"
	MetricLongestStreak    Metric = "LONGEST_STREAK"
	MetricLinksCreated     Metric = "LINKS_CREATED"
	MetricGraphOpenedDays  Metric = "GRAPH_OPENED_DAYS"
)

// Achievement is one entry of the catalogue.
//
// It carries no name or description: those are localised by the client from
// the code, so the server never has to know which language a user reads.
type Achievement struct {
	ID         uuid.UUID
	Code       string
	Category   string
	Metric     Metric
	Threshold  int64
	RewardXP   int32
	Repeatable bool
	SortOrder  int
}

// Metrics is everything the catalogue can watch, read once.
//
// One struct rather than a lookup per achievement: eight entries would
// otherwise be eight round trips on a path that runs after every completion.
type Metrics struct {
	TasksCompleted   int64
	Q1TasksCompleted int64
	LifetimeXP       int64
	Level            int64
	LongestStreak    int64
	LinksCreated     int64
	GraphOpenedDays  int64
}

// Value reads the metric an achievement watches. The second result is false
// for a metric nothing computes.
//
// Not an error, deliberately: this is read after every completion, every link
// and every streak touch, inside the caller's transaction. A catalogue row
// naming a metric this release does not know — a typo in a seed, a placeholder
// for an unreleased feature — must cost that one achievement, not the user's
// ability to finish a task.
func (m *Metrics) Value(metric Metric) (int64, bool) {
	switch metric {
	case MetricTasksCompleted:
		return m.TasksCompleted, true
	case MetricQ1TasksCompleted:
		return m.Q1TasksCompleted, true
	case MetricLifetimeXP:
		return m.LifetimeXP, true
	case MetricLevel:
		return m.Level, true
	case MetricLongestStreak:
		return m.LongestStreak, true
	case MetricLinksCreated:
		return m.LinksCreated, true
	case MetricGraphOpenedDays:
		return m.GraphOpenedDays, true
	}
	return 0, false
}

// Progress is how far the user has come, never past the threshold: the client
// sorts by completion ratio and a value above one would sort wrongly.
func (a *Achievement) Progress(metrics *Metrics) (int64, bool) {
	value, known := metrics.Value(a.Metric)
	if !known {
		return 0, false
	}
	if value > a.Threshold {
		return a.Threshold, true
	}
	return value, true
}

// Earned reports whether the condition holds now. An unknown metric is never
// earned — it cannot be measured, so it cannot have been met.
func (a *Achievement) Earned(metrics *Metrics) (bool, bool) {
	value, known := metrics.Value(a.Metric)
	return known && value >= a.Threshold, known
}

// Unlock is an achievement a user already holds.
type Unlock struct {
	AchievementID uuid.UUID
	UnlockedAt    time.Time
}
