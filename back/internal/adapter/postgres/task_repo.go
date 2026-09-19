package postgres

import (
	"context"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/task"
)

type TaskRepository struct {
	db db
}

func NewTaskRepository(pool *pgxpool.Pool) *TaskRepository {
	return &TaskRepository{db: db{pool: pool}}
}

// ListBoard returns the tasks matching the filter, each with its tag names.
func (r *TaskRepository) ListBoard(ctx context.Context, filter task.Filter) ([]task.Task, error) {
	if err := filter.Validate(); err != nil {
		return nil, err
	}

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
		Where(sq.Eq{"t.user_id": filter.UserID}).
		Where("t.deleted_at IS NULL")

	query = applyStatus(query, filter.Status)
	query = applyQuadrants(query, filter.Quadrants)
	query = applyColors(query, filter.Colors)
	query = applyTags(query, filter.Tags)
	query = applyDeadline(query, filter)
	query = applyTopology(query, filter.Topology)
	query = applySearch(query, filter.Query)

	order, err := orderClause(filter.Sort, filter.Direction)
	if err != nil {
		return nil, err
	}
	query = query.OrderBy(order, "t.position ASC", "t.id ASC")

	// One row past the limit: the extra row is how the caller learns the
	// result was cut without counting the whole table first.
	if filter.Limit > 0 {
		query = query.Limit(uint64(filter.Limit) + 1)
	}

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
			&row.ID,
			&row.UserID,
			&row.ParentTaskID,
			&row.Title,
			&row.Description,
			&row.Quadrant,
			&row.Position,
			&row.Color,
			&row.DeadlineAt,
			&row.DeadlineHasTime,
			&row.Status,
			&row.CreatedAt,
			&row.UpdatedAt,
			&row.CompletedAt,
			&row.XPAwarded,
			&row.QuadrantAtCompletion,
			&row.CompletedVia,
			&row.DeletedAt,
			&row.Tags,
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

func applyStatus(query sq.SelectBuilder, status task.StatusFilter) sq.SelectBuilder {
	switch status {
	case task.StatusFilterActive:
		return query.Where("t.status = ?::task_status_enum", string(shared.TaskStatusActive))
	case task.StatusFilterCompleted:
		return query.Where("t.status = ?::task_status_enum", string(shared.TaskStatusCompleted))
	default:
		return query
	}
}

func applyQuadrants(query sq.SelectBuilder, quadrants []shared.Quadrant) sq.SelectBuilder {
	if len(quadrants) == 0 {
		return query
	}
	return query.Where(
		"COALESCE(t.quadrant, p.quadrant) = ANY(?::quadrant_enum[])",
		enumStrings(quadrants),
	)
}

func applyColors(query sq.SelectBuilder, colors []shared.TaskColor) sq.SelectBuilder {
	if len(colors) == 0 {
		return query
	}
	return query.Where("t.color = ANY(?)", enumStrings(colors))
}

func applyTags(query sq.SelectBuilder, tags []string) sq.SelectBuilder {
	if len(tags) == 0 {
		return query
	}
	lowered := make([]string, len(tags))
	for i, tag := range tags {
		lowered[i] = strings.ToLower(strings.TrimSpace(tag))
	}
	return query.Where(`EXISTS (
		SELECT 1 FROM task_tags tt
		JOIN tags g ON g.id = tt.tag_id
		WHERE tt.task_id = t.id AND lower(g.name) = ANY(?)
	)`, lowered)
}

func applyDeadline(query sq.SelectBuilder, filter task.Filter) sq.SelectBuilder {
	switch filter.Deadline {
	case task.DeadlineNone:
		return query.Where("t.deadline_at IS NULL")
	case task.DeadlineOverdue:
		return query.Where(sq.Lt{"t.deadline_at": filter.Now})
	case task.DeadlineToday:
		start, end := localDayBounds(filter.Now, filter.Location)
		return query.Where(sq.And{
			sq.GtOrEq{"t.deadline_at": start},
			sq.Lt{"t.deadline_at": end},
		})
	case task.DeadlineWeek:
		start, _ := localDayBounds(filter.Now, filter.Location)
		return query.Where(sq.And{
			sq.GtOrEq{"t.deadline_at": start},
			sq.Lt{"t.deadline_at": start.AddDate(0, 0, 7)},
		})
	default:
		return query
	}
}

func applyTopology(query sq.SelectBuilder, topology task.TopologyFilter) sq.SelectBuilder {
	const linked = `EXISTS (
		SELECT 1 FROM task_links l
		WHERE l.source_task_id = t.id OR l.target_task_id = t.id
	)`
	switch topology {
	case task.TopologyLinked:
		return query.Where(linked)
	case task.TopologyUnlinked:
		return query.Where("NOT " + linked)
	default:
		return query
	}
}

func applySearch(query sq.SelectBuilder, term string) sq.SelectBuilder {
	term = strings.TrimSpace(term)
	if term == "" {
		return query
	}
	pattern := containsPattern(term)
	return query.Where(`(
		t.title ILIKE ? ESCAPE '\'
		OR t.description ILIKE ? ESCAPE '\'
		OR EXISTS (
			SELECT 1 FROM task_tags tt
			JOIN tags g ON g.id = tt.tag_id
			WHERE tt.task_id = t.id AND g.name ILIKE ? ESCAPE '\'
		)
	)`, pattern, pattern, pattern)
}
