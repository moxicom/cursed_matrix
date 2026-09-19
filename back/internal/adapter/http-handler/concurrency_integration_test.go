//go:build integration

package httphandler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
)

// sessions hands out callers that share one signed-in session but no mutable
// state, so they can be driven in parallel.
func sessions(t *testing.T, c *client, n int) []*client {
	t.Helper()

	out := make([]*client, n)
	for i := range out {
		cookies := make(map[string]string, len(c.cookies))
		for name, value := range c.cookies {
			cookies[name] = value
		}
		out[i] = &client{handler: c.handler, cookies: cookies}
	}
	return out
}

func inParallel(t *testing.T, callers []*client, call func(c *client) int) map[int]int {
	t.Helper()

	var (
		start   = make(chan struct{})
		wait    sync.WaitGroup
		mu      sync.Mutex
		outcome = map[int]int{}
	)

	for _, caller := range callers {
		wait.Add(1)
		go func(c *client) {
			defer wait.Done()
			<-start
			code := call(c)
			mu.Lock()
			outcome[code]++
			mu.Unlock()
		}(caller)
	}

	close(start)
	wait.Wait()
	return outcome
}

// TestConcurrentCompletionPaysOnce is the double-click and the retry: one
// completion is one payment, however many requests arrive for it at once.
func TestConcurrentCompletionPaysOnce(t *testing.T) {
	c := signedInClient(t)
	created := c.do(t, http.MethodPost, "/tasks", `{"title":"contended","quadrant":"IMPORTANT_URGENT"}`)
	item := decodeTask(t, created.Body.Bytes())

	const callers = 8
	outcome := inParallel(t, sessions(t, c, callers), func(caller *client) int {
		return caller.do(t, http.MethodPost, "/tasks/"+item.ID+"/complete", "").Code
	})

	if outcome[http.StatusOK] != 1 {
		t.Errorf("%d callers were told they completed it, want 1 (outcome: %v)", outcome[http.StatusOK], outcome)
	}
	if outcome[http.StatusConflict] != callers-1 {
		t.Errorf("%d callers were refused, want %d (outcome: %v)", outcome[http.StatusConflict], callers-1, outcome)
	}

	// The ledger is the fact that matters: an IMPORTANT_URGENT task is 50,
	// paid once.
	if xp, _ := lifetimeXP(t, c); xp != 50 {
		t.Errorf("lifetimeXp = %d, want 50", xp)
	}
}

// TestConcurrentReopenWithdrawsOnce is the mirror: withdrawing twice would
// take XP the user earned once and can drive the counters below zero.
func TestConcurrentReopenWithdrawsOnce(t *testing.T) {
	c := signedInClient(t)
	created := c.do(t, http.MethodPost, "/tasks", `{"title":"contended","quadrant":"IMPORTANT_URGENT"}`)
	item := decodeTask(t, created.Body.Bytes())

	if done := c.do(t, http.MethodPost, "/tasks/"+item.ID+"/complete", ""); done.Code != http.StatusOK {
		t.Fatalf("complete = %d: %s", done.Code, done.Body)
	}

	const callers = 8
	outcome := inParallel(t, sessions(t, c, callers), func(caller *client) int {
		return caller.do(t, http.MethodPost, "/tasks/"+item.ID+"/reopen", "").Code
	})

	if outcome[http.StatusOK] != 1 {
		t.Errorf("%d callers reopened it, want 1 (outcome: %v)", outcome[http.StatusOK], outcome)
	}

	xp, _ := lifetimeXP(t, c)
	if xp != 0 {
		t.Errorf("lifetimeXp = %d, want 0 — the 50 was withdrawn once", xp)
	}

	var me struct {
		Stats struct {
			TasksCompleted int32 `json:"tasksCompleted"`
		} `json:"stats"`
	}
	response := c.do(t, http.MethodGet, "/me", "")
	if err := json.Unmarshal(response.Body.Bytes(), &me); err != nil {
		t.Fatalf("me: %v", err)
	}
	if me.Stats.TasksCompleted != 0 {
		t.Errorf("tasksCompleted = %d, want 0 and never negative", me.Stats.TasksCompleted)
	}
}

// TestConcurrentCreationRespectsTheQuota is the paywall: counting the account's
// tasks and then inserting is a decision that has to be taken alone.
func TestConcurrentCreationRespectsTheQuota(t *testing.T) {
	c := signedInClient(t)

	// One slot left under the free plan's 35.
	for i := range 34 {
		body := fmt.Sprintf(`{"title":"task %d","quadrant":"NOT_IMPORTANT_NOT_URGENT"}`, i)
		if response := c.do(t, http.MethodPost, "/tasks", body); response.Code != http.StatusCreated {
			t.Fatalf("task %d = %d: %s", i, response.Code, response.Body)
		}
	}

	const callers = 8
	outcome := inParallel(t, sessions(t, c, callers), func(caller *client) int {
		return caller.do(t, http.MethodPost, "/tasks",
			`{"title":"the last slot","quadrant":"IMPORTANT_URGENT"}`).Code
	})

	if outcome[http.StatusCreated] != 1 {
		t.Errorf("%d callers took the last slot, want 1 (outcome: %v)", outcome[http.StatusCreated], outcome)
	}
	if outcome[http.StatusPaymentRequired] != callers-1 {
		t.Errorf("%d callers were refused, want %d (outcome: %v)",
			outcome[http.StatusPaymentRequired], callers-1, outcome)
	}

	listed := c.do(t, http.MethodGet, "/tasks", "")
	var board struct {
		Tasks []taskView `json:"tasks"`
	}
	if err := json.Unmarshal(listed.Body.Bytes(), &board); err != nil {
		t.Fatalf("board: %v", err)
	}
	if len(board.Tasks) != 35 {
		t.Errorf("%d active tasks, want the plan's 35", len(board.Tasks))
	}
}
