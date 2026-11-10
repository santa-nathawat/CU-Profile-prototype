package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

type ratingPerson struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

type ratingTeammate struct {
	User     ratingPerson `json:"user"`
	RoleName string       `json:"roleName"`
	MyScore  *int         `json:"myScore"`
}

type publishedProjectRating struct {
	Reviewer  ratingPerson `json:"reviewer"`
	Recipient ratingPerson `json:"recipient"`
	Score     int          `json:"score"`
}

type ProjectRatings struct {
	ClosesAt   time.Time                `json:"closesAt"`
	Open       bool                     `json:"open"`
	Eligible   bool                     `json:"eligible"`
	Teammates  []ratingTeammate         `json:"teammates"`
	Published  []publishedProjectRating `json:"published"`
	StatusText *string                  `json:"statusText"`
}

// completedMembersSQL fixes eligibility to the membership rows that are frozen
// by completion. Closing an account hides its scores without changing that history.
const completedMembersSQL = `SELECT m.user_id FROM project_members m JOIN users u ON u.id=m.user_id AND u.closed_at IS NULL WHERE m.project_id=$1 AND m.left_at IS NULL`

func loadProjectRatings(ctx context.Context, q querier, id, viewerID string) (ProjectRatings, error) {
	out := ProjectRatings{Teammates: []ratingTeammate{}, Published: []publishedProjectRating{}}
	var status string
	var completedAt *time.Time
	err := q.QueryRow(ctx, `SELECT status::text,completed_at,COALESCE(completed_at + `+ratingWindowSQL+` > now(),false) FROM projects WHERE id=$1 AND deleted_at IS NULL`, id).Scan(&status, &completedAt, &out.Open)
	if err != nil {
		return out, noRows(err, errProjectNotFound)
	}
	if status != "completed" {
		return out, conflict("project_not_completed", "Ratings open after the project is completed.")
	}
	out.ClosesAt = completedAt.Add(14 * 24 * time.Hour)
	if err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM (`+completedMembersSQL+`) eligible WHERE user_id=$2)`, id, viewerID).Scan(&out.Eligible); err != nil {
		return out, err
	}
	if out.Open {
		pending := "Ratings pending"
		out.StatusText = &pending
	}
	if out.Eligible {
		rows, err := q.Query(ctx, `SELECT u.id,`+displayNameSQL+`,role.name,given.score
   FROM project_members m JOIN users u ON u.id=m.user_id AND u.closed_at IS NULL
   LEFT JOIN profiles p ON p.user_id=u.id JOIN project_roles role ON role.id=m.role_id
   LEFT JOIN ratings given ON given.project_id=m.project_id AND given.reviewer_id=$2 AND given.recipient_id=m.user_id
   WHERE m.project_id=$1 AND m.left_at IS NULL AND m.user_id<>$2 ORDER BY m.joined_at,m.id`, id, viewerID)
		if err != nil {
			return out, err
		}
		out.Teammates, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (ratingTeammate, error) {
			var item ratingTeammate
			err := row.Scan(&item.User.ID, &item.User.DisplayName, &item.RoleName, &item.MyScore)
			return item, err
		})
		if err != nil {
			return out, err
		}
	}
	if !out.Open {
		rows, err := q.Query(ctx, `SELECT reviewer.id,COALESCE(reviewer_profile.display_name,'Student'),recipient.id,COALESCE(recipient_profile.display_name,'Student'),visible.score
   FROM (`+publishedRatingsSQL+`) visible
   JOIN users reviewer ON reviewer.id=visible.reviewer_id LEFT JOIN profiles reviewer_profile ON reviewer_profile.user_id=reviewer.id
   JOIN users recipient ON recipient.id=visible.recipient_id LEFT JOIN profiles recipient_profile ON recipient_profile.user_id=recipient.id
   WHERE visible.project_id=$1 ORDER BY visible.created_at,visible.id`, id)
		if err != nil {
			return out, err
		}
		out.Published, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (publishedProjectRating, error) {
			var item publishedProjectRating
			err := row.Scan(&item.Reviewer.ID, &item.Reviewer.DisplayName, &item.Recipient.ID, &item.Recipient.DisplayName, &item.Score)
			return item, err
		})
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

func (s *Server) getProjectRatings(w http.ResponseWriter, r *http.Request, user User) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, r, errProjectNotFound)
		return
	}
	ratings, err := loadProjectRatings(r.Context(), s.db, id, user.ID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, ratings)
}

func (s *Server) putRating(w http.ResponseWriter, r *http.Request, user User) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, r, errProjectNotFound)
		return
	}
	recipientID, ok := pathID(r, "recipientId")
	if !ok {
		writeErr(w, r, forbidden("not_allowed", "Only teammates present at completion can rate each other."))
		return
	}
	var in struct {
		Score *int `json:"score"`
	}
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, r, err)
		return
	}
	if in.Score == nil || *in.Score < 1 || *in.Score > 5 {
		writeErr(w, r, badRequest("score_required", "Choose a score from one to five stars."))
		return
	}
	ctx := r.Context()
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		if err := lockOpenAccounts(ctx, tx, user.ID, recipientID); err != nil {
			if errors.Is(err, errStudentNotFound) {
				return forbidden("not_allowed", "Only teammates present at completion can rate each other.")
			}
			return err
		}
		project, err := lockProject(ctx, tx, id, lockForRead)
		if err != nil {
			return err
		}
		if project.Status != "completed" {
			return conflict("project_not_completed", "Ratings open after the project is completed.")
		}
		var count int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM (`+completedMembersSQL+`) eligible WHERE user_id=ANY($2::uuid[])`, id, []string{user.ID, recipientID}).Scan(&count); err != nil {
			return err
		}
		if user.ID == recipientID || count != 2 {
			return forbidden("not_allowed", "Only teammates present at completion can rate each other.")
		}
		// Check after an upsert waits for a concurrent edit too, so waiting on the
		// pair's unique row cannot extend the 14 day submission window.
		tag, err := tx.Exec(ctx, `INSERT INTO ratings(project_id,reviewer_id,recipient_id,score)
   SELECT $1::uuid,$2::uuid,$3::uuid,$4::smallint FROM projects pr
   WHERE pr.id=$1 AND pr.completed_at + `+ratingWindowSQL+` > clock_timestamp()
   ON CONFLICT(project_id,reviewer_id,recipient_id) DO UPDATE SET score=excluded.score,updated_at=now()
   WHERE (SELECT pr.completed_at + `+ratingWindowSQL+` > clock_timestamp() FROM projects pr WHERE pr.id=$1)`, id, user.ID, recipientID, *in.Score)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return conflict("rating_window_closed", "The 14 day rating window has closed.")
		}
		return nil
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		RecipientID string `json:"recipientId"`
		Score       int    `json:"score"`
	}{recipientID, *in.Score})
}
