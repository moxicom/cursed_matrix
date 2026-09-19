package postgres

import (
	"context"
	"errors"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/task"
)

// ByID reads one task of one user, tags included.
//
// The user is part of the predicate rather than checked afterwards: a task
// belonging to someone else must be indistinguishable from one that does not
// exist.
func (r *TaskRepository) ByID(ctx context.Context, userID, taskID uuid.UUID) (*task.Task, error) {
	query := builder.
		Select(taskColumns...).
		Column("COALESCE(tag_names.names, ARRAY[]::text[]) AS tags").
		From("tasks t").
		JoinClause(`LEFT JOIN LATERAL (
			SELECT array_agg(g.name ORDER BY g.name) AS names
			FROM task_tags tt
			JOIN tags g ON g.id = tt.tag_id
			WHERE tt.task_id = t.id
		) tag_names ON TRUE`).
		Where(sq.Eq{"t.id": taskID, "t.user_id": userID}).
		Where("t.deleted_at IS NULL")

	statement, args, err := query.ToSql()
	if err != nil {
		return nil, shared.WrapError(err, shared.CodeInternal, nil)
	}

	var row taskRow
	err = r.db.querier(ctx).QueryRow(ctx, statement, args...).Scan(
		&row.ID, &row.UserID, &row.ParentTaskID, &row.Title, &row.Description,
		&row.Quadrant, &row.Position, &row.Color, &row.DeadlineAt, &row.DeadlineHasTime,
		&row.Status, &row.CreatedAt, &row.UpdatedAt, &row.CompletedAt, &row.XPAwarded,
		&row.QuadrantAtCompletion, &row.CompletedVia, &row.DeletedAt, &row.Tags,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, shared.NewError(shared.CodeTaskNotFound, map[string]any{"taskId": taskID.String()})
	}
	if err != nil {
		return nil, mapError(err, shared.CodeTaskNotFound)
	}

	item, err := row.toDomain()
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// Create stores a new task and returns the row the database actually wrote.
func (r *TaskRepository) Create(ctx context.Context, item *task.Task) error {
	query := builder.
		Insert("tasks").
		Columns(
			"id", "user_id", "parent_task_id", "title", "description", "quadrant",
			"position", "color", "deadline_at", "deadline_has_time", "status",
		).
		Values(
			item.ID, item.UserID, item.ParentID, item.Title, item.Description, quadrantArg(item.Quadrant),
			item.Position, string(item.Color), item.DeadlineAt, item.DeadlineHasTime, string(item.Status),
		).
		Suffix("RETURNING created_at, updated_at")

	statement, args, err := query.ToSql()
	if err != nil {
		return shared.WrapError(err, shared.CodeInternal, nil)
	}

	if err := r.db.querier(ctx).QueryRow(ctx, statement, args...).
		Scan(&item.CreatedAt, &item.UpdatedAt); err != nil {
		return mapError(err, shared.CodeValidationFailed)
	}
	return nil
}

// Update writes back every mutable column of one task.
//
// The whole row goes at once because the domain object is the unit the use
// case reasoned about: writing a subset would let a caller persist half a
// decision — a completed status without its XP snapshot, say, which the
// table's own CHECK would then refuse.
func (r *TaskRepository) Update(ctx context.Context, item *task.Task) error {
	query := builder.
		Update("tasks").
		Set("parent_task_id", item.ParentID).
		Set("title", item.Title).
		Set("description", item.Description).
		Set("quadrant", quadrantArg(item.Quadrant)).
		Set("position", item.Position).
		Set("color", string(item.Color)).
		Set("deadline_at", item.DeadlineAt).
		Set("deadline_has_time", item.DeadlineHasTime).
		Set("status", string(item.Status)).
		Set("completed_at", item.CompletedAt).
		Set("xp_awarded", item.XPAwarded).
		Set("quadrant_at_completion", quadrantArg(item.QuadrantAtCompletion)).
		Set("completed_via", sourceArg(item.CompletedVia)).
		Set("updated_at", sq.Expr("now()")).
		Where(sq.Eq{"id": item.ID, "user_id": item.UserID}).
		Where("deleted_at IS NULL").
		Suffix("RETURNING updated_at")

	statement, args, err := query.ToSql()
	if err != nil {
		return shared.WrapError(err, shared.CodeInternal, nil)
	}

	err = r.db.querier(ctx).QueryRow(ctx, statement, args...).Scan(&item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return shared.NewError(shared.CodeTaskNotFound, map[string]any{"taskId": item.ID.String()})
	}
	if err != nil {
		return mapError(err, shared.CodeValidationFailed)
	}
	return nil
}

// SoftDelete marks the task removed without losing it.
//
// The row stays so the XP ledger, the activity history and the graph keep
// referring to something real; only the user's working views stop showing it.
func (r *TaskRepository) SoftDelete(ctx context.Context, userID, taskID uuid.UUID, at time.Time) error {
	query := builder.
		Update("tasks").
		Set("deleted_at", at).
		Set("updated_at", at).
		Where(sq.Eq{"user_id": userID}).
		Where(sq.Or{
			sq.Eq{"id": taskID},
			// A subtask cannot outlive its parent: left behind it would be a
			// task with a parent the board no longer shows.
			sq.Eq{"parent_task_id": taskID},
		}).
		Where("deleted_at IS NULL")

	statement, args, err := query.ToSql()
	if err != nil {
		return shared.WrapError(err, shared.CodeInternal, nil)
	}

	tag, err := r.db.querier(ctx).Exec(ctx, statement, args...)
	if err != nil {
		return mapError(err, shared.CodeTaskNotFound)
	}
	if tag.RowsAffected() == 0 {
		return shared.NewError(shared.CodeTaskNotFound, map[string]any{"taskId": taskID.String()})
	}
	return nil
}

// NextPosition returns the position a new task takes at the end of its list.
//
// Positions are spaced by PositionGap so a later reorder can drop a task
// between two others without renumbering the column.
func (r *TaskRepository) NextPosition(
	ctx context.Context,
	userID uuid.UUID,
	quadrant *shared.Quadrant,
	parentID *uuid.UUID,
) (int32, error) {
	query := builder.
		Select("COALESCE(MAX(position), 0)").
		From("tasks").
		Where(sq.Eq{"user_id": userID}).
		Where("deleted_at IS NULL")

	if parentID != nil {
		query = query.Where(sq.Eq{"parent_task_id": *parentID})
	} else {
		query = query.Where("parent_task_id IS NULL").Where(sq.Eq{"quadrant": quadrantArg(quadrant)})
	}

	statement, args, err := query.ToSql()
	if err != nil {
		return 0, shared.WrapError(err, shared.CodeInternal, nil)
	}

	var highest int32
	if err := r.db.querier(ctx).QueryRow(ctx, statement, args...).Scan(&highest); err != nil {
		return 0, mapError(err, shared.CodeInternal)
	}
	return highest + task.PositionGap, nil
}

// CountActive counts what the free plan's quota is measured against.
//
// Subtasks count: they are tasks, and the product caps tasks. Completed and
// deleted ones do not, which is what makes finishing something free up room.
func (r *TaskRepository) CountActive(ctx context.Context, userID uuid.UUID) (int, error) {
	query := builder.
		Select("COUNT(*)").
		From("tasks").
		Where(sq.Eq{"user_id": userID, "status": string(shared.TaskStatusActive)}).
		Where("deleted_at IS NULL")

	statement, args, err := query.ToSql()
	if err != nil {
		return 0, shared.WrapError(err, shared.CodeInternal, nil)
	}

	var count int
	if err := r.db.querier(ctx).QueryRow(ctx, statement, args...).Scan(&count); err != nil {
		return 0, mapError(err, shared.CodeInternal)
	}
	return count, nil
}

// quadrantArg renders an optional enum as a bind value.
//
// A typed nil pointer is not the same as an untyped one: passed straight
// through it reaches the enum's own Value method, which dereferences it.
func quadrantArg(quadrant *shared.Quadrant) any {
	if quadrant == nil {
		return nil
	}
	return string(*quadrant)
}

func sourceArg(source *shared.CompletionSource) any {
	if source == nil {
		return nil
	}
	return string(*source)
}
