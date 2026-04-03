package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"usage-billing-control-plane/migrations"
)

const (
	defaultQueryTimeout     = 5 * time.Second
	defaultMigrationTimeout = 60 * time.Second
	defaultTenantID         = "default"
)

type PostgresStore struct {
	db               *sql.DB
	queryTimeout     time.Duration
	migrationTimeout time.Duration
}

type txSessionMode int

const (
	txSessionTenant txSessionMode = iota
	txSessionBypass
)

type PostgresOption func(*PostgresStore)

func WithQueryTimeout(timeout time.Duration) PostgresOption {
	return func(s *PostgresStore) {
		if timeout > 0 {
			s.queryTimeout = timeout
		}
	}
}

func WithMigrationTimeout(timeout time.Duration) PostgresOption {
	return func(s *PostgresStore) {
		if timeout > 0 {
			s.migrationTimeout = timeout
		}
	}
}

func NewPostgresStore(db *sql.DB, opts ...PostgresOption) *PostgresStore {
	s := &PostgresStore{
		db:               db,
		queryTimeout:     defaultQueryTimeout,
		migrationTimeout: defaultMigrationTimeout,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *PostgresStore) Migrate() error {
	runner := migrations.NewRunner(s.db, migrations.WithTimeout(s.migrationTimeout))
	return runner.Run(context.Background())
}


func (s *PostgresStore) beginTxWithSession(ctx context.Context, mode txSessionMode, tenantID string) (*sql.Tx, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}

	switch mode {
	case txSessionTenant:
		if _, err := tx.ExecContext(ctx, `SELECT set_config('app.bypass_rls', 'off', true)`); err != nil {
			_ = tx.Rollback()
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `SELECT set_config('app.tenant_id', $1, true)`, normalizeTenantID(tenantID)); err != nil {
			_ = tx.Rollback()
			return nil, err
		}
	case txSessionBypass:
		if _, err := tx.ExecContext(ctx, `SELECT set_config('app.bypass_rls', 'on', true)`); err != nil {
			_ = tx.Rollback()
			return nil, err
		}
	}

	return tx, nil
}

func rollbackSilently(tx *sql.Tx) {
	if tx != nil {
		_ = tx.Rollback()
	}
}


func (s *PostgresStore) withTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), s.queryTimeout)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func nullableString(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}

func nullableBoolPtr(v *bool) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullableInt64Ptr(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

func normalizeTenantID(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return defaultTenantID
	}
	return v
}


func newID(prefix string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		fallback := fmt.Sprintf("%d", time.Now().UnixNano())
		return fmt.Sprintf("%s_%s", prefix, fallback)
	}
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(buf))
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "duplicate key value") || strings.Contains(text, "unique constraint")
}

func normalizeOptionalText(v string) string {
	return strings.TrimSpace(v)
}

func nullIfEmpty(v string) *string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return &v
}


func normalizeStringList(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, item := range values {
		normalized := strings.TrimSpace(item)
		if normalized == "" {
			continue
		}
		normalized = strings.ToUpper(normalized)
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}
