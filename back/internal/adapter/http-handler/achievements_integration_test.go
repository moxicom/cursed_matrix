//go:build integration

package httphandler_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

type achievementView struct {
	Code       string  `json:"code"`
	Category   string  `json:"category"`
	Threshold  int64   `json:"threshold"`
	Progress   int64   `json:"progress"`
	RewardXP   int32   `json:"rewardXp"`
	UnlockedAt *string `json:"unlockedAt"`
}

func achievements(t *testing.T, c *client) map[string]achievementView {
	t.Helper()
	response := c.do(t, http.MethodGet, "/achievements", "")
	if response.Code != http.StatusOK {
		t.Fatalf("achievements = %d: %s", response.Code, response.Body)
	}
	var body struct {
		Items []achievementView `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("achievements: %v", err)
	}

	byCode := make(map[string]achievementView, len(body.Items))
	for _, item := range body.Items {
		byCode[item.Code] = item
	}
	return byCode
}

func TestAchievementsThroughTheRouter(t *testing.T) {
	c := signedInClient(t)

	t.Run("the whole catalogue is returned, earned or not", func(t *testing.T) {
		all := achievements(t, c)
		if len(all) != 8 {
			t.Fatalf("%d achievements, want the eight the migration seeds", len(all))
		}
		for code, item := range all {
			if item.UnlockedAt != nil {
				t.Errorf("%s is already unlocked on a fresh account", code)
			}
			// A fresh account is level 1 with a streak of 1 — the visit that
			// fetched this counted — so those two start above zero, and
			// nothing starts at or past its threshold.
			if item.Progress < 0 || item.Progress >= item.Threshold {
				t.Errorf("%s progress = %d of %d", code, item.Progress, item.Threshold)
			}
			if item.Threshold <= 0 {
				t.Errorf("%s threshold = %d", code, item.Threshold)
			}
		}
		if all["FIRST_BLOOD"].Progress != 0 {
			t.Errorf("FIRST_BLOOD progress = %d before any task", all["FIRST_BLOOD"].Progress)
		}
		// No name and no description: the client localises the code.
		if all["FIRST_BLOOD"].Category != "TASKS" {
			t.Errorf("category = %q", all["FIRST_BLOOD"].Category)
		}
	})

	item := newTask(t, c, "the first one", "IMPORTANT_URGENT")

	t.Run("the completion that earns it says so", func(t *testing.T) {
		response := c.do(t, http.MethodPost, "/tasks/"+item.ID+"/complete", "")
		if response.Code != http.StatusOK {
			t.Fatalf("complete = %d: %s", response.Code, response.Body)
		}

		progress := decodeProgress(t, response.Body.Bytes())
		var found bool
		for _, code := range progress.UnlockedAchievements {
			if code == "FIRST_BLOOD" {
				found = true
			}
		}
		if !found {
			t.Fatalf("unlockedAchievements = %v, want FIRST_BLOOD", progress.UnlockedAchievements)
		}
	})

	t.Run("and it stays unlocked", func(t *testing.T) {
		all := achievements(t, c)
		if all["FIRST_BLOOD"].UnlockedAt == nil {
			t.Error("FIRST_BLOOD came back locked")
		}
		if all["FIRST_BLOOD"].Progress != 1 {
			t.Errorf("progress = %d, want 1", all["FIRST_BLOOD"].Progress)
		}
		// The Q1 achievement moved too: the task was IMPORTANT_URGENT.
		if all["FIREFIGHTER"].Progress != 1 {
			t.Errorf("FIREFIGHTER progress = %d, want 1", all["FIREFIGHTER"].Progress)
		}
		if all["FIREFIGHTER"].UnlockedAt != nil {
			t.Error("FIREFIGHTER unlocked at one task of twenty-five")
		}
	})

	t.Run("it is not announced twice", func(t *testing.T) {
		second := newTask(t, c, "the second one", "IMPORTANT_URGENT")
		response := c.do(t, http.MethodPost, "/tasks/"+second.ID+"/complete", "")
		if response.Code != http.StatusOK {
			t.Fatalf("complete = %d: %s", response.Code, response.Body)
		}

		progress := decodeProgress(t, response.Body.Bytes())
		for _, code := range progress.UnlockedAchievements {
			if code == "FIRST_BLOOD" {
				t.Fatal("FIRST_BLOOD was announced a second time")
			}
		}
	})

	t.Run("the profile counts what was unlocked", func(t *testing.T) {
		response := c.do(t, http.MethodGet, "/me", "")
		var me struct {
			Stats struct {
				AchievementsUnlocked int32 `json:"achievementsUnlocked"`
			} `json:"stats"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &me); err != nil {
			t.Fatalf("me: %v", err)
		}
		if me.Stats.AchievementsUnlocked != 1 {
			t.Errorf("achievementsUnlocked = %d, want 1", me.Stats.AchievementsUnlocked)
		}
	})

	t.Run("opening the graph moves the exploration achievement", func(t *testing.T) {
		if response := c.do(t, http.MethodPost, "/activity/graph-opened", ""); response.Code != http.StatusNoContent {
			t.Fatalf("graph-opened = %d: %s", response.Code, response.Body)
		}

		all := achievements(t, c)
		if all["CARTOGRAPHER"].Progress != 1 {
			t.Errorf("CARTOGRAPHER progress = %d, want one day", all["CARTOGRAPHER"].Progress)
		}
		if all["CARTOGRAPHER"].Threshold != 14 {
			t.Errorf("threshold = %d, want 14", all["CARTOGRAPHER"].Threshold)
		}
	})
}
