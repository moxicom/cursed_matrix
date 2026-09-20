//go:build integration

package httphandler_test

import (
	"encoding/json"
	"net/http"
	"slices"
	"sort"
	"testing"
)

func taskList(t *testing.T, body []byte) []taskView {
	t.Helper()
	var list struct {
		Tasks []taskView `json:"tasks"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("decode task list: %v (%s)", err, body)
	}
	return list.Tasks
}

func boardTitles(t *testing.T, c *client, query string) []string {
	t.Helper()
	response := c.do(t, http.MethodGet, "/tasks"+query, "")
	var board struct {
		Tasks []taskView `json:"tasks"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &board); err != nil {
		t.Fatalf("board: %v", err)
	}
	titles := make([]string, 0, len(board.Tasks))
	for _, item := range board.Tasks {
		titles = append(titles, item.Title)
	}
	return titles
}

func TestMovingTasksWithinAndBetweenQuadrants(t *testing.T) {
	c := signedInClient(t)

	created := map[string]taskView{}
	for _, title := range []string{"first", "second", "third"} {
		response := c.do(t, http.MethodPost, "/tasks",
			`{"title":"`+title+`","quadrant":"IMPORTANT_URGENT"}`)
		if response.Code != http.StatusCreated {
			t.Fatalf("create %q = %d: %s", title, response.Code, response.Body)
		}
		created[title] = decodeTask(t, response.Body.Bytes())
	}

	t.Run("they start in the order they were made", func(t *testing.T) {
		got := boardTitles(t, c, "?quadrants=IMPORTANT_URGENT")
		want := []string{"first", "second", "third"}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("order = %v, want %v", got, want)
			}
		}
	})

	t.Run("moving before the first puts it at the front", func(t *testing.T) {
		body := `{"targetQuadrant":"IMPORTANT_URGENT","beforeTaskId":"` + created["first"].ID + `"}`
		response := c.do(t, http.MethodPost, "/tasks/"+created["third"].ID+"/move", body)
		if response.Code != http.StatusOK {
			t.Fatalf("move = %d: %s", response.Code, response.Body)
		}

		changed := taskList(t, response.Body.Bytes())
		if len(changed) == 0 {
			t.Fatal("the move reported nothing changed")
		}

		got := boardTitles(t, c, "?quadrants=IMPORTANT_URGENT")
		want := []string{"third", "first", "second"}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("order = %v, want %v", got, want)
			}
		}
	})

	t.Run("moving to another quadrant takes it out of this one", func(t *testing.T) {
		body := `{"targetQuadrant":"NOT_IMPORTANT_NOT_URGENT"}`
		if response := c.do(t, http.MethodPost, "/tasks/"+created["first"].ID+"/move", body); response.Code != http.StatusOK {
			t.Fatalf("move = %d: %s", response.Code, response.Body)
		}

		here := boardTitles(t, c, "?quadrants=IMPORTANT_URGENT")
		if len(here) != 2 {
			t.Errorf("IMPORTANT_URGENT = %v, want two tasks", here)
		}
		there := boardTitles(t, c, "?quadrants=NOT_IMPORTANT_NOT_URGENT")
		if len(there) != 1 || there[0] != "first" {
			t.Errorf("NOT_IMPORTANT_NOT_URGENT = %v, want [first]", there)
		}
	})

	t.Run("an unknown quadrant is refused", func(t *testing.T) {
		body := `{"targetQuadrant":"SOMEWHAT_URGENT"}`
		response := c.do(t, http.MethodPost, "/tasks/"+created["second"].ID+"/move", body)
		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422: %s", response.Code, response.Body)
		}
	})

	t.Run("a neighbour that is not there is refused", func(t *testing.T) {
		body := `{"targetQuadrant":"IMPORTANT_URGENT","beforeTaskId":"11111111-1111-1111-1111-111111111111"}`
		response := c.do(t, http.MethodPost, "/tasks/"+created["second"].ID+"/move", body)
		if response.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404: %s", response.Code, response.Body)
		}
	})
}

func TestPromotingASubtask(t *testing.T) {
	c := signedInClient(t)

	created := c.do(t, http.MethodPost, "/tasks", `{"title":"parent","quadrant":"IMPORTANT_NOT_URGENT"}`)
	parent := decodeTask(t, created.Body.Bytes())

	response := c.do(t, http.MethodPost, "/tasks/"+parent.ID+"/subtasks", `{"title":"a step","color":"ROSE"}`)
	child := decodeTask(t, response.Body.Bytes())

	t.Run("a task with no parent has nothing to promote", func(t *testing.T) {
		response := c.do(t, http.MethodPost, "/tasks/"+parent.ID+"/promote", "")
		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422: %s", response.Code, response.Body)
		}
	})

	t.Run("without a body it takes the parent's quadrant", func(t *testing.T) {
		response := c.do(t, http.MethodPost, "/tasks/"+child.ID+"/promote", "")
		if response.Code != http.StatusOK {
			t.Fatalf("promote = %d: %s", response.Code, response.Body)
		}

		changed := taskList(t, response.Body.Bytes())
		promoted := changed[len(changed)-1]

		if promoted.ParentTaskID != nil {
			t.Errorf("parentTaskId = %v, want null", *promoted.ParentTaskID)
		}
		if promoted.Quadrant == nil || *promoted.Quadrant != "IMPORTANT_NOT_URGENT" {
			t.Errorf("quadrant = %v, want the parent's", promoted.Quadrant)
		}
		// Metadata is the user's, not the relation's, and survives.
		if promoted.Color != "ROSE" {
			t.Errorf("color = %q, want it kept", promoted.Color)
		}
		if promoted.Position == 0 {
			t.Error("the promoted task was given no position")
		}
	})

	t.Run("the former parent no longer has it", func(t *testing.T) {
		titles := boardTitles(t, c, "?quadrants=IMPORTANT_NOT_URGENT")
		if len(titles) != 2 {
			t.Fatalf("quadrant = %v, want both tasks side by side", titles)
		}
	})
}

// TestPromotingKeepsTheXPSnapshot is CLAUDE.md §14 and §32 together: promoting
// changes where a task sits, never what it earned.
func TestPromotingKeepsTheXPSnapshot(t *testing.T) {
	c := signedInClient(t)

	created := c.do(t, http.MethodPost, "/tasks", `{"title":"parent","quadrant":"IMPORTANT_URGENT"}`)
	parent := decodeTask(t, created.Body.Bytes())
	response := c.do(t, http.MethodPost, "/tasks/"+parent.ID+"/subtasks", `{"title":"a step"}`)
	child := decodeTask(t, response.Body.Bytes())

	if done := c.do(t, http.MethodPost, "/tasks/"+child.ID+"/complete", ""); done.Code != http.StatusOK {
		t.Fatalf("complete = %d: %s", done.Code, done.Body)
	}
	before, _ := lifetimeXP(t, c)

	if promoted := c.do(t, http.MethodPost, "/tasks/"+child.ID+"/promote", ""); promoted.Code != http.StatusOK {
		t.Fatalf("promote = %d: %s", promoted.Code, promoted.Body)
	}

	var board struct {
		Tasks []struct {
			taskView
			XPAwarded            *int32  `json:"xpAwarded"`
			QuadrantAtCompletion *string `json:"quadrantAtCompletion"`
		} `json:"tasks"`
	}
	listed := c.do(t, http.MethodGet, "/tasks?status=COMPLETED", "")
	if err := json.Unmarshal(listed.Body.Bytes(), &board); err != nil {
		t.Fatalf("board: %v", err)
	}
	if len(board.Tasks) != 1 {
		t.Fatalf("%d completed tasks, want 1", len(board.Tasks))
	}

	snapshot := board.Tasks[0]
	// Priced as a subtask of an IMPORTANT_URGENT parent: round(50 × 0.35).
	if snapshot.XPAwarded == nil || *snapshot.XPAwarded != 18 {
		t.Errorf("xpAwarded = %v, want 18", snapshot.XPAwarded)
	}
	if snapshot.QuadrantAtCompletion == nil || *snapshot.QuadrantAtCompletion != "IMPORTANT_URGENT" {
		t.Errorf("quadrantAtCompletion = %v, want the quadrant it was judged by", snapshot.QuadrantAtCompletion)
	}
	if after, _ := lifetimeXP(t, c); after != before {
		t.Errorf("lifetimeXp = %d, want it unchanged at %d", after, before)
	}
}

// TestCompletedTasksCannotBeMoved is SPEC §1768: the quadrant a task was
// completed in is what priced its XP, so it is frozen with the snapshot.
func TestCompletedTasksCannotBeMoved(t *testing.T) {
	c := signedInClient(t)
	created := c.do(t, http.MethodPost, "/tasks", `{"title":"done","quadrant":"IMPORTANT_URGENT"}`)
	item := decodeTask(t, created.Body.Bytes())

	if done := c.do(t, http.MethodPost, "/tasks/"+item.ID+"/complete", ""); done.Code != http.StatusOK {
		t.Fatalf("complete = %d: %s", done.Code, done.Body)
	}

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "to another quadrant",
			body:       `{"targetQuadrant":"NOT_IMPORTANT_NOT_URGENT"}`,
			wantStatus: http.StatusConflict,
		},
		{
			name:       "even within the same one",
			body:       `{"targetQuadrant":"IMPORTANT_URGENT"}`,
			wantStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := c.do(t, http.MethodPost, "/tasks/"+item.ID+"/move", tt.body)
			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", response.Code, tt.wantStatus, response.Body)
			}
		})
	}

	t.Run("reopening unfreezes it", func(t *testing.T) {
		if response := c.do(t, http.MethodPost, "/tasks/"+item.ID+"/reopen", ""); response.Code != http.StatusOK {
			t.Fatalf("reopen = %d: %s", response.Code, response.Body)
		}
		response := c.do(t, http.MethodPost, "/tasks/"+item.ID+"/move",
			`{"targetQuadrant":"NOT_IMPORTANT_NOT_URGENT"}`)
		if response.Code != http.StatusOK {
			t.Fatalf("move = %d, want 200: %s", response.Code, response.Body)
		}
	})
}

// TestReorderingSubtasks is CLAUDE.md §11: the subtasks of one parent have
// their own order, and the user can change it.
func TestReorderingSubtasks(t *testing.T) {
	c := signedInClient(t)
	parent := newTask(t, c, "parent", "IMPORTANT_URGENT")

	subtask := func(title string) taskView {
		t.Helper()
		response := c.do(t, http.MethodPost, "/tasks/"+parent.ID+"/subtasks", `{"title":"`+title+`"}`)
		if response.Code != http.StatusCreated {
			t.Fatalf("subtask %q = %d: %s", title, response.Code, response.Body)
		}
		return decodeTask(t, response.Body.Bytes())
	}

	first, second, third := subtask("first"), subtask("second"), subtask("third")

	order := func() []string {
		t.Helper()
		response := c.do(t, http.MethodGet, "/tasks", "")
		var board struct {
			Tasks []taskView `json:"tasks"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &board); err != nil {
			t.Fatalf("board: %v", err)
		}

		children := make([]taskView, 0, 3)
		for _, item := range board.Tasks {
			if item.ParentTaskID != nil && *item.ParentTaskID == parent.ID {
				children = append(children, item)
			}
		}
		sort.Slice(children, func(i, j int) bool { return children[i].Position < children[j].Position })

		titles := make([]string, 0, len(children))
		for _, item := range children {
			titles = append(titles, item.Title)
		}
		return titles
	}

	t.Run("they start in the order they were added", func(t *testing.T) {
		if got := order(); !slices.Equal(got, []string{"first", "second", "third"}) {
			t.Fatalf("order = %v", got)
		}
	})

	t.Run("one can be moved before another", func(t *testing.T) {
		body := `{"parentTaskId":"` + parent.ID + `","beforeTaskId":"` + first.ID + `"}`
		response := c.do(t, http.MethodPost, "/tasks/"+third.ID+"/move", body)
		if response.Code != http.StatusOK {
			t.Fatalf("move = %d: %s", response.Code, response.Body)
		}

		if got := order(); !slices.Equal(got, []string{"third", "first", "second"}) {
			t.Fatalf("order = %v", got)
		}
	})

	t.Run("and it is still a subtask", func(t *testing.T) {
		// Reordering is not promotion: the parent relation survives.
		response := c.do(t, http.MethodGet, "/tasks", "")
		var board struct {
			Tasks []taskView `json:"tasks"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &board); err != nil {
			t.Fatalf("board: %v", err)
		}
		for _, item := range board.Tasks {
			if item.ID != third.ID {
				continue
			}
			if item.ParentTaskID == nil || *item.ParentTaskID != parent.ID {
				t.Errorf("parentTaskId = %v, want it kept", item.ParentTaskID)
			}
			if item.Quadrant != nil && item.Quadrant == nil {
				t.Error("a subtask gained a quadrant of its own")
			}
		}
	})

	t.Run("moving to the end appends", func(t *testing.T) {
		body := `{"parentTaskId":"` + parent.ID + `"}`
		if response := c.do(t, http.MethodPost, "/tasks/"+third.ID+"/move", body); response.Code != http.StatusOK {
			t.Fatalf("move = %d: %s", response.Code, response.Body)
		}
		if got := order(); !slices.Equal(got, []string{"first", "second", "third"}) {
			t.Fatalf("order = %v", got)
		}
	})

	tests := []struct {
		name       string
		taskID     string
		body       string
		wantStatus int
	}{
		{
			name:       "naming both destinations is ambiguous",
			taskID:     second.ID,
			body:       `{"parentTaskId":"` + parent.ID + `","targetQuadrant":"IMPORTANT_URGENT"}`,
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "naming neither says nothing",
			taskID:     second.ID,
			body:       `{}`,
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "a task that has no parent cannot be reordered under one",
			taskID:     parent.ID,
			body:       `{"parentTaskId":"` + parent.ID + `"}`,
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			// A task does not become somebody's subtask by being moved.
			name:       "nor moved under a parent that is not its own",
			taskID:     second.ID,
			body:       `{"parentTaskId":"` + third.ID + `"}`,
			wantStatus: http.StatusUnprocessableEntity,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := c.do(t, http.MethodPost, "/tasks/"+tt.taskID+"/move", tt.body)
			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", response.Code, tt.wantStatus, response.Body)
			}
		})
	}

	t.Run("a quadrant still promotes it out", func(t *testing.T) {
		body := `{"targetQuadrant":"NOT_IMPORTANT_URGENT"}`
		if response := c.do(t, http.MethodPost, "/tasks/"+second.ID+"/move", body); response.Code != http.StatusOK {
			t.Fatalf("move = %d: %s", response.Code, response.Body)
		}
		if got := order(); !slices.Equal(got, []string{"first", "third"}) {
			t.Errorf("order = %v, want the promoted one gone from the parent", got)
		}
	})
}
