package httpapi

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// requestScope picks which pending requests a bulk change applies to. Empty fields
// don't narrow it.
type requestScope struct {
	ProjectID       string
	RoleID          string
	StudentID       string
	RequestID       string
	ExceptRequestID string
}

func (scope requestScope) narrowed() bool {
	return scope.ProjectID != "" || scope.RoleID != "" || scope.StudentID != "" || scope.RequestID != ""
}

// pendingSQL is the WHERE condition for this scope, with its args. It expects join_requests as jr
// and numbers its parameters after the args it's given.
func (scope requestScope) pendingSQL(args ...any) (string, []any) {
	sql := `jr.status = 'pending'`
	add := func(column, operator, value string) {
		if value != "" {
			args = append(args, value)
			sql += fmt.Sprintf(" AND jr.%s %s $%d", column, operator, len(args))
		}
	}
	add("project_id", "=", scope.ProjectID)
	add("role_id", "=", scope.RoleID)
	add("student_id", "=", scope.StudentID)
	add("id", "=", scope.RequestID)
	add("id", "<>", scope.ExceptRequestID)
	return sql, args
}

// expireDueRequests marks past-due pending requests as expired and notifies both
// sides. An empty scope sweeps everything. SKIP LOCKED means a sweep never waits: a
// row someone holds is being decided or cancelled right now, and that transaction
// settles it, so two sweeps also can't both notify about one row.
func expireDueRequests(ctx context.Context, tx pgx.Tx, scope requestScope) error {
	where, args := scope.pendingSQL()
	rows, err := tx.Query(ctx, `
		WITH due AS (
			SELECT jr.id FROM join_requests jr
			WHERE `+where+` AND jr.expires_at <= now()
			ORDER BY jr.id FOR UPDATE SKIP LOCKED
		)
		UPDATE join_requests jr SET status = 'expired', resolved_at = now(), updated_at = now()
		FROM due, projects pr, project_roles role
		WHERE jr.id = due.id AND pr.id = jr.project_id AND role.id = jr.role_id
		RETURNING jr.id, jr.project_id, jr.student_id, pr.lead_user_id, role.name`, args...)
	if err != nil {
		return err
	}
	type expired struct{ ID, ProjectID, StudentID, LeadID, RoleName string }
	due, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (expired, error) {
		var item expired
		err := row.Scan(&item.ID, &item.ProjectID, &item.StudentID, &item.LeadID, &item.RoleName)
		return item, err
	})
	if err != nil {
		return err
	}
	for _, request := range due {
		data := notificationData{"roleName": request.RoleName}
		for _, userID := range []string{request.StudentID, request.LeadID} {
			if err := notify(ctx, tx, userID, notifRequestExpired, request.ProjectID, request.ID, data); err != nil {
				return err
			}
		}
	}
	return nil
}

// cancelPendingRequests cancels the pending requests in scope with the given
// request_cancel_reason and notifies each student. A request already past due
// expires instead, because that's what happened to it first. The caller holds the
// project lock and writes whatever change caused this in the same transaction.
//
// For example, closing recruitment: requestScope{ProjectID: id}, "recruitment_closed".
func cancelPendingRequests(ctx context.Context, tx pgx.Tx, scope requestScope, reason string) error {
	if !scope.narrowed() {
		return errors.New("cancelPendingRequests needs a scope")
	}
	if err := expireDueRequests(ctx, tx, scope); err != nil {
		return err
	}
	where, args := scope.pendingSQL(reason)
	rows, err := tx.Query(ctx, `
		UPDATE join_requests jr
		SET status = 'cancelled', cancel_reason = $1::request_cancel_reason, resolved_at = now(), updated_at = now()
		FROM project_roles role
		WHERE role.id = jr.role_id AND `+where+`
		RETURNING jr.id, jr.project_id, jr.student_id, role.name`, args...)
	if err != nil {
		return err
	}
	type cancelled struct{ ID, ProjectID, StudentID, RoleName string }
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (cancelled, error) {
		var item cancelled
		err := row.Scan(&item.ID, &item.ProjectID, &item.StudentID, &item.RoleName)
		return item, err
	})
	if err != nil {
		return err
	}
	for _, request := range items {
		err := notify(ctx, tx, request.StudentID, notifRequestCancelled, request.ProjectID, request.ID,
			notificationData{"reason": reason, "roleName": request.RoleName})
		if err != nil {
			return err
		}
	}
	return nil
}

// latestRequest backs the retry rule. A pending request past its deadline reads as
// expired, so the rule never depends on the sweep having reached it. Both results
// are empty when there has been no request.
func latestRequest(ctx context.Context, q querier, projectID, studentID string) (requestType, status string, err error) {
	err = q.QueryRow(ctx, `
		SELECT jr.type::text, `+effectiveRequestStatusSQL+` FROM join_requests jr
		WHERE jr.project_id = $1 AND jr.student_id = $2
		ORDER BY jr.created_at DESC, jr.id LIMIT 1`, projectID, studentID).Scan(&requestType, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", nil
	}
	return requestType, status, err
}

func isCurrentMember(ctx context.Context, q querier, projectID, userID string) (bool, error) {
	var member bool
	err := q.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM project_members WHERE project_id = $1 AND user_id = $2 AND left_at IS NULL)`, projectID, userID).Scan(&member)
	return member, err
}

// displayName is the name other students see, for writing into a notification.
func displayName(ctx context.Context, q querier, userID string) (string, error) {
	var name string
	err := q.QueryRow(ctx, `SELECT `+displayNameSQL+` FROM users u LEFT JOIN profiles p ON p.user_id = u.id WHERE u.id = $1`, userID).Scan(&name)
	return name, err
}
