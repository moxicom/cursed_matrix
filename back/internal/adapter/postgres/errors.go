package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
)

const (
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
	pgCheckViolation      = "23514"
	pgNotNullViolation    = "23502"
)

var constraintCodes = map[string]shared.ErrorCode{
	"xp_transactions_grant_once": shared.CodeXPAlreadyGranted,
	"task_links_directed_key":    shared.CodeDuplicateLink,
	"task_links_undirected_key":  shared.CodeDuplicateLink,
	"task_links_no_self":         shared.CodeSelfLink,
	"tasks_quadrant_xor_parent":  shared.CodeValidationFailed,
	"tasks_completion_snapshot":  shared.CodeValidationFailed,
	"tasks_title_len":            shared.CodeValidationFailed,
	"tasks_description_len":      shared.CodeValidationFailed,
	"tasks_color_known":          shared.CodeValidationFailed,
	"tags_user_name_key":         shared.CodeValidationFailed,
	"users_username_key":         shared.CodeValidationFailed,
	"users_email_key":            shared.CodeValidationFailed,
}

func mapError(err error, notFound shared.ErrorCode) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return shared.NewError(notFound, nil)
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return shared.WrapError(err, shared.CodeInternal, nil)
	}

	if code, ok := constraintCodes[pgErr.ConstraintName]; ok {
		return shared.WrapError(err, code, map[string]any{"constraint": pgErr.ConstraintName})
	}

	switch pgErr.Code {
	case pgUniqueViolation:
		return shared.WrapError(err, shared.CodeValidationFailed,
			map[string]any{"constraint": pgErr.ConstraintName})
	case pgForeignKeyViolation:
		return shared.WrapError(err, notFound, map[string]any{"constraint": pgErr.ConstraintName})
	case pgCheckViolation, pgNotNullViolation:
		return shared.WrapError(err, shared.CodeValidationFailed,
			map[string]any{"constraint": pgErr.ConstraintName})
	default:
		return shared.WrapError(err, shared.CodeInternal, nil)
	}
}
