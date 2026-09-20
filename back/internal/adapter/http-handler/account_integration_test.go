//go:build integration

package httphandler_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	httphandler "github.com/moxicom/cursed_matrix/back/internal/adapter/http-handler"
)

func TestExportAndDeleteAccount(t *testing.T) {
	handler := server(t)
	username := "acc_" + randomSuffix()
	c := registerAs(t, handler, username)

	kept := newTask(t, c, "kept", "IMPORTANT_URGENT")
	gone := newTask(t, c, "gone", "IMPORTANT_URGENT")
	if response := c.do(t, http.MethodPost, "/tasks/"+kept.ID+"/tags", `{"name":"backend"}`); response.Code != http.StatusOK {
		t.Fatalf("tag = %d: %s", response.Code, response.Body)
	}
	body := `{"sourceTaskId":"` + kept.ID + `","targetTaskId":"` + gone.ID + `"}`
	if response := c.do(t, http.MethodPost, "/links", body); response.Code != http.StatusCreated {
		t.Fatalf("link = %d: %s", response.Code, response.Body)
	}
	if response := c.do(t, http.MethodPost, "/tasks/"+kept.ID+"/complete", ""); response.Code != http.StatusOK {
		t.Fatalf("complete = %d: %s", response.Code, response.Body)
	}

	type exportView struct {
		ExportedAt string `json:"exportedAt"`
		Account    struct {
			Username string `json:"username"`
		} `json:"account"`
		Tasks []struct {
			Title     string   `json:"title"`
			Tags      []string `json:"tags"`
			DeletedAt *string  `json:"deletedAt"`
		} `json:"tasks"`
		Links        []linkView `json:"links"`
		Tags         []tagView  `json:"tags"`
		Achievements []struct {
			Code       string `json:"code"`
			UnlockedAt string `json:"unlockedAt"`
		} `json:"achievements"`
	}

	var export exportView
	t.Run("the export holds everything the account has", func(t *testing.T) {
		response := c.do(t, http.MethodPost, "/me/export", "")
		if response.Code != http.StatusOK {
			t.Fatalf("export = %d: %s", response.Code, response.Body)
		}
		// A file to keep, not a page to read.
		if disposition := response.Header().Get("Content-Disposition"); !strings.Contains(disposition, "attachment") {
			t.Errorf("Content-Disposition = %q", disposition)
		}
		if err := json.Unmarshal(response.Body.Bytes(), &export); err != nil {
			t.Fatalf("export: %v", err)
		}

		if export.Account.Username != username {
			t.Errorf("account = %q, want %q", export.Account.Username, username)
		}
		if len(export.Tasks) != 2 {
			t.Fatalf("%d tasks, want both", len(export.Tasks))
		}
		if len(export.Links) != 1 || len(export.Tags) != 1 {
			t.Errorf("links = %d, tags = %d, want one each", len(export.Links), len(export.Tags))
		}
		if len(export.Achievements) != 1 || export.Achievements[0].Code != "FIRST_BLOOD" {
			t.Errorf("achievements = %+v, want FIRST_BLOOD by code", export.Achievements)
		}
	})

	t.Run("a deleted task is still the user's data", func(t *testing.T) {
		if response := c.do(t, http.MethodDelete, "/tasks/"+gone.ID, ""); response.Code != http.StatusOK {
			t.Fatalf("delete = %d: %s", response.Code, response.Body)
		}

		response := c.do(t, http.MethodPost, "/me/export", "")
		var after exportView
		if err := json.Unmarshal(response.Body.Bytes(), &after); err != nil {
			t.Fatalf("export: %v", err)
		}
		if len(after.Tasks) != 2 {
			t.Fatalf("%d tasks, want the deleted one kept", len(after.Tasks))
		}

		var deleted int
		for _, item := range after.Tasks {
			if item.DeletedAt != nil {
				deleted++
			}
		}
		if deleted != 1 {
			t.Errorf("%d tasks marked deleted, want 1", deleted)
		}
	})

	t.Run("the wrong password does not close the account", func(t *testing.T) {
		response := c.do(t, http.MethodPost, "/me/delete", `{"password":"not the password"}`)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401: %s", response.Code, response.Body)
		}
		if response := c.do(t, http.MethodGet, "/me", ""); response.Code != http.StatusOK {
			t.Fatalf("the account was closed anyway: %d", response.Code)
		}
	})

	// Left active on purpose: completing it is a write that never reads the
	// account row, so it is what a disowned token would otherwise still do.
	leftover := newTask(t, c, "leftover", "IMPORTANT_URGENT")
	access := c.cookies[httphandler.AccessCookie]
	t.Run("the right password closes it", func(t *testing.T) {
		response := c.do(t, http.MethodPost, "/me/delete", `{"password":"correct horse battery"}`)
		if response.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204: %s", response.Code, response.Body)
		}
		// The session goes with the account.
		if response := c.do(t, http.MethodGet, "/me", ""); response.Code == http.StatusOK {
			t.Error("the account still answers after being closed")
		}
	})

	t.Run("the token that closed it no longer opens anything", func(t *testing.T) {
		// An access token is believed on its signature, and the task tables
		// never look at the account row — so without disowning the token a
		// closed account could keep reading and writing until it expired.
		//
		// The reads are the proof: they reach the authentication check, where
		// the token is now refused. The writes are refused one step earlier,
		// because closing the account also cleared the CSRF cookie.
		for _, path := range []string{"/tasks", "/graph", "/search?query=kept", "/achievements"} {
			response := c.do(t, http.MethodGet, path, "")
			if response.Code != http.StatusUnauthorized {
				t.Errorf("GET %s = %d, want 401: %s", path, response.Code, response.Body)
			}
		}

		for _, call := range []struct{ path, body string }{
			{"/tasks", `{"title":"after the end","quadrant":"IMPORTANT_URGENT"}`},
			{"/me/export", ""},
			{"/billing/checkout", ""},
		} {
			response := c.do(t, http.MethodPost, call.path, call.body)
			if response.Code < 400 {
				t.Errorf("POST %s = %d, want it refused: %s", call.path, response.Code, response.Body)
			}
		}
	})

	t.Run("nor does a request that carries the CSRF token back", func(t *testing.T) {
		// Past the CSRF check, the authentication check is what refuses it,
		// which is the thing being tested.
		carried := &client{handler: handler, cookies: map[string]string{}}
		for name, value := range c.cookies {
			carried.cookies[name] = value
		}
		carried.cookies[httphandler.CSRFCookie] = "carried-back"
		carried.cookies[httphandler.AccessCookie] = access

		// Completing a task touches the tasks, ledger and stats tables and
		// none of them look at the account row, so without the token being
		// disowned this would succeed and earn XP into a closed account.
		response := carried.do(t, http.MethodPost, "/tasks/"+leftover.ID+"/complete", "")
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401: %s", response.Code, response.Body)
		}
	})

	t.Run("and signing in again is refused", func(t *testing.T) {
		fresh := &client{handler: handler, cookies: map[string]string{}}
		body := `{"username":"` + username + `","password":"correct horse battery"}`
		response := fresh.do(t, http.MethodPost, "/auth/login", body)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("login = %d, want 401: %s", response.Code, response.Body)
		}
	})

	t.Run("the closed account is out of every ranking", func(t *testing.T) {
		observer := signedInClient(t)
		showInLeaderboard(t, observer, true)

		view := ranking(t, observer, "?period=ALL_TIME&limit=50")
		for _, entry := range view.Entries {
			if entry.Username == username {
				t.Fatalf("a closed account is still ranked: %+v", entry)
			}
		}
	})
}
