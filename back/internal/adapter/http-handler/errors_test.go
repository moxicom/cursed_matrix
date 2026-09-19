package httphandler_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httphandler "github.com/moxicom/cursed_matrix/back/internal/adapter/http-handler"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
)

func TestStatusFor(t *testing.T) {
	tests := []struct {
		name string
		code shared.ErrorCode
		want int
	}{
		{name: "bad credentials", code: shared.CodeInvalidCredentials, want: http.StatusUnauthorized},
		{name: "expired session", code: shared.CodeSessionExpired, want: http.StatusUnauthorized},
		{name: "csrf mismatch", code: shared.CodeCSRFTokenInvalid, want: http.StatusForbidden},
		{name: "subscription required", code: shared.CodeSubscriptionRequired, want: http.StatusPaymentRequired},
		{name: "quota reached", code: shared.CodeQuotaLimitReached, want: http.StatusPaymentRequired},
		{name: "task not found", code: shared.CodeTaskNotFound, want: http.StatusNotFound},
		{name: "user not found", code: shared.CodeUserNotFound, want: http.StatusNotFound},
		{name: "already completed", code: shared.CodeTaskAlreadyDone, want: http.StatusConflict},
		{name: "completed task frozen", code: shared.CodeCompletedTaskFrozen, want: http.StatusConflict},
		{name: "has active subtasks", code: shared.CodeHasActiveSubtasks, want: http.StatusConflict},
		{name: "orphaned node", code: shared.CodeOrphanedNode, want: http.StatusConflict},
		{name: "duplicate link", code: shared.CodeDuplicateLink, want: http.StatusConflict},
		{name: "xp already granted", code: shared.CodeXPAlreadyGranted, want: http.StatusConflict},
		{name: "validation failed", code: shared.CodeValidationFailed, want: http.StatusUnprocessableEntity},
		{name: "self link", code: shared.CodeSelfLink, want: http.StatusUnprocessableEntity},
		{name: "nesting not allowed", code: shared.CodeNestingNotAllowed, want: http.StatusUnprocessableEntity},
		{name: "not a subtask", code: shared.CodeNotASubtask, want: http.StatusUnprocessableEntity},
		{name: "rate limited", code: shared.CodeRateLimited, want: http.StatusTooManyRequests},
		{name: "internal", code: shared.CodeInternal, want: http.StatusInternalServerError},
		{name: "a code nobody mapped", code: "SOMETHING_NEW", want: http.StatusInternalServerError},
		{name: "empty code", code: "", want: http.StatusInternalServerError},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := httphandler.StatusFor(tc.code); got != tc.want {
				t.Errorf("StatusFor(%q) = %d, want %d", tc.code, got, tc.want)
			}
		})
	}
}

func TestWriteError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   shared.ErrorCode
		wantParams map[string]any
	}{
		{
			name:       "domain error keeps its params",
			err:        shared.NewError(shared.CodeValidationFailed, map[string]any{"field": "title"}),
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   shared.CodeValidationFailed,
			wantParams: map[string]any{"field": "title"},
		},
		{
			name:       "domain error without params",
			err:        shared.NewError(shared.CodeTaskNotFound, nil),
			wantStatus: http.StatusNotFound,
			wantCode:   shared.CodeTaskNotFound,
		},
		{
			name:       "wrapped cause is not exposed",
			err:        shared.WrapError(errors.New("pq: duplicate key on users_email_key"), shared.CodeDuplicateLink, nil),
			wantStatus: http.StatusConflict,
			wantCode:   shared.CodeDuplicateLink,
		},
		{
			name:       "an unclassified error is internal",
			err:        errors.New("connection reset by peer"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   shared.CodeInternal,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)

			httphandler.WriteError(recorder, request, tc.err)

			if recorder.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", recorder.Code, tc.wantStatus)
			}
			if got := recorder.Header().Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q", got)
			}

			var body struct {
				Error struct {
					Code   shared.ErrorCode `json:"code"`
					Params map[string]any   `json:"params"`
				} `json:"error"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode %q: %v", recorder.Body.String(), err)
			}
			if body.Error.Code != tc.wantCode {
				t.Errorf("code = %q, want %q", body.Error.Code, tc.wantCode)
			}
			for key, want := range tc.wantParams {
				if body.Error.Params[key] != want {
					t.Errorf("params[%q] = %v, want %v", key, body.Error.Params[key], want)
				}
			}
			for _, leaked := range []string{"pq:", "users_email_key", "connection reset"} {
				if strings.Contains(recorder.Body.String(), leaked) {
					t.Errorf("the response leaks the cause %q: %s", leaked, recorder.Body.String())
				}
			}
		})
	}
}
