package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/moxicom/cursed_matrix/back/internal/domain/activity"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
)

type ActivityRepository struct {
	db db
}

func NewActivityRepository(pool *pgxpool.Pool) *ActivityRepository {
	return &ActivityRepository{db: db{pool: pool}}
}

// insertEvent derives the calendar day from the account's own timezone.
//
// Doing it here rather than in the caller keeps one rule in one place: every
// day-based figure in the product — the heatmap, the streak — is the user's
// day, and a caller that forgot to convert would quietly file an event under
// the server's.
const insertEvent = `INSERT INTO activity_events (id, user_id, type, task_id, occurred_at, local_date, metadata)
	SELECT $1, $2, $3, $4, $5,
	       ($5 AT TIME ZONE COALESCE(NULLIF(s.timezone, ''), 'UTC'))::date,
	       $6
	FROM user_settings s
	WHERE s.user_id = $2`

// Record files one event.
func (r *ActivityRepository) Record(ctx context.Context, event *activity.Event) error {
	metadata, err := json.Marshal(event.Metadata)
	if err != nil {
		return shared.WrapError(err, shared.CodeInternal, nil)
	}

	tag, err := r.db.querier(ctx).Exec(ctx, insertEvent,
		event.ID, event.UserID, string(event.Type), event.TaskID, event.OccurredAt, metadata)
	if err != nil {
		return mapError(err, shared.CodeUserNotFound)
	}
	if tag.RowsAffected() == 0 {
		return shared.NewError(shared.CodeUserNotFound, nil)
	}
	return nil
}

// RecordMany files several events as one statement, which is what a
// completion cascade produces.
func (r *ActivityRepository) RecordMany(ctx context.Context, events []activity.Event) error {
	if len(events) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for i := range events {
		metadata, err := json.Marshal(events[i].Metadata)
		if err != nil {
			return shared.WrapError(err, shared.CodeInternal, nil)
		}
		batch.Queue(insertEvent, events[i].ID, events[i].UserID, string(events[i].Type),
			events[i].TaskID, events[i].OccurredAt, metadata)
	}

	results := r.db.querier(ctx).SendBatch(ctx, batch)
	for range events {
		tag, err := results.Exec()
		if err != nil {
			_ = results.Close()
			return mapError(err, shared.CodeUserNotFound)
		}
		// The same check the single write makes: an insert that matched no
		// settings row wrote nothing, and a silently missing event is a hole
		// in the history nobody would notice.
		if tag.RowsAffected() == 0 {
			_ = results.Close()
			return shared.NewError(shared.CodeUserNotFound, nil)
		}
	}
	// Closing reports what the unread part of the batch did, so it is the
	// second place a failure can surface and must not be discarded.
	if err := results.Close(); err != nil {
		return mapError(err, shared.CodeUserNotFound)
	}
	return nil
}

// RecordDaily files an event that may happen at most once in a user's day.
//
// The second call of the day writes nothing and says so by succeeding: the
// caller asked for the day to be marked, and it is.
func (r *ActivityRepository) RecordDaily(ctx context.Context, event *activity.Event) error {
	metadata, err := json.Marshal(event.Metadata)
	if err != nil {
		return shared.WrapError(err, shared.CodeInternal, nil)
	}

	const once = insertEvent + ` ON CONFLICT DO NOTHING`
	if _, err := r.db.querier(ctx).Exec(ctx, once,
		event.ID, event.UserID, string(event.Type), event.TaskID, event.OccurredAt, metadata); err != nil {
		return mapError(err, shared.CodeUserNotFound)
	}
	return nil
}

// Heatmap counts what each of the user's days holds.
func (r *ActivityRepository) Heatmap(
	ctx context.Context,
	userID uuid.UUID,
	from, to time.Time,
) ([]activity.Day, error) {
	// Which types count is the domain's rule, bound as parameters rather than
	// written into the text here, so the SQL cannot drift away from it.
	const query = `SELECT local_date,
			COUNT(*) FILTER (WHERE type::text = ANY($4)) AS created,
			COUNT(*) FILTER (WHERE type::text = ANY($5)) AS completed
		FROM activity_events
		WHERE user_id = $1 AND local_date BETWEEN $2 AND $3
		GROUP BY local_date
		ORDER BY local_date`

	rows, err := r.db.querier(ctx).Query(ctx, query, userID, from, to,
		activity.CreationTypes(), activity.CompletionTypes())
	if err != nil {
		return nil, mapError(err, shared.CodeInternal)
	}
	defer rows.Close()

	var days []activity.Day
	for rows.Next() {
		var day activity.Day
		if err := rows.Scan(&day.Date, &day.Created, &day.Completed); err != nil {
			return nil, mapError(err, shared.CodeInternal)
		}
		days = append(days, day)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err, shared.CodeInternal)
	}
	return days, nil
}
