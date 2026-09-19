package postgres

import (
	"context"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/tag"
)

type TagRepository struct {
	db db
}

func NewTagRepository(pool *pgxpool.Pool) *TagRepository {
	return &TagRepository{db: db{pool: pool}}
}

// List returns the user's tags with how many live tasks carry each.
//
// Deleted tasks are not counted: a tag whose only tasks are gone reads as
// empty, which is what the filter should show.
func (r *TagRepository) List(ctx context.Context, userID uuid.UUID) ([]tag.Tag, error) {
	query := builder.
		Select("g.id", "g.name", "COUNT(t.id) AS task_count").
		From("tags g").
		LeftJoin("task_tags tt ON tt.tag_id = g.id").
		LeftJoin("tasks t ON t.id = tt.task_id AND t.deleted_at IS NULL").
		Where(sq.Eq{"g.user_id": userID}).
		GroupBy("g.id", "g.name").
		OrderBy("task_count DESC", "lower(g.name) ASC")

	statement, args, err := query.ToSql()
	if err != nil {
		return nil, shared.WrapError(err, shared.CodeInternal, nil)
	}

	rows, err := r.db.querier(ctx).Query(ctx, statement, args...)
	if err != nil {
		return nil, mapError(err, shared.CodeInternal)
	}
	defer rows.Close()

	var tags []tag.Tag
	for rows.Next() {
		item := tag.Tag{UserID: userID}
		if err := rows.Scan(&item.ID, &item.Name, &item.TaskCount); err != nil {
			return nil, mapError(err, shared.CodeInternal)
		}
		tags = append(tags, item)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err, shared.CodeInternal)
	}
	return tags, nil
}

// Upsert returns the user's tag of that name, creating it if it is new.
//
// Uniqueness folds case, so asking for "Backend" when "backend" exists returns
// the existing one rather than a second tag the user would see as a duplicate.
func (r *TagRepository) Upsert(ctx context.Context, userID uuid.UUID, name string) (*tag.Tag, error) {
	const upsert = `INSERT INTO tags (user_id, name) VALUES ($1, $2)
		ON CONFLICT (user_id, lower(name)) DO UPDATE SET name = tags.name
		RETURNING id, name`

	item := tag.Tag{UserID: userID}
	if err := r.db.querier(ctx).QueryRow(ctx, upsert, userID, name).
		Scan(&item.ID, &item.Name); err != nil {
		return nil, mapError(err, shared.CodeValidationFailed)
	}
	return &item, nil
}

// Attach puts a tag on a task; putting it there twice is not an error.
func (r *TagRepository) Attach(ctx context.Context, userID, taskID, tagID uuid.UUID) error {
	const attach = `INSERT INTO task_tags (task_id, tag_id, user_id) VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING`

	if _, err := r.db.querier(ctx).Exec(ctx, attach, taskID, tagID, userID); err != nil {
		return mapError(err, shared.CodeTaskNotFound)
	}
	return nil
}

// Detach takes a tag off a task.
func (r *TagRepository) Detach(ctx context.Context, userID, taskID, tagID uuid.UUID) error {
	query := builder.
		Delete("task_tags").
		Where(sq.Eq{"task_id": taskID, "tag_id": tagID, "user_id": userID})

	statement, args, err := query.ToSql()
	if err != nil {
		return shared.WrapError(err, shared.CodeInternal, nil)
	}

	result, err := r.db.querier(ctx).Exec(ctx, statement, args...)
	if err != nil {
		return mapError(err, shared.CodeTaskNotFound)
	}
	if result.RowsAffected() == 0 {
		return shared.NewError(shared.CodeTaskNotFound,
			map[string]any{"taskId": taskID.String(), "tagId": tagID.String()})
	}
	return nil
}

// DeleteOrphans removes the user's tags that no longer label anything.
//
// A tag exists to be filtered by, and one on no tasks is clutter in the filter
// list. The client is not told, because it cannot observe the difference.
func (r *TagRepository) DeleteOrphans(ctx context.Context, userID uuid.UUID) error {
	const prune = `DELETE FROM tags g
		WHERE g.user_id = $1
		  AND NOT EXISTS (SELECT 1 FROM task_tags tt WHERE tt.tag_id = g.id)`

	if _, err := r.db.querier(ctx).Exec(ctx, prune, userID); err != nil {
		return mapError(err, shared.CodeInternal)
	}
	return nil
}
