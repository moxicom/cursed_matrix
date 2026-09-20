package postgres

import (
	"context"
	"encoding/json"
	"time"

	sq "github.com/Masterminds/squirrel"
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

// Events lists the user's history, newest first.
//
// The page stops at the cursor's row rather than at an offset: events keep
// arriving while someone reads, and counting from the start would show one
// twice or skip one entirely.
func (r *ActivityRepository) Events(
	ctx context.Context,
	userID uuid.UUID,
	after *activity.Cursor,
	limit int,
) ([]activity.Entry, error) {
	query := builder.
		Select("e.id", "e.type::text", "e.occurred_at", "e.local_date", "e.task_id", "t.title", "e.metadata").
		From("activity_events e").
		LeftJoin("tasks t ON t.id = e.task_id").
		Where(sq.Eq{"e.user_id": userID}).
		OrderBy("e.occurred_at DESC", "e.id DESC").
		Limit(uint64(limit))

	if after != nil {
		query = query.Where("(e.occurred_at, e.id) < (?, ?)", after.OccurredAt, after.ID)
	}

	statement, args, err := query.ToSql()
	if err != nil {
		return nil, shared.WrapError(err, shared.CodeInternal, nil)
	}

	rows, err := r.db.querier(ctx).Query(ctx, statement, args...)
	if err != nil {
		return nil, mapError(err, shared.CodeInternal)
	}
	defer rows.Close()

	var entries []activity.Entry
	for rows.Next() {
		var (
			entry     activity.Entry
			eventType string
			metadata  []byte
		)
		if err := rows.Scan(&entry.ID, &eventType, &entry.OccurredAt, &entry.LocalDate,
			&entry.TaskID, &entry.TaskTitle, &metadata); err != nil {
			return nil, mapError(err, shared.CodeInternal)
		}

		entry.Type = shared.ActivityEventType(eventType)
		if err := json.Unmarshal(metadata, &entry.Metadata); err != nil {
			return nil, shared.WrapError(err, shared.CodeInternal, nil)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err, shared.CodeInternal)
	}
	return entries, nil
}

// Stats are the figures above the heatmap, read in one go.
func (r *ActivityRepository) Stats(
	ctx context.Context,
	userID uuid.UUID,
	from time.Time,
) (activity.Stats, error) {
	const query = `SELECT
			(SELECT COUNT(*) FROM activity_events e
				WHERE e.user_id = $1 AND e.local_date >= $2 AND e.type::text = ANY($3)),
			(SELECT COUNT(*) FROM activity_events e
				WHERE e.user_id = $1 AND e.local_date >= $2 AND e.type::text = ANY($4)),
			st.current_streak, st.longest_streak, st.lifetime_xp, st.level,
			(SELECT COUNT(*) FROM tasks t
				WHERE t.user_id = $1 AND t.deleted_at IS NULL AND t.status = 'ACTIVE'),
			(SELECT COUNT(*) FROM tasks t
				WHERE t.user_id = $1 AND t.deleted_at IS NULL AND t.status = 'COMPLETED')
		FROM user_stats st
		WHERE st.user_id = $1`

	var stats activity.Stats
	err := r.db.querier(ctx).QueryRow(ctx, query, userID, from,
		activity.CreationTypes(), activity.CompletionTypes()).Scan(
		&stats.CreatedLastYear, &stats.CompletedLastYear,
		&stats.CurrentStreak, &stats.LongestStreak,
		&stats.LifetimeXP, &stats.Level,
		&stats.ActiveTasks, &stats.ArchivedTasks,
	)
	if err != nil {
		return activity.Stats{}, mapError(err, shared.CodeUserNotFound)
	}
	return stats, nil
}
