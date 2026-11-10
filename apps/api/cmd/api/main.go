package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thinc-org/CU-Profile/apps/api/internal/config"
	"github.com/thinc-org/CU-Profile/apps/api/internal/database"
	"github.com/thinc-org/CU-Profile/apps/api/internal/httpapi"
)

func main() {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var pool *pgxpool.Pool
	if cfg.DatabaseURL == "" {
		log.Print("DATABASE_URL is not set: only the health route will work")
	} else {
		var err error
		if pool, err = database.Open(ctx, cfg.DatabaseURL); err != nil {
			log.Fatal(err)
		}
		defer pool.Close()
		if err := database.Migrate(ctx, pool); err != nil {
			log.Fatal(err)
		}
	}

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpapi.New(httpapi.Options{DB: pool, DemoSignIn: cfg.DemoSignIn, CookieSecure: cfg.CookieSecure}),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second,
	}
	done := make(chan struct{})
	go func() {
		<-ctx.Done()
		deadline, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(deadline); err != nil {
			log.Printf("shutdown: %v", err)
			server.Close()
		}
		close(done)
	}()
	log.Printf("API listening on %s", server.Addr)
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
	<-done
}
