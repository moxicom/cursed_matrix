package postgres

import (
	"context"

	sq "github.com/Masterminds/squirrel"

	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/task"
)

// ListGraph returns the nodes the canvas draws, filtered the same way the
// board is.
//
// The quadrant is the effective one — a subtask answers to its parent's — so
// the client does not have to resolve it while drawing, and the two counts are
// computed here for the same reason.
func (r *TaskRepository) ListGraph(ctx context.Context, filter task.Filter) ([]task.Node, error) {
	if err := filter.Validate(); err != nil {
		return nil, err
	}

	query := builder.
		Select(
			"t.id",
			"t.title",
			"t.status::text",
			"COALESCE(t.quadrant, p.quadrant)::text AS effective_quadrant",
			"t.color",
			"t.parent_task_id",
			"t.deadline_at",
		).
		Column("COALESCE(tag_names.names, ARRAY[]::text[]) AS tags").
		Column(`(SELECT COUNT(*) FROM task_links l
			WHERE l.source_task_id = t.id OR l.target_task_id = t.id) AS link_count`).
		Column(`(SELECT COUNT(*) FROM tasks c
			WHERE c.parent_task_id = t.id AND c.deleted_at IS NULL) AS subtask_count`).
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

	query = query.OrderBy("t.position ASC", "t.id ASC").Limit(uint64(filter.PageSize()) + 1)

	statement, args, err := query.ToSql()
	if err != nil {
		return nil, shared.WrapError(err, shared.CodeInternal, nil)
	}

	rows, err := r.db.querier(ctx).Query(ctx, statement, args...)
	if err != nil {
		return nil, mapError(err, shared.CodeTaskNotFound)
	}
	defer rows.Close()

	var nodes []task.Node
	for rows.Next() {
		var (
			node      task.Node
			status    string
			quadrant  *string
			colorName string
		)
		if err := rows.Scan(&node.ID, &node.Title, &status, &quadrant, &colorName,
			&node.ParentID, &node.DeadlineAt, &node.Tags,
			&node.LinkCount, &node.SubtaskCount); err != nil {
			return nil, mapError(err, shared.CodeTaskNotFound)
		}

		node.Status = shared.TaskStatus(status)
		node.Color = shared.TaskColor(colorName)
		node.IsSubtask = node.ParentID != nil
		// A subtask whose parent is gone has no quadrant to inherit; the graph
		// shows it rather than hiding the corruption.
		if quadrant != nil {
			node.Quadrant = shared.Quadrant(*quadrant)
		}
		nodes = append(nodes, node)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err, shared.CodeTaskNotFound)
	}
	return nodes, nil
}
