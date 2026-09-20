package postgres

import (
	"context"
	"encoding/json"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/moxicom/cursed_matrix/back/internal/domain/achievement"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
)

type AchievementRepository struct {
	db db
}

func NewAchievementRepository(pool *pgxpool.Pool) *AchievementRepository {
	return &AchievementRepository{db: db{pool: pool}}
}

// condition is the shape the catalogue stores its rule in.
type condition struct {
	Metric    string `json:"metric"`
	Threshold int64  `json:"threshold"`
}

// Catalogue returns every achievement there is, in the order the screen shows
// them.
func (r *AchievementRepository) Catalogue(ctx context.Context) ([]achievement.Achievement, error) {
	query := builder.
		Select("id", "code", "category::text", "condition", "reward_xp", "repeatable", "sort_order").
		From("achievements").
		OrderBy("sort_order ASC", "code ASC")

	statement, args, err := query.ToSql()
	if err != nil {
		return nil, shared.WrapError(err, shared.CodeInternal, nil)
	}

	rows, err := r.db.querier(ctx).Query(ctx, statement, args...)
	if err != nil {
		return nil, mapError(err, shared.CodeInternal)
	}
	defer rows.Close()

	var catalogue []achievement.Achievement
	for rows.Next() {
		var (
			entry []byte
			item  achievement.Achievement
		)
		if err := rows.Scan(&item.ID, &item.Code, &item.Category, &entry,
			&item.RewardXP, &item.Repeatable, &item.SortOrder); err != nil {
			return nil, mapError(err, shared.CodeInternal)
		}

		var rule condition
		if err := json.Unmarshal(entry, &rule); err != nil {
			return nil, shared.WrapError(err, shared.CodeInternal,
				map[string]any{"code": item.Code})
		}
		item.Metric = achievement.Metric(rule.Metric)
		item.Threshold = rule.Threshold
		catalogue = append(catalogue, item)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err, shared.CodeInternal)
	}
	return catalogue, nil
}

// Metrics reads everything the catalogue watches in one go.
//
// Eight achievements would otherwise be eight queries on a path that runs
// after every completion.
func (r *AchievementRepository) Metrics(ctx context.Context, userID uuid.UUID) (achievement.Metrics, error) {
	const query = `SELECT
			st.tasks_completed + st.subtasks_completed,
			(SELECT COUNT(*) FROM tasks t
				WHERE t.user_id = $1
				  AND t.deleted_at IS NULL
				  AND t.quadrant_at_completion = 'IMPORTANT_URGENT'),
			st.lifetime_xp,
			st.level,
			st.longest_streak,
			st.links_created,
			(SELECT COUNT(DISTINCT e.local_date) FROM activity_events e
				WHERE e.user_id = $1 AND e.type = 'GRAPH_OPENED')
		FROM user_stats st
		WHERE st.user_id = $1`

	var metrics achievement.Metrics
	err := r.db.querier(ctx).QueryRow(ctx, query, userID).Scan(
		&metrics.TasksCompleted, &metrics.Q1TasksCompleted, &metrics.LifetimeXP,
		&metrics.Level, &metrics.LongestStreak, &metrics.LinksCreated,
		&metrics.GraphOpenedDays,
	)
	if err != nil {
		return achievement.Metrics{}, mapError(err, shared.CodeUserNotFound)
	}
	return metrics, nil
}

// Unlocked returns what the user already holds.
func (r *AchievementRepository) Unlocked(ctx context.Context, userID uuid.UUID) ([]achievement.Unlock, error) {
	query := builder.
		Select("achievement_id", "unlocked_at").
		From("user_achievements").
		Where(sq.Eq{"user_id": userID})

	statement, args, err := query.ToSql()
	if err != nil {
		return nil, shared.WrapError(err, shared.CodeInternal, nil)
	}

	rows, err := r.db.querier(ctx).Query(ctx, statement, args...)
	if err != nil {
		return nil, mapError(err, shared.CodeInternal)
	}
	defer rows.Close()

	var held []achievement.Unlock
	for rows.Next() {
		var unlock achievement.Unlock
		if err := rows.Scan(&unlock.AchievementID, &unlock.UnlockedAt); err != nil {
			return nil, mapError(err, shared.CodeInternal)
		}
		held = append(held, unlock)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err, shared.CodeInternal)
	}
	return held, nil
}

// Unlock records an achievement and says whether it was new.
//
// Two requests can earn the same one at the same moment; the primary key
// settles it, and the loser is told nothing was unlocked rather than being
// failed.
func (r *AchievementRepository) Unlock(
	ctx context.Context,
	userID, achievementID uuid.UUID,
	at time.Time,
) (bool, error) {
	const insert = `INSERT INTO user_achievements (user_id, achievement_id, unlocked_at)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING`

	tag, err := r.db.querier(ctx).Exec(ctx, insert, userID, achievementID, at)
	if err != nil {
		return false, mapError(err, shared.CodeUserNotFound)
	}
	return tag.RowsAffected() == 1, nil
}
