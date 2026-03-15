package migrations

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"

	migrate "github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed *.up.sql
var migrationFiles embed.FS

const (
	defaultTimeout         = 60 * time.Second
	defaultMigrationsTable = "schema_migrations"
)

type Runner struct {
	db      *sql.DB
	timeout time.Duration
	table   string
}

type Option func(*Runner)

func WithTimeout(timeout time.Duration) Option {
	return func(r *Runner) {
		if timeout > 0 {
			r.timeout = timeout
		}
	}
}

func WithMigrationsTable(table string) Option {
	return func(r *Runner) {
		table = strings.TrimSpace(table)
		if table != "" {
			r.table = table
		}
	}
}

func NewRunner(db *sql.DB, opts ...Option) *Runner {
	r := &Runner{
		db:      db,
		timeout: defaultTimeout,
		table:   defaultMigrationsTable,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func (r *Runner) Run(ctx context.Context) error {
	ctx, cancel := r.withTimeoutContext(ctx)
	defer cancel()

	m, err := r.newMigrator()
	if err != nil {
		return err
	}

	err = runWithContext(ctx, func() error {
		err := m.Up()
		if err == nil || errors.Is(err, migrate.ErrNoChange) {
			return nil
		}
		return err
	})
	if err != nil {
		return fmt.Errorf("apply migrations with golang-migrate: %w", err)
	}

	return nil
}

func (r *Runner) Status(ctx context.Context) (StatusReport, error) {
	ctx, cancel := r.withTimeoutContext(ctx)
	defer cancel()

	available, err := listAvailableMigrations()
	if err != nil {
		return StatusReport{}, err
	}

	m, err := r.newMigrator()
	if err != nil {
		return StatusReport{}, err
	}

	currentVersion, dirty, hasVersion, err := currentVersion(m)
	if err != nil {
		return StatusReport{}, err
	}

	report := buildStatusReport(available, currentVersion, dirty, hasVersion)
	return report, nil
}

func (r *Runner) Verify(ctx context.Context) error {
	status, err := r.Status(ctx)
	if err != nil {
		return err
	}

	issues := make([]string, 0, 3)
	if status.Dirty {
		issues = append(issues, "database migration state is dirty")
	}
	if status.UnknownCurrent {
		issues = append(issues, "current database version is unknown to local migration files")
	}
	if status.PendingCount > 0 {
		issues = append(issues, fmt.Sprintf("pending migrations=%d", status.PendingCount))
	}

	if len(issues) > 0 {
		return fmt.Errorf("migration verification failed: %s", strings.Join(issues, "; "))
	}
	return nil
}

func GetStatus(ctx context.Context, db *sql.DB, opts ...Option) (StatusReport, error) {
	return NewRunner(db, opts...).Status(ctx)
}

func Verify(ctx context.Context, db *sql.DB, opts ...Option) error {
	return NewRunner(db, opts...).Verify(ctx)
}

func (r *Runner) withTimeoutContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, hasDeadline := ctx.Deadline(); hasDeadline || r.timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, r.timeout)
}

func (r *Runner) newMigrator() (*migrate.Migrate, error) {
	sourceDriver, err := iofs.New(migrationFiles, ".")
	if err != nil {
		return nil, fmt.Errorf("create iofs migration source: %w", err)
	}

	dbDriver, err := postgres.WithInstance(r.db, &postgres.Config{MigrationsTable: r.table})
	if err != nil {
		return nil, fmt.Errorf("create postgres migration driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "postgres", dbDriver)
	if err != nil {
		return nil, fmt.Errorf("create golang-migrate instance: %w", err)
	}

	return m, nil
}

func runWithContext(ctx context.Context, fn func() error) error {
	if ctx == nil {
		return fn()
	}
	done := make(chan error, 1)
	go func() {
		done <- fn()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

func currentVersion(m *migrate.Migrate) (version uint, dirty bool, hasVersion bool, err error) {
	v, d, err := m.Version()
	if err == nil {
		return v, d, true, nil
	}
	if errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, false, nil
	}
	return 0, false, false, fmt.Errorf("read current migration version: %w", err)
}

type availableMigration struct {
	Version uint
	Name    string
}

func listAvailableMigrations() ([]availableMigration, error) {
	entries, err := fs.ReadDir(migrationFiles, ".")
	if err != nil {
		return nil, fmt.Errorf("read migration files: %w", err)
	}

	out := make([]availableMigration, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}

		version, err := parseVersion(name)
		if err != nil {
			return nil, err
		}
		out = append(out, availableMigration{Version: version, Name: name})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Version < out[j].Version
	})

	return out, nil
}

func parseVersion(filename string) (uint, error) {
	parts := strings.SplitN(filename, "_", 2)
	if len(parts) < 2 || strings.TrimSpace(parts[0]) == "" {
		return 0, fmt.Errorf("invalid migration name %q: expected <version>_<name>.up.sql", filename)
	}

	v, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid migration version in %q: %w", filename, err)
	}
	if v == 0 {
		return 0, fmt.Errorf("invalid migration version in %q: version must be > 0", filename)
	}
	return uint(v), nil
}
