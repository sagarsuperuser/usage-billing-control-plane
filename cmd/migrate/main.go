package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"lago-usage-billing-alpha/migrations"
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

	migrationTimeout := time.Duration(getIntEnv("DB_MIGRATION_TIMEOUT_SEC", 60)) * time.Second
	runner := migrations.NewRunner(db, migrations.WithTimeout(migrationTimeout))

	before, err := countAppliedMigrations(db)
	if err != nil {
		log.Fatalf("failed to count applied migrations before run: %v", err)
	}

	started := time.Now().UTC()
	if err := runner.Run(context.Background()); err != nil {
		log.Fatalf("migration run failed: %v", err)
	}
	after, err := countAppliedMigrations(db)
	if err != nil {
		log.Fatalf("failed to count applied migrations after run: %v", err)
	}

	appliedThisRun := after - before
	if appliedThisRun < 0 {
		appliedThisRun = 0
	}
	durationMs := time.Since(started).Milliseconds()
	log.Printf("level=info component=migrate event=completed applied_this_run=%d total_applied=%d duration_ms=%d", appliedThisRun, after, durationMs)
}

func countAppliedMigrations(db *sql.DB) (int, error) {
	var regclass sql.NullString
	if err := db.QueryRow(`SELECT to_regclass('public.schema_migrations')::text`).Scan(&regclass); err != nil {
		return 0, fmt.Errorf("check schema_migrations existence: %w", err)
	}
	if !regclass.Valid {
		return 0, nil
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count applied migrations: %w", err)
	}
	return count, nil
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
