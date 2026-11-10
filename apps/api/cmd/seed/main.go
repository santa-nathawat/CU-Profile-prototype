// Command seed fills the database in DATABASE_URL with demo data.
// With --reset it first empties the app tables, on a local database only.
package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/thinc-org/CU-Profile/apps/api/internal/database"
	"github.com/thinc-org/CU-Profile/apps/api/internal/seed"
)

func main() {
	reset := flag.Bool("reset", false, "empty the app tables first (localhost only)")
	ifEmpty := flag.Bool("if-empty", false, "seed only when there are no users (localhost only)")
	flag.Parse()
	if *reset && *ifEmpty {
		log.Fatal("use either --reset or --if-empty")
	}

	url := os.Getenv("DATABASE_URL")
	if url == "" {
		log.Fatal("DATABASE_URL is not set")
	}
	if *reset {
		if err := seed.CheckResetAllowed(url); err != nil {
			log.Fatal(err)
		}
	}
	if *ifEmpty {
		if err := seed.CheckAutomaticSeedAllowed(url); err != nil {
			log.Fatal(err)
		}
	}
	ctx := context.Background()
	pool, err := database.Open(ctx, url)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool); err != nil {
		log.Fatal(err)
	}
	if *reset {
		if err := seed.Reset(ctx, pool); err != nil {
			log.Fatal(err)
		}
		log.Print("reset app tables")
	}
	if *ifEmpty {
		seeded, err := seed.RunIfEmpty(ctx, pool)
		if err != nil {
			log.Fatal(err)
		}
		if seeded {
			log.Print("seeded new demo database")
		} else {
			log.Print("kept existing database data")
		}
		return
	}
	if err := seed.Run(ctx, pool); err != nil {
		log.Fatal(err)
	}
	log.Print("seed complete")
}
