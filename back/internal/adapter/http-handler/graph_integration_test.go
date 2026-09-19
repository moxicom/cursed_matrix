//go:build integration

package httphandler_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

type graphView struct {
	Nodes []struct {
		ID           string   `json:"id"`
		Title        string   `json:"title"`
		Status       string   `json:"status"`
		Quadrant     *string  `json:"quadrant"`
		IsSubtask    bool     `json:"isSubtask"`
		ParentTaskID *string  `json:"parentTaskId"`
		Tags         []string `json:"tags"`
		LinkCount    int      `json:"linkCount"`
		SubtaskCount int      `json:"subtaskCount"`
	} `json:"nodes"`
	Edges []struct {
		ID     string  `json:"id"`
		Source string  `json:"source"`
		Target string  `json:"target"`
		Kind   string  `json:"kind"`
		Type   *string `json:"type"`
	} `json:"edges"`
}

func readGraph(t *testing.T, c *client, query string) graphView {
	t.Helper()
	response := c.do(t, http.MethodGet, "/graph"+query, "")
	if response.Code != http.StatusOK {
		t.Fatalf("graph%s = %d: %s", query, response.Code, response.Body)
	}
	var view graphView
	if err := json.Unmarshal(response.Body.Bytes(), &view); err != nil {
		t.Fatalf("graph: %v", err)
	}
	return view
}

func TestGraphSnapshot(t *testing.T) {
	c := signedInClient(t)

	hub := newTask(t, c, "hub", "IMPORTANT_URGENT")
	spoke := newTask(t, c, "spoke", "NOT_IMPORTANT_NOT_URGENT")

	response := c.do(t, http.MethodPost, "/tasks/"+hub.ID+"/subtasks", `{"title":"a step"}`)
	child := decodeTask(t, response.Body.Bytes())

	body := `{"sourceTaskId":"` + hub.ID + `","targetTaskId":"` + spoke.ID + `","type":"BLOCKS"}`
	if response := c.do(t, http.MethodPost, "/links", body); response.Code != http.StatusCreated {
		t.Fatalf("link = %d: %s", response.Code, response.Body)
	}
	if response := c.do(t, http.MethodPost, "/tasks/"+hub.ID+"/tags", `{"name":"backend"}`); response.Code != http.StatusOK {
		t.Fatalf("tag = %d: %s", response.Code, response.Body)
	}

	view := readGraph(t, c, "")

	t.Run("every task is a node", func(t *testing.T) {
		if len(view.Nodes) != 3 {
			t.Fatalf("%d nodes, want 3", len(view.Nodes))
		}
	})

	byID := map[string]int{}
	for i, node := range view.Nodes {
		byID[node.ID] = i
	}

	t.Run("the counts come from the server", func(t *testing.T) {
		hubNode := view.Nodes[byID[hub.ID]]
		if hubNode.LinkCount != 1 {
			t.Errorf("linkCount = %d, want 1", hubNode.LinkCount)
		}
		if hubNode.SubtaskCount != 1 {
			t.Errorf("subtaskCount = %d, want 1", hubNode.SubtaskCount)
		}
		if len(hubNode.Tags) != 1 || hubNode.Tags[0] != "backend" {
			t.Errorf("tags = %v", hubNode.Tags)
		}

		spokeNode := view.Nodes[byID[spoke.ID]]
		if spokeNode.LinkCount != 1 {
			t.Errorf("the other end of the link has linkCount %d, want 1", spokeNode.LinkCount)
		}
		if spokeNode.SubtaskCount != 0 {
			t.Errorf("subtaskCount = %d, want 0", spokeNode.SubtaskCount)
		}
	})

	t.Run("a subtask draws with its parent's quadrant", func(t *testing.T) {
		node := view.Nodes[byID[child.ID]]
		if !node.IsSubtask || node.ParentTaskID == nil || *node.ParentTaskID != hub.ID {
			t.Fatalf("node = %+v", node)
		}
		// The effective quadrant, so the canvas does not have to resolve it
		// while drawing.
		if node.Quadrant == nil || *node.Quadrant != "IMPORTANT_URGENT" {
			t.Errorf("quadrant = %v, want the parent's", node.Quadrant)
		}
	})

	t.Run("edges are of two kinds", func(t *testing.T) {
		var links, parents int
		for _, edge := range view.Edges {
			switch edge.Kind {
			case "LINK":
				links++
				if edge.Type == nil || *edge.Type != "BLOCKS" {
					t.Errorf("link type = %v", edge.Type)
				}
			case "PARENT_CHILD":
				parents++
				if edge.Type != nil {
					t.Errorf("a parent relation has no type, got %v", *edge.Type)
				}
				if edge.Source != child.ID || edge.Target != hub.ID {
					t.Errorf("parent edge = %s → %s", edge.Source, edge.Target)
				}
				// Derived, so its id must not look like a link's.
				if edge.ID != "p-"+child.ID {
					t.Errorf("edge id = %q", edge.ID)
				}
			default:
				t.Errorf("unknown edge kind %q", edge.Kind)
			}
		}
		if links != 1 || parents != 1 {
			t.Errorf("%d links and %d parent relations, want one of each", links, parents)
		}
	})

	t.Run("filters narrow the nodes on the server", func(t *testing.T) {
		narrow := readGraph(t, c, "?quadrants=NOT_IMPORTANT_NOT_URGENT")
		if len(narrow.Nodes) != 1 || narrow.Nodes[0].ID != spoke.ID {
			t.Fatalf("nodes = %+v, want only the spoke", narrow.Nodes)
		}
		// The edges are the whole network: an edge whose other end is filtered
		// out still tells the canvas this node is connected.
		if len(narrow.Edges) == 0 {
			t.Error("the filtered graph lost its edges")
		}
	})

	t.Run("completed tasks stay in the graph", func(t *testing.T) {
		if done := c.do(t, http.MethodPost, "/tasks/"+spoke.ID+"/complete", ""); done.Code != http.StatusOK {
			t.Fatalf("complete = %d: %s", done.Code, done.Body)
		}

		all := readGraph(t, c, "?status=ALL")
		if len(all.Nodes) != 3 {
			t.Errorf("%d nodes, want the completed task still drawn", len(all.Nodes))
		}
		active := readGraph(t, c, "?status=ACTIVE")
		if len(active.Nodes) != 2 {
			t.Errorf("%d active nodes, want the status filter to remove it", len(active.Nodes))
		}
	})

	t.Run("an unknown filter value is refused", func(t *testing.T) {
		response := c.do(t, http.MethodGet, "/graph?status=NOPE", "")
		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422: %s", response.Code, response.Body)
		}
	})
}
