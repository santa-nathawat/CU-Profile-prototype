// Package seed fills a database with demo accounts and one completed project.
package seed

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// steps run in order in one transaction.
var steps = []func(ctx context.Context, tx pgx.Tx) error{
	seedStudents,
	seedProjects,
}

func Run(ctx context.Context, pool *pgxpool.Pool) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		return runSteps(ctx, tx)
	})
}

// RunIfEmpty initializes a new demo without overwriting an existing student's data.
func RunIfEmpty(ctx context.Context, pool *pgxpool.Pool) (bool, error) {
	seeded := false
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		// Serialize the empty check with inserts and other startup attempts.
		if _, err := tx.Exec(ctx, `LOCK TABLE users IN SHARE ROW EXCLUSIVE MODE`); err != nil {
			return err
		}
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users)`).Scan(&exists); err != nil {
			return err
		}
		if exists {
			return nil
		}
		if err := runSteps(ctx, tx); err != nil {
			return err
		}
		seeded = true
		return nil
	})
	return seeded && err == nil, err
}

func runSteps(ctx context.Context, tx pgx.Tx) error {
	for _, step := range steps {
		if err := step(ctx, tx); err != nil {
			return err
		}
	}
	return nil
}

// CheckResetAllowed refuses a reset unless every host in the URL is this machine,
// so a production DATABASE_URL can never be wiped by accident.
func CheckResetAllowed(databaseURL string) error {
	return checkLocalAllowed(databaseURL, "reset")
}

// CheckAutomaticSeedAllowed keeps local startup from migrating or seeding a remote database.
func CheckAutomaticSeedAllowed(databaseURL string) error {
	return checkLocalAllowed(databaseURL, "seed automatically")
}

func checkLocalAllowed(databaseURL, action string) error {
	cfg, err := pgconn.ParseConfig(databaseURL)
	if err != nil {
		return fmt.Errorf("invalid database URL")
	}
	hosts := []string{cfg.Host}
	for _, fallback := range cfg.Fallbacks {
		hosts = append(hosts, fallback.Host)
	}
	for _, host := range hosts {
		if host != "localhost" && host != "127.0.0.1" && host != "::1" {
			return fmt.Errorf("refusing to %s: host %q is not a loopback address", action, host)
		}
	}
	return nil
}

// starterTables stay through a reset. The list covers tables that aren't app data;
// any table added later is wiped by default.
var starterTables = []string{"goose_db_version", "faculties", "project_categories", "skills"}

// Reset empties the app tables, keeping the starter lists. Custom skills count as app data.
func Reset(ctx context.Context, pool *pgxpool.Pool) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT table_name::text FROM information_schema.tables
			WHERE table_schema = current_schema() AND table_type = 'BASE TABLE' AND table_name <> ALL($1)`, starterTables)
		if err != nil {
			return err
		}
		tables, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			return err
		}
		list := ""
		for i, table := range tables {
			if i > 0 {
				list += ", "
			}
			list += pgx.Identifier{table}.Sanitize()
		}
		if list != "" {
			if _, err := tx.Exec(ctx, "TRUNCATE "+list); err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `DELETE FROM skills WHERE NOT is_predefined`)
		return err
	})
}

// userIDByEmail lets later seed steps find a demo student.
func userIDByEmail(ctx context.Context, tx pgx.Tx, email string) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `SELECT id FROM users WHERE email = $1`, email).Scan(&id)
	return id, err
}
