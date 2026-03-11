package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"
)

//go:embed *.up.sql
var migrationFiles embed.FS

const advisoryLockKey int64 = 22042026

type Runner struct {
	db      *sql.DB
	timeout time.Duration
}

type Option func(*Runner)

func WithTimeout(timeout time.Duration) Option {
	return func(r *Runner) {
		if timeout > 0 {
			r.timeout = timeout
		}
	}
}

func NewRunner(db *sql.DB, opts ...Option) *Runner {
	r := &Runner{
		db:      db,
		timeout: 60 * time.Second,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func (r *Runner) Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	if _, ok := ctx.Deadline(); !ok && r.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.timeout)
		defer cancel()
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := ensureSchemaMigrationsTable(ctx, tx); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, advisoryLockKey); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}

	applied, err := loadAppliedVersionSet(ctx, tx)
	if err != nil {
		return err
	}

	entries, err := loadMigrationEntries()
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if _, exists := applied[entry.Version]; exists {
			continue
		}

		if _, err := tx.ExecContext(ctx, entry.SQL); err != nil {
			return fmt.Errorf("apply migration %s: %w", entry.Name, err)
		}

		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`, entry.Version, entry.Name); err != nil {
			return fmt.Errorf("record migration %s: %w", entry.Name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migrations: %w", err)
	}

	return nil
}

type migrationEntry struct {
	Version string
	Name    string
	SQL     string
}

type execContexter interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type queryContexter interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func ensureSchemaMigrationsTable(ctx context.Context, db execContexter) error {
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`); err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}
	return nil
}

func loadAppliedVersionSet(ctx context.Context, db queryContexter) (map[string]struct{}, error) {
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("load applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]struct{})
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("scan applied migration: %w", err)
		}
		applied[version] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied migrations: %w", err)
	}
	return applied, nil
}

func loadMigrationEntries() ([]migrationEntry, error) {
	entries, err := fs.ReadDir(migrationFiles, ".")
	if err != nil {
		return nil, fmt.Errorf("read migration files: %w", err)
	}

	upEntries := make([]fs.DirEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		upEntries = append(upEntries, entry)
	}
	sort.Slice(upEntries, func(i, j int) bool {
		return upEntries[i].Name() < upEntries[j].Name()
	})

	out := make([]migrationEntry, 0, len(upEntries))
	for _, entry := range upEntries {
		name := entry.Name()
		version, err := parseVersion(name)
		if err != nil {
			return nil, err
		}

		sqlBytes, err := fs.ReadFile(migrationFiles, name)
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", name, err)
		}
		sqlText := strings.TrimSpace(string(sqlBytes))
		if sqlText == "" {
			return nil, fmt.Errorf("migration %s is empty", name)
		}

		out = append(out, migrationEntry{Version: version, Name: name, SQL: sqlText})
	}

	return out, nil
}

func parseVersion(filename string) (string, error) {
	parts := strings.SplitN(filename, "_", 2)
	if len(parts) < 2 || strings.TrimSpace(parts[0]) == "" {
		return "", fmt.Errorf("invalid migration name %q: expected <version>_<name>.up.sql", filename)
	}
	return parts[0], nil
}
