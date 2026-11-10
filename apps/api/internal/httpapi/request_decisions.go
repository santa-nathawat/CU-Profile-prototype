package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
)

// decision describes one way a pending request ends. Each maps to one route.
type decision struct {
	RequestType  string
	ByLead       bool // false means only the student can do it
	NewStatus    string
	CancelReason string
	FillsPlace   bool
	Notification string // sent to the other side
}

var (
	approveDecision  = decision{RequestType: "application", ByLead: true, NewStatus: "approved", FillsPlace: true, Notification: notifApplicationApproved}
	rejectDecision   = decision{RequestType: "application", ByLead: true, NewStatus: "rejected", Notification: notifApplicationRejected}
	withdrawDecision = decision{RequestType: "application", NewStatus: "withdrawn", Notification: notifApplicationWithdrawn}
	acceptDecision   = decision{RequestType: "invitation", NewStatus: "accepted", FillsPlace: true, Notification: notifInvitationAccepted}
	declineDecision  = decision{RequestType: "invitation", NewStatus: "declined", Notification: notifInvitationDeclined}
	cancelDecision   = decision{RequestType: "invitation", ByLead: true, NewStatus: "cancelled", CancelReason: "lead_cancelled", Notification: notifInvitationCancelled}
)

var (
	errRequestExpired = conflict("request_expired", "This request has expired.")
	errRoleFull       = conflict("role_full", "This role is already full.")
)

func (s *Server) decide(d decision) userHandler {
	return func(w http.ResponseWriter, r *http.Request, user User) {
		id, ok := pathID(r, "id")
		if !ok {
			writeErr(w, r, errRequestNotFound)
			return
		}
		if err := readEmptyJSON(w, r); err != nil {
			writeErr(w, r, err)
			return
		}
		ctx := r.Context()
		if err := s.runLazyRules(ctx); err != nil {
			writeErr(w, r, err)
			return
		}
		var request Request
		var expiredNow bool
		err := s.inTx(ctx, func(tx pgx.Tx) error {
			var err error
			request, expiredNow, err = decideTx(ctx, tx, id, user, d)
			return err
		})
		if err == nil && expiredNow {
			err = errRequestExpired
		}
		if err != nil {
			writeErr(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, request)
	}
}

// decideTx applies d to the request. When the deadline passed after the sweep ran, it
// reports expiredNow with no error so the transaction commits: the expiry must stick
// even though the caller answers 409.
func decideTx(ctx context.Context, tx pgx.Tx, id string, user User, d decision) (request Request, expiredNow bool, err error) {
	// These columns never change, so reading them before taking any lock is safe.
	var projectID, roleID, roleName, requestType, studentID, initialLeadID string
	err = tx.QueryRow(ctx, `
		SELECT jr.project_id, jr.role_id, role.name, jr.type::text, jr.student_id,pr.lead_user_id
		FROM join_requests jr JOIN project_roles role ON role.id = jr.role_id JOIN projects pr ON pr.id=jr.project_id
		WHERE jr.id = $1`, id).Scan(&projectID, &roleID, &roleName, &requestType, &studentID, &initialLeadID)
	if errors.Is(err, pgx.ErrNoRows) {
		return request, false, errRequestNotFound
	}
	if err != nil {
		return request, false, err
	}
	if user.ID != studentID && user.ID != initialLeadID {
		return request, false, errRequestNotFound
	}
	accountsOpen, err := lockAccounts(ctx, tx, user.ID, studentID)
	if err != nil {
		return request, false, err
	}
	project, err := lockProjectRow(ctx, tx, projectID, lockForRead)
	if err != nil {
		return request, false, err
	}
	isLead, isStudent := project.LeadUserID == user.ID, studentID == user.ID
	if !isLead && !isStudent {
		return request, false, errRequestNotFound
	}
	if requestType != d.RequestType {
		return request, false, conflict("wrong_request_type", "This action doesn't apply to this kind of request.")
	}
	if (d.ByLead && !isLead) || (!d.ByLead && !isStudent) {
		return request, false, forbidden("not_allowed", "You can't do this for this request.")
	}

	var capacity int
	var roleRemoved bool
	if d.FillsPlace {
		err = tx.QueryRow(ctx, `SELECT capacity, removed_at IS NOT NULL FROM project_roles WHERE id = $1 FOR UPDATE`, roleID).Scan(&capacity, &roleRemoved)
		if err != nil {
			return request, false, err
		}
	}
	var status string
	var cancelReason *string
	var pastDue bool
	err = tx.QueryRow(ctx, `
		SELECT status::text, cancel_reason::text, expires_at <= now() FROM join_requests WHERE id = $1 FOR UPDATE`, id).Scan(&status, &cancelReason, &pastDue)
	if err != nil {
		return request, false, err
	}
	switch {
	case status == "pending" && pastDue:
		if err := expireDueRequests(ctx, tx, requestScope{RequestID: id}); err != nil {
			return request, false, err
		}
		return request, true, nil
	case status == "expired":
		return request, false, errRequestExpired
	case status == "cancelled" && d.FillsPlace && cancelReason != nil && *cancelReason == "role_full":
		// This is how the loser of a race for the last place ends up, so say what happened.
		return request, false, errRoleFull
	case status != "pending":
		return request, false, conflict("request_not_pending", "This request has already been answered.")
	}

	if d.FillsPlace {
		if !accountsOpen {
			return request, false, errStudentNotFound
		}
		if project.ended() {
			return request, false, errTeamEnded
		}
		if project.Recruitment != "open" {
			return request, false, conflict("recruitment_closed", "This team isn't recruiting right now.")
		}
		if roleRemoved {
			return request, false, errRoleFull
		}
		if err := fillPlace(ctx, tx, id, projectID, roleID, studentID, capacity); err != nil {
			return request, false, err
		}
	}
	_, err = tx.Exec(ctx, `
		UPDATE join_requests
		SET status = $2::request_status, cancel_reason = NULLIF($3, '')::request_cancel_reason, resolved_at = now(), updated_at = now()
		WHERE id = $1`, id, d.NewStatus, d.CancelReason)
	if err != nil {
		return request, false, err
	}
	actorName, err := displayName(ctx, tx, user.ID)
	if err != nil {
		return request, false, err
	}
	recipient := project.LeadUserID
	if d.ByLead {
		recipient = studentID
	}
	data := notificationData{"actorName": actorName, "roleName": roleName}
	if err := notify(ctx, tx, recipient, d.Notification, projectID, id, data); err != nil {
		return request, false, err
	}
	request, err = loadRequestFor(ctx, tx, id, user.ID)
	return request, false, err
}

// fillPlace adds the member and, if that took the last place, closes the role's other
// pending requests. The caller holds the role lock, so the count can't go stale here.
func fillPlace(ctx context.Context, tx pgx.Tx, requestID, projectID, roleID, studentID string, capacity int) error {
	taken, err := countRoleMembers(ctx, tx, roleID)
	if err != nil {
		return err
	}
	if taken >= capacity {
		return errRoleFull
	}
	member, err := isCurrentMember(ctx, tx, projectID, studentID)
	if err != nil {
		return err
	}
	if member {
		return conflict("already_member", "This student is already on the team.")
	}
	_, err = tx.Exec(ctx, `INSERT INTO project_members (project_id, role_id, user_id) VALUES ($1, $2, $3)`, projectID, roleID, studentID)
	if err != nil {
		return err
	}
	if taken+1 < capacity {
		return nil
	}
	return cancelPendingRequests(ctx, tx, requestScope{RoleID: roleID, ExceptRequestID: requestID}, "role_full")
}
