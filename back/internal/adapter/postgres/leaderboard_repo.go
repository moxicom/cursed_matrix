package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/moxicom/cursed_matrix/back/internal/domain/leaderboard"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
)

type LeaderboardRepository struct {
	db db
}

func NewLeaderboardRepository(pool *pgxpool.Pool) *LeaderboardRepository {
	return &LeaderboardRepository{db: db{pool: pool}}
}

// ranked is the ordering both the page and the standing read from.
//
// Ranks are dense, so two people on the same XP share a rank and the next one
// down is not skipped. Ties are broken by who reached the total first — which
// is the last transaction that got them there, not their first ever — and
// finally by id, so the order is the same on every request. A list that
// reshuffled between pages would show or hide people at random.
const ranked = `SELECT u.id, u.username, u.avatar_url, %s AS xp,
		st.level, st.current_streak,
		DENSE_RANK() OVER (ORDER BY %s DESC) AS rank,
		ROW_NUMBER() OVER (ORDER BY %s DESC, earned.reached_at ASC NULLS LAST, u.id ASC) AS position
	FROM users u
	JOIN user_settings s ON s.user_id = u.id
	JOIN user_stats st ON st.user_id = u.id
	LEFT JOIN LATERAL (
		SELECT COALESCE(SUM(x.amount), 0) AS total, MAX(x.created_at) AS reached_at
		FROM xp_transactions x
		WHERE x.user_id = u.id AND ($1::timestamptz IS NULL OR x.created_at >= $1)
	) earned ON TRUE
	WHERE u.deleted_at IS NULL AND s.show_in_leaderboard`

// Page returns one page of the ranking.
//
// Only users who chose to appear are counted at all: someone hidden does not
// occupy a rank that others would be pushed down by.
func (r *LeaderboardRepository) Page(
	ctx context.Context,
	from *time.Time,
	limit, offset int,
) ([]leaderboard.Entry, error) {
	query := `WITH ranking AS (` + rankedFor(from) + `)
		SELECT id, username, avatar_url, xp, level, current_streak, rank
		FROM ranking
		ORDER BY position
		LIMIT $2 OFFSET $3`

	rows, err := r.db.querier(ctx).Query(ctx, query, from, limit, offset)
	if err != nil {
		return nil, mapError(err, shared.CodeInternal)
	}
	defer rows.Close()

	var entries []leaderboard.Entry
	for rows.Next() {
		var entry leaderboard.Entry
		if err := rows.Scan(&entry.UserID, &entry.Username, &entry.AvatarURL,
			&entry.XP, &entry.Level, &entry.CurrentStreak, &entry.Rank); err != nil {
			return nil, mapError(err, shared.CodeInternal)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err, shared.CodeInternal)
	}
	return entries, nil
}

// Standing finds where one user sits, whether or not that page was asked for.
//
// The XP is read whatever their choice: opting out removes them from the
// public ranking, not from their own screen. Only the rank is withheld,
// because publicly it does not exist.
func (r *LeaderboardRepository) Standing(
	ctx context.Context,
	userID uuid.UUID,
	from *time.Time,
) (leaderboard.Standing, error) {
	query := `WITH ranking AS (` + rankedFor(from) + `),
		own AS (
			SELECT st.lifetime_xp,
			       COALESCE((SELECT SUM(x.amount) FROM xp_transactions x
					WHERE x.user_id = st.user_id
					  AND ($1::timestamptz IS NULL OR x.created_at >= $1)), 0) AS period_xp
			FROM user_stats st
			WHERE st.user_id = $2
		)
		SELECT ` + ownMetric(from) + `, r.rank IS NOT NULL, COALESCE(r.rank, 0)
		FROM own
		LEFT JOIN ranking r ON r.id = $2`

	var standing leaderboard.Standing
	err := r.db.querier(ctx).QueryRow(ctx, query, from, userID).
		Scan(&standing.XP, &standing.Visible, &standing.Rank)
	if errors.Is(err, pgx.ErrNoRows) {
		// No stats row at all, which only happens before the account is set
		// up; nothing to report and nothing wrong.
		return leaderboard.Standing{}, nil
	}
	if err != nil {
		return leaderboard.Standing{}, mapError(err, shared.CodeInternal)
	}
	return standing, nil
}

// ownMetric is the figure the user sees for themselves, matching whichever
// one the ranking is built on.
func ownMetric(from *time.Time) string {
	if from == nil {
		return "own.lifetime_xp"
	}
	return "own.period_xp"
}

// rankedFor picks the figure the ranking is built on: the period's earnings,
// or the lifetime total when there is no period.
func rankedFor(from *time.Time) string {
	metric := "earned.total"
	if from == nil {
		metric = "st.lifetime_xp"
	}
	// Only this identifier varies, and it is chosen here between two
	// constants — never from anything a caller supplied.
	return strings.ReplaceAll(ranked, "%s", metric)
}
