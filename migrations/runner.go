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

	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`); err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, advisoryLockKey); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}

	applied := make(map[string]struct{})
	rows, err := tx.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("load applied migrations: %w", err)
	}
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			rows.Close()
			return fmt.Errorf("scan applied migration: %w", err)
		}
		applied[version] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate applied migrations: %w", err)
	}
	rows.Close()

	entries, err := fs.ReadDir(migrationFiles, ".")
	if err != nil {
		return fmt.Errorf("read migration files: %w", err)
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

	for _, entry := range upEntries {
		name := entry.Name()
		version, err := parseVersion(name)
		if err != nil {
			return err
		}
		if _, exists := applied[version]; exists {
			continue
		}

		sqlBytes, err := fs.ReadFile(migrationFiles, name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		sqlText := strings.TrimSpace(string(sqlBytes))
		if sqlText == "" {
			return fmt.Errorf("migration %s is empty", name)
		}

		if _, err := tx.ExecContext(ctx, sqlText); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}

		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`, version, name); err != nil {
			return fmt.Errorf("record migration %s: %w", name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migrations: %w", err)
	}

	return nil
}

func parseVersion(filename string) (string, error) {
	parts := strings.SplitN(filename, "_", 2)
	if len(parts) < 2 || strings.TrimSpace(parts[0]) == "" {
		return "", fmt.Errorf("invalid migration name %q: expected <version>_<name>.up.sql", filename)
	}
	return parts[0], nil
}
