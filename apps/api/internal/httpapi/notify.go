package httpapi

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Notification types from the contract.
const (
	notifApplicationReceived  = "application_received"
	notifApplicationApproved  = "application_approved"
	notifApplicationRejected  = "application_rejected"
	notifApplicationWithdrawn = "application_withdrawn"
	notifInvitationReceived   = "invitation_received"
	notifInvitationAccepted   = "invitation_accepted"
	notifInvitationDeclined   = "invitation_declined"
	notifInvitationCancelled  = "invitation_cancelled"
	notifRequestExpired       = "request_expired"
	notifRequestCancelled     = "request_cancelled"
	notifMemberLeft           = "member_left"
	notifMemberRemoved        = "member_removed"
	notifTransferRequested    = "transfer_requested"
	notifTransferAccepted     = "transfer_accepted"
	notifTransferDeclined     = "transfer_declined"
	notifTransferCancelled    = "transfer_cancelled"
	notifTeamDeleted          = "team_deleted"
	notifProjectStarted       = "project_started"
	notifProjectCompleted     = "project_completed"
	notifProjectCancelled     = "project_cancelled"
	notifEndDateReached       = "end_date_reached"
	notifRateTeammates        = "rate_teammates"
)

// notificationData holds the values a message needs. The renderer reads the keys
// actorName, roleName and, for request_cancelled, reason (the cancel reason).
type notificationData map[string]any

// notify records a notification in the same transaction as the change that caused
// it. Pass "" for a projectID or requestID that doesn't apply.
func notify(ctx context.Context, tx pgx.Tx, userID, notificationType, projectID, requestID string, data notificationData) error {
	if data == nil {
		data = notificationData{}
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO notifications (user_id, type, project_id, join_request_id, data)
		VALUES ($1, $2, NULLIF($3, '')::uuid, NULLIF($4, '')::uuid, $5)`,
		userID, notificationType, projectID, requestID, data)
	return err
}

var notificationMessages = map[string]string{
	notifApplicationReceived:  "{actor} applied for {role} on {project}.",
	notifApplicationApproved:  "Your application to {project} was approved.",
	notifApplicationRejected:  "Your application to {project} was not accepted.",
	notifApplicationWithdrawn: "{actor} withdrew their application to {project}.",
	notifInvitationReceived:   "{actor} invited you to join {project} as {role}.",
	notifInvitationAccepted:   "{actor} accepted your invitation to {project}.",
	notifInvitationDeclined:   "{actor} declined your invitation to {project}.",
	notifInvitationCancelled:  "Your invitation to {project} was cancelled.",
	notifRequestExpired:       "A request for {project} expired without an answer.",
	notifMemberLeft:           "{actor} left {project}.",
	notifMemberRemoved:        "You were removed from {project}.",
	notifTransferRequested:    "{actor} asked you to take over as lead of {project}.",
	notifTransferAccepted:     "{actor} is now the lead of {project}.",
	notifTransferDeclined:     "{actor} declined to take over as lead of {project}.",
	notifTransferCancelled:    "The lead transfer for {project} was cancelled.",
	notifTeamDeleted:          "{project} was deleted by its lead.",
	notifProjectStarted:       "{project} has started.",
	notifProjectCompleted:     "{project} is complete.",
	notifProjectCancelled:     "{project} was cancelled.",
	notifEndDateReached:       "{project} has reached its expected end date. Complete it or extend it.",
	notifRateTeammates:        "{project} is complete. Rate your teammates within 14 days.",
}

var requestCancelledMessages = map[string]string{
	"lead_cancelled":     "The lead cancelled a request for {project}.",
	"role_removed":       "The {role} role on {project} was removed, so your request was cancelled.",
	"role_full":          "The {role} role on {project} is now full, so your request was closed.",
	"recruitment_closed": "{project} closed recruitment, so your request was cancelled.",
	"project_completed":  "{project} is complete, so your request was cancelled.",
	"project_cancelled":  "{project} was cancelled, so your request was cancelled.",
	"team_deleted":       "{project} was deleted, so your request was cancelled.",
	"account_closed":     "A request for {project} was cancelled because the student closed their account.",
}

type storedNotification struct {
	Type           string
	ProjectID      *string
	RequestID      *string
	ProjectName    *string
	ProjectDeleted bool
	Data           map[string]any
}

// renderNotification turns a stored notification into what the web shows. It runs
// at read time, so a renamed project shows its current name.
func renderNotification(n storedNotification) (message, link string) {
	template, known := notificationMessages[n.Type]
	if n.Type == notifRequestCancelled {
		reason, _ := n.Data["reason"].(string)
		template, known = requestCancelledMessages[reason]
		if !known {
			template, known = "A request for {project} was cancelled.", true
		}
	}
	if !known {
		template = "You have a new notification."
	}
	project := "a team"
	if n.ProjectName != nil {
		project = *n.ProjectName
	}
	message = strings.NewReplacer(
		"{actor}", dataString(n.Data, "actorName", "Someone"),
		"{role}", dataString(n.Data, "roleName", "a"),
		"{project}", project,
	).Replace(template)
	return message, notificationLink(n)
}

// notificationLink points at the page to open. A deleted team has no page, so it stays on the inbox.
func notificationLink(n storedNotification) string {
	switch {
	case n.RequestID != nil:
		return "/requests/" + *n.RequestID
	case n.ProjectID == nil || n.Type == notifTeamDeleted || n.ProjectDeleted:
		return "/notifications"
	case n.Type == notifRateTeammates:
		return "/projects/" + *n.ProjectID + "/rate"
	default:
		return "/projects/" + *n.ProjectID
	}
}

func dataString(data map[string]any, key, fallback string) string {
	if value, ok := data[key].(string); ok && value != "" {
		return value
	}
	return fallback
}
