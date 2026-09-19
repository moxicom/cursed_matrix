package postgres

import (
	"context"
	"errors"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/moxicom/cursed_matrix/back/internal/domain/link"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
)

type LinkRepository struct {
	db db
}

func NewLinkRepository(pool *pgxpool.Pool) *LinkRepository {
	return &LinkRepository{db: db{pool: pool}}
}

var linkColumns = []string{"id", "user_id", "source_task_id", "target_task_id", "type::text", "created_at"}

// Create stores a link. The two uniqueness indexes are what refuse a duplicate,
// including a reversed pair of an undirected type.
func (r *LinkRepository) Create(ctx context.Context, item *link.Link) error {
	query := builder.
		Insert("task_links").
		Columns("id", "user_id", "source_task_id", "target_task_id", "type").
		Values(item.ID, item.UserID, item.SourceID, item.TargetID, string(item.Type)).
		Suffix("RETURNING created_at")

	statement, args, err := query.ToSql()
	if err != nil {
		return shared.WrapError(err, shared.CodeInternal, nil)
	}

	if err := r.db.querier(ctx).QueryRow(ctx, statement, args...).Scan(&item.CreatedAt); err != nil {
		return mapError(err, shared.CodeTaskNotFound)
	}
	return nil
}

func (r *LinkRepository) ByID(ctx context.Context, userID, linkID uuid.UUID) (*link.Link, error) {
	query := builder.
		Select(linkColumns...).
		From("task_links").
		Where(sq.Eq{"id": linkID, "user_id": userID})

	statement, args, err := query.ToSql()
	if err != nil {
		return nil, shared.WrapError(err, shared.CodeInternal, nil)
	}

	var (
		item     link.Link
		linkType string
	)
	err = r.db.querier(ctx).QueryRow(ctx, statement, args...).Scan(
		&item.ID, &item.UserID, &item.SourceID, &item.TargetID, &linkType, &item.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, shared.NewError(shared.CodeTaskNotFound, map[string]any{"linkId": linkID.String()})
	}
	if err != nil {
		return nil, mapError(err, shared.CodeTaskNotFound)
	}

	item.Type = shared.LinkType(linkType)
	return &item, nil
}

// List returns every link of the user, which is what the graph draws as edges.
func (r *LinkRepository) List(ctx context.Context, userID uuid.UUID) ([]link.Link, error) {
	query := builder.
		Select(linkColumns...).
		From("task_links").
		Where(sq.Eq{"user_id": userID}).
		OrderBy("created_at ASC", "id ASC")

	statement, args, err := query.ToSql()
	if err != nil {
		return nil, shared.WrapError(err, shared.CodeInternal, nil)
	}

	rows, err := r.db.querier(ctx).Query(ctx, statement, args...)
	if err != nil {
		return nil, mapError(err, shared.CodeInternal)
	}
	defer rows.Close()

	var links []link.Link
	for rows.Next() {
		var (
			item     link.Link
			linkType string
		)
		if err := rows.Scan(&item.ID, &item.UserID, &item.SourceID, &item.TargetID,
			&linkType, &item.CreatedAt); err != nil {
			return nil, mapError(err, shared.CodeInternal)
		}
		item.Type = shared.LinkType(linkType)
		links = append(links, item)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err, shared.CodeInternal)
	}
	return links, nil
}

// Update changes what a link means. The uniqueness indexes apply to the new
// type, so a change that collides with an existing link is refused there.
func (r *LinkRepository) Update(ctx context.Context, item *link.Link) error {
	query := builder.
		Update("task_links").
		Set("type", string(item.Type)).
		Where(sq.Eq{"id": item.ID, "user_id": item.UserID})

	statement, args, err := query.ToSql()
	if err != nil {
		return shared.WrapError(err, shared.CodeInternal, nil)
	}

	result, err := r.db.querier(ctx).Exec(ctx, statement, args...)
	if err != nil {
		return mapError(err, shared.CodeTaskNotFound)
	}
	if result.RowsAffected() == 0 {
		return shared.NewError(shared.CodeTaskNotFound, map[string]any{"linkId": item.ID.String()})
	}
	return nil
}

// Remove deletes a link outright.
//
// Unlike a task, a link carries no history: nothing was earned for making one,
// and nothing in the ledger or the activity record points at it.
func (r *LinkRepository) Remove(ctx context.Context, userID, linkID uuid.UUID) error {
	query := builder.
		Delete("task_links").
		Where(sq.Eq{"id": linkID, "user_id": userID})

	statement, args, err := query.ToSql()
	if err != nil {
		return shared.WrapError(err, shared.CodeInternal, nil)
	}

	result, err := r.db.querier(ctx).Exec(ctx, statement, args...)
	if err != nil {
		return mapError(err, shared.CodeTaskNotFound)
	}
	if result.RowsAffected() == 0 {
		return shared.NewError(shared.CodeTaskNotFound, map[string]any{"linkId": linkID.String()})
	}
	return nil
}

// Count is what the plan's link quota is measured against.
func (r *LinkRepository) Count(ctx context.Context, userID uuid.UUID) (int, error) {
	query := builder.
		Select("COUNT(*)").
		From("task_links").
		Where(sq.Eq{"user_id": userID})

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

// RemoveForTasks breaks every link touching the given tasks.
//
// Deleting a task has to release its links or the free plan's link quota would
// never recover: a link to a task the user removed still occupies a slot, and
// the graph would hold an edge pointing at a node it no longer draws.
func (r *LinkRepository) RemoveForTasks(ctx context.Context, userID uuid.UUID, taskIDs []uuid.UUID) error {
	if len(taskIDs) == 0 {
		return nil
	}

	query := builder.
		Delete("task_links").
		Where(sq.Eq{"user_id": userID}).
		Where(sq.Or{
			sq.Eq{"source_task_id": taskIDs},
			sq.Eq{"target_task_id": taskIDs},
		})

	statement, args, err := query.ToSql()
	if err != nil {
		return shared.WrapError(err, shared.CodeInternal, nil)
	}

	if _, err := r.db.querier(ctx).Exec(ctx, statement, args...); err != nil {
		return mapError(err, shared.CodeTaskNotFound)
	}
	return nil
}
