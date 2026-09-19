//go:build integration

package httphandler_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	httphandler "github.com/moxicom/cursed_matrix/back/internal/adapter/http-handler"
)

func TestListTasksThroughTheRouter(t *testing.T) {
	signedIn := newClient(t)
	if response := signedIn.do(t, http.MethodPost, "/auth/register", registerBody("b_"+uuid.NewString()[:8])); response.Code != http.StatusCreated {
		t.Fatalf("register: %d %s", response.Code, response.Body)
	}

	tests := []struct {
		name       string
		path       string
		anonymous  bool
		wantStatus int
		wantCode   string
	}{
		{
			name:       "a fresh account has an empty board",
			path:       "/tasks",
			wantStatus: http.StatusOK,
		},
		{
			name:       "every filter is accepted together",
			path:       "/tasks?status=ALL&quadrants=IMPORTANT_URGENT&colors=CYAN&tags=work&deadline=WEEK&topology=LINKED&query=matrix&sort=DEADLINE&direction=DESC",
			wantStatus: http.StatusOK,
		},
		{
			name:       "an unknown enum is refused by the domain, not the database",
			path:       "/tasks?status=NOPE",
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "VALIDATION_FAILED",
		},
		{
			name:       "a bad sort direction cannot reach the ORDER BY",
			path:       "/tasks?direction=DROP+TABLE",
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "VALIDATION_FAILED",
		},
		{
			name:       "the board needs a session",
			path:       "/tasks",
			anonymous:  true,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "SESSION_EXPIRED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caller := signedIn
			if tt.anonymous {
				caller = newClient(t)
			}

			response := caller.do(t, http.MethodGet, tt.path, "")
			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", response.Code, tt.wantStatus, response.Body)
			}

			if tt.wantCode != "" {
				var body struct {
					Error struct {
						Code string `json:"code"`
					} `json:"error"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
					t.Fatalf("body: %v", err)
				}
				if body.Error.Code != tt.wantCode {
					t.Fatalf("code = %q, want %q", body.Error.Code, tt.wantCode)
				}
				return
			}

			var board struct {
				Tasks     []json.RawMessage `json:"tasks"`
				Truncated bool              `json:"truncated"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &board); err != nil {
				t.Fatalf("body: %v", err)
			}
			// The field must be present and a list even when nothing matches:
			// the client iterates it without a nil check.
			if board.Tasks == nil {
				t.Fatalf("tasks rendered as null: %s", response.Body)
			}
			if board.Truncated {
				t.Fatalf("a board of %d tasks reported itself truncated", len(board.Tasks))
			}
		})
	}
}

func TestReadCeilingStopsARunawayClient(t *testing.T) {
	handler := serverWithLimits(t, httphandler.RateLimits{
		AddressAttempts: 1000, AddressWindow: time.Minute,
		AccountAttempts: 1000, AccountWindow: time.Minute,
		ReadAttempts: 2, ReadWindow: time.Minute,
	})
	caller := &client{handler: handler, cookies: map[string]string{}}

	if response := caller.do(t, http.MethodPost, "/auth/register", registerBody("r_"+uuid.NewString()[:8])); response.Code != http.StatusCreated {
		t.Fatalf("register: %d %s", response.Code, response.Body)
	}

	tests := []struct {
		name       string
		wantStatus int
	}{
		{name: "first read", wantStatus: http.StatusOK},
		{name: "second read", wantStatus: http.StatusOK},
		{name: "third read is over the ceiling", wantStatus: http.StatusTooManyRequests},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := caller.do(t, http.MethodGet, "/tasks", "")
			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", response.Code, tt.wantStatus, response.Body)
			}
			if tt.wantStatus == http.StatusTooManyRequests && response.Header().Get("Retry-After") == "" {
				t.Error("no Retry-After on a refusal")
			}
		})
	}
}
