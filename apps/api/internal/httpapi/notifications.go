package httpapi

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Server) registerNotifications() {
	s.handle("GET /notifications", s.requireUser(s.listNotifications))
	s.handle("POST /notifications/read-all", s.requireUser(s.readAllNotifications))
	s.handle("POST /notifications/{id}/read", s.requireUser(s.readNotification))
}

type notificationItem struct {
	ID        string     `json:"id"`
	Type      string     `json:"type"`
	Message   string     `json:"message"`
	Link      string     `json:"link"`
	CreatedAt time.Time  `json:"createdAt"`
	ReadAt    *time.Time `json:"readAt"`
}

func (s *Server) listNotifications(w http.ResponseWriter, r *http.Request, user User) {
	ctx := r.Context()
	if err := s.runLazyRules(ctx); err != nil {
		writeErr(w, r, err)
		return
	}
	req := parsePage(r)
	unreadParam := r.URL.Query().Get("unread")
	unreadOnly := unreadParam == "true" || unreadParam == "1"

	var total, unreadCount int
	err := s.db.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE NOT $2 OR read_at IS NULL), count(*) FILTER (WHERE read_at IS NULL)
		FROM notifications WHERE user_id = $1`, user.ID, unreadOnly).Scan(&total, &unreadCount)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	rows, err := s.db.Query(ctx, `
		SELECT n.id, n.type, n.project_id, n.join_request_id, pr.name, n.data, n.created_at, n.read_at, pr.deleted_at IS NOT NULL
		FROM notifications n LEFT JOIN projects pr ON pr.id = n.project_id
		WHERE n.user_id = $1 AND (NOT $2 OR n.read_at IS NULL)
		ORDER BY n.created_at DESC, n.id
		LIMIT $3 OFFSET $4`, user.ID, unreadOnly, req.PageSize, req.Offset())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (notificationItem, error) {
		var item notificationItem
		var stored storedNotification
		err := row.Scan(&item.ID, &stored.Type, &stored.ProjectID, &stored.RequestID, &stored.ProjectName, &stored.Data, &item.CreatedAt, &item.ReadAt, &stored.ProjectDeleted)
		item.Type = stored.Type
		item.Message, item.Link = renderNotification(stored)
		return item, err
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		pageResponse[notificationItem]
		UnreadCount int `json:"unreadCount"`
	}{newPage(req, items, total), unreadCount})
}

func (s *Server) readNotification(w http.ResponseWriter, r *http.Request, user User) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, r, notFound("notification_not_found", "That notification doesn't exist."))
		return
	}
	if err := readEmptyJSON(w, r); err != nil {
		writeErr(w, r, err)
		return
	}
	tag, err := s.db.Exec(r.Context(), `
		UPDATE notifications SET read_at = COALESCE(read_at, now()) WHERE id = $1 AND user_id = $2`, id, user.ID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, r, notFound("notification_not_found", "That notification doesn't exist."))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) readAllNotifications(w http.ResponseWriter, r *http.Request, user User) {
	if err := readEmptyJSON(w, r); err != nil {
		writeErr(w, r, err)
		return
	}
	_, err := s.db.Exec(r.Context(), `UPDATE notifications SET read_at = now() WHERE user_id = $1 AND read_at IS NULL`, user.ID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
