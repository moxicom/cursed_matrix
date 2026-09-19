//go:build integration

package httphandler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

type taskView struct {
	ID              string  `json:"id"`
	ParentTaskID    *string `json:"parentTaskId"`
	Title           string  `json:"title"`
	Description     string  `json:"description"`
	Quadrant        *string `json:"quadrant"`
	Position        int32   `json:"position"`
	Color           string  `json:"color"`
	DeadlineAt      *string `json:"deadlineAt"`
	DeadlineHasTime bool    `json:"deadlineHasTime"`
	Status          string  `json:"status"`
	Tags            []string
}

func decodeTask(t *testing.T, body []byte) taskView {
	t.Helper()
	var view taskView
	if err := json.Unmarshal(body, &view); err != nil {
		t.Fatalf("decode task: %v (%s)", err, body)
	}
	return view
}

func signedInClient(t *testing.T) *client {
	t.Helper()
	c := newClient(t)
	if response := c.do(t, http.MethodPost, "/auth/register", registerBody("w_"+uuid.NewString()[:8])); response.Code != http.StatusCreated {
		t.Fatalf("register: %d %s", response.Code, response.Body)
	}
	return c
}

func TestTaskLifecycleThroughTheRouter(t *testing.T) {
	c := signedInClient(t)

	created := c.do(t, http.MethodPost, "/tasks",
		`{"title":"write the write path","quadrant":"IMPORTANT_URGENT","description":"the whole slice","color":"CYAN","deadlineAt":"2026-09-30T10:00:00Z","deadlineHasTime":true}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create = %d: %s", created.Code, created.Body)
	}
	parent := decodeTask(t, created.Body.Bytes())

	t.Run("the task comes back as the server stored it", func(t *testing.T) {
		if parent.ID == "" || parent.Status != "ACTIVE" {
			t.Fatalf("task = %+v", parent)
		}
		if parent.Quadrant == nil || *parent.Quadrant != "IMPORTANT_URGENT" {
			t.Errorf("quadrant = %v", parent.Quadrant)
		}
		// Position is the server's decision, and the first task in an empty
		// quadrant sits one gap in.
		if parent.Position != 1024 {
			t.Errorf("position = %d, want 1024", parent.Position)
		}
		if parent.Color != "CYAN" || !parent.DeadlineHasTime {
			t.Errorf("metadata = %+v", parent)
		}
	})

	var child taskView
	t.Run("a subtask inherits nothing and points at its parent", func(t *testing.T) {
		response := c.do(t, http.MethodPost, "/tasks/"+parent.ID+"/subtasks", `{"title":"a step"}`)
		if response.Code != http.StatusCreated {
			t.Fatalf("create subtask = %d: %s", response.Code, response.Body)
		}
		child = decodeTask(t, response.Body.Bytes())

		if child.ParentTaskID == nil || *child.ParentTaskID != parent.ID {
			t.Errorf("parentTaskId = %v, want %s", child.ParentTaskID, parent.ID)
		}
		if child.Quadrant != nil {
			t.Errorf("a subtask came back with a quadrant: %v", *child.Quadrant)
		}
	})

	t.Run("a subtask of a subtask is refused", func(t *testing.T) {
		response := c.do(t, http.MethodPost, "/tasks/"+child.ID+"/subtasks", `{"title":"too deep"}`)
		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422: %s", response.Code, response.Body)
		}
	})

	t.Run("another account cannot see the task", func(t *testing.T) {
		stranger := signedInClient(t)

		for _, call := range []struct{ method, path, body string }{
			{http.MethodPatch, "/tasks/" + parent.ID, `{"title":"mine now"}`},
			{http.MethodDelete, "/tasks/" + parent.ID, ""},
			{http.MethodPost, "/tasks/" + parent.ID + "/subtasks", `{"title":"mine now"}`},
		} {
			response := stranger.do(t, call.method, call.path, call.body)
			if response.Code != http.StatusNotFound {
				t.Errorf("%s %s = %d, want 404: %s", call.method, call.path, response.Code, response.Body)
			}
		}
	})

	t.Run("a patch changes only what it mentions", func(t *testing.T) {
		response := c.do(t, http.MethodPatch, "/tasks/"+parent.ID, `{"title":"renamed"}`)
		if response.Code != http.StatusOK {
			t.Fatalf("patch = %d: %s", response.Code, response.Body)
		}
		patched := decodeTask(t, response.Body.Bytes())

		if patched.Title != "renamed" {
			t.Errorf("title = %q", patched.Title)
		}
		if patched.Description != parent.Description || patched.Color != parent.Color {
			t.Errorf("an unmentioned field changed: %+v", patched)
		}
		if patched.DeadlineAt == nil {
			t.Error("the deadline was dropped by a patch that never mentioned it")
		}
	})

	t.Run("null clears the deadline where absent would not", func(t *testing.T) {
		response := c.do(t, http.MethodPatch, "/tasks/"+parent.ID, `{"deadlineAt":null}`)
		if response.Code != http.StatusOK {
			t.Fatalf("patch = %d: %s", response.Code, response.Body)
		}
		patched := decodeTask(t, response.Body.Bytes())

		if patched.DeadlineAt != nil {
			t.Errorf("deadlineAt = %v, want cleared", *patched.DeadlineAt)
		}
		if patched.DeadlineHasTime {
			t.Error("a cleared deadline still claims to have a time")
		}
	})

	t.Run("an over-long title is refused at the edge", func(t *testing.T) {
		long := `{"title":"` + string(make([]byte, 0, 101)) + ""
		for range 101 {
			long += "x"
		}
		long += `"}`

		response := c.do(t, http.MethodPatch, "/tasks/"+parent.ID, long)
		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422: %s", response.Code, response.Body)
		}
	})

	t.Run("a parent with unfinished subtasks will not be deleted", func(t *testing.T) {
		response := c.do(t, http.MethodDelete, "/tasks/"+parent.ID, "")
		if response.Code != http.StatusConflict {
			t.Fatalf("status = %d, want 409: %s", response.Code, response.Body)
		}

		var body struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatalf("body: %v", err)
		}
		if body.Error.Code != "HAS_ACTIVE_SUBTASKS" {
			t.Errorf("code = %q", body.Error.Code)
		}
	})

	t.Run("a completed subtask goes with its parent", func(t *testing.T) {
		if response := c.do(t, http.MethodPost, "/tasks/"+child.ID+"/complete", ""); response.Code != http.StatusOK {
			t.Fatalf("complete subtask = %d: %s", response.Code, response.Body)
		}

		response := c.do(t, http.MethodDelete, "/tasks/"+parent.ID, "")
		if response.Code != http.StatusOK {
			t.Fatalf("delete = %d: %s", response.Code, response.Body)
		}

		var body struct {
			DeletedIDs []string `json:"deletedIds"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatalf("body: %v", err)
		}
		if len(body.DeletedIDs) != 2 {
			t.Fatalf("deletedIds = %v, want the parent and its subtask", body.DeletedIDs)
		}

		listed := c.do(t, http.MethodGet, "/tasks?status=ALL", "")
		var board struct {
			Tasks []taskView `json:"tasks"`
		}
		if err := json.Unmarshal(listed.Body.Bytes(), &board); err != nil {
			t.Fatalf("board: %v", err)
		}
		if len(board.Tasks) != 0 {
			t.Fatalf("%d tasks survived the delete", len(board.Tasks))
		}
	})

	t.Run("the withdrawn XP leaves the account", func(t *testing.T) {
		// The subtask was worth round(50 × 0.35) = 18 and is gone, so the
		// lifetime total has to be back where it started.
		if xp, _ := lifetimeXP(t, c); xp != 0 {
			t.Errorf("lifetimeXp = %d, want 0 after the deletion", xp)
		}
	})

	t.Run("deleting twice is a 404, not a second delete", func(t *testing.T) {
		if response := c.do(t, http.MethodDelete, "/tasks/"+parent.ID, ""); response.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404: %s", response.Code, response.Body)
		}
	})
}

// TestFreePlanTaskQuota is the product rule from CLAUDE.md §64: the free plan
// allows 35 active tasks, and finishing or removing one frees a slot.
func TestFreePlanTaskQuota(t *testing.T) {
	c := signedInClient(t)

	var last string
	for i := range 35 {
		body := fmt.Sprintf(`{"title":"task %d","quadrant":"NOT_IMPORTANT_NOT_URGENT"}`, i)
		response := c.do(t, http.MethodPost, "/tasks", body)
		if response.Code != http.StatusCreated {
			t.Fatalf("task %d = %d: %s", i, response.Code, response.Body)
		}
		last = decodeTask(t, response.Body.Bytes()).ID
	}

	t.Run("the thirty-sixth is refused with the payment code", func(t *testing.T) {
		response := c.do(t, http.MethodPost, "/tasks", `{"title":"one too many","quadrant":"IMPORTANT_URGENT"}`)
		if response.Code != http.StatusPaymentRequired {
			t.Fatalf("status = %d, want 402: %s", response.Code, response.Body)
		}

		var body struct {
			Error struct {
				Code   string         `json:"code"`
				Params map[string]any `json:"params"`
			} `json:"error"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatalf("body: %v", err)
		}
		if body.Error.Code != "QUOTA_LIMIT_REACHED" {
			t.Errorf("code = %q", body.Error.Code)
		}
		if body.Error.Params["limit"] != float64(35) {
			t.Errorf("limit = %v, want 35", body.Error.Params["limit"])
		}
	})

	t.Run("a subtask counts against the same quota", func(t *testing.T) {
		response := c.do(t, http.MethodPost, "/tasks/"+last+"/subtasks", `{"title":"also too many"}`)
		if response.Code != http.StatusPaymentRequired {
			t.Fatalf("status = %d, want 402: %s", response.Code, response.Body)
		}
	})

	t.Run("deleting one frees a slot", func(t *testing.T) {
		if response := c.do(t, http.MethodDelete, "/tasks/"+last, ""); response.Code != http.StatusOK {
			t.Fatalf("delete = %d: %s", response.Code, response.Body)
		}

		response := c.do(t, http.MethodPost, "/tasks", `{"title":"room again","quadrant":"IMPORTANT_URGENT"}`)
		if response.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201: %s", response.Code, response.Body)
		}
	})
}
