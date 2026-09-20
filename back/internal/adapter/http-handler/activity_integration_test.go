//go:build integration

package httphandler_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

type heatmapView struct {
	Days []struct {
		Date           string `json:"date"`
		CreatedCount   int    `json:"createdCount"`
		CompletedCount int    `json:"completedCount"`
		TotalActivity  int    `json:"totalActivity"`
	} `json:"days"`
	Totals struct {
		Created   int `json:"created"`
		Completed int `json:"completed"`
	} `json:"totals"`
}

func heatmap(t *testing.T, c *client) heatmapView {
	t.Helper()
	response := c.do(t, http.MethodGet, "/activity/heatmap", "")
	if response.Code != http.StatusOK {
		t.Fatalf("heatmap = %d: %s", response.Code, response.Body)
	}
	var view heatmapView
	if err := json.Unmarshal(response.Body.Bytes(), &view); err != nil {
		t.Fatalf("heatmap: %v", err)
	}
	return view
}

func TestActivityIsRecorded(t *testing.T) {
	c := signedInClient(t)

	t.Run("a fresh account has a full, empty grid", func(t *testing.T) {
		view := heatmap(t, c)
		if len(view.Days) != 365 {
			t.Fatalf("%d days, want 365 — the client draws a fixed grid", len(view.Days))
		}
		if view.Totals.Created != 0 || view.Totals.Completed != 0 {
			t.Errorf("totals = %+v, want empty", view.Totals)
		}

		// Consecutive, ascending, ending today: a gap would move every square
		// after it.
		for i := 1; i < len(view.Days); i++ {
			before, _ := time.Parse(time.DateOnly, view.Days[i-1].Date)
			this, _ := time.Parse(time.DateOnly, view.Days[i].Date)
			if this.Sub(before) != 24*time.Hour {
				t.Fatalf("%s follows %s", view.Days[i].Date, view.Days[i-1].Date)
			}
		}
	})

	parent := newTask(t, c, "counted", "IMPORTANT_URGENT")
	response := c.do(t, http.MethodPost, "/tasks/"+parent.ID+"/subtasks", `{"title":"a step"}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("subtask = %d: %s", response.Code, response.Body)
	}

	t.Run("creating counts on today's square", func(t *testing.T) {
		view := heatmap(t, c)
		today := view.Days[len(view.Days)-1]

		if today.CreatedCount != 2 {
			t.Errorf("createdCount = %d, want 2 (the task and its subtask)", today.CreatedCount)
		}
		if view.Totals.Created != 2 {
			t.Errorf("totals.created = %d, want 2", view.Totals.Created)
		}
	})

	t.Run("completing counts the cascade", func(t *testing.T) {
		if done := c.do(t, http.MethodPost, "/tasks/"+parent.ID+"/complete", ""); done.Code != http.StatusOK {
			t.Fatalf("complete = %d: %s", done.Code, done.Body)
		}

		view := heatmap(t, c)
		today := view.Days[len(view.Days)-1]

		if today.CompletedCount != 2 {
			t.Errorf("completedCount = %d, want 2 (the parent and the cascaded subtask)", today.CompletedCount)
		}
		if today.TotalActivity != 4 {
			t.Errorf("totalActivity = %d, want 4", today.TotalActivity)
		}
	})

	t.Run("levelling up does not colour the day a second time", func(t *testing.T) {
		// The level-up is recorded, but the heatmap counts work, and the work
		// that caused it is already counted.
		view := heatmap(t, c)
		today := view.Days[len(view.Days)-1]
		if today.TotalActivity != 4 {
			t.Errorf("totalActivity = %d, want the two creations and two completions only", today.TotalActivity)
		}
	})

	t.Run("opening the graph is noted but not counted", func(t *testing.T) {
		if response := c.do(t, http.MethodPost, "/activity/graph-opened", ""); response.Code != http.StatusNoContent {
			t.Fatalf("graph-opened = %d: %s", response.Code, response.Body)
		}

		view := heatmap(t, c)
		if today := view.Days[len(view.Days)-1]; today.TotalActivity != 4 {
			t.Errorf("totalActivity = %d, want the graph visit left out", today.TotalActivity)
		}
	})
}

// TestStreakStartsOnTheFirstVisit is CLAUDE.md §36: the streak follows from
// opening the app, not from finishing anything.
func TestStreakStartsOnTheFirstVisit(t *testing.T) {
	c := signedInClient(t)

	stats := func() (int32, int32) {
		t.Helper()
		response := c.do(t, http.MethodGet, "/me", "")
		var me struct {
			Stats struct {
				CurrentStreak int32 `json:"currentStreak"`
				LongestStreak int32 `json:"longestStreak"`
			} `json:"stats"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &me); err != nil {
			t.Fatalf("me: %v", err)
		}
		return me.Stats.CurrentStreak, me.Stats.LongestStreak
	}

	current, longest := stats()
	if current != 1 || longest != 1 {
		t.Fatalf("streak = %d/%d, want 1/1 after the first visit", current, longest)
	}

	// Visiting again the same day must not count twice.
	for range 3 {
		c.do(t, http.MethodGet, "/tasks", "")
	}
	current, longest = stats()
	if current != 1 || longest != 1 {
		t.Errorf("streak = %d/%d, want the day counted once", current, longest)
	}
}

// TestGraphOpenedCountsDaysNotClicks is what the exploration achievements
// rest on: "open the graph on fourteen days" must not be unlockable in
// fourteen seconds.
func TestGraphOpenedCountsDaysNotClicks(t *testing.T) {
	c := signedInClient(t)

	for range 5 {
		if response := c.do(t, http.MethodPost, "/activity/graph-opened", ""); response.Code != http.StatusNoContent {
			t.Fatalf("graph-opened = %d: %s", response.Code, response.Body)
		}
	}

	// Nothing in the HTTP surface exposes the count, so the proof is that the
	// repeats neither failed nor changed the heatmap, and the achievement
	// evaluator will read one row per day.
	view := heatmap(t, c)
	if today := view.Days[len(view.Days)-1]; today.TotalActivity != 0 {
		t.Errorf("totalActivity = %d, want the graph visits left out entirely", today.TotalActivity)
	}
}

func TestActivityEventsAndStats(t *testing.T) {
	c := signedInClient(t)

	parent := newTask(t, c, "counted", "IMPORTANT_URGENT")
	response := c.do(t, http.MethodPost, "/tasks/"+parent.ID+"/subtasks", `{"title":"a step"}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("subtask = %d: %s", response.Code, response.Body)
	}
	if done := c.do(t, http.MethodPost, "/tasks/"+parent.ID+"/complete", ""); done.Code != http.StatusOK {
		t.Fatalf("complete = %d: %s", done.Code, done.Body)
	}

	type eventView struct {
		ID         string         `json:"id"`
		Type       string         `json:"type"`
		OccurredAt string         `json:"occurredAt"`
		LocalDate  string         `json:"localDate"`
		TaskID     *string        `json:"taskId"`
		TaskTitle  *string        `json:"taskTitle"`
		Detail     map[string]any `json:"detail"`
	}
	read := func(query string) ([]eventView, *string) {
		t.Helper()
		response := c.do(t, http.MethodGet, "/activity/events"+query, "")
		if response.Code != http.StatusOK {
			t.Fatalf("events%s = %d: %s", query, response.Code, response.Body)
		}
		var body struct {
			Items      []eventView `json:"items"`
			NextCursor *string     `json:"nextCursor"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatalf("events: %v", err)
		}
		return body.Items, body.NextCursor
	}

	t.Run("the history is newest first and names its tasks", func(t *testing.T) {
		items, _ := read("")
		if len(items) < 5 {
			t.Fatalf("%d events, want the streak, two creations, two completions and the level up", len(items))
		}

		var completion eventView
		for _, item := range items {
			if item.Type == "TASK_COMPLETED" {
				completion = item
			}
		}
		if completion.TaskID == nil || *completion.TaskID != parent.ID {
			t.Fatalf("completion = %+v", completion)
		}
		if completion.TaskTitle == nil || *completion.TaskTitle != "counted" {
			t.Errorf("taskTitle = %v, want the row to name its task", completion.TaskTitle)
		}
		// Parameters, not a sentence: the client renders it in either
		// language.
		if completion.Detail["xp"] != float64(50) {
			t.Errorf("detail = %v, want the XP among the parameters", completion.Detail)
		}
		if completion.Detail["via"] != "DIRECT" {
			t.Errorf("via = %v", completion.Detail["via"])
		}

		for i := 1; i < len(items); i++ {
			if items[i].OccurredAt > items[i-1].OccurredAt {
				t.Fatalf("out of order at %d", i)
			}
		}
	})

	t.Run("the cursor walks the history without repeating a row", func(t *testing.T) {
		first, cursor := read("?limit=2")
		if len(first) != 2 || cursor == nil {
			t.Fatalf("%d items, cursor %v", len(first), cursor)
		}

		second, _ := read("?limit=2&cursor=" + *cursor)
		if len(second) == 0 {
			t.Fatal("the second page is empty")
		}
		for _, a := range first {
			for _, b := range second {
				if a.ID == b.ID {
					t.Fatalf("event %s appeared on both pages", a.ID)
				}
			}
		}
	})

	t.Run("a cursor that is not one is refused", func(t *testing.T) {
		response := c.do(t, http.MethodGet, "/activity/events?cursor=not-a-cursor", "")
		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422: %s", response.Code, response.Body)
		}
	})

	t.Run("the stats agree with the work", func(t *testing.T) {
		response := c.do(t, http.MethodGet, "/activity/stats", "")
		if response.Code != http.StatusOK {
			t.Fatalf("stats = %d: %s", response.Code, response.Body)
		}

		var stats struct {
			CreatedLastYear   int64 `json:"createdLastYear"`
			CompletedLastYear int64 `json:"completedLastYear"`
			CurrentStreak     int32 `json:"currentStreak"`
			LongestStreak     int32 `json:"longestStreak"`
			LifetimeXP        int64 `json:"lifetimeXp"`
			Level             int32 `json:"level"`
			ActiveTasks       int64 `json:"activeTasks"`
			ArchivedTasks     int64 `json:"archivedTasks"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &stats); err != nil {
			t.Fatalf("stats: %v", err)
		}

		if stats.CreatedLastYear != 2 {
			t.Errorf("createdLastYear = %d, want 2", stats.CreatedLastYear)
		}
		if stats.CompletedLastYear != 2 {
			t.Errorf("completedLastYear = %d, want the parent and the cascade", stats.CompletedLastYear)
		}
		// 50 for the task and round(50 × 0.35) for the subtask.
		if stats.LifetimeXP != 68 {
			t.Errorf("lifetimeXp = %d, want 68", stats.LifetimeXP)
		}
		if stats.ActiveTasks != 0 || stats.ArchivedTasks != 2 {
			t.Errorf("active/archived = %d/%d, want 0/2", stats.ActiveTasks, stats.ArchivedTasks)
		}
		if stats.CurrentStreak != 1 || stats.LongestStreak != 1 {
			t.Errorf("streak = %d/%d, want 1/1", stats.CurrentStreak, stats.LongestStreak)
		}
	})
}
