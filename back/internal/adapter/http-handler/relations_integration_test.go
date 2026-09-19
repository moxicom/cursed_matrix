//go:build integration

package httphandler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"testing"
)

type tagView struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	TaskCount int    `json:"taskCount"`
}

type linkView struct {
	ID           string `json:"id"`
	SourceTaskID string `json:"sourceTaskId"`
	TargetTaskID string `json:"targetTaskId"`
	Type         string `json:"type"`
}

func newTask(t *testing.T, c *client, title, quadrant string) taskView {
	t.Helper()
	response := c.do(t, http.MethodPost, "/tasks",
		`{"title":"`+title+`","quadrant":"`+quadrant+`"}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("create %q = %d: %s", title, response.Code, response.Body)
	}
	return decodeTask(t, response.Body.Bytes())
}

func errorCodeOf(t *testing.T, body []byte) string {
	t.Helper()
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("error body: %v (%s)", err, body)
	}
	return envelope.Error.Code
}

func TestTagsThroughTheRouter(t *testing.T) {
	c := signedInClient(t)
	first := newTask(t, c, "first", "IMPORTANT_URGENT")
	second := newTask(t, c, "second", "IMPORTANT_URGENT")

	var backend tagView
	t.Run("a new label is created and attached", func(t *testing.T) {
		response := c.do(t, http.MethodPost, "/tasks/"+first.ID+"/tags", `{"name":"backend"}`)
		if response.Code != http.StatusOK {
			t.Fatalf("attach = %d: %s", response.Code, response.Body)
		}
		if err := json.Unmarshal(response.Body.Bytes(), &backend); err != nil {
			t.Fatalf("tag: %v", err)
		}
		if backend.Name != "backend" || backend.ID == "" {
			t.Fatalf("tag = %+v", backend)
		}
	})

	t.Run("a different spelling is the same label", func(t *testing.T) {
		response := c.do(t, http.MethodPost, "/tasks/"+second.ID+"/tags", `{"name":"BACKEND"}`)
		if response.Code != http.StatusOK {
			t.Fatalf("attach = %d: %s", response.Code, response.Body)
		}

		var again tagView
		if err := json.Unmarshal(response.Body.Bytes(), &again); err != nil {
			t.Fatalf("tag: %v", err)
		}
		if again.ID != backend.ID {
			t.Errorf("a second label was made: %s vs %s", again.ID, backend.ID)
		}
		// The first spelling is the one everyone sees.
		if again.Name != "backend" {
			t.Errorf("name = %q, want the spelling it was created with", again.Name)
		}
	})

	t.Run("attaching twice is not an error", func(t *testing.T) {
		if response := c.do(t, http.MethodPost, "/tasks/"+first.ID+"/tags", `{"name":"backend"}`); response.Code != http.StatusOK {
			t.Fatalf("attach = %d: %s", response.Code, response.Body)
		}
	})

	t.Run("the listing counts the tasks", func(t *testing.T) {
		response := c.do(t, http.MethodGet, "/tags", "")
		var body struct {
			Tags []tagView `json:"tags"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatalf("tags: %v", err)
		}
		if len(body.Tags) != 1 {
			t.Fatalf("tags = %+v, want one", body.Tags)
		}
		if body.Tags[0].TaskCount != 2 {
			t.Errorf("taskCount = %d, want 2", body.Tags[0].TaskCount)
		}
	})

	t.Run("the board carries the label", func(t *testing.T) {
		var board struct {
			Tasks []struct {
				Title string   `json:"title"`
				Tags  []string `json:"tags"`
			} `json:"tasks"`
		}
		listed := c.do(t, http.MethodGet, "/tasks", "")
		if err := json.Unmarshal(listed.Body.Bytes(), &board); err != nil {
			t.Fatalf("board: %v", err)
		}
		for _, item := range board.Tasks {
			if len(item.Tags) != 1 || item.Tags[0] != "backend" {
				t.Errorf("%q tags = %v", item.Title, item.Tags)
			}
		}
	})

	t.Run("filtering by the label finds both", func(t *testing.T) {
		if titles := boardTitles(t, c, "?tags=backend"); len(titles) != 2 {
			t.Errorf("tags=backend = %v, want both tasks", titles)
		}
	})

	t.Run("a name that is too long is refused", func(t *testing.T) {
		long := `{"name":"` + fmt.Sprintf("%025d", 0) + `"}`
		response := c.do(t, http.MethodPost, "/tasks/"+first.ID+"/tags", long)
		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422: %s", response.Code, response.Body)
		}
	})

	t.Run("another account cannot tag the task", func(t *testing.T) {
		stranger := signedInClient(t)
		response := stranger.do(t, http.MethodPost, "/tasks/"+first.ID+"/tags", `{"name":"mine"}`)
		if response.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404: %s", response.Code, response.Body)
		}
	})

	t.Run("detaching from the last task forgets the label", func(t *testing.T) {
		for _, id := range []string{first.ID, second.ID} {
			response := c.do(t, http.MethodDelete, "/tasks/"+id+"/tags/"+backend.ID, "")
			if response.Code != http.StatusNoContent {
				t.Fatalf("detach = %d: %s", response.Code, response.Body)
			}
		}

		response := c.do(t, http.MethodGet, "/tags", "")
		var body struct {
			Tags []tagView `json:"tags"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatalf("tags: %v", err)
		}
		if len(body.Tags) != 0 {
			t.Errorf("tags = %+v, want the orphan gone", body.Tags)
		}
	})

	t.Run("detaching what is not attached is a 404", func(t *testing.T) {
		response := c.do(t, http.MethodDelete, "/tasks/"+first.ID+"/tags/"+backend.ID, "")
		if response.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404: %s", response.Code, response.Body)
		}
	})
}

func TestLinksThroughTheRouter(t *testing.T) {
	c := signedInClient(t)
	a := newTask(t, c, "a", "IMPORTANT_URGENT")
	b := newTask(t, c, "b", "IMPORTANT_URGENT")

	link := func(source, target, linkType string) (int, []byte) {
		body := `{"sourceTaskId":"` + source + `","targetTaskId":"` + target + `"`
		if linkType != "" {
			body += `,"type":"` + linkType + `"`
		}
		response := c.do(t, http.MethodPost, "/links", body+`}`)
		return response.Code, response.Body.Bytes()
	}

	var related linkView
	t.Run("a link defaults to RELATED", func(t *testing.T) {
		code, body := link(a.ID, b.ID, "")
		if code != http.StatusCreated {
			t.Fatalf("status = %d: %s", code, body)
		}
		if err := json.Unmarshal(body, &related); err != nil {
			t.Fatalf("link: %v", err)
		}
		if related.Type != "RELATED" {
			t.Errorf("type = %q", related.Type)
		}
	})

	t.Run("the same undirected pair reversed is the same link", func(t *testing.T) {
		code, body := link(b.ID, a.ID, "RELATED")
		if code != http.StatusConflict {
			t.Fatalf("status = %d, want 409: %s", code, body)
		}
		if got := errorCodeOf(t, body); got != "DUPLICATE_LINK" {
			t.Errorf("code = %q", got)
		}
	})

	t.Run("a directed pair reversed is a different link", func(t *testing.T) {
		if code, body := link(a.ID, b.ID, "BLOCKS"); code != http.StatusCreated {
			t.Fatalf("status = %d: %s", code, body)
		}
		if code, body := link(b.ID, a.ID, "BLOCKS"); code != http.StatusCreated {
			t.Fatalf("reversed = %d, want 201: %s", code, body)
		}
	})

	t.Run("a task cannot be linked to itself", func(t *testing.T) {
		code, body := link(a.ID, a.ID, "")
		if code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422: %s", code, body)
		}
		if got := errorCodeOf(t, body); got != "SELF_LINK" {
			t.Errorf("code = %q", got)
		}
	})

	t.Run("a parent and its own subtask are already related", func(t *testing.T) {
		response := c.do(t, http.MethodPost, "/tasks/"+a.ID+"/subtasks", `{"title":"a step"}`)
		child := decodeTask(t, response.Body.Bytes())

		for _, pair := range [][2]string{{a.ID, child.ID}, {child.ID, a.ID}} {
			code, body := link(pair[0], pair[1], "")
			if code != http.StatusConflict {
				t.Fatalf("status = %d, want 409: %s", code, body)
			}
		}
	})

	t.Run("a task that is not ours is not found", func(t *testing.T) {
		stranger := signedInClient(t)
		theirs := newTask(t, stranger, "theirs", "IMPORTANT_URGENT")

		code, body := link(a.ID, theirs.ID, "")
		if code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404: %s", code, body)
		}
	})

	t.Run("changing the type is allowed until it collides", func(t *testing.T) {
		response := c.do(t, http.MethodPatch, "/links/"+related.ID, `{"type":"CONNECTED_TO"}`)
		if response.Code != http.StatusOK {
			t.Fatalf("patch = %d: %s", response.Code, response.Body)
		}

		// Back to a type that another link between the same pair now holds.
		if code, body := link(a.ID, b.ID, "RELATED"); code != http.StatusCreated {
			t.Fatalf("status = %d: %s", code, body)
		}
		clash := c.do(t, http.MethodPatch, "/links/"+related.ID, `{"type":"RELATED"}`)
		if clash.Code != http.StatusConflict {
			t.Fatalf("status = %d, want 409: %s", clash.Code, clash.Body)
		}
	})

	t.Run("deleting a link leaves the tasks alone", func(t *testing.T) {
		if response := c.do(t, http.MethodDelete, "/links/"+related.ID, ""); response.Code != http.StatusNoContent {
			t.Fatalf("delete = %d: %s", response.Code, response.Body)
		}
		if response := c.do(t, http.MethodDelete, "/links/"+related.ID, ""); response.Code != http.StatusNotFound {
			t.Fatalf("second delete = %d, want 404", response.Code)
		}
		// The subtask made earlier answers this filter too, by its parent's
		// quadrant; what matters is that breaking a link removed no task.
		titles := boardTitles(t, c, "?quadrants=IMPORTANT_URGENT")
		for _, want := range []string{"a", "b"} {
			if !slices.Contains(titles, want) {
				t.Errorf("%q is gone: %v", want, titles)
			}
		}
	})
}

// TestFreePlanLinkQuota is CLAUDE.md §64: 25 links on the free plan.
func TestFreePlanLinkQuota(t *testing.T) {
	c := signedInClient(t)

	hub := newTask(t, c, "hub", "IMPORTANT_URGENT")
	spokes := make([]taskView, 0, 26)
	for i := range 26 {
		spokes = append(spokes, newTask(t, c, fmt.Sprintf("spoke %d", i), "NOT_IMPORTANT_NOT_URGENT"))
	}

	for i := range 25 {
		body := `{"sourceTaskId":"` + hub.ID + `","targetTaskId":"` + spokes[i].ID + `"}`
		if response := c.do(t, http.MethodPost, "/links", body); response.Code != http.StatusCreated {
			t.Fatalf("link %d = %d: %s", i, response.Code, response.Body)
		}
	}

	body := `{"sourceTaskId":"` + hub.ID + `","targetTaskId":"` + spokes[25].ID + `"}`
	response := c.do(t, http.MethodPost, "/links", body)
	if response.Code != http.StatusPaymentRequired {
		t.Fatalf("the 26th link = %d, want 402: %s", response.Code, response.Body)
	}
	if got := errorCodeOf(t, response.Body.Bytes()); got != "QUOTA_LIMIT_REACHED" {
		t.Errorf("code = %q", got)
	}
}

// TestTheBoardCarriesItsLinks is what lets the client draw a link count on a
// card without a second request that could disagree with the first.
func TestTheBoardCarriesItsLinks(t *testing.T) {
	c := signedInClient(t)
	a := newTask(t, c, "a", "IMPORTANT_URGENT")
	b := newTask(t, c, "b", "IMPORTANT_URGENT")

	body := `{"sourceTaskId":"` + a.ID + `","targetTaskId":"` + b.ID + `","type":"DEPENDS_ON"}`
	if response := c.do(t, http.MethodPost, "/links", body); response.Code != http.StatusCreated {
		t.Fatalf("link = %d: %s", response.Code, response.Body)
	}

	var board struct {
		Tasks []taskView `json:"tasks"`
		Links []linkView `json:"links"`
	}
	listed := c.do(t, http.MethodGet, "/tasks", "")
	if err := json.Unmarshal(listed.Body.Bytes(), &board); err != nil {
		t.Fatalf("board: %v", err)
	}

	if len(board.Links) != 1 {
		t.Fatalf("links = %+v, want one", board.Links)
	}
	edge := board.Links[0]
	if edge.SourceTaskID != a.ID || edge.TargetTaskID != b.ID || edge.Type != "DEPENDS_ON" {
		t.Errorf("link = %+v", edge)
	}

	t.Run("a filtered board still carries every link", func(t *testing.T) {
		// The edges are the whole network, not only the part joining the
		// tasks that came back: the graph filters nodes, not edges.
		filtered := c.do(t, http.MethodGet, "/tasks?quadrants=NOT_IMPORTANT_URGENT", "")
		var narrow struct {
			Tasks []taskView `json:"tasks"`
			Links []linkView `json:"links"`
		}
		if err := json.Unmarshal(filtered.Body.Bytes(), &narrow); err != nil {
			t.Fatalf("board: %v", err)
		}
		if len(narrow.Tasks) != 0 {
			t.Errorf("tasks = %+v, want none in that quadrant", narrow.Tasks)
		}
		if len(narrow.Links) != 1 {
			t.Errorf("links = %+v, want the network unchanged", narrow.Links)
		}
	})
}
