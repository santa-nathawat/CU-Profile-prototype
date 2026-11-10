package httpapi

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// querier is satisfied by both the pool and a transaction, so shared reads work in either.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// inTx commits when fn returns nil and rolls back otherwise. Every multi-table
// write goes through here, with its notifications in the same transaction.
func (s *Server) inTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	return pgx.BeginFunc(ctx, s.db, fn)
}

// searchableSQL is the one definition of a complete profile, used for search and
// for profileComplete. It expects users as u and profiles as p.
const searchableSQL = `(p.display_name IS NOT NULL AND p.display_name <> '' AND p.faculty_id IS NOT NULL
	AND u.closed_at IS NULL
	AND EXISTS (SELECT 1 FROM user_skills searchable_skill WHERE searchable_skill.user_id = p.user_id))`

// displayNameSQL is what other students see as a name. It expects users as u and profiles as p.
const displayNameSQL = `CASE WHEN u.closed_at IS NOT NULL THEN 'Former student' ELSE COALESCE(p.display_name, 'Student') END`

// ratingWindowSQL is how long a completed project stays open for ratings.
const ratingWindowSQL = `interval '14 days'`

// publishedRatingsSQL is every rating anyone may see: the project's window has
// closed and neither account is closed.
const publishedRatingsSQL = `SELECT published.* FROM ratings published
	JOIN projects published_project ON published_project.id = published.project_id
	JOIN users published_reviewer ON published_reviewer.id = published.reviewer_id
	JOIN users published_recipient ON published_recipient.id = published.recipient_id
	WHERE published_project.status = 'completed'
		AND published_project.completed_at + ` + ratingWindowSQL + ` <= now()
		AND published_reviewer.closed_at IS NULL
		AND published_recipient.closed_at IS NULL`

// ratingJoinSQL adds rating.average and rating.count for the user in the given
// column, for example ratingJoinSQL("p.lead_user_id").
func ratingJoinSQL(userColumn string) string {
	return ` CROSS JOIN LATERAL (
		SELECT round(avg(score), 2)::float8 AS average, count(*)::int AS count
		FROM (` + publishedRatingsSQL + `) visible WHERE visible.recipient_id = ` + userColumn + `
	) rating `
}

// isProfileComplete backs the 409 profile_incomplete rule on posting, applying and inviting.
func isProfileComplete(ctx context.Context, q querier, userID string) (bool, error) {
	var complete bool
	err := q.QueryRow(ctx, `SELECT `+searchableSQL+` FROM users u JOIN profiles p ON p.user_id = u.id WHERE u.id = $1`, userID).Scan(&complete)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	return complete, err
}
