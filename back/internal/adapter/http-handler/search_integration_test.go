//go:build integration

package httphandler_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
)

type searchView struct {
	Items []struct {
		ID           string  `json:"id"`
		Title        string  `json:"title"`
		Quadrant     *string `json:"quadrant"`
		IsSubtask    bool    `json:"isSubtask"`
		Status       string  `json:"status"`
		MatchedField string  `json:"matchedField"`
		MatchedText  string  `json:"matchedText"`
	} `json:"items"`
}

func search(t *testing.T, c *client, term string) searchView {
	t.Helper()
	response := c.do(t, http.MethodGet, "/search?query="+url.QueryEscape(term), "")
	if response.Code != http.StatusOK {
		t.Fatalf("search %q = %d: %s", term, response.Code, response.Body)
	}
	var view searchView
	if err := json.Unmarshal(response.Body.Bytes(), &view); err != nil {
		t.Fatalf("search: %v", err)
	}
	return view
}

func TestSearchThroughTheRouter(t *testing.T) {
	c := signedInClient(t)

	titled := c.do(t, http.MethodPost, "/tasks",
		`{"title":"Patch the auth refresh","quadrant":"IMPORTANT_URGENT"}`)
	byTitle := decodeTask(t, titled.Body.Bytes())

	described := c.do(t, http.MethodPost, "/tasks",
		`{"title":"Look into it","description":"the refresh token races with the cookie","quadrant":"IMPORTANT_NOT_URGENT"}`)
	byDescription := decodeTask(t, described.Body.Bytes())

	tagged := c.do(t, http.MethodPost, "/tasks", `{"title":"Something else","quadrant":"NOT_IMPORTANT_URGENT"}`)
	byTag := decodeTask(t, tagged.Body.Bytes())
	if response := c.do(t, http.MethodPost, "/tasks/"+byTag.ID+"/tags", `{"name":"refreshing"}`); response.Code != http.StatusOK {
		t.Fatalf("tag = %d: %s", response.Code, response.Body)
	}

	t.Run("each row says why it matched", func(t *testing.T) {
		view := search(t, c, "refresh")

		found := map[string]string{}
		text := map[string]string{}
		for _, item := range view.Items {
			found[item.ID] = item.MatchedField
			text[item.ID] = item.MatchedText
		}

		if found[byTitle.ID] != "TITLE" {
			t.Errorf("title hit = %q", found[byTitle.ID])
		}
		if found[byDescription.ID] != "DESCRIPTION" {
			t.Errorf("description hit = %q", found[byDescription.ID])
		}
		if found[byTag.ID] != "TAG" || text[byTag.ID] != "#refreshing" {
			t.Errorf("tag hit = %q / %q", found[byTag.ID], text[byTag.ID])
		}
	})

	t.Run("archived tasks are searchable", func(t *testing.T) {
		if done := c.do(t, http.MethodPost, "/tasks/"+byTitle.ID+"/complete", ""); done.Code != http.StatusOK {
			t.Fatalf("complete = %d: %s", done.Code, done.Body)
		}

		view := search(t, c, "refresh")
		var seen bool
		for _, item := range view.Items {
			if item.ID == byTitle.ID {
				seen = true
				if item.Status != "COMPLETED" {
					t.Errorf("status = %q", item.Status)
				}
			}
		}
		if !seen {
			t.Error("a completed task fell out of the search")
		}
	})

	t.Run("active work comes first", func(t *testing.T) {
		view := search(t, c, "refresh")
		if len(view.Items) < 2 {
			t.Fatalf("%d items, want several", len(view.Items))
		}
		if view.Items[len(view.Items)-1].Status != "COMPLETED" {
			t.Errorf("order = %+v, want the completed task last", view.Items)
		}
	})

	t.Run("wildcards are text, not syntax", func(t *testing.T) {
		for _, term := range []string{"%", "_", "refre%sh", "refre_h"} {
			if view := search(t, c, term); len(view.Items) != 0 {
				t.Errorf("%q matched %d tasks, want none", term, len(view.Items))
			}
		}
	})

	t.Run("an empty query is refused", func(t *testing.T) {
		response := c.do(t, http.MethodGet, "/search?query=%20", "")
		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422: %s", response.Code, response.Body)
		}
	})

	t.Run("another account's tasks are not searchable", func(t *testing.T) {
		stranger := signedInClient(t)
		if view := search(t, stranger, "refresh"); len(view.Items) != 0 {
			t.Fatalf("a stranger found %d of someone else's tasks", len(view.Items))
		}
	})
}
