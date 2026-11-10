package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

const maxRequestMessage = 1000

func (s *Server) registerRequests() {
	s.handle("POST /projects/{id}/applications", s.requireUser(s.apply))
	s.handle("POST /projects/{id}/invitations", s.requireUser(s.invite))
	s.handle("GET /projects/{id}/requests", s.requireUser(s.listProjectRequests))
	s.handle("GET /me/requests", s.requireUser(s.listMyRequests))
	s.handle("GET /requests/{id}", s.requireUser(s.getRequest))
	s.handle("POST /requests/{id}/approve", s.requireUser(s.decide(approveDecision)))
	s.handle("POST /requests/{id}/reject", s.requireUser(s.decide(rejectDecision)))
	s.handle("POST /requests/{id}/withdraw", s.requireUser(s.decide(withdrawDecision)))
	s.handle("POST /requests/{id}/accept", s.requireUser(s.decide(acceptDecision)))
	s.handle("POST /requests/{id}/decline", s.requireUser(s.decide(declineDecision)))
	s.handle("POST /requests/{id}/cancel", s.requireUser(s.decide(cancelDecision)))
}

var (
	errRequestNotFound = notFound("request_not_found", "We couldn't find that request.")
	errStudentNotFound = notFound("student_not_found", "We couldn't find that student.")
)

type requestProject struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Category idName `json:"category"`
}

type requestStudent struct {
	ID          string  `json:"id"`
	DisplayName string  `json:"displayName"`
	Faculty     *string `json:"faculty"`
	Year        *int    `json:"year"`
	Rating      Rating  `json:"rating"`
}

type requestInviter struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

type requestViewer struct {
	IsStudent   bool `json:"isStudent"`
	IsLead      bool `json:"isLead"`
	CanApprove  bool `json:"canApprove"`
	CanReject   bool `json:"canReject"`
	CanWithdraw bool `json:"canWithdraw"`
	CanAccept   bool `json:"canAccept"`
	CanDecline  bool `json:"canDecline"`
	CanCancel   bool `json:"canCancel"`
}

type Request struct {
	ID           string          `json:"id"`
	Type         string          `json:"type"`
	Status       string          `json:"status"`
	CancelReason *string         `json:"cancelReason"`
	Message      *string         `json:"message"`
	Project      requestProject  `json:"project"`
	Role         idName          `json:"role"`
	Student      requestStudent  `json:"student"`
	InvitedBy    *requestInviter `json:"invitedBy"`
	CreatedAt    time.Time       `json:"createdAt"`
	ExpiresAt    time.Time       `json:"expiresAt"`
	ResolvedAt   *time.Time      `json:"resolvedAt"`
	Viewer       requestViewer   `json:"viewer"`
}

// effectiveRequestStatusSQL reads a past-due request as expired even if the sweep
// hasn't reached it yet, so no reader ever treats it as pending. It expects join_requests as jr.
const effectiveRequestStatusSQL = `CASE WHEN jr.status = 'pending' AND jr.expires_at <= now() THEN 'expired' ELSE jr.status::text END`

var requestSelectSQL = `
	SELECT jr.id, jr.type::text, ` + effectiveRequestStatusSQL + `, jr.cancel_reason::text, jr.message,
		pr.id, pr.name, pr.status::text, c.id, c.name, role.id, role.name,
		u.id, ` + displayNameSQL + `, CASE WHEN u.closed_at IS NULL THEN f.name END, CASE WHEN u.closed_at IS NULL THEN p.year END,
		rating.average, rating.count, inviter.id, inviter.name,
		jr.created_at, jr.expires_at,
		CASE WHEN jr.status = 'pending' AND jr.expires_at <= now() THEN jr.expires_at ELSE jr.resolved_at END,
		pr.lead_user_id
	FROM join_requests jr
	JOIN projects pr ON pr.id = jr.project_id
	JOIN project_categories c ON c.id = pr.category_id
	JOIN project_roles role ON role.id = jr.role_id
	JOIN users u ON u.id = jr.student_id
	LEFT JOIN profiles p ON p.user_id = u.id
	LEFT JOIN faculties f ON f.id = p.faculty_id
	` + ratingJoinSQL("u.id") + `
	LEFT JOIN LATERAL (
		SELECT u.id, ` + displayNameSQL + ` AS name
		FROM users u LEFT JOIN profiles p ON p.user_id = u.id
		WHERE u.id = jr.invited_by_id
	) inviter ON true `

// loadRequests reads requests as viewerID sees them, newest first. The clause is
// everything up to the ORDER BY. Checking the viewer may see them is up to the caller.
func loadRequests(ctx context.Context, q querier, viewerID, clause string, args ...any) ([]Request, error) {
	rows, err := q.Query(ctx, requestSelectSQL+clause+` ORDER BY jr.created_at DESC, jr.id`, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Request, error) {
		var request Request
		var inviterID, inviterName *string
		var leadID string
		err := row.Scan(&request.ID, &request.Type, &request.Status, &request.CancelReason, &request.Message,
			&request.Project.ID, &request.Project.Name, &request.Project.Status, &request.Project.Category.ID, &request.Project.Category.Name,
			&request.Role.ID, &request.Role.Name,
			&request.Student.ID, &request.Student.DisplayName, &request.Student.Faculty, &request.Student.Year,
			&request.Student.Rating.Average, &request.Student.Rating.Count, &inviterID, &inviterName,
			&request.CreatedAt, &request.ExpiresAt, &request.ResolvedAt, &leadID)
		if inviterID != nil && inviterName != nil {
			request.InvitedBy = &requestInviter{*inviterID, *inviterName}
		}
		request.Viewer = viewerOfRequest(request, leadID, viewerID)
		return request, err
	})
}

func viewerOfRequest(request Request, leadID, viewerID string) requestViewer {
	viewer := requestViewer{IsStudent: request.Student.ID == viewerID, IsLead: leadID == viewerID}
	if request.Status != "pending" {
		return viewer
	}
	if request.Type == "application" {
		viewer.CanApprove, viewer.CanReject, viewer.CanWithdraw = viewer.IsLead, viewer.IsLead, viewer.IsStudent
	} else {
		viewer.CanAccept, viewer.CanDecline, viewer.CanCancel = viewer.IsStudent, viewer.IsStudent, viewer.IsLead
	}
	return viewer
}

// loadRequestFor answers 404 unless the viewer is the student or the team's current lead.
func loadRequestFor(ctx context.Context, q querier, id, viewerID string) (Request, error) {
	requests, err := loadRequests(ctx, q, viewerID, `WHERE jr.id = $1`, id)
	if err != nil {
		return Request{}, err
	}
	if len(requests) == 0 || (!requests[0].Viewer.IsStudent && !requests[0].Viewer.IsLead) {
		return Request{}, errRequestNotFound
	}
	return requests[0], nil
}

func (s *Server) getRequest(w http.ResponseWriter, r *http.Request, user User) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, r, errRequestNotFound)
		return
	}
	ctx := r.Context()
	if err := s.runLazyRules(ctx); err != nil {
		writeErr(w, r, err)
		return
	}
	request, err := loadRequestFor(ctx, s.db, id, user.ID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, request)
}

func (s *Server) listMyRequests(w http.ResponseWriter, r *http.Request, user User) {
	ctx := r.Context()
	if err := s.runLazyRules(ctx); err != nil {
		writeErr(w, r, err)
		return
	}
	// Invitations follow the team's current lead, the only person who can still act on
	// them, so after a transfer they move to the new lead's list along with the team.
	sent, err := loadRequests(ctx, s.db, user.ID, `
		WHERE (jr.type = 'application' AND jr.student_id = $1) OR (jr.type = 'invitation' AND pr.lead_user_id = $1)`, user.ID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	received, err := loadRequests(ctx, s.db, user.ID, `
		WHERE (jr.type = 'invitation' AND jr.student_id = $1) OR (jr.type = 'application' AND pr.lead_user_id = $1)`, user.ID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]Request{"sent": sent, "received": received})
}

func (s *Server) listProjectRequests(w http.ResponseWriter, r *http.Request, user User) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, r, errProjectNotFound)
		return
	}
	ctx := r.Context()
	if err := s.runLazyRules(ctx); err != nil {
		writeErr(w, r, err)
		return
	}
	var leadID string
	err := s.db.QueryRow(ctx, `SELECT lead_user_id FROM projects WHERE id = $1 AND deleted_at IS NULL`, id).Scan(&leadID)
	if err == pgx.ErrNoRows {
		err = errProjectNotFound
	} else if err == nil && leadID != user.ID {
		err = errNotLead
	}
	if err != nil {
		writeErr(w, r, err)
		return
	}
	requests, err := loadRequests(ctx, s.db, user.ID, `WHERE jr.project_id = $1`, id)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]Request{"items": requests})
}

func requireRoleID(id *string) error {
	if id == nil || !validUUID(*id) {
		return invalidFields(map[string]string{"roleId": "Choose a role."})
	}
	return nil
}

func (s *Server) apply(w http.ResponseWriter, r *http.Request, user User) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, r, errProjectNotFound)
		return
	}
	var body struct {
		RoleID  *string `json:"roleId"`
		Message *string `json:"message"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if err := requireRoleID(body.RoleID); err != nil {
		writeErr(w, r, err)
		return
	}
	message := cleanText(body.Message)
	fields := map[string]string{}
	checkLength(fields, "message", message, maxRequestMessage)
	if len(fields) > 0 {
		writeErr(w, r, invalidFields(fields))
		return
	}
	s.finishCreate(w, r, user, newRequest{Type: "application", ProjectID: id, RoleID: *body.RoleID, StudentID: user.ID, Message: message})
}

func (s *Server) invite(w http.ResponseWriter, r *http.Request, user User) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, r, errProjectNotFound)
		return
	}
	var body struct {
		RoleID    *string `json:"roleId"`
		StudentID *string `json:"studentId"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if err := requireRoleID(body.RoleID); err != nil {
		writeErr(w, r, err)
		return
	}
	if body.StudentID == nil || !validUUID(*body.StudentID) {
		writeErr(w, r, invalidFields(map[string]string{"studentId": "Choose a student."}))
		return
	}
	s.finishCreate(w, r, user, newRequest{Type: "invitation", ProjectID: id, RoleID: *body.RoleID, StudentID: *body.StudentID, InvitedBy: &user.ID})
}

type newRequest struct {
	Type      string
	ProjectID string
	RoleID    string
	StudentID string
	InvitedBy *string
	Message   *string
}

func (s *Server) finishCreate(w http.ResponseWriter, r *http.Request, user User, in newRequest) {
	ctx := r.Context()
	if err := s.runLazyRules(ctx); err != nil {
		writeErr(w, r, err)
		return
	}
	var request Request
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		id, err := createRequestTx(ctx, tx, user, in)
		if err != nil {
			return err
		}
		request, err = loadRequestFor(ctx, tx, id, user.ID)
		return err
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, request)
}

// createRequestTx checks every rule for a new application or invitation, writes it
// and notifies the other side. The role lock is what makes the full check hold.
func createRequestTx(ctx context.Context, tx pgx.Tx, user User, in newRequest) (string, error) {
	invitation := in.Type == "invitation"
	if invitation {
		var leadID string
		if err := tx.QueryRow(ctx, `SELECT lead_user_id FROM projects WHERE id=$1 AND deleted_at IS NULL`, in.ProjectID).Scan(&leadID); err != nil {
			return "", noRows(err, errProjectNotFound)
		}
		if leadID != user.ID {
			return "", errNotLead
		}
	}
	if err := lockOpenAccounts(ctx, tx, user.ID, in.StudentID); err != nil {
		return "", err
	}
	project, err := lockProject(ctx, tx, in.ProjectID, lockForRead)
	if err != nil {
		return "", err
	}
	if invitation && project.LeadUserID != user.ID {
		return "", errNotLead
	}
	complete, err := isProfileComplete(ctx, tx, user.ID)
	if err != nil {
		return "", err
	}
	if !complete {
		return "", errProfileNeeded
	}
	if project.ended() {
		return "", errTeamEnded
	}
	if project.Recruitment != "open" {
		return "", conflict("recruitment_closed", "This team isn't recruiting right now.")
	}
	role, err := lockRole(ctx, tx, in.ProjectID, in.RoleID)
	if err != nil {
		return "", err
	}
	if invitation {
		if err := requireInvitableStudent(ctx, tx, in.StudentID); err != nil {
			return "", err
		}
	}
	member, err := isCurrentMember(ctx, tx, in.ProjectID, in.StudentID)
	if err != nil {
		return "", err
	}
	if member {
		if invitation {
			return "", conflict("already_member", "This student is already on your team.")
		}
		return "", conflict("already_member", "You're already on this team.")
	}
	// One that just passed its deadline would otherwise block this one in the unique index.
	if err := expireDueRequests(ctx, tx, requestScope{ProjectID: in.ProjectID, StudentID: in.StudentID}); err != nil {
		return "", err
	}
	if err := checkRetryRule(ctx, tx, in); err != nil {
		return "", err
	}
	if role.full() {
		return "", conflict("role_full", "This role is full. Choose another role.")
	}
	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO join_requests (type, project_id, role_id, student_id, invited_by_id, message)
		VALUES ($1::request_type, $2, $3, $4, $5, $6) RETURNING id`,
		in.Type, in.ProjectID, in.RoleID, in.StudentID, in.InvitedBy, in.Message).Scan(&id)
	if isUniqueViolation(err, "join_requests_one_pending") {
		return "", errPendingRequest(invitation)
	}
	if err != nil {
		return "", err
	}
	actorName, err := displayName(ctx, tx, user.ID)
	if err != nil {
		return "", err
	}
	data := notificationData{"actorName": actorName, "roleName": role.Name}
	if invitation {
		return id, notify(ctx, tx, in.StudentID, notifInvitationReceived, in.ProjectID, id, data)
	}
	return id, notify(ctx, tx, project.LeadUserID, notifApplicationReceived, in.ProjectID, id, data)
}

// requireInvitableStudent treats a closed account or one with no name as not there, like its profile page.
func requireInvitableStudent(ctx context.Context, q querier, studentID string) error {
	var exists bool
	err := q.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM users u JOIN profiles p ON p.user_id = u.id
			WHERE u.id = $1 AND u.closed_at IS NULL AND p.display_name IS NOT NULL)`, studentID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return errStudentNotFound
	}
	return nil
}

func errPendingRequest(invitation bool) error {
	if invitation {
		return conflict("pending_request", "This student already has a pending request with your team.")
	}
	return conflict("pending_request", "You already have a pending request with this team.")
}

// checkRetryRule reads the latest request for this student and team. A rejected
// application means only the lead can make the next move, and a declined invitation
// means only the student can. Any other outcome allows a new request.
func checkRetryRule(ctx context.Context, q querier, in newRequest) error {
	invitation := in.Type == "invitation"
	latestType, latestStatus, err := latestRequest(ctx, q, in.ProjectID, in.StudentID)
	if err != nil {
		return err
	}
	switch {
	case latestStatus == "pending":
		return errPendingRequest(invitation)
	case !invitation && latestType == "application" && latestStatus == "rejected":
		return conflict("previously_rejected", "The lead turned down your application, so they need to invite you to try again.")
	case invitation && latestType == "invitation" && latestStatus == "declined":
		return conflict("previously_declined", "This student declined your invitation, so they need to apply to try again.")
	}
	return nil
}
