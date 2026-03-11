package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"lago-usage-billing-alpha/internal/api"
	"lago-usage-billing-alpha/internal/replay"
	"lago-usage-billing-alpha/internal/store"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(getIntEnv("DB_MAX_OPEN_CONNS", 20))
	db.SetMaxIdleConns(getIntEnv("DB_MAX_IDLE_CONNS", 5))
	db.SetConnMaxLifetime(time.Duration(getIntEnv("DB_CONN_MAX_LIFETIME_MIN", 30)) * time.Minute)

	pingCtx, pingCancel := context.WithTimeout(context.Background(), time.Duration(getIntEnv("DB_PING_TIMEOUT_SEC", 5))*time.Second)
	defer pingCancel()
	if err := db.PingContext(pingCtx); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	queryTimeout := time.Duration(getIntEnv("DB_QUERY_TIMEOUT_MS", 5000)) * time.Millisecond
	migrationTimeout := time.Duration(getIntEnv("DB_MIGRATION_TIMEOUT_SEC", 60)) * time.Second
	repo := store.NewPostgresStore(
		db,
		store.WithQueryTimeout(queryTimeout),
		store.WithMigrationTimeout(migrationTimeout),
	)
	if err := repo.Migrate(); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	pollInterval := time.Duration(getIntEnv("REPLAY_WORKER_POLL_MS", 500)) * time.Millisecond
	errorBackoffMin := time.Duration(getIntEnv("REPLAY_WORKER_ERR_BACKOFF_MIN_MS", 250)) * time.Millisecond
	errorBackoffMax := time.Duration(getIntEnv("REPLAY_WORKER_ERR_BACKOFF_MAX_MS", 5000)) * time.Millisecond

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	worker := replay.NewWorker(
		repo,
		pollInterval,
		replay.WithErrorBackoff(errorBackoffMin, errorBackoffMax),
	)
	go worker.Run(ctx)

	httpServer := &http.Server{
		Addr:              ":" + port,
		Handler:           api.NewServer(repo, api.WithMetricsProvider(buildMetricsProvider(worker, db))).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	log.Printf("level=info component=server event=start addr=%s", httpServer.Addr)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server failed: %v", err)
	}
}

func buildMetricsProvider(worker *replay.Worker, db *sql.DB) func() map[string]any {
	return func() map[string]any {
		ws := worker.Stats()
		ds := db.Stats()
		return map[string]any{
			"replay_worker": ws,
			"database": map[string]any{
				"max_open_connections": ds.MaxOpenConnections,
				"open_connections":     ds.OpenConnections,
				"in_use":               ds.InUse,
				"idle":                 ds.Idle,
				"wait_count":           ds.WaitCount,
				"wait_duration_ms":     ds.WaitDuration.Milliseconds(),
				"max_idle_closed":      ds.MaxIdleClosed,
				"max_lifetime_closed":  ds.MaxLifetimeClosed,
			},
		}
	}
}

func getIntEnv(key string, defaultVal int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultVal
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return defaultVal
	}
	return parsed
}
