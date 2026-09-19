//go:build integration

package httphandler_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

type progressView struct {
	Tasks     []taskView `json:"tasks"`
	XPAwarded int32      `json:"xpAwarded"`
	LevelUp   *struct {
		FromLevel int32 `json:"fromLevel"`
		ToLevel   int32 `json:"toLevel"`
	} `json:"levelUp"`
	UnlockedAchievements []string `json:"unlockedAchievements"`
}

func decodeProgress(t *testing.T, body []byte) progressView {
	t.Helper()
	var view progressView
	if err := json.Unmarshal(body, &view); err != nil {
		t.Fatalf("decode progress: %v (%s)", err, body)
	}
	return view
}

func lifetimeXP(t *testing.T, c *client) (int64, int32) {
	t.Helper()
	response := c.do(t, http.MethodGet, "/me", "")
	var me struct {
		Stats struct {
			LifetimeXp        int64 `json:"lifetimeXp"`
			Level             int32 `json:"level"`
			TasksCompleted    int32 `json:"tasksCompleted"`
			SubtasksCompleted int32 `json:"subtasksCompleted"`
		} `json:"stats"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &me); err != nil {
		t.Fatalf("me: %v", err)
	}
	return me.Stats.LifetimeXp, me.Stats.Level
}

// TestCompletionCascadeAndXP walks the whole rule set from CLAUDE.md §13, §31,
// §32 and §65: an IMPORTANT_URGENT task is worth 50, a subtask 35% of its
// parent's quadrant rounded to a whole number, completing a parent completes
// its subtasks, and the snapshot is frozen on each row.
func TestCompletionCascadeAndXP(t *testing.T) {
	c := signedInClient(t)

	created := c.do(t, http.MethodPost, "/tasks", `{"title":"ship it","quadrant":"IMPORTANT_URGENT"}`)
	parent := decodeTask(t, created.Body.Bytes())

	for _, title := range []string{"first step", "second step"} {
		response := c.do(t, http.MethodPost, "/tasks/"+parent.ID+"/subtasks", `{"title":"`+title+`"}`)
		if response.Code != http.StatusCreated {
			t.Fatalf("subtask %q = %d: %s", title, response.Code, response.Body)
		}
	}

	response := c.do(t, http.MethodPost, "/tasks/"+parent.ID+"/complete", "")
	if response.Code != http.StatusOK {
		t.Fatalf("complete = %d: %s", response.Code, response.Body)
	}
	progress := decodeProgress(t, response.Body.Bytes())

	// 50 for the task, round(50 × 0.35) = 18 for each subtask.
	const wantXP = 50 + 18 + 18

	t.Run("the parent and both subtasks come back completed", func(t *testing.T) {
		if len(progress.Tasks) != 3 {
			t.Fatalf("%d tasks changed, want 3", len(progress.Tasks))
		}
		for _, item := range progress.Tasks {
			if item.Status != "COMPLETED" {
				t.Errorf("%q = %s", item.Title, item.Status)
			}
		}
	})

	t.Run("the payout follows the configuration", func(t *testing.T) {
		if progress.XPAwarded != wantXP {
			t.Errorf("xpAwarded = %d, want %d", progress.XPAwarded, wantXP)
		}
	})

	t.Run("the snapshot is frozen on every row", func(t *testing.T) {
		var board struct {
			Tasks []struct {
				taskView
				XPAwarded            *int32  `json:"xpAwarded"`
				QuadrantAtCompletion *string `json:"quadrantAtCompletion"`
				CompletedVia         *string `json:"completedVia"`
				CompletedAt          *string `json:"completedAt"`
			} `json:"tasks"`
		}
		listed := c.do(t, http.MethodGet, "/tasks?status=COMPLETED", "")
		if err := json.Unmarshal(listed.Body.Bytes(), &board); err != nil {
			t.Fatalf("board: %v", err)
		}
		if len(board.Tasks) != 3 {
			t.Fatalf("%d completed tasks, want 3", len(board.Tasks))
		}

		var completedAt string
		for _, item := range board.Tasks {
			if item.XPAwarded == nil || item.QuadrantAtCompletion == nil || item.CompletedVia == nil {
				t.Fatalf("%q has an incomplete snapshot: %+v", item.Title, item)
			}
			// A subtask is priced by its parent's quadrant, and records that
			// quadrant even though it has none of its own.
			if *item.QuadrantAtCompletion != "IMPORTANT_URGENT" {
				t.Errorf("%q quadrantAtCompletion = %s", item.Title, *item.QuadrantAtCompletion)
			}

			want := int32(50)
			via := "DIRECT"
			if item.ParentTaskID != nil {
				want, via = 18, "PARENT_CASCADE"
			}
			if *item.XPAwarded != want {
				t.Errorf("%q xpAwarded = %d, want %d", item.Title, *item.XPAwarded, want)
			}
			if *item.CompletedVia != via {
				t.Errorf("%q completedVia = %s, want %s", item.Title, *item.CompletedVia, via)
			}

			// The cascade is one event, so every row shares its instant.
			if completedAt == "" {
				completedAt = *item.CompletedAt
			} else if *item.CompletedAt != completedAt {
				t.Errorf("%q completedAt = %s, want %s", item.Title, *item.CompletedAt, completedAt)
			}
		}
	})

	t.Run("the account totals moved and the level followed", func(t *testing.T) {
		xp, level := lifetimeXP(t, c)
		if xp != wantXP {
			t.Errorf("lifetimeXp = %d, want %d", xp, wantXP)
		}
		// 86 XP is level 2 by floor(sqrt(xp/45)) + 1.
		if level != 2 {
			t.Errorf("level = %d, want 2", level)
		}
		if progress.LevelUp == nil || progress.LevelUp.FromLevel != 1 || progress.LevelUp.ToLevel != 2 {
			t.Errorf("levelUp = %+v, want 1 → 2", progress.LevelUp)
		}
	})

	t.Run("completing it again is refused", func(t *testing.T) {
		again := c.do(t, http.MethodPost, "/tasks/"+parent.ID+"/complete", "")
		if again.Code != http.StatusConflict {
			t.Fatalf("status = %d, want 409: %s", again.Code, again.Body)
		}
	})

	t.Run("reopening withdraws only the parent's own XP", func(t *testing.T) {
		response := c.do(t, http.MethodPost, "/tasks/"+parent.ID+"/reopen", "")
		if response.Code != http.StatusOK {
			t.Fatalf("reopen = %d: %s", response.Code, response.Body)
		}
		reopened := decodeProgress(t, response.Body.Bytes())

		if reopened.XPAwarded != -50 {
			t.Errorf("xpAwarded = %d, want -50", reopened.XPAwarded)
		}
		if len(reopened.Tasks) != 1 || reopened.Tasks[0].Status != "ACTIVE" {
			t.Fatalf("tasks = %+v, want only the parent, active", reopened.Tasks)
		}

		xp, level := lifetimeXP(t, c)
		if xp != wantXP-50 {
			t.Errorf("lifetimeXp = %d, want %d", xp, wantXP-50)
		}
		if level != 1 {
			t.Errorf("level = %d, want 1 after the withdrawal", level)
		}
	})

	t.Run("the subtasks stay completed", func(t *testing.T) {
		listed := c.do(t, http.MethodGet, "/tasks?status=COMPLETED", "")
		var board struct {
			Tasks []taskView `json:"tasks"`
		}
		if err := json.Unmarshal(listed.Body.Bytes(), &board); err != nil {
			t.Fatalf("board: %v", err)
		}
		if len(board.Tasks) != 2 {
			t.Fatalf("%d tasks still completed, want the two subtasks", len(board.Tasks))
		}
	})

	t.Run("finishing it again earns the XP again", func(t *testing.T) {
		response := c.do(t, http.MethodPost, "/tasks/"+parent.ID+"/complete", "")
		if response.Code != http.StatusOK {
			t.Fatalf("complete = %d: %s", response.Code, response.Body)
		}
		// Only the parent was active, so only the parent is paid: the ledger's
		// grant-once index must not refuse the second honest completion.
		again := decodeProgress(t, response.Body.Bytes())
		if again.XPAwarded != 50 {
			t.Errorf("xpAwarded = %d, want 50", again.XPAwarded)
		}

		xp, _ := lifetimeXP(t, c)
		if xp != wantXP {
			t.Errorf("lifetimeXp = %d, want %d", xp, wantXP)
		}
	})
}

func TestReopeningSomethingActiveIsRefused(t *testing.T) {
	c := signedInClient(t)
	created := c.do(t, http.MethodPost, "/tasks", `{"title":"still going","quadrant":"NOT_IMPORTANT_URGENT"}`)
	item := decodeTask(t, created.Body.Bytes())

	response := c.do(t, http.MethodPost, "/tasks/"+item.ID+"/reopen", "")
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422: %s", response.Code, response.Body)
	}
}
