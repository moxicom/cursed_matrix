//go:build integration

package httphandler_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

type rankingView struct {
	Entries []struct {
		Rank          int    `json:"rank"`
		UserID        string `json:"userId"`
		Username      string `json:"username"`
		XP            int64  `json:"xp"`
		Level         int32  `json:"level"`
		CurrentStreak int32  `json:"currentStreak"`
		IsCurrentUser bool   `json:"isCurrentUser"`
	} `json:"entries"`
	Me struct {
		Visible bool  `json:"visible"`
		Rank    *int  `json:"rank"`
		XP      int64 `json:"xp"`
	} `json:"me"`
	NextOffset *int `json:"nextOffset"`
}

func ranking(t *testing.T, c *client, query string) rankingView {
	t.Helper()
	response := c.do(t, http.MethodGet, "/leaderboard"+query, "")
	if response.Code != http.StatusOK {
		t.Fatalf("leaderboard%s = %d: %s", query, response.Code, response.Body)
	}
	var view rankingView
	if err := json.Unmarshal(response.Body.Bytes(), &view); err != nil {
		t.Fatalf("leaderboard: %v", err)
	}
	return view
}

func showInLeaderboard(t *testing.T, c *client, visible bool) {
	t.Helper()
	body := `{"showInLeaderboard":false}`
	if visible {
		body = `{"showInLeaderboard":true}`
	}
	if response := c.do(t, http.MethodPatch, "/me", body); response.Code != http.StatusOK {
		t.Fatalf("settings = %d: %s", response.Code, response.Body)
	}
}

func earnXP(t *testing.T, c *client, quadrant string, times int) {
	t.Helper()
	for i := range times {
		item := newTask(t, c, "earned", quadrant)
		if response := c.do(t, http.MethodPost, "/tasks/"+item.ID+"/complete", ""); response.Code != http.StatusOK {
			t.Fatalf("complete %d = %d: %s", i, response.Code, response.Body)
		}
	}
}

func TestLeaderboardThroughTheRouter(t *testing.T) {
	// Three visible accounts with different totals, and one that opted out.
	rich := signedInClient(t)
	showInLeaderboard(t, rich, true)
	earnXP(t, rich, "IMPORTANT_URGENT", 3) // 150

	middling := signedInClient(t)
	showInLeaderboard(t, middling, true)
	earnXP(t, middling, "IMPORTANT_URGENT", 2) // 100

	hidden := signedInClient(t)
	earnXP(t, hidden, "IMPORTANT_URGENT", 4) // 200, but opted out

	t.Run("the ranking is ordered and dense", func(t *testing.T) {
		view := ranking(t, rich, "?period=ALL_TIME&limit=50")
		if len(view.Entries) < 2 {
			t.Fatalf("%d entries, want at least the two visible accounts", len(view.Entries))
		}

		for i := 1; i < len(view.Entries); i++ {
			if view.Entries[i].XP > view.Entries[i-1].XP {
				t.Fatalf("out of order at %d: %d after %d", i, view.Entries[i].XP, view.Entries[i-1].XP)
			}
			if view.Entries[i].Rank < view.Entries[i-1].Rank {
				t.Fatalf("rank went backwards at %d", i)
			}
		}
	})

	t.Run("a hidden account occupies no rank", func(t *testing.T) {
		view := ranking(t, rich, "?period=ALL_TIME&limit=50")
		for _, entry := range view.Entries {
			if entry.XP == 200 {
				t.Fatalf("a user who opted out is in the ranking: %+v", entry)
			}
		}
	})

	t.Run("the asking user is marked", func(t *testing.T) {
		view := ranking(t, rich, "?period=ALL_TIME&limit=50")
		var marked int
		for _, entry := range view.Entries {
			if entry.IsCurrentUser {
				marked++
				if entry.XP != 150 {
					t.Errorf("marked entry has %d xp", entry.XP)
				}
			}
		}
		if marked != 1 {
			t.Errorf("%d entries marked as the caller, want 1", marked)
		}
	})

	t.Run("a hidden user has no public rank but still sees their own XP", func(t *testing.T) {
		view := ranking(t, hidden, "?period=ALL_TIME")
		if view.Me.Visible {
			t.Fatal("a user who opted out was told they are visible")
		}
		if view.Me.Rank != nil {
			t.Errorf("rank = %d, want null — publicly it does not exist", *view.Me.Rank)
		}
		// Opting out removes them from the public ranking, not from their own
		// screen: their XP is theirs whether or not anyone else may see it.
		if view.Me.XP != 200 {
			t.Errorf("me.xp = %d, want the 200 they earned", view.Me.XP)
		}
	})

	t.Run("a tie is broken by who reached the total first", func(t *testing.T) {
		// Two accounts on the same total: the one whose last earning came
		// earlier got there first, so it sorts ahead. They are found by id,
		// because the test database holds accounts from every earlier run and
		// several of them share this total.
		early := signedInClient(t)
		showInLeaderboard(t, early, true)
		earnXP(t, early, "NOT_IMPORTANT_NOT_URGENT", 1) // 10

		late := signedInClient(t)
		showInLeaderboard(t, late, true)
		earnXP(t, late, "NOT_IMPORTANT_NOT_URGENT", 1) // 10, a moment later

		earlyID, lateID := accountID(t, early), accountID(t, late)

		view := ranking(t, early, "?period=ALL_TIME&limit=50")
		positions := map[string]int{}
		ranks := map[string]int{}
		for i, entry := range view.Entries {
			positions[entry.UserID] = i
			ranks[entry.UserID] = entry.Rank
		}

		earlyAt, ok := positions[earlyID]
		if !ok {
			t.Skip("the tied accounts are past the first page")
		}
		lateAt, ok := positions[lateID]
		if !ok {
			t.Skip("the tied accounts are past the first page")
		}

		if earlyAt > lateAt {
			t.Errorf("the account that finished first sorts at %d, behind %d", earlyAt, lateAt)
		}
		if ranks[earlyID] != ranks[lateID] {
			t.Errorf("two accounts on the same total got ranks %d and %d",
				ranks[earlyID], ranks[lateID])
		}
	})

	t.Run("a visible user gets their own standing", func(t *testing.T) {
		view := ranking(t, middling, "?period=ALL_TIME")
		if !view.Me.Visible || view.Me.Rank == nil {
			t.Fatalf("me = %+v", view.Me)
		}
		if view.Me.XP != 100 {
			t.Errorf("me.xp = %d, want 100", view.Me.XP)
		}
	})

	t.Run("the standing is returned even off the page", func(t *testing.T) {
		// A page of one cannot contain everybody, and the screen still shows
		// where the user is.
		view := ranking(t, middling, "?period=ALL_TIME&limit=1")
		if len(view.Entries) != 1 {
			t.Fatalf("%d entries, want the page respected", len(view.Entries))
		}
		if !view.Me.Visible || view.Me.Rank == nil {
			t.Fatalf("me = %+v, want the standing anyway", view.Me)
		}
		if view.NextOffset == nil {
			t.Error("nextOffset = null, want the client told there is more")
		}
	})

	t.Run("the periods all answer", func(t *testing.T) {
		for _, period := range []string{"WEEK", "MONTH", "ALL_TIME"} {
			view := ranking(t, rich, "?period="+period)
			// Everything was earned just now, so every window contains it.
			if view.Me.XP != 150 {
				t.Errorf("%s: me.xp = %d, want 150", period, view.Me.XP)
			}
		}
	})

	t.Run("an unknown period is refused", func(t *testing.T) {
		response := rich.do(t, http.MethodGet, "/leaderboard?period=YEAR", "")
		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422: %s", response.Code, response.Body)
		}
	})
}

func accountID(t *testing.T, c *client) string {
	t.Helper()
	response := c.do(t, http.MethodGet, "/me", "")
	var me struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &me); err != nil {
		t.Fatalf("me: %v", err)
	}
	return me.ID
}
