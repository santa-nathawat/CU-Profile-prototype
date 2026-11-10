package httpapi

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

// Profile sections that read project, membership and rating rows. They return
// empty data until those features have written any.
func (s *Server) registerProfileActivity() {
	s.handle("GET /profiles/{id}/history", s.requireUser(s.profileHistory))
	s.handle("GET /profiles/{id}/ratings", s.requireUser(s.profileRatings))
	s.handle("GET /profiles/{id}/teammates", s.requireUser(s.profileTeammates))
}

type historyProject struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type HistoryItem struct {
	Project    historyProject `json:"project"`
	RoleName   string         `json:"roleName"`
	JoinedAt   time.Time      `json:"joinedAt"`
	LeftAt     *time.Time     `json:"leftAt"`
	LeftReason *string        `json:"leftReason"`
	EndedAt    *time.Time     `json:"endedAt"`
}

func (s *Server) profileHistory(w http.ResponseWriter, r *http.Request, _ User) {
	id, err := s.visibleProfileID(r)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	rows, err := s.db.Query(r.Context(), `
		SELECT pr.id, pr.name, pr.status::text, role.name, m.joined_at, m.left_at, m.left_reason::text,
			COALESCE(m.left_at, pr.completed_at, pr.cancelled_at)
		FROM project_members m
		JOIN projects pr ON pr.id = m.project_id
		JOIN project_roles role ON role.id = m.role_id
		WHERE m.user_id = $1 AND pr.deleted_at IS NULL
		ORDER BY m.joined_at DESC, m.id`, id)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (HistoryItem, error) {
		var item HistoryItem
		err := row.Scan(&item.Project.ID, &item.Project.Name, &item.Project.Status, &item.RoleName,
			&item.JoinedAt, &item.LeftAt, &item.LeftReason, &item.EndedAt)
		return item, err
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	current, past := []HistoryItem{}, []HistoryItem{}
	for _, item := range items {
		live := item.Project.Status == "not_started" || item.Project.Status == "in_progress"
		if live && item.LeftAt == nil {
			current = append(current, item)
		} else {
			past = append(past, item)
		}
	}
	writeJSON(w, http.StatusOK, map[string][]HistoryItem{"current": current, "past": past})
}

func (s *Server) profileRatings(w http.ResponseWriter, r *http.Request, _ User) {
	id, err := s.visibleProfileID(r)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	ctx := r.Context()
	type reviewerRef struct {
		ID          string `json:"id"`
		DisplayName string `json:"displayName"`
	}
	var rating Rating
	err = s.db.QueryRow(ctx, `SELECT rating.average, rating.count FROM users u `+ratingJoinSQL("u.id")+` WHERE u.id = $1`, id).
		Scan(&rating.Average, &rating.Count)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	rows, err := s.db.Query(ctx, `
		SELECT visible.score, u.id, `+displayNameSQL+`, pr.id, pr.name
		FROM (`+publishedRatingsSQL+`) visible
		JOIN users u ON u.id = visible.reviewer_id
		LEFT JOIN profiles p ON p.user_id = u.id
		JOIN projects pr ON pr.id = visible.project_id
		WHERE visible.recipient_id = $1
		ORDER BY pr.completed_at DESC, visible.id`, id)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	type publishedItem struct {
		Score    int         `json:"score"`
		Reviewer reviewerRef `json:"reviewer"`
		Project  idName      `json:"project"`
	}
	published, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (publishedItem, error) {
		var item publishedItem
		err := row.Scan(&item.Score, &item.Reviewer.ID, &item.Reviewer.DisplayName, &item.Project.ID, &item.Project.Name)
		return item, err
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	// Pending lists every open window this student is part of, whether or not anyone
	// has rated yet, so it doesn't reveal that a score exists.
	rows, err = s.db.Query(ctx, `
		SELECT pr.id, pr.name
		FROM project_members m JOIN projects pr ON pr.id = m.project_id
		WHERE m.user_id = $1 AND m.left_at IS NULL AND pr.status = 'completed'
			AND pr.completed_at + `+ratingWindowSQL+` > now()
		ORDER BY pr.completed_at DESC, pr.id`, id)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	pending, err := pgx.CollectRows(rows, scanIDName)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Rating          Rating          `json:"rating"`
		Published       []publishedItem `json:"published"`
		PendingProjects []idName        `json:"pendingProjects"`
	}{rating, published, pending})
}

// Past teammates shared an actual membership period. Current pairs on live teams
// are omitted. Only the frozen completion participants can have scores or pending
// ratings; a former member or cancelled project is always not rated.
func (s *Server) profileTeammates(w http.ResponseWriter, r *http.Request, _ User) {
	id, err := s.visibleProfileID(r)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	type teammateRef struct {
		ID          string `json:"id"`
		DisplayName string `json:"displayName"`
	}
	type teammateItem struct {
		Teammate teammateRef `json:"teammate"`
		Project  idName      `json:"project"`
		State    string      `json:"state"`
		Score    *int        `json:"score"`
	}
	rows, err := s.db.Query(r.Context(), `
  WITH past_pairs AS (
   SELECT pr.id,pr.name,pr.completed_at,u.id AS teammate_id,`+displayNameSQL+` AS teammate_name,
    bool_or(pr.status='completed' AND mine.left_at IS NULL AND other.left_at IS NULL) AS eligible
   FROM project_members mine
   JOIN projects pr ON pr.id=mine.project_id AND pr.deleted_at IS NULL
   JOIN project_members other ON other.project_id=pr.id AND other.user_id<>mine.user_id
   JOIN users u ON u.id=other.user_id AND u.closed_at IS NULL
   LEFT JOIN profiles p ON p.user_id=u.id
   WHERE mine.user_id=$1
    AND mine.joined_at < COALESCE(other.left_at,pr.completed_at,pr.cancelled_at,'infinity'::timestamptz)
    AND other.joined_at < COALESCE(mine.left_at,pr.completed_at,pr.cancelled_at,'infinity'::timestamptz)
    AND NOT(pr.status IN ('not_started','in_progress')
     AND EXISTS(SELECT 1 FROM project_members active_mine WHERE active_mine.project_id=pr.id AND active_mine.user_id=mine.user_id AND active_mine.left_at IS NULL)
     AND EXISTS(SELECT 1 FROM project_members active_other WHERE active_other.project_id=pr.id AND active_other.user_id=other.user_id AND active_other.left_at IS NULL))
   GROUP BY pr.id,pr.name,pr.completed_at,u.id,p.display_name
  )
  SELECT pair.teammate_id,pair.teammate_name,pair.id,pair.name,
   CASE WHEN pair.eligible AND pair.completed_at + `+ratingWindowSQL+` > now() THEN 'pending'
    WHEN pair.eligible AND given.score IS NOT NULL THEN 'published' ELSE 'not_rated' END,
   CASE WHEN pair.eligible AND pair.completed_at + `+ratingWindowSQL+` <= now() THEN given.score END
  FROM past_pairs pair
  LEFT JOIN ratings given ON pair.eligible AND given.project_id=pair.id AND given.reviewer_id=pair.teammate_id AND given.recipient_id=$1
  ORDER BY pair.completed_at DESC NULLS LAST,pair.id,pair.teammate_name`, id)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (teammateItem, error) {
		var item teammateItem
		err := row.Scan(&item.Teammate.ID, &item.Teammate.DisplayName, &item.Project.ID, &item.Project.Name, &item.State, &item.Score)
		return item, err
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]teammateItem{"items": items})
}
