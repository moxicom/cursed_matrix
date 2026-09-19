package postgres

import (
	"context"

	sq "github.com/Masterminds/squirrel"
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
