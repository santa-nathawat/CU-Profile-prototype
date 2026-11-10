package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Server) registerTeamLifecycle() {
	s.handle("PUT /projects/{id}/contact", s.requireUser(s.setContact))
	s.handle("POST /projects/{id}/start", s.requireUser(s.changeTimeline(false)))
	s.handle("POST /projects/{id}/extend", s.requireUser(s.changeTimeline(true)))
	s.handle("POST /projects/{id}/complete", s.requireUser(s.endProject(false)))
	s.handle("POST /projects/{id}/cancel", s.requireUser(s.endProject(true)))
	s.handle("DELETE /projects/{id}", s.requireUser(s.deleteTeam))
	s.handle("POST /projects/{id}/leave", s.requireUser(s.leaveTeam))
	s.handle("DELETE /projects/{id}/members/{userId}", s.requireUser(s.removeMember))
	s.handle("POST /projects/{id}/transfers", s.requireUser(s.createTransfer))
	s.handle("POST /transfers/{id}/accept", s.requireUser(s.decideTransfer("accepted")))
	s.handle("POST /transfers/{id}/decline", s.requireUser(s.decideTransfer("declined")))
	s.handle("POST /transfers/{id}/cancel", s.requireUser(s.decideTransfer("cancelled")))
	s.handle("GET /projects/{id}/ratings", s.requireUser(s.getProjectRatings))
	s.handle("PUT /projects/{id}/ratings/{recipientId}", s.requireUser(s.putRating))
}

func validContactLink(raw *string, discord bool) bool {
	if raw == nil {
		return true
	}
	u, err := url.Parse(*raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Opaque != "" || u.Host == "" || (u.Port() != "" && u.Port() != "443") {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if discord {
		return (host == "discord.gg" && strings.Trim(u.Path, "/") != "") || (host == "discord.com" && strings.HasPrefix(u.Path, "/invite/") && strings.TrimPrefix(u.Path, "/invite/") != "")
	}
	return host == "line.me" || strings.HasSuffix(host, ".line.me")
}

func (s *Server) setContact(w http.ResponseWriter, r *http.Request, user User) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, r, errProjectNotFound)
		return
	}
	var in projectContact
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, r, err)
		return
	}
	in.LineURL, in.DiscordURL = cleanText(in.LineURL), cleanText(in.DiscordURL)
	if !validContactLink(in.LineURL, false) || !validContactLink(in.DiscordURL, true) {
		writeErr(w, r, badRequest("unsupported_link", "Use an HTTPS invitation link from LINE or Discord."))
		return
	}
	var detail ProjectDetail
	ctx := r.Context()
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := lockLedProject(ctx, tx, id, user.ID, lockForWrite); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE projects SET line_url=$2, discord_url=$3, updated_at=now() WHERE id=$1`, id, in.LineURL, in.DiscordURL); err != nil {
			return err
		}
		var err error
		detail, err = loadProjectDetail(ctx, tx, id, user.ID)
		return err
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) changeTimeline(extend bool) userHandler {
	return func(w http.ResponseWriter, r *http.Request, user User) {
		id, ok := pathID(r, "id")
		if !ok {
			writeErr(w, r, errProjectNotFound)
			return
		}
		var in struct {
			ExpectedEndDate string `json:"expectedEndDate"`
		}
		if err := readJSON(w, r, &in); err != nil {
			writeErr(w, r, err)
			return
		}
		if _, err := time.Parse("2006-01-02", in.ExpectedEndDate); err != nil {
			writeErr(w, r, invalidFields(map[string]string{"expectedEndDate": "Choose a valid end date."}))
			return
		}
		ctx := r.Context()
		var detail ProjectDetail
		err := s.inTx(ctx, func(tx pgx.Tx) error {
			project, err := lockLedProject(ctx, tx, id, user.ID, lockForWrite)
			if err != nil {
				return err
			}
			if project.ended() {
				return errTeamEnded
			}
			if extend && project.Status != "in_progress" {
				return conflict("project_not_in_progress", "Start the project before extending it.")
			}
			if !extend && project.Status != "not_started" {
				return conflict("project_already_started", "This project has already started.")
			}
			if !extend && project.Recruitment != "closed" {
				return conflict("recruitment_open", "Close recruitment before starting the project.")
			}
			var valid bool
			if extend {
				err = tx.QueryRow(ctx, `SELECT $2::date > expected_end_date FROM projects WHERE id=$1`, id, in.ExpectedEndDate).Scan(&valid)
			} else {
				err = tx.QueryRow(ctx, `SELECT $1::date >= `+bangkokTodaySQL, in.ExpectedEndDate).Scan(&valid)
			}
			if err != nil {
				return err
			}
			if !valid {
				return invalidFields(map[string]string{"expectedEndDate": "Choose a later end date."})
			}
			if extend {
				_, err = tx.Exec(ctx, `UPDATE projects SET expected_end_date=$2::date, end_reminder_sent_at=NULL, updated_at=now() WHERE id=$1`, id, in.ExpectedEndDate)
			} else {
				_, err = tx.Exec(ctx, `UPDATE projects SET status='in_progress', started_at=now(), expected_end_date=$2::date, updated_at=now() WHERE id=$1`, id, in.ExpectedEndDate)
			}
			if err != nil {
				return err
			}
			if !extend {
				if err := notifyCurrentMembers(ctx, tx, id, notifProjectStarted, nil); err != nil {
					return err
				}
			}
			detail, err = loadProjectDetail(ctx, tx, id, user.ID)
			return err
		})
		if err != nil {
			writeErr(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, detail)
	}
}

func (s *Server) endProject(cancel bool) userHandler {
	return func(w http.ResponseWriter, r *http.Request, user User) {
		id, ok := pathID(r, "id")
		if !ok {
			writeErr(w, r, errProjectNotFound)
			return
		}
		if err := readEmptyJSON(w, r); err != nil {
			writeErr(w, r, err)
			return
		}
		ctx := r.Context()
		var detail ProjectDetail
		err := s.inTx(ctx, func(tx pgx.Tx) error {
			project, err := lockLedProject(ctx, tx, id, user.ID, lockForWrite)
			if err != nil {
				return err
			}
			if project.ended() {
				return errTeamEnded
			}
			if project.Status != "in_progress" {
				return conflict("project_not_in_progress", "Start the project before ending it.")
			}
			reason, kind := "project_completed", notifProjectCompleted
			if cancel {
				reason, kind = "project_cancelled", notifProjectCancelled
			}
			statement := `UPDATE projects SET status='completed', completed_at=now(), recruitment_status='closed', updated_at=now() WHERE id=$1`
			if cancel {
				statement = `UPDATE projects SET status='cancelled', cancelled_at=now(), recruitment_status='closed', updated_at=now() WHERE id=$1`
			}
			if _, err := tx.Exec(ctx, statement, id); err != nil {
				return err
			}
			if err := cancelPendingRequests(ctx, tx, requestScope{ProjectID: id}, reason); err != nil {
				return err
			}
			if err := cancelTeamTransfers(ctx, tx, id, ""); err != nil {
				return err
			}
			if err := notifyCurrentMembers(ctx, tx, id, kind, nil); err != nil {
				return err
			}
			if !cancel {
				if err := notifyCurrentMembers(ctx, tx, id, notifRateTeammates, nil); err != nil {
					return err
				}
			}
			detail, err = loadProjectDetail(ctx, tx, id, user.ID)
			return err
		})
		if err != nil {
			writeErr(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, detail)
	}
}

func notifyCurrentMembers(ctx context.Context, tx pgx.Tx, id, kind string, data notificationData) error {
	rows, err := tx.Query(ctx, `SELECT m.user_id FROM project_members m JOIN users u ON u.id=m.user_id AND u.closed_at IS NULL WHERE m.project_id=$1 AND m.left_at IS NULL ORDER BY m.user_id`, id)
	if err != nil {
		return err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	for _, userID := range ids {
		if err := notify(ctx, tx, userID, kind, id, "", data); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) deleteTeam(w http.ResponseWriter, r *http.Request, user User) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, r, errProjectNotFound)
		return
	}
	if err := readEmptyJSON(w, r); err != nil {
		writeErr(w, r, err)
		return
	}
	ctx := r.Context()
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		project, err := lockLedProject(ctx, tx, id, user.ID, lockForWrite)
		if err != nil {
			return err
		}
		if project.Status != "not_started" {
			return conflict("project_started", "Only a project that has not started can be deleted.")
		}
		if err := notifyCurrentMembers(ctx, tx, id, notifTeamDeleted, nil); err != nil {
			return err
		}
		if err := cancelPendingRequests(ctx, tx, requestScope{ProjectID: id}, "team_deleted"); err != nil {
			return err
		}
		if err := cancelTeamTransfers(ctx, tx, id, ""); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE projects SET deleted_at=now(), recruitment_status='closed', updated_at=now() WHERE id=$1`, id); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE project_members SET left_at=now(), left_reason='team_deleted', updated_at=now() WHERE project_id=$1 AND left_at IS NULL`, id)
		return err
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

var errMemberNotFound = notFound("member_not_found", "We couldn't find that current team member.")

func (s *Server) leaveTeam(w http.ResponseWriter, r *http.Request, user User) {
	s.endMembership(w, r, user, false)
}
func (s *Server) removeMember(w http.ResponseWriter, r *http.Request, user User) {
	s.endMembership(w, r, user, true)
}

func (s *Server) endMembership(w http.ResponseWriter, r *http.Request, user User, remove bool) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, r, errProjectNotFound)
		return
	}
	target := user.ID
	if remove {
		target, ok = pathID(r, "userId")
		if !ok {
			writeErr(w, r, errMemberNotFound)
			return
		}
	}
	if err := readEmptyJSON(w, r); err != nil {
		writeErr(w, r, err)
		return
	}
	ctx := r.Context()
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		project, err := lockProject(ctx, tx, id, lockForWrite)
		if err != nil {
			return err
		}
		if remove && project.LeadUserID != user.ID {
			return errNotLead
		}
		member, err := isCurrentMember(ctx, tx, id, target)
		if err != nil {
			return err
		}
		if !member {
			return errMemberNotFound
		}
		if project.ended() {
			return errTeamEnded
		}
		if target == project.LeadUserID {
			if remove {
				return forbidden("not_allowed", "You can't remove yourself as team lead.")
			}
			return conflict("lead_must_transfer", "Transfer leadership before leaving the team.")
		}
		// Project write lock excludes every join and capacity edit, so freeing this place cannot race those writes.
		reason := "left"
		if remove {
			reason = "removed"
		}
		if _, err := tx.Exec(ctx, `UPDATE project_members SET left_at=now(),left_reason=$3::member_left_reason,updated_at=now() WHERE project_id=$1 AND user_id=$2 AND left_at IS NULL`, id, target, reason); err != nil {
			return err
		}
		if err := cancelTeamTransfers(ctx, tx, id, target); err != nil {
			return err
		}
		name, err := displayName(ctx, tx, user.ID)
		if err != nil {
			return err
		}
		if remove {
			return notify(ctx, tx, target, notifMemberRemoved, id, "", notificationData{"actorName": name})
		}
		return notify(ctx, tx, project.LeadUserID, notifMemberLeft, id, "", notificationData{"actorName": name})
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// cancelTeamTransfers is called with the project write lock held. A nonempty
// recipient narrows this to a member who has just left or been removed.
func cancelTeamTransfers(ctx context.Context, tx pgx.Tx, projectID, recipientID string) error {
	rows, err := tx.Query(ctx, `UPDATE leadership_transfers SET status='cancelled',resolved_at=now(),updated_at=now() WHERE project_id=$1 AND status='pending' AND ($2::text='' OR to_user_id::text=$2) RETURNING from_user_id,to_user_id`, projectID, recipientID)
	if err != nil {
		return err
	}
	type pair struct{ From, To string }
	pairs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (pair, error) { var p pair; err := row.Scan(&p.From, &p.To); return p, err })
	if err != nil {
		return err
	}
	for _, p := range pairs {
		for _, id := range []string{p.From, p.To} {
			if err := notify(ctx, tx, id, notifTransferCancelled, projectID, "", nil); err != nil {
				return err
			}
		}
	}
	return nil
}

func noRows(err error, replacement error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return replacement
	}
	return err
}
