package postgres

import (
	"context"
	"encoding/json"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/moxicom/cursed_matrix/back/internal/domain/progression"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
)

type XPLedger struct {
	db db
}

func NewXPLedger(pool *pgxpool.Pool) *XPLedger {
	return &XPLedger{db: db{pool: pool}}
}

// Record appends one entry to the ledger.
//
// A unique violation here means the same completion is being rewarded twice —
// two requests racing, or a retry — and is reported as such rather than as a
// generic conflict.
func (l *XPLedger) Record(ctx context.Context, entry *progression.Entry) error {
	metadata, err := json.Marshal(entry.Metadata)
	if err != nil {
		return shared.WrapError(err, shared.CodeInternal, nil)
	}

	query := builder.
		Insert("xp_transactions").
		Columns("id", "user_id", "task_id", "amount", "source", "grant_seq", "created_at", "metadata").
		Values(entry.ID, entry.UserID, entry.TaskID, entry.Amount,
			string(entry.Source), entry.GrantSeq, entry.CreatedAt, metadata)

	statement, args, err := query.ToSql()
	if err != nil {
		return shared.WrapError(err, shared.CodeInternal, nil)
	}

	// A unique violation on xp_transactions_grant_once means the same
	// completion is being rewarded twice — two requests racing, or a retry.
	// The constraint map turns it into XP_ALREADY_GRANTED.
	if _, err := l.db.querier(ctx).Exec(ctx, statement, args...); err != nil {
		return mapError(err, shared.CodeInternal)
	}
	return nil
}

// GrantCount is how many times this task has already been rewarded, which is
// what the next grant's sequence number counts from.
func (l *XPLedger) GrantCount(ctx context.Context, taskID uuid.UUID, source shared.XPSource) (int, error) {
	query := builder.
		Select("COUNT(*)").
		From("xp_transactions").
		Where(sq.Eq{"task_id": taskID, "source": string(source)})

	statement, args, err := query.ToSql()
	if err != nil {
		return 0, shared.WrapError(err, shared.CodeInternal, nil)
	}

	var count int
	if err := l.db.querier(ctx).QueryRow(ctx, statement, args...).Scan(&count); err != nil {
		return 0, mapError(err, shared.CodeInternal)
	}
	return count, nil
}
