// Package httphandler turns domain results into HTTP responses. It is the only
// place that knows a status code.
package httphandler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/pkg/utils"
)

var statuses = map[shared.ErrorCode]int{
	shared.CodeInvalidCredentials: http.StatusUnauthorized,
	shared.CodeSessionExpired:     http.StatusUnauthorized,

	shared.CodeCSRFTokenInvalid: http.StatusForbidden,

	shared.CodeSubscriptionRequired: http.StatusPaymentRequired,
	shared.CodeQuotaLimitReached:    http.StatusPaymentRequired,
	// Billing is switched on but no provider is wired: the server cannot take
	// the money, and saying so is better than accepting the instruction.
	shared.CodeBillingUnavailable: http.StatusServiceUnavailable,

	shared.CodeTaskNotFound: http.StatusNotFound,
	shared.CodeUserNotFound: http.StatusNotFound,

	shared.CodeTaskAlreadyDone:     http.StatusConflict,
	shared.CodeCompletedTaskFrozen: http.StatusConflict,
	shared.CodeHasActiveSubtasks:   http.StatusConflict,
	shared.CodeOrphanedNode:        http.StatusConflict,
	shared.CodeDuplicateLink:       http.StatusConflict,
	shared.CodeXPAlreadyGranted:    http.StatusConflict,

	shared.CodeValidationFailed:  http.StatusUnprocessableEntity,
	shared.CodeSelfLink:          http.StatusUnprocessableEntity,
	shared.CodeNestingNotAllowed: http.StatusUnprocessableEntity,
	shared.CodeNotASubtask:       http.StatusUnprocessableEntity,

	shared.CodeRateLimited: http.StatusTooManyRequests,
	shared.CodeInternal:    http.StatusInternalServerError,
}

// StatusFor maps a domain code to its HTTP status. An unmapped code is a 500:
// a code nobody classified is a bug, not a client error.
func StatusFor(code shared.ErrorCode) int {
	if status, ok := statuses[code]; ok {
		return status
	}
	return http.StatusInternalServerError
}

type errorBody struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code      shared.ErrorCode `json:"code"`
	Params    map[string]any   `json:"params,omitempty"`
	RequestID string           `json:"requestId,omitempty"`
}

// WriteError renders a failure in the shape the client expects, and logs the
// cause. Only the code and its parameters cross the wire: the underlying error
// can carry row data, which is not the client's business.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	ctx := r.Context()
	code := shared.CodeOf(err)
	status := StatusFor(code)

	log := utils.LoggerFromContext(ctx)
	if status >= http.StatusInternalServerError {
		log.ErrorContext(ctx, "request failed", "code", code, "err", err)
	} else {
		log.DebugContext(ctx, "request refused", "code", code, "err", err)
	}

	body := errorBody{Error: errorPayload{Code: code, RequestID: utils.RequestID(ctx)}}
	if domainErr, ok := errors.AsType[*shared.Error](err); ok {
		body.Error.Params = domainErr.Params
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.ErrorContext(ctx, "writing the error body", "err", err)
	}
}

// WriteJSON renders a successful result.
func WriteJSON(w http.ResponseWriter, r *http.Request, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if payload == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		ctx := r.Context()
		utils.LoggerFromContext(ctx).ErrorContext(ctx, "writing the response body",
			"err", err, "status", slog.IntValue(status))
	}
}
