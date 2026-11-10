package seed

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

const (
	mint   = "6730123421@student.chula.ac.th"
	ploy   = "6830456726@student.chula.ac.th"
	tonkla = "6630789035@student.chula.ac.th"
	beam   = "6730567823@student.chula.ac.th"
)

// Stable IDs let a repeat seed preserve the project and its submitted ratings.
func seedID(key string) string {
	sum := md5.Sum([]byte("cu-profile-seed:" + key))
	return fmt.Sprintf("%x-%x-%x-%x-%x", sum[0:4], sum[4:6], sum[6:8], sum[8:10], sum[10:16])
}

func seedProjects(ctx context.Context, tx pgx.Tx) error {
	people := map[string]string{}
	for _, email := range []string{mint, ploy, tonkla, beam} {
		id, err := userIDByEmail(ctx, tx, email)
		if err != nil {
			return err
		}
		people[email] = id
	}
	projectID := seedID("project:campus-study-planner")
	var inserted string
	err := tx.QueryRow(ctx, `
		INSERT INTO projects (id, name, description, category_id, lead_user_id, status, recruitment_status,
			expected_duration_value, expected_duration_unit, expected_end_date, created_at, started_at, completed_at)
		SELECT $1, 'Campus Study Planner', 'A shared planner for study sessions and coursework.', c.id, $2,
			'completed', 'closed', 29, 'day', ((now() - interval '20 days') AT TIME ZONE 'Asia/Bangkok')::date,
			now() - interval '50 days', now() - interval '49 days', now() - interval '20 days'
		FROM project_categories c WHERE c.name = 'Side project'
		ON CONFLICT DO NOTHING RETURNING id`, projectID, people[mint]).Scan(&inserted)
	if errors.Is(err, pgx.ErrNoRows) {
		// The whole example is inserted once, so a rerun cannot undo demo edits.
		return nil
	}
	if err != nil {
		return err
	}
	for i, role := range []struct {
		name     string
		capacity int
		members  []string
	}{
		{"Developer", 2, []string{mint, beam}},
		{"Designer", 1, []string{tonkla}},
		{"Marketing", 1, []string{ploy}},
	} {
		roleID := seedID("role:campus-study-planner:" + role.name)
		_, err := tx.Exec(ctx, `
			INSERT INTO project_roles (id, project_id, name, capacity, created_at)
			SELECT $1, id, $3, $4, created_at + $5 * interval '1 millisecond' FROM projects WHERE id=$2`,
			roleID, projectID, role.name, role.capacity, i)
		if err != nil {
			return err
		}
		for j, email := range role.members {
			_, err := tx.Exec(ctx, `
				INSERT INTO project_members (id, project_id, role_id, user_id, joined_at)
				SELECT $1, id, $3, $4, created_at + $5 * interval '1 hour' FROM projects WHERE id=$2`,
				seedID("member:campus-study-planner:"+email), projectID, roleID, people[email], i+j)
			if err != nil {
				return err
			}
		}
	}
	for i, rating := range []struct {
		reviewer  string
		recipient string
		score     int
	}{
		{mint, ploy, 5},
		{mint, tonkla, 4},
		{ploy, mint, 4},
	} {
		_, err := tx.Exec(ctx, `
			INSERT INTO ratings (id, project_id, reviewer_id, recipient_id, score, created_at, updated_at)
			SELECT $1, id, $3, $4, $5, completed_at + $6 * interval '1 hour', completed_at + $6 * interval '1 hour'
			FROM projects WHERE id=$2`, seedID("rating:campus-study-planner:"+rating.reviewer+":"+rating.recipient),
			projectID, people[rating.reviewer], people[rating.recipient], rating.score, i+1)
		if err != nil {
			return err
		}
	}
	return nil
}
