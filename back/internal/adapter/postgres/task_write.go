package postgres

import (
	"context"
	"errors"
	"strconv"
	"strings"
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

// SoftDelete marks tasks removed without losing them.
//
// The rows stay so the XP ledger, the activity history and the graph keep
// referring to something real; only the user's working views stop showing
// them.
func (r *TaskRepository) SoftDelete(
	ctx context.Context,
	userID uuid.UUID,
	taskIDs []uuid.UUID,
	at time.Time,
) error {
	if len(taskIDs) == 0 {
		return nil
	}

	query := builder.
		Update("tasks").
		Set("deleted_at", at).
		Set("updated_at", at).
		Where(sq.Eq{"user_id": userID, "id": taskIDs}).
		Where("deleted_at IS NULL")

	statement, args, err := query.ToSql()
	if err != nil {
		return shared.WrapError(err, shared.CodeInternal, nil)
	}

	tag, err := r.db.querier(ctx).Exec(ctx, statement, args...)
	if err != nil {
		return mapError(err, shared.CodeTaskNotFound)
	}
	if tag.RowsAffected() != int64(len(taskIDs)) {
		return shared.NewError(shared.CodeTaskNotFound,
			map[string]any{"expected": len(taskIDs), "deleted": tag.RowsAffected()})
	}
	return nil
}

// ScopeTasks returns the top-level tasks of one quadrant, in user order.
//
// This is the list a move is placed into, so it carries no subtasks: they are
// ordered within their parent, not within the quadrant.
func (r *TaskRepository) ScopeTasks(
	ctx context.Context,
	userID uuid.UUID,
	quadrant shared.Quadrant,
) ([]task.Task, error) {
	query := builder.
		Select("t.id", "t.position").
		From("tasks t").
		Where(sq.Eq{"t.user_id": userID, "t.quadrant": string(quadrant)}).
		Where("t.parent_task_id IS NULL").
		Where("t.deleted_at IS NULL").
		OrderBy("t.position ASC", "t.id ASC")

	statement, args, err := query.ToSql()
	if err != nil {
		return nil, shared.WrapError(err, shared.CodeInternal, nil)
	}

	rows, err := r.db.querier(ctx).Query(ctx, statement, args...)
	if err != nil {
		return nil, mapError(err, shared.CodeTaskNotFound)
	}
	defer rows.Close()

	var scope []task.Task
	for rows.Next() {
		var item task.Task
		if err := rows.Scan(&item.ID, &item.Position); err != nil {
			return nil, mapError(err, shared.CodeTaskNotFound)
		}
		scope = append(scope, item)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err, shared.CodeTaskNotFound)
	}
	return scope, nil
}

// Reposition writes a rebalanced scope back in one statement.
func (r *TaskRepository) Reposition(
	ctx context.Context,
	userID uuid.UUID,
	placements []task.Placement,
) error {
	if len(placements) == 0 {
		return nil
	}

	values := make([]string, 0, len(placements))
	args := make([]any, 0, len(placements)*2+1)
	args = append(args, userID)

	// The only thing built into the text is the placeholder number; every
	// value is bound. The package forbids fmt.Sprintf here so that SQL is
	// never assembled from anything that could carry a value.
	for i, placement := range placements {
		values = append(values,
			"($"+strconv.Itoa(i*2+2)+"::uuid, $"+strconv.Itoa(i*2+3)+"::int)")
		args = append(args, placement.ID, placement.Position)
	}

	statement := `UPDATE tasks AS t SET position = v.position, updated_at = now()
		FROM (VALUES ` + strings.Join(values, ", ") + `) AS v (id, position)
		WHERE t.id = v.id AND t.user_id = $1 AND t.deleted_at IS NULL`

	if _, err := r.db.querier(ctx).Exec(ctx, statement, args...); err != nil {
		return mapError(err, shared.CodeTaskNotFound)
	}
	return nil
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

// Subtasks returns the children of one task, in their user order.
func (r *TaskRepository) Subtasks(ctx context.Context, userID, parentID uuid.UUID) ([]task.Task, error) {
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
		Where(sq.Eq{"t.user_id": userID, "t.parent_task_id": parentID}).
		Where("t.deleted_at IS NULL").
		OrderBy("t.position ASC", "t.id ASC")

	statement, args, err := query.ToSql()
	if err != nil {
		return nil, shared.WrapError(err, shared.CodeInternal, nil)
	}

	rows, err := r.db.querier(ctx).Query(ctx, statement, args...)
	if err != nil {
		return nil, mapError(err, shared.CodeTaskNotFound)
	}
	defer rows.Close()

	var subtasks []task.Task
	for rows.Next() {
		var row taskRow
		if err := rows.Scan(
			&row.ID, &row.UserID, &row.ParentTaskID, &row.Title, &row.Description,
			&row.Quadrant, &row.Position, &row.Color, &row.DeadlineAt, &row.DeadlineHasTime,
			&row.Status, &row.CreatedAt, &row.UpdatedAt, &row.CompletedAt, &row.XPAwarded,
			&row.QuadrantAtCompletion, &row.CompletedVia, &row.DeletedAt, &row.Tags,
		); err != nil {
			return nil, mapError(err, shared.CodeTaskNotFound)
		}
		item, err := row.toDomain()
		if err != nil {
			return nil, err
		}
		subtasks = append(subtasks, item)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err, shared.CodeTaskNotFound)
	}
	return subtasks, nil
}

// ByIDForUpdate reads a task and holds its row for the rest of the
// transaction.
//
// Completing, reopening, deleting and moving all decide what to do from the
// task's current state. Without the lock two of them can read the same state,
// both decide it is safe, and both act — paying XP twice for one completion,
// or withdrawing it twice for one reopening.
func (r *TaskRepository) ByIDForUpdate(ctx context.Context, userID, taskID uuid.UUID) (*task.Task, error) {
	// Locking on its own statement rather than on the read: the read joins the
	// tags laterally, and a lock cannot be taken through the nullable side of
	// an outer join.
	const lock = `SELECT 1 FROM tasks
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
		FOR UPDATE`

	var held int
	err := r.db.querier(ctx).QueryRow(ctx, lock, taskID, userID).Scan(&held)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, shared.NewError(shared.CodeTaskNotFound, map[string]any{"taskId": taskID.String()})
	}
	if err != nil {
		return nil, mapError(err, shared.CodeTaskNotFound)
	}
	return r.ByID(ctx, userID, taskID)
}

// All returns every task the account ever had, deleted ones included.
//
// The board's cap does not apply: an export that quietly stopped at five
// hundred tasks would be worse than no export, because nothing would say so.
func (r *TaskRepository) All(ctx context.Context, userID uuid.UUID) ([]task.Task, error) {
	query := builder.
		Select(taskColumns...).
		Column("COALESCE(tag_names.names, ARRAY[]::text[]) AS tags").
		From("tasks t").
		LeftJoin("tasks p ON p.id = t.parent_task_id").
		JoinClause(`LEFT JOIN LATERAL (
			SELECT array_agg(g.name ORDER BY g.name) AS names
			FROM task_tags tt
			JOIN tags g ON g.id = tt.tag_id
			WHERE tt.task_id = t.id
		) tag_names ON TRUE`).
		Where(sq.Eq{"t.user_id": userID}).
		OrderBy("t.created_at ASC", "t.id ASC")

	statement, args, err := query.ToSql()
	if err != nil {
		return nil, shared.WrapError(err, shared.CodeInternal, nil)
	}

	rows, err := r.db.querier(ctx).Query(ctx, statement, args...)
	if err != nil {
		return nil, mapError(err, shared.CodeTaskNotFound)
	}
	defer rows.Close()

	var tasks []task.Task
	for rows.Next() {
		var row taskRow
		if err := rows.Scan(
			&row.ID, &row.UserID, &row.ParentTaskID, &row.Title, &row.Description,
			&row.Quadrant, &row.Position, &row.Color, &row.DeadlineAt, &row.DeadlineHasTime,
			&row.Status, &row.CreatedAt, &row.UpdatedAt, &row.CompletedAt, &row.XPAwarded,
			&row.QuadrantAtCompletion, &row.CompletedVia, &row.DeletedAt, &row.Tags,
		); err != nil {
			return nil, mapError(err, shared.CodeTaskNotFound)
		}
		item, err := row.toDomain()
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, item)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err, shared.CodeTaskNotFound)
	}
	return tasks, nil
}
