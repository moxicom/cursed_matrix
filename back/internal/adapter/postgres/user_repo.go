package postgres

import (
	"context"
	"errors"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/app/port"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/user"
)

type UserRepository struct {
	db db
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db{pool: pool}}
}

// ByID loads an account with its settings and progression aggregate.
func (r *UserRepository) ByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	return r.findBy(ctx, sq.Eq{"u.id": id})
}

func (r *UserRepository) findBy(ctx context.Context, where sq.Sqlizer) (*user.User, error) {
	query := builder.
		Select(
			"u.id",
			"u.username::text",
			"u.email::text",
			"u.password_hash",
			"u.avatar_url",
			"u.created_at",
			"u.last_login_at",
			"u.deleted_at",
			"u.plan::text",
			"u.plan_expires_at",
			"s.language::text",
			"s.timezone",
			"s.show_in_leaderboard",
			"s.notifications_enabled",
			"st.lifetime_xp",
			"st.level",
			"st.current_streak",
			"st.longest_streak",
			"st.last_streak_date",
			"st.tasks_created",
			"st.tasks_completed",
			"st.subtasks_completed",
			"st.links_created",
			"st.achievements_unlocked",
			"st.updated_at",
		).
		From("users u").
		LeftJoin("user_settings s ON s.user_id = u.id").
		LeftJoin("user_stats st ON st.user_id = u.id").
		Where(where).
		Where("u.deleted_at IS NULL")

	statement, args, err := query.ToSql()
	if err != nil {
		return nil, shared.WrapError(err, shared.CodeInternal, nil)
	}

	var row userRow
	err = r.db.querier(ctx).QueryRow(ctx, statement, args...).Scan(
		&row.ID,
		&row.Username,
		&row.Email,
		&row.PasswordHash,
		&row.AvatarURL,
		&row.CreatedAt,
		&row.LastLoginAt,
		&row.DeletedAt,
		&row.Plan,
		&row.PlanExpiresAt,
		&row.Language,
		&row.Timezone,
		&row.ShowInLeaderboard,
		&row.NotificationsEnabled,
		&row.LifetimeXP,
		&row.Level,
		&row.CurrentStreak,
		&row.LongestStreak,
		&row.LastStreakDate,
		&row.TasksCreated,
		&row.TasksCompleted,
		&row.SubtasksCompleted,
		&row.LinksCreated,
		&row.AchievementsUnlocked,
		&row.StatsUpdatedAt,
	)
	if err != nil {
		return nil, mapError(err, shared.CodeUserNotFound)
	}

	result, err := row.toDomain()
	if err != nil {
		return nil, err
	}
	return &result, nil
}

var (
	_ port.TaskRepository = (*TaskRepository)(nil)
	_ port.UserRepository = (*UserRepository)(nil)
	_ port.TxManager      = (*TxManager)(nil)
)

// ApplyStats adds a delta to the progression aggregate and returns the totals.
//
// The arithmetic happens in the statement rather than in the service so two
// completions arriving together add up: read-modify-write would let one
// overwrite the other.
func (r *UserRepository) ApplyStats(
	ctx context.Context,
	id uuid.UUID,
	delta user.StatsDelta,
) (user.Stats, error) {
	query := builder.
		Update("user_stats").
		Set("lifetime_xp", sq.Expr("lifetime_xp + ?", delta.XP)).
		Set("tasks_created", sq.Expr("tasks_created + ?", delta.TasksCreated)).
		Set("tasks_completed", sq.Expr("tasks_completed + ?", delta.TasksCompleted)).
		Set("subtasks_completed", sq.Expr("subtasks_completed + ?", delta.SubtasksCompleted)).
		Set("links_created", sq.Expr("links_created + ?", delta.LinksCreated)).
		Set("achievements_unlocked", sq.Expr("achievements_unlocked + ?", delta.AchievementsUnlocked)).
		Where(sq.Eq{"user_id": id}).
		Suffix(`RETURNING lifetime_xp, level, current_streak, longest_streak,
			last_streak_date, tasks_created, tasks_completed, subtasks_completed,
			links_created, achievements_unlocked, updated_at`)

	statement, args, err := query.ToSql()
	if err != nil {
		return user.Stats{}, shared.WrapError(err, shared.CodeInternal, nil)
	}

	var stats user.Stats
	err = r.db.querier(ctx).QueryRow(ctx, statement, args...).Scan(
		&stats.LifetimeXP, &stats.Level, &stats.CurrentStreak, &stats.LongestStreak,
		&stats.LastStreakDate, &stats.TasksCreated, &stats.TasksCompleted,
		&stats.SubtasksCompleted, &stats.LinksCreated, &stats.AchievementsUnlocked,
		&stats.UpdatedAt,
	)
	if err != nil {
		return user.Stats{}, mapError(err, shared.CodeUserNotFound)
	}
	return stats, nil
}

// SetLevel writes the level the lifetime XP now implies.
func (r *UserRepository) SetLevel(ctx context.Context, id uuid.UUID, level int32) error {
	query := builder.
		Update("user_stats").
		Set("level", level).
		Where(sq.Eq{"user_id": id})

	statement, args, err := query.ToSql()
	if err != nil {
		return shared.WrapError(err, shared.CodeInternal, nil)
	}

	if _, err := r.db.querier(ctx).Exec(ctx, statement, args...); err != nil {
		return mapError(err, shared.CodeUserNotFound)
	}
	return nil
}

// LockAccount holds the account row until the transaction ends.
//
// The free plan's quota is counted and then acted on. Two creations running at
// once would both count the same total, both find room, and both insert, since
// two new rows share nothing to serialise on. The account row is what they
// share.
func (r *UserRepository) LockAccount(ctx context.Context, id uuid.UUID) error {
	const lock = `SELECT 1 FROM users WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`

	var held int
	err := r.db.querier(ctx).QueryRow(ctx, lock, id).Scan(&held)
	if errors.Is(err, pgx.ErrNoRows) {
		return shared.NewError(shared.CodeUserNotFound, nil)
	}
	if err != nil {
		return mapError(err, shared.CodeUserNotFound)
	}
	return nil
}

// TouchStreak counts today towards the user's streak.
//
// The whole rule is one statement so that many requests arriving together
// cannot each read "not counted yet" and each extend it. Today is the user's
// calendar day, taken from their own timezone: a streak that used the server's
// day would break for anyone far enough east or west.
func (r *UserRepository) TouchStreak(
	ctx context.Context,
	id uuid.UUID,
	at time.Time,
) (user.StreakChange, error) {
	const touch = `WITH today AS (
			SELECT ($2 AT TIME ZONE COALESCE(NULLIF(s.timezone, ''), 'UTC'))::date AS day
			FROM user_settings s
			WHERE s.user_id = $1
		)
		UPDATE user_stats st
		SET current_streak = CASE
				WHEN st.last_streak_date = today.day - 1 THEN st.current_streak + 1
				ELSE 1
			END,
			longest_streak = GREATEST(st.longest_streak, CASE
				WHEN st.last_streak_date = today.day - 1 THEN st.current_streak + 1
				ELSE 1
			END),
			last_streak_date = today.day
		FROM today
		WHERE st.user_id = $1
		  AND st.last_streak_date IS DISTINCT FROM today.day
		RETURNING st.current_streak, st.longest_streak`

	var change user.StreakChange
	err := r.db.querier(ctx).QueryRow(ctx, touch, id, at).Scan(&change.Current, &change.Longest)
	if errors.Is(err, pgx.ErrNoRows) {
		// The day was already counted; nothing moved and nothing is wrong.
		return user.StreakChange{}, nil
	}
	if err != nil {
		return user.StreakChange{}, mapError(err, shared.CodeUserNotFound)
	}

	change.Extended = true
	return change, nil
}
