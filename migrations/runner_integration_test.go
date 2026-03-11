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

	if _, err := db.Exec(`DROP TABLE IF EXISTS schema_migrations, replay_jobs, billed_entries, usage_events, meters, rating_rule_versions CASCADE`); err != nil {
		t.Fatalf("drop existing tables: %v", err)
	}

	runner := migrations.NewRunner(db)
	if err := runner.Run(context.Background()); err != nil {
		t.Fatalf("first migration run: %v", err)
	}
	if err := runner.Run(context.Background()); err != nil {
		t.Fatalf("second migration run should be idempotent: %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("count schema_migrations: %v", err)
	}
	if count < 3 {
		t.Fatalf("expected at least 3 applied migrations, got %d", count)
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

	for _, constraint := range []string{
		"chk_rating_rule_mode_allowed",
		"chk_meter_aggregation_allowed",
		"chk_replay_job_status_allowed",
	} {
		var exists bool
		if err := db.QueryRow(`SELECT EXISTS (
			SELECT 1
			FROM pg_constraint
			WHERE conname = $1
		)`, constraint).Scan(&exists); err != nil {
			t.Fatalf("check constraint %s existence: %v", constraint, err)
		}
		if !exists {
			t.Fatalf("constraint %s should exist after migrations", constraint)
		}
	}

	status, err := migrations.GetStatus(context.Background(), db)
	if err != nil {
		t.Fatalf("get migration status: %v", err)
	}
	if len(status.Pending) != 0 {
		t.Fatalf("expected zero pending migrations after run, got %d", len(status.Pending))
	}
	if len(status.UnknownApplied) != 0 {
		t.Fatalf("expected zero unknown applied migrations after run, got %d", len(status.UnknownApplied))
	}

	if err := migrations.Verify(context.Background(), db); err != nil {
		t.Fatalf("verify migrations should pass: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO schema_migrations (version, name) VALUES ('9999', '9999_manual_unknown.up.sql')`); err != nil {
		t.Fatalf("insert unknown applied migration: %v", err)
	}
	if err := migrations.Verify(context.Background(), db); err == nil {
		t.Fatalf("verify migrations should fail when unknown applied migrations exist")
	}
}
