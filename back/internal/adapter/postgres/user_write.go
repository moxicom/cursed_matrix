package postgres

import (
	"context"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/user"
)

// ByUsername loads an account by the credential it signs in with. The lookup is
// case-insensitive because the column is CITEXT.
func (r *UserRepository) ByUsername(ctx context.Context, username string) (*user.User, error) {
	return r.findBy(ctx, sq.Eq{"u.username": username})
}

// Create writes the account and the two rows that always accompany it. All
// three go in one transaction: an account without its settings or its
// aggregate is a half-made account the rest of the code does not expect.
func (r *UserRepository) Create(ctx context.Context, account *user.User) error {
	statements := []sq.Sqlizer{
		builder.Insert("users").
			Columns("id", "username", "email", "password_hash", "created_at",
				"plan", "plan_expires_at").
			Values(account.ID, account.Username, account.Email, account.PasswordHash,
				account.CreatedAt, string(account.Subscription.Plan), account.Subscription.ExpiresAt),

		builder.Insert("user_settings").
			Columns("user_id", "language", "timezone", "show_in_leaderboard").
			Values(account.ID, string(account.Settings.Language), account.Settings.Timezone,
				account.Settings.ShowInLeaderboard),

		builder.Insert("user_stats").Columns("user_id").Values(account.ID),
	}

	for _, statement := range statements {
		sql, args, err := statement.ToSql()
		if err != nil {
			return shared.WrapError(err, shared.CodeInternal, nil)
		}
		if _, err := r.db.querier(ctx).Exec(ctx, sql, args...); err != nil {
			return mapError(err, shared.CodeUserNotFound)
		}
	}
	return nil
}

// UpdateSettings replaces the preferences that belong to the account.
func (r *UserRepository) UpdateSettings(ctx context.Context, id uuid.UUID, settings user.Settings) error {
	sql, args, err := builder.Update("user_settings").
		Set("language", string(settings.Language)).
		Set("timezone", settings.Timezone).
		Set("show_in_leaderboard", settings.ShowInLeaderboard).
		Where(sq.Eq{"user_id": id}).
		ToSql()
	if err != nil {
		return shared.WrapError(err, shared.CodeInternal, nil)
	}

	tag, err := r.db.querier(ctx).Exec(ctx, sql, args...)
	if err != nil {
		return mapError(err, shared.CodeUserNotFound)
	}
	if tag.RowsAffected() == 0 {
		return shared.NewError(shared.CodeUserNotFound, map[string]any{"userId": id.String()})
	}
	return nil
}

// UpdateEmail sets or clears the optional address.
func (r *UserRepository) UpdateEmail(ctx context.Context, id uuid.UUID, email *string) error {
	sql, args, err := builder.Update("users").
		Set("email", email).
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return shared.WrapError(err, shared.CodeInternal, nil)
	}
	if _, err := r.db.querier(ctx).Exec(ctx, sql, args...); err != nil {
		return mapError(err, shared.CodeUserNotFound)
	}
	return nil
}

// TouchLogin records the moment of a successful sign-in.
func (r *UserRepository) TouchLogin(ctx context.Context, id uuid.UUID, at time.Time) error {
	sql, args, err := builder.Update("users").
		Set("last_login_at", at).
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return shared.WrapError(err, shared.CodeInternal, nil)
	}
	if _, err := r.db.querier(ctx).Exec(ctx, sql, args...); err != nil {
		return mapError(err, shared.CodeUserNotFound)
	}
	return nil
}
