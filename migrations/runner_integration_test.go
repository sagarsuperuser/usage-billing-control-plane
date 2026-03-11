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
	if count < 1 {
		t.Fatalf("expected at least 1 applied migration, got %d", count)
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
}
