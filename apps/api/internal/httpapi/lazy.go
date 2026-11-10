package httpapi

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// bangkokTodaySQL is today's date where the students are, which is what end dates mean.
const bangkokTodaySQL = `(now() AT TIME ZONE 'Asia/Bangkok')::date`

// runLazyRules applies rules that depend on time passing, because free hosting
// sleeps and there are no timers: expiring due requests and writing end-date
// reminders. It runs on notification and request reads, and before a request is
// created or decided. It sweeps every due row, not just the viewer's.
func (s *Server) runLazyRules(ctx context.Context) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if err := expireDueRequests(ctx, tx, requestScope{}); err != nil {
			return err
		}
		return sendEndDateReminders(ctx, tx)
	})
}

// sendEndDateReminders prompts the lead once. Nothing else changes: the team stays in progress.
func sendEndDateReminders(ctx context.Context, tx pgx.Tx) error {
	rows, err := tx.Query(ctx, `
		WITH due AS (
			SELECT id FROM projects
			WHERE status = 'in_progress' AND end_reminder_sent_at IS NULL AND expected_end_date <= `+bangkokTodaySQL+`
			ORDER BY id FOR NO KEY UPDATE SKIP LOCKED
		)
		UPDATE projects pr SET end_reminder_sent_at = now(), updated_at = now()
		FROM due WHERE pr.id = due.id
		RETURNING pr.lead_user_id, pr.id, ''`)
	if err != nil {
		return err
	}
	due, err := collectRecipients(rows)
	if err != nil {
		return err
	}
	for _, reminder := range due {
		if err := notify(ctx, tx, reminder.UserID, notifEndDateReached, reminder.ProjectID, "", nil); err != nil {
			return err
		}
	}
	return nil
}
