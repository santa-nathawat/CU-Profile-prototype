package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
)

type Transfer struct {
	ID         string `json:"id"`
	ProjectID  string `json:"projectId"`
	FromUserID string `json:"fromUserId"`
	ToUserID   string `json:"toUserId"`
	Status     string `json:"status"`
}

var errTransferNotFound = notFound("transfer_not_found", "We couldn't find that leadership transfer.")

func (s *Server) createTransfer(w http.ResponseWriter, r *http.Request, user User) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, r, errProjectNotFound)
		return
	}
	var in struct {
		ToUserID string `json:"toUserId"`
	}
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, r, err)
		return
	}
	if !validUUID(in.ToUserID) {
		writeErr(w, r, invalidFields(map[string]string{"toUserId": "Choose a current team member."}))
		return
	}
	in.ToUserID = strings.ToLower(in.ToUserID)
	ctx := r.Context()
	var transfer Transfer
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		var leadID string
		if err := tx.QueryRow(ctx, `SELECT lead_user_id FROM projects WHERE id=$1 AND deleted_at IS NULL`, id).Scan(&leadID); err != nil {
			return noRows(err, errProjectNotFound)
		}
		if leadID != user.ID {
			return errNotLead
		}
		if err := lockOpenAccounts(ctx, tx, user.ID, in.ToUserID); err != nil {
			if errors.Is(err, errStudentNotFound) {
				return errMemberNotFound
			}
			return err
		}
		project, err := lockLedProject(ctx, tx, id, user.ID, lockForWrite)
		if err != nil {
			return err
		}
		if project.ended() {
			return errTeamEnded
		}
		if in.ToUserID == user.ID {
			return forbidden("not_allowed", "Choose another member to become team lead.")
		}
		var member bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM project_members m JOIN users u ON u.id=m.user_id AND u.closed_at IS NULL WHERE m.project_id=$1 AND m.user_id=$2 AND m.left_at IS NULL)`, id, in.ToUserID).Scan(&member)
		if err != nil {
			return err
		}
		if !member {
			return errMemberNotFound
		}
		err = tx.QueryRow(ctx, `INSERT INTO leadership_transfers(project_id,from_user_id,to_user_id) VALUES($1,$2,$3) RETURNING id,project_id,from_user_id,to_user_id,status::text`, id, user.ID, in.ToUserID).Scan(&transfer.ID, &transfer.ProjectID, &transfer.FromUserID, &transfer.ToUserID, &transfer.Status)
		if isUniqueViolation(err, "leadership_transfers_one_pending") {
			return conflict("transfer_pending", "This team already has a pending leadership transfer.")
		}
		if err != nil {
			return err
		}
		name, err := displayName(ctx, tx, user.ID)
		if err != nil {
			return err
		}
		return notify(ctx, tx, in.ToUserID, notifTransferRequested, id, "", notificationData{"actorName": name})
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, transfer)
}

func (s *Server) decideTransfer(status string) userHandler {
	return func(w http.ResponseWriter, r *http.Request, user User) {
		id, ok := pathID(r, "id")
		if !ok {
			writeErr(w, r, errTransferNotFound)
			return
		}
		if err := readEmptyJSON(w, r); err != nil {
			writeErr(w, r, err)
			return
		}
		ctx := r.Context()
		var transfer Transfer
		err := s.inTx(ctx, func(tx pgx.Tx) error {
			// The project ID is immutable. Lock the project before the transfer so all team writes share an order.
			var leadID string
			err := tx.QueryRow(ctx, `SELECT t.project_id,t.from_user_id,t.to_user_id,pr.lead_user_id FROM leadership_transfers t JOIN projects pr ON pr.id=t.project_id WHERE t.id=$1`, id).Scan(&transfer.ProjectID, &transfer.FromUserID, &transfer.ToUserID, &leadID)
			if err != nil {
				return noRows(err, errTransferNotFound)
			}
			if user.ID != transfer.FromUserID && user.ID != transfer.ToUserID && user.ID != leadID {
				return errTransferNotFound
			}
			if _, err := lockAccounts(ctx, tx, transfer.FromUserID, transfer.ToUserID); err != nil {
				return err
			}
			project, err := lockProject(ctx, tx, transfer.ProjectID, lockForWrite)
			if err != nil {
				return err
			}
			err = tx.QueryRow(ctx, `SELECT id,from_user_id,to_user_id,status::text FROM leadership_transfers WHERE id=$1 FOR UPDATE`, id).Scan(&transfer.ID, &transfer.FromUserID, &transfer.ToUserID, &transfer.Status)
			if err != nil {
				return err
			}
			if user.ID != transfer.FromUserID && user.ID != transfer.ToUserID && user.ID != project.LeadUserID {
				return errTransferNotFound
			}
			if (status == "cancelled" && user.ID != project.LeadUserID) || (status != "cancelled" && user.ID != transfer.ToUserID) {
				return forbidden("not_allowed", "You can't do this for this leadership transfer.")
			}
			if project.ended() {
				return errTeamEnded
			}
			if transfer.Status != "pending" {
				return conflict("transfer_not_pending", "This leadership transfer has already been answered.")
			}
			if project.LeadUserID != transfer.FromUserID {
				return conflict("transfer_not_pending", "This leadership transfer is no longer available.")
			}
			member, err := isCurrentMember(ctx, tx, transfer.ProjectID, transfer.ToUserID)
			if err != nil {
				return err
			}
			if !member {
				return errMemberNotFound
			}
			var open bool
			if err := tx.QueryRow(ctx, `SELECT closed_at IS NULL FROM users WHERE id=$1`, transfer.ToUserID).Scan(&open); err != nil {
				return err
			}
			if !open {
				return errMemberNotFound
			}
			if status == "accepted" {
				if _, err := tx.Exec(ctx, `UPDATE projects SET lead_user_id=$2,updated_at=now() WHERE id=$1`, transfer.ProjectID, transfer.ToUserID); err != nil {
					return err
				}
			}
			if _, err := tx.Exec(ctx, `UPDATE leadership_transfers SET status=$2::transfer_status,resolved_at=now(),updated_at=now() WHERE id=$1`, id, status); err != nil {
				return err
			}
			transfer.Status = status
			actorName, err := displayName(ctx, tx, user.ID)
			if err != nil {
				return err
			}
			kind, recipients := notifTransferDeclined, []string{transfer.FromUserID}
			if status == "accepted" {
				kind, recipients = notifTransferAccepted, []string{transfer.FromUserID, transfer.ToUserID}
			}
			if status == "cancelled" {
				kind, recipients = notifTransferCancelled, []string{transfer.ToUserID}
			}
			for _, recipientID := range recipients {
				if err := notify(ctx, tx, recipientID, kind, transfer.ProjectID, "", notificationData{"actorName": actorName}); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			writeErr(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, transfer)
	}
}
