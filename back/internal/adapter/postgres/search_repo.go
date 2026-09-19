package postgres

import (
	"context"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/task"
)

// Search finds tasks by title, description or tag name, archived ones
// included: a user looking for something they finished is exactly who needs
// the search.
func (r *TaskRepository) Search(
	ctx context.Context,
	userID uuid.UUID,
	term string,
	limit int,
) ([]task.Hit, error) {
	pattern := containsPattern(term)

	query := builder.
		Select(
			"t.id",
			"t.title",
			"t.description",
			"t.status::text",
			"COALESCE(t.quadrant, p.quadrant)::text AS effective_quadrant",
			"t.parent_task_id IS NOT NULL AS is_subtask",
		).
		Column(`(SELECT g.name FROM task_tags tt
			JOIN tags g ON g.id = tt.tag_id
			WHERE tt.task_id = t.id AND g.name ILIKE ? ESCAPE '\'
			ORDER BY g.name LIMIT 1) AS matched_tag`, pattern).
		From("tasks t").
		LeftJoin("tasks p ON p.id = t.parent_task_id").
		Where(sq.Eq{"t.user_id": userID}).
		Where("t.deleted_at IS NULL").
		Where(`(
			t.title ILIKE ? ESCAPE '\'
			OR t.description ILIKE ? ESCAPE '\'
			OR EXISTS (
				SELECT 1 FROM task_tags tt
				JOIN tags g ON g.id = tt.tag_id
				WHERE tt.task_id = t.id AND g.name ILIKE ? ESCAPE '\'
			)
		)`, pattern, pattern, pattern).
		// Active work first, then the most recently touched: someone
		// searching is usually looking for what they are doing now.
		OrderBy("(t.status = 'ACTIVE') DESC", "t.updated_at DESC", "t.id ASC").
		Limit(uint64(limit))

	statement, args, err := query.ToSql()
	if err != nil {
		return nil, shared.WrapError(err, shared.CodeInternal, nil)
	}

	rows, err := r.db.querier(ctx).Query(ctx, statement, args...)
	if err != nil {
		return nil, mapError(err, shared.CodeTaskNotFound)
	}
	defer rows.Close()

	var hits []task.Hit
	for rows.Next() {
		var (
			hit         task.Hit
			description string
			status      string
			quadrant    *string
			matchedTag  *string
		)
		if err := rows.Scan(&hit.ID, &hit.Title, &description, &status,
			&quadrant, &hit.IsSubtask, &matchedTag); err != nil {
			return nil, mapError(err, shared.CodeTaskNotFound)
		}

		hit.Status = shared.TaskStatus(status)
		if quadrant != nil {
			hit.Quadrant = shared.Quadrant(*quadrant)
		}
		hit.MatchedField, hit.MatchedText = task.DescribeMatch(hit.Title, description, matchedTag, term)
		hits = append(hits, hit)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err, shared.CodeTaskNotFound)
	}
	return hits, nil
}
