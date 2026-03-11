package migrations_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"lago-usage-billing-alpha/migrations"
)

func TestRunnerAppliesMigrationsIdempotently(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for integration tests")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`DROP TABLE IF EXISTS schema_migrations, schema_migrations_legacy_custom, replay_jobs, billed_entries, usage_events, meters, rating_rule_versions CASCADE`); err != nil {
		t.Fatalf("drop existing tables: %v", err)
	}

	runner := migrations.NewRunner(db)
	if err := runner.Run(context.Background()); err != nil {
		t.Fatalf("first migration run: %v", err)
	}
	if err := runner.Run(context.Background()); err != nil {
		t.Fatalf("second migration run should be idempotent: %v", err)
	}

	status, err := runner.Status(context.Background())
	if err != nil {
		t.Fatalf("get migration status: %v", err)
	}
	if len(status.Available) < 3 {
		t.Fatalf("expected at least 3 available migrations, got %d", len(status.Available))
	}
	if status.PendingCount != 0 {
		t.Fatalf("expected zero pending migrations after run, got %d", status.PendingCount)
	}
	if status.Dirty {
		t.Fatalf("expected clean migration state")
	}
	if status.CurrentVersion == nil {
		t.Fatalf("expected current version to be set")
	}
	if *status.CurrentVersion != status.LatestVersion {
		t.Fatalf("expected current version %d to match latest %d", *status.CurrentVersion, status.LatestVersion)
	}

	for _, tableName := range []string{"rating_rule_versions", "meters", "usage_events", "billed_entries", "replay_jobs"} {
		var exists bool
		if err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1)`, tableName).Scan(&exists); err != nil {
			t.Fatalf("check table %s existence: %v", tableName, err)
		}
		if !exists {
			t.Fatalf("table %s should exist after migrations", tableName)
		}
	}

	for _, column := range []string{"attempt_count", "last_attempt_at"} {
		var exists bool
		if err := db.QueryRow(`SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND table_name = 'replay_jobs'
			  AND column_name = $1
		)`, column).Scan(&exists); err != nil {
			t.Fatalf("check replay_jobs.%s existence: %v", column, err)
		}
		if !exists {
			t.Fatalf("column replay_jobs.%s should exist after migrations", column)
		}
	}

	if err := runner.Verify(context.Background()); err != nil {
		t.Fatalf("verify migrations should pass: %v", err)
	}

	if _, err := db.Exec(`UPDATE schema_migrations SET version = 1, dirty = false`); err != nil {
		t.Fatalf("set schema_migrations to pending state: %v", err)
	}
	if err := runner.Verify(context.Background()); err == nil {
		t.Fatalf("verify should fail when pending migrations exist")
	}

	if _, err := db.Exec(`UPDATE schema_migrations SET version = 9999, dirty = false`); err != nil {
		t.Fatalf("set schema_migrations to unknown version: %v", err)
	}
	if err := runner.Verify(context.Background()); err == nil {
		t.Fatalf("verify should fail when current version is unknown")
	}

	if _, err := db.Exec(`UPDATE schema_migrations SET version = 3, dirty = true`); err != nil {
		t.Fatalf("set schema_migrations to dirty state: %v", err)
	}
	if err := runner.Verify(context.Background()); err == nil {
		t.Fatalf("verify should fail when migration state is dirty")
	}
}
