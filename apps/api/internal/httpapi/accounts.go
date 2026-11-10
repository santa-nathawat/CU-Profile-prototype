package httpapi

import (
	"context"
	"log"
	"net/http"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (s *Server) registerAccounts() {
	s.registerDemoSignIn()
	s.handle("POST /auth/sign-out", s.signOut)
	s.handle("GET /me", s.requireUser(s.getMe))
	s.handle("POST /me/close", s.requireUser(s.closeAccount))
	s.handle("GET /faculties", s.requireUser(s.listLookup(`SELECT id, name FROM faculties ORDER BY name`)))
	s.handle("GET /categories", s.requireUser(s.listLookup(`SELECT id, name FROM project_categories ORDER BY name = 'Others', name`)))
	s.handle("GET /skills", s.requireUser(s.listSkills))
}

type Me struct {
	ID                  string  `json:"id"`
	Email               string  `json:"email"`
	DisplayName         *string `json:"displayName"`
	ProfileComplete     bool    `json:"profileComplete"`
	UnreadNotifications int     `json:"unreadNotifications"`
}

func (s *Server) loadMe(ctx context.Context, user User) (Me, error) {
	me := Me{ID: user.ID, Email: user.Email}
	err := s.db.QueryRow(ctx, `
		SELECT p.display_name, `+searchableSQL+`,
			(SELECT count(*) FROM notifications n WHERE n.user_id = u.id AND n.read_at IS NULL)
		FROM users u LEFT JOIN profiles p ON p.user_id = u.id
		WHERE u.id = $1`, user.ID).Scan(&me.DisplayName, &me.ProfileComplete, &me.UnreadNotifications)
	return me, err
}

func (s *Server) getMe(w http.ResponseWriter, r *http.Request, user User) {
	// The unread count is a notification read. A failed sweep shouldn't lock anyone out of every page.
	if err := s.runLazyRules(r.Context()); err != nil {
		log.Printf("lazy rules: %v", err)
	}
	me, err := s.loadMe(r.Context(), user)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, me)
}

type idName struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func scanIDName(row pgx.CollectableRow) (idName, error) {
	var item idName
	err := row.Scan(&item.ID, &item.Name)
	return item, err
}

func (s *Server) listLookup(query string) userHandler {
	return func(w http.ResponseWriter, r *http.Request, _ User) {
		rows, err := s.db.Query(r.Context(), query)
		if err != nil {
			writeErr(w, r, err)
			return
		}
		items, err := pgx.CollectRows(rows, scanIDName)
		if err != nil {
			writeErr(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string][]idName{"items": items})
	}
}

func (s *Server) listSkills(w http.ResponseWriter, r *http.Request, _ User) {
	type skill struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		IsPredefined bool   `json:"isPredefined"`
	}
	rows, err := s.db.Query(r.Context(), `
		SELECT id, name, is_predefined FROM skills
		WHERE name ILIKE $1
		ORDER BY is_predefined DESC, name
		LIMIT 20`, likePattern(r.URL.Query().Get("q")))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (skill, error) {
		var item skill
		err := row.Scan(&item.ID, &item.Name, &item.IsPredefined)
		return item, err
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]skill{"items": items})
}

func (s *Server) closeAccount(w http.ResponseWriter, r *http.Request, user User) {
	if err := readEmptyJSON(w, r); err != nil {
		writeErr(w, r, err)
		return
	}
	if err := s.inTx(r.Context(), func(tx pgx.Tx) error { return closeAccountTx(r.Context(), tx, user.ID) }); err != nil {
		writeErr(w, r, err)
		return
	}
	s.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// Account locks precede project locks in writes that can add a project, request,
// membership or lead. Closing takes the conflicting account lock first.
func lockAccounts(ctx context.Context, tx pgx.Tx, ids ...string) (bool, error) {
	for i := range ids {
		ids[i] = strings.ToLower(ids[i])
	}
	sort.Strings(ids)
	previous, allOpen := "", true
	for _, id := range ids {
		if id == previous {
			continue
		}
		previous = id
		var open bool
		if err := tx.QueryRow(ctx, `SELECT closed_at IS NULL FROM users WHERE id=$1 FOR SHARE`, id).Scan(&open); err != nil {
			return false, noRows(err, errStudentNotFound)
		}
		allOpen = allOpen && open
	}
	return allOpen, nil
}

func lockOpenAccounts(ctx context.Context, tx pgx.Tx, ids ...string) error {
	open, err := lockAccounts(ctx, tx, ids...)
	if err != nil {
		return err
	}
	if !open {
		return errStudentNotFound
	}
	return nil
}

func closeAccountTx(ctx context.Context, tx pgx.Tx, userID string) error {
	// NO KEY UPDATE blocks the shared open-account checks without blocking foreign
	// key checks from notifications written while we wait for a team transaction.
	var closed bool
	if err := tx.QueryRow(ctx, `SELECT closed_at IS NOT NULL FROM users WHERE id=$1 FOR NO KEY UPDATE`, userID).Scan(&closed); err != nil {
		return err
	}
	if closed {
		return nil
	}
	rows, err := tx.Query(ctx, `SELECT pr.id FROM projects pr WHERE pr.deleted_at IS NULL AND (
  pr.lead_user_id=$1 OR EXISTS(SELECT 1 FROM project_members m WHERE m.project_id=pr.id AND m.user_id=$1 AND m.left_at IS NULL)
  OR EXISTS(SELECT 1 FROM join_requests jr WHERE jr.project_id=pr.id AND jr.student_id=$1 AND jr.status='pending')
  OR EXISTS(SELECT 1 FROM leadership_transfers t WHERE t.project_id=pr.id AND t.status='pending' AND (t.from_user_id=$1 OR t.to_user_id=$1))
 ) ORDER BY pr.id`, userID)
	if err != nil {
		return err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	for _, id := range ids {
		project, err := lockProjectRow(ctx, tx, id, lockForWrite)
		if err != nil {
			return err
		}
		if !project.Deleted && !project.ended() && project.LeadUserID == userID {
			return conflict("must_transfer_or_delete", "Transfer or delete the teams you lead before closing your account.")
		}
	}
	if err := cancelRequestsForClosedAccount(ctx, tx, userID); err != nil {
		return err
	}
	if err := endMembershipsForClosedAccount(ctx, tx, userID); err != nil {
		return err
	}
	if err := cancelTransfersForClosedAccount(ctx, tx, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE sessions SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE users SET closed_at = now(), updated_at = now() WHERE id = $1`, userID)
	return err
}

// recipient is one person to notify about a change on a project.
type recipient struct {
	UserID, ProjectID, RequestID string
}

func collectRecipients(rows pgx.Rows) ([]recipient, error) {
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (recipient, error) {
		var item recipient
		err := row.Scan(&item.UserID, &item.ProjectID, &item.RequestID)
		return item, err
	})
}

func cancelRequestsForClosedAccount(ctx context.Context, tx pgx.Tx, userID string) error {
	rows, err := tx.Query(ctx, `
		UPDATE join_requests jr
		SET status = 'cancelled', cancel_reason = 'account_closed', resolved_at = now(), updated_at = now()
		FROM projects pr
		WHERE pr.id = jr.project_id AND jr.student_id = $1 AND jr.status = 'pending'
		RETURNING pr.lead_user_id, pr.id, jr.id::text`, userID)
	if err != nil {
		return err
	}
	cancelled, err := collectRecipients(rows)
	if err != nil {
		return err
	}
	for _, c := range cancelled {
		if err := notify(ctx, tx, c.UserID, notifRequestCancelled, c.ProjectID, c.RequestID, notificationData{"reason": "account_closed"}); err != nil {
			return err
		}
	}
	return nil
}

func endMembershipsForClosedAccount(ctx context.Context, tx pgx.Tx, userID string) error {
	rows, err := tx.Query(ctx, `
		UPDATE project_members m
		SET left_at = now(), left_reason = 'account_closed', updated_at = now()
		FROM projects pr
		WHERE pr.id = m.project_id AND m.user_id = $1 AND m.left_at IS NULL
			AND pr.status IN ('not_started', 'in_progress')
		RETURNING pr.lead_user_id, pr.id, ''`, userID)
	if err != nil {
		return err
	}
	ended, err := collectRecipients(rows)
	if err != nil {
		return err
	}
	for _, e := range ended {
		if err := notify(ctx, tx, e.UserID, notifMemberLeft, e.ProjectID, "", notificationData{"actorName": "A teammate"}); err != nil {
			return err
		}
	}
	return nil
}

func cancelTransfersForClosedAccount(ctx context.Context, tx pgx.Tx, userID string) error {
	rows, err := tx.Query(ctx, `
		UPDATE leadership_transfers
		SET status = 'cancelled', resolved_at = now(), updated_at = now()
		WHERE status = 'pending' AND (to_user_id = $1 OR from_user_id = $1)
		RETURNING CASE WHEN to_user_id = $1 THEN from_user_id ELSE to_user_id END, project_id, ''`, userID)
	if err != nil {
		return err
	}
	cancelled, err := collectRecipients(rows)
	if err != nil {
		return err
	}
	for _, c := range cancelled {
		if err := notify(ctx, tx, c.UserID, notifTransferCancelled, c.ProjectID, "", nil); err != nil {
			return err
		}
	}
	return nil
}
