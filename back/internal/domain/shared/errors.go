// Package shared holds the enums and errors the domain speaks.
package shared

import (
	"errors"
	"fmt"
)

// ErrorCode is the stable, language-independent error identifier.
type ErrorCode string

const (
	CodeValidationFailed ErrorCode = "VALIDATION_FAILED"
	CodeInternal         ErrorCode = "INTERNAL"

	// #nosec G101 -- an error code the client renders, not a credential.

	CodeInvalidCredentials ErrorCode = "INVALID_CREDENTIALS"
	CodeSessionExpired     ErrorCode = "SESSION_EXPIRED"
	// #nosec G101 -- an error code the client renders, not a credential.
	CodeCSRFTokenInvalid ErrorCode = "CSRF_TOKEN_INVALID"

	CodeSubscriptionRequired ErrorCode = "SUBSCRIPTION_REQUIRED"
	CodeQuotaLimitReached    ErrorCode = "QUOTA_LIMIT_REACHED"

	CodeTaskNotFound        ErrorCode = "TASK_NOT_FOUND"
	CodeUserNotFound        ErrorCode = "USER_NOT_FOUND"
	CodeTaskAlreadyDone     ErrorCode = "TASK_ALREADY_COMPLETED"
	CodeCompletedTaskFrozen ErrorCode = "COMPLETED_TASK_FROZEN"
	CodeHasActiveSubtasks   ErrorCode = "HAS_ACTIVE_SUBTASKS"
	CodeOrphanedNode        ErrorCode = "ORPHANED_NODE"
	CodeDuplicateLink       ErrorCode = "DUPLICATE_LINK"
	CodeSelfLink            ErrorCode = "SELF_LINK"
	CodeNestingNotAllowed   ErrorCode = "NESTING_NOT_ALLOWED"
	CodeNotASubtask         ErrorCode = "NOT_A_SUBTASK"
	CodeXPAlreadyGranted    ErrorCode = "XP_ALREADY_GRANTED"
	CodeRateLimited         ErrorCode = "RATE_LIMITED"
)

type Error struct {
	Code   ErrorCode
	Params map[string]any
	cause  error
}

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.Code, e.cause)
	}
	return string(e.Code)
}

func (e *Error) Unwrap() error { return e.cause }

func NewError(code ErrorCode, params map[string]any) *Error {
	return &Error{Code: code, Params: params}
}

func WrapError(cause error, code ErrorCode, params map[string]any) *Error {
	return &Error{Code: code, Params: params, cause: cause}
}

// CodeOf reports the domain code of err, or CodeInternal.
func CodeOf(err error) ErrorCode {
	if domainErr, ok := errors.AsType[*Error](err); ok {
		return domainErr.Code
	}
	return CodeInternal
}
