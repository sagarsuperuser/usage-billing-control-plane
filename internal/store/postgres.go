package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lib/pq"

	"usage-billing-control-plane/internal/domain"
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

func (s *PostgresStore) CreateTenant(input domain.Tenant) (domain.Tenant, error) {
	input.ID = normalizeTenantID(input.ID)
	input.Name = strings.TrimSpace(input.Name)
	input.Status = normalizeTenantStatus(input.Status)
	input.BillingProviderConnectionID = normalizeOptionalText(input.BillingProviderConnectionID)
	input.LagoOrganizationID = normalizeOptionalText(input.LagoOrganizationID)
	input.LagoBillingProviderCode = normalizeOptionalText(input.LagoBillingProviderCode)
	if input.Name == "" {
		input.Name = input.ID
	}
	now := time.Now().UTC()
	if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = now
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.Tenant{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`INSERT INTO tenants (id, name, status, billing_provider_connection_id, lago_organization_id, lago_billing_provider_code, created_at, updated_at)
		VALUES ($1, $2, $3, NULLIF($4,''), NULLIF($5,''), NULLIF($6,''), $7, $8)
		RETURNING id, name, status, billing_provider_connection_id, lago_organization_id, lago_billing_provider_code, created_at, updated_at`,
		input.ID,
		input.Name,
		string(input.Status),
		input.BillingProviderConnectionID,
		input.LagoOrganizationID,
		input.LagoBillingProviderCode,
		input.CreatedAt,
		input.UpdatedAt,
	)
	tenant, err := scanTenant(row)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Tenant{}, ErrAlreadyExists
		}
		return domain.Tenant{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Tenant{}, err
	}
	return tenant, nil
}

func (s *PostgresStore) GetTenant(id string) (domain.Tenant, error) {
	id = normalizeTenantID(id)

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.Tenant{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`SELECT id, name, status, billing_provider_connection_id, lago_organization_id, lago_billing_provider_code, created_at, updated_at
		FROM tenants
		WHERE id = $1`,
		id,
	)
	tenant, err := scanTenant(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Tenant{}, ErrNotFound
		}
		return domain.Tenant{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Tenant{}, err
	}
	return tenant, nil
}

func (s *PostgresStore) GetTenantByLagoOrganizationID(organizationID string) (domain.Tenant, error) {
	organizationID = normalizeOptionalText(organizationID)
	if organizationID == "" {
		return domain.Tenant{}, ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.Tenant{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`SELECT id, name, status, billing_provider_connection_id, lago_organization_id, lago_billing_provider_code, created_at, updated_at
		FROM tenants
		WHERE lago_organization_id = $1`,
		organizationID,
	)
	tenant, err := scanTenant(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Tenant{}, ErrNotFound
		}
		return domain.Tenant{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Tenant{}, err
	}
	return tenant, nil
}

func (s *PostgresStore) UpdateTenant(input domain.Tenant) (domain.Tenant, error) {
	input.ID = normalizeTenantID(input.ID)
	input.Name = strings.TrimSpace(input.Name)
	input.Status = normalizeTenantStatus(input.Status)
	input.BillingProviderConnectionID = normalizeOptionalText(input.BillingProviderConnectionID)
	input.LagoOrganizationID = normalizeOptionalText(input.LagoOrganizationID)
	input.LagoBillingProviderCode = normalizeOptionalText(input.LagoBillingProviderCode)
	if input.Name == "" {
		return domain.Tenant{}, fmt.Errorf("validation failed: tenant name is required")
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = time.Now().UTC()
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.Tenant{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`UPDATE tenants
		SET name = $1,
		    status = $2,
		    billing_provider_connection_id = NULLIF($3,''),
		    lago_organization_id = NULLIF($4,''),
		    lago_billing_provider_code = NULLIF($5,''),
		    updated_at = $6
		WHERE id = $7
		RETURNING id, name, status, billing_provider_connection_id, lago_organization_id, lago_billing_provider_code, created_at, updated_at`,
		input.Name,
		string(input.Status),
		input.BillingProviderConnectionID,
		input.LagoOrganizationID,
		input.LagoBillingProviderCode,
		input.UpdatedAt,
		input.ID,
	)
	tenant, err := scanTenant(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Tenant{}, ErrNotFound
		}
		if isUniqueViolation(err) {
			return domain.Tenant{}, ErrAlreadyExists
		}
		return domain.Tenant{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Tenant{}, err
	}
	return tenant, nil
}

func (s *PostgresStore) ListTenants(status string) ([]domain.Tenant, error) {
	status = strings.ToLower(strings.TrimSpace(status))

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return nil, err
	}
	defer rollbackSilently(tx)

	query := `SELECT id, name, status, billing_provider_connection_id, lago_organization_id, lago_billing_provider_code, created_at, updated_at FROM tenants`
	args := []any{}
	if status != "" {
		query += ` WHERE status = $1`
		args = append(args, status)
	}
	query += ` ORDER BY created_at ASC, id ASC`

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.Tenant, 0)
	for rows.Next() {
		tenant, scanErr := scanTenant(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, tenant)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PostgresStore) UpdateTenantStatus(id string, status domain.TenantStatus, updatedAt time.Time) (domain.Tenant, error) {
	id = normalizeTenantID(id)
	status = normalizeTenantStatus(status)
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.Tenant{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`UPDATE tenants
		SET status = $1, updated_at = $2
		WHERE id = $3
		RETURNING id, name, status, billing_provider_connection_id, lago_organization_id, lago_billing_provider_code, created_at, updated_at`,
		string(status),
		updatedAt,
		id,
	)
	tenant, err := scanTenant(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Tenant{}, ErrNotFound
		}
		return domain.Tenant{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Tenant{}, err
	}
	return tenant, nil
}

func (s *PostgresStore) CreateTenantAuditEvent(input domain.TenantAuditEvent) (domain.TenantAuditEvent, error) {
	input.TenantID = normalizeTenantID(input.TenantID)
	input.ActorAPIKeyID = strings.TrimSpace(input.ActorAPIKeyID)
	input.Action = strings.ToLower(strings.TrimSpace(input.Action))
	if input.TenantID == "" || input.Action == "" {
		return domain.TenantAuditEvent{}, fmt.Errorf("validation failed: tenant_id and action are required")
	}
	if input.ID == "" {
		input.ID = newID("tae")
	}
	if input.CreatedAt.IsZero() {
		input.CreatedAt = time.Now().UTC()
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
	metadata, err := json.Marshal(input.Metadata)
	if err != nil {
		return domain.TenantAuditEvent{}, err
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.TenantAuditEvent{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`INSERT INTO tenant_audit_events (
			id, tenant_id, actor_api_key_id, action, metadata, created_at
		) VALUES ($1,$2,NULLIF($3,''),$4,$5::jsonb,$6)
		RETURNING id, tenant_id, actor_api_key_id, action, metadata, created_at`,
		input.ID,
		input.TenantID,
		input.ActorAPIKeyID,
		input.Action,
		string(metadata),
		input.CreatedAt,
	)
	event, err := scanTenantAuditEvent(row)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.TenantAuditEvent{}, ErrAlreadyExists
		}
		return domain.TenantAuditEvent{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.TenantAuditEvent{}, err
	}
	return event, nil
}

func (s *PostgresStore) ListTenantAuditEvents(filter TenantAuditFilter) (TenantAuditResult, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return TenantAuditResult{}, err
	}
	defer rollbackSilently(tx)

	clauses := []string{"1=1"}
	args := []any{}
	add := func(format string, val any) {
		args = append(args, val)
		clauses = append(clauses, fmt.Sprintf(format, len(args)))
	}
	if strings.TrimSpace(filter.TenantID) != "" {
		add("tenant_id = $%d", normalizeTenantID(filter.TenantID))
	}
	if strings.TrimSpace(filter.ActorAPIKeyID) != "" {
		add("actor_api_key_id = $%d", strings.TrimSpace(filter.ActorAPIKeyID))
	}
	if strings.TrimSpace(filter.Action) != "" {
		add("action = $%d", strings.TrimSpace(filter.Action))
	}
	where := strings.Join(clauses, " AND ")

	var total int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM tenant_audit_events WHERE `+where, args...).Scan(&total); err != nil {
		return TenantAuditResult{}, err
	}

	pagedArgs := append(append([]any{}, args...), limit, offset)
	rows, err := tx.QueryContext(
		ctx,
		`SELECT id, tenant_id, actor_api_key_id, action, metadata, created_at
		FROM tenant_audit_events
		WHERE `+where+`
		ORDER BY created_at DESC, id DESC
		LIMIT $`+strconv.Itoa(len(args)+1)+` OFFSET $`+strconv.Itoa(len(args)+2),
		pagedArgs...,
	)
	if err != nil {
		return TenantAuditResult{}, err
	}
	defer rows.Close()

	items := make([]domain.TenantAuditEvent, 0, limit)
	for rows.Next() {
		item, scanErr := scanTenantAuditEvent(rows)
		if scanErr != nil {
			return TenantAuditResult{}, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return TenantAuditResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return TenantAuditResult{}, err
	}
	return TenantAuditResult{
		Items:  items,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, nil
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

func (s *PostgresStore) CreateBillingProviderConnection(input domain.BillingProviderConnection) (domain.BillingProviderConnection, error) {
	input.ProviderType = domain.BillingProviderType(strings.ToLower(strings.TrimSpace(string(input.ProviderType))))
	input.Environment = strings.ToLower(strings.TrimSpace(input.Environment))
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Scope = domain.BillingProviderConnectionScope(strings.ToLower(strings.TrimSpace(string(input.Scope))))
	input.OwnerTenantID = normalizeOptionalText(input.OwnerTenantID)
	input.Status = domain.BillingProviderConnectionStatus(strings.ToLower(strings.TrimSpace(string(input.Status))))
	input.LagoOrganizationID = normalizeOptionalText(input.LagoOrganizationID)
	input.LagoProviderCode = normalizeOptionalText(input.LagoProviderCode)
	input.SecretRef = normalizeOptionalText(input.SecretRef)
	input.LastSyncError = normalizeOptionalText(input.LastSyncError)
	input.CreatedByType = strings.ToLower(strings.TrimSpace(input.CreatedByType))
	input.CreatedByID = normalizeOptionalText(input.CreatedByID)

	if input.ProviderType == "" || input.Environment == "" || input.DisplayName == "" || input.Scope == "" || input.Status == "" || input.CreatedByType == "" {
		return domain.BillingProviderConnection{}, fmt.Errorf("validation failed: provider_type, environment, display_name, scope, status, and created_by_type are required")
	}
	if input.ID == "" {
		input.ID = newID("bpc")
	}
	now := time.Now().UTC()
	if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = now
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.BillingProviderConnection{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`INSERT INTO billing_provider_connections (
			id, provider_type, environment, display_name, scope, owner_tenant_id, status,
			lago_organization_id, lago_provider_code, secret_ref, last_synced_at, last_sync_error,
			connected_at, disabled_at, created_by_type, created_by_id, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),$7,NULLIF($8,''),NULLIF($9,''),NULLIF($10,''),$11,NULLIF($12,''),$13,$14,$15,NULLIF($16,''),$17,$18)
		RETURNING id, provider_type, environment, display_name, scope, owner_tenant_id, status,
			lago_organization_id, lago_provider_code, secret_ref, last_synced_at, last_sync_error,
			connected_at, disabled_at, created_by_type, created_by_id, created_at, updated_at`,
		input.ID,
		string(input.ProviderType),
		input.Environment,
		input.DisplayName,
		string(input.Scope),
		input.OwnerTenantID,
		string(input.Status),
		input.LagoOrganizationID,
		input.LagoProviderCode,
		input.SecretRef,
		input.LastSyncedAt,
		input.LastSyncError,
		input.ConnectedAt,
		input.DisabledAt,
		input.CreatedByType,
		input.CreatedByID,
		input.CreatedAt,
		input.UpdatedAt,
	)
	out, err := scanBillingProviderConnection(row)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.BillingProviderConnection{}, ErrAlreadyExists
		}
		return domain.BillingProviderConnection{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.BillingProviderConnection{}, err
	}
	return out, nil
}

func (s *PostgresStore) GetBillingProviderConnection(id string) (domain.BillingProviderConnection, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.BillingProviderConnection{}, ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.BillingProviderConnection{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`SELECT id, provider_type, environment, display_name, scope, owner_tenant_id, status,
			lago_organization_id, lago_provider_code, secret_ref, last_synced_at, last_sync_error,
			connected_at, disabled_at, created_by_type, created_by_id, created_at, updated_at
		FROM billing_provider_connections
		WHERE id = $1`,
		id,
	)
	out, err := scanBillingProviderConnection(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.BillingProviderConnection{}, ErrNotFound
		}
		return domain.BillingProviderConnection{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.BillingProviderConnection{}, err
	}
	return out, nil
}

func (s *PostgresStore) ListBillingProviderConnections(filter BillingProviderConnectionListFilter) ([]domain.BillingProviderConnection, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return nil, err
	}
	defer rollbackSilently(tx)

	clauses := []string{"1=1"}
	args := []any{}
	nextArg := 1
	if providerType := strings.ToLower(strings.TrimSpace(filter.ProviderType)); providerType != "" {
		clauses = append(clauses, fmt.Sprintf("provider_type = $%d", nextArg))
		args = append(args, providerType)
		nextArg++
	}
	if environment := strings.ToLower(strings.TrimSpace(filter.Environment)); environment != "" {
		clauses = append(clauses, fmt.Sprintf("environment = $%d", nextArg))
		args = append(args, environment)
		nextArg++
	}
	if status := strings.ToLower(strings.TrimSpace(filter.Status)); status != "" {
		clauses = append(clauses, fmt.Sprintf("status = $%d", nextArg))
		args = append(args, status)
		nextArg++
	}
	if scope := strings.ToLower(strings.TrimSpace(filter.Scope)); scope != "" {
		clauses = append(clauses, fmt.Sprintf("scope = $%d", nextArg))
		args = append(args, scope)
		nextArg++
	}
	if ownerTenantID := normalizeOptionalText(filter.OwnerTenantID); ownerTenantID != "" {
		clauses = append(clauses, fmt.Sprintf("owner_tenant_id = $%d", nextArg))
		args = append(args, ownerTenantID)
		nextArg++
	}
	query := fmt.Sprintf(`SELECT id, provider_type, environment, display_name, scope, owner_tenant_id, status,
		lago_organization_id, lago_provider_code, secret_ref, last_synced_at, last_sync_error,
		connected_at, disabled_at, created_by_type, created_by_id, created_at, updated_at
	FROM billing_provider_connections
	WHERE %s
	ORDER BY created_at DESC, id ASC
	LIMIT $%d OFFSET $%d`, strings.Join(clauses, " AND "), nextArg, nextArg+1)
	args = append(args, limit, offset)

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.BillingProviderConnection, 0)
	for rows.Next() {
		item, scanErr := scanBillingProviderConnection(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PostgresStore) CountTenantsByBillingProviderConnections(connectionIDs []string) (map[string]int, error) {
	ids := make([]string, 0, len(connectionIDs))
	seen := make(map[string]struct{}, len(connectionIDs))
	for _, item := range connectionIDs {
		id := strings.TrimSpace(item)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	counts := make(map[string]int, len(ids))
	if len(ids) == 0 {
		return counts, nil
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return nil, err
	}
	defer rollbackSilently(tx)

	rows, err := tx.QueryContext(
		ctx,
		`SELECT billing_provider_connection_id, COUNT(*)
		 FROM tenants
		 WHERE billing_provider_connection_id = ANY($1)
		 GROUP BY billing_provider_connection_id`,
		pq.Array(ids),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		var count int
		if err := rows.Scan(&id, &count); err != nil {
			return nil, err
		}
		counts[id] = count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return counts, nil
}

func (s *PostgresStore) UpdateBillingProviderConnection(input domain.BillingProviderConnection) (domain.BillingProviderConnection, error) {
	input.ID = strings.TrimSpace(input.ID)
	input.ProviderType = domain.BillingProviderType(strings.ToLower(strings.TrimSpace(string(input.ProviderType))))
	input.Environment = strings.ToLower(strings.TrimSpace(input.Environment))
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Scope = domain.BillingProviderConnectionScope(strings.ToLower(strings.TrimSpace(string(input.Scope))))
	input.OwnerTenantID = normalizeOptionalText(input.OwnerTenantID)
	input.Status = domain.BillingProviderConnectionStatus(strings.ToLower(strings.TrimSpace(string(input.Status))))
	input.LagoOrganizationID = normalizeOptionalText(input.LagoOrganizationID)
	input.LagoProviderCode = normalizeOptionalText(input.LagoProviderCode)
	input.SecretRef = normalizeOptionalText(input.SecretRef)
	input.LastSyncError = normalizeOptionalText(input.LastSyncError)
	input.CreatedByType = strings.ToLower(strings.TrimSpace(input.CreatedByType))
	input.CreatedByID = normalizeOptionalText(input.CreatedByID)
	if input.ID == "" || input.ProviderType == "" || input.Environment == "" || input.DisplayName == "" || input.Scope == "" || input.Status == "" || input.CreatedByType == "" {
		return domain.BillingProviderConnection{}, fmt.Errorf("validation failed: id, provider_type, environment, display_name, scope, status, and created_by_type are required")
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = time.Now().UTC()
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.BillingProviderConnection{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`UPDATE billing_provider_connections
		SET provider_type = $1,
		    environment = $2,
		    display_name = $3,
		    scope = $4,
		    owner_tenant_id = NULLIF($5,''),
		    status = $6,
		    lago_organization_id = NULLIF($7,''),
		    lago_provider_code = NULLIF($8,''),
		    secret_ref = NULLIF($9,''),
		    last_synced_at = $10,
		    last_sync_error = NULLIF($11,''),
		    connected_at = $12,
		    disabled_at = $13,
		    created_by_type = $14,
		    created_by_id = NULLIF($15,''),
		    updated_at = $16
		WHERE id = $17
		RETURNING id, provider_type, environment, display_name, scope, owner_tenant_id, status,
			lago_organization_id, lago_provider_code, secret_ref, last_synced_at, last_sync_error,
			connected_at, disabled_at, created_by_type, created_by_id, created_at, updated_at`,
		string(input.ProviderType),
		input.Environment,
		input.DisplayName,
		string(input.Scope),
		input.OwnerTenantID,
		string(input.Status),
		input.LagoOrganizationID,
		input.LagoProviderCode,
		input.SecretRef,
		input.LastSyncedAt,
		input.LastSyncError,
		input.ConnectedAt,
		input.DisabledAt,
		input.CreatedByType,
		input.CreatedByID,
		input.UpdatedAt,
		input.ID,
	)
	out, err := scanBillingProviderConnection(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.BillingProviderConnection{}, ErrNotFound
		}
		if isUniqueViolation(err) {
			return domain.BillingProviderConnection{}, ErrAlreadyExists
		}
		return domain.BillingProviderConnection{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.BillingProviderConnection{}, err
	}
	return out, nil
}

func (s *PostgresStore) CreateWorkspaceBillingBinding(input domain.WorkspaceBillingBinding) (domain.WorkspaceBillingBinding, error) {
	input.ID = strings.TrimSpace(input.ID)
	input.WorkspaceID = normalizeTenantID(input.WorkspaceID)
	input.BillingProviderConnectionID = strings.TrimSpace(input.BillingProviderConnectionID)
	input.Backend = domain.WorkspaceBillingBackend(strings.ToLower(strings.TrimSpace(string(input.Backend))))
	input.BackendOrganizationID = normalizeOptionalText(input.BackendOrganizationID)
	input.BackendProviderCode = normalizeOptionalText(input.BackendProviderCode)
	input.IsolationMode = domain.WorkspaceBillingIsolationMode(strings.ToLower(strings.TrimSpace(string(input.IsolationMode))))
	input.Status = domain.WorkspaceBillingBindingStatus(strings.ToLower(strings.TrimSpace(string(input.Status))))
	input.ProvisioningError = normalizeOptionalText(input.ProvisioningError)
	input.CreatedByType = strings.ToLower(strings.TrimSpace(input.CreatedByType))
	input.CreatedByID = normalizeOptionalText(input.CreatedByID)
	if input.WorkspaceID == "" || input.BillingProviderConnectionID == "" || input.Backend == "" || input.IsolationMode == "" || input.Status == "" || input.CreatedByType == "" {
		return domain.WorkspaceBillingBinding{}, fmt.Errorf("validation failed: workspace_id, billing_provider_connection_id, backend, isolation_mode, status, and created_by_type are required")
	}
	if input.ID == "" {
		input.ID = newID("wbb")
	}
	now := time.Now().UTC()
	if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = now
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.WorkspaceBillingBinding{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`INSERT INTO workspace_billing_bindings (
			id, workspace_id, billing_provider_connection_id, backend, backend_organization_id, backend_provider_code,
			isolation_mode, status, provisioning_error, last_verified_at, connected_at, disabled_at,
			created_by_type, created_by_id, created_at, updated_at
		) VALUES ($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),$7,$8,$9,$10,$11,$12,$13,NULLIF($14,''),$15,$16)
		RETURNING id, workspace_id, billing_provider_connection_id, backend, backend_organization_id, backend_provider_code,
			isolation_mode, status, provisioning_error, last_verified_at, connected_at, disabled_at,
			created_by_type, created_by_id, created_at, updated_at`,
		input.ID,
		input.WorkspaceID,
		input.BillingProviderConnectionID,
		string(input.Backend),
		input.BackendOrganizationID,
		input.BackendProviderCode,
		string(input.IsolationMode),
		string(input.Status),
		input.ProvisioningError,
		input.LastVerifiedAt,
		input.ConnectedAt,
		input.DisabledAt,
		input.CreatedByType,
		input.CreatedByID,
		input.CreatedAt,
		input.UpdatedAt,
	)
	out, err := scanWorkspaceBillingBinding(row)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.WorkspaceBillingBinding{}, ErrAlreadyExists
		}
		return domain.WorkspaceBillingBinding{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.WorkspaceBillingBinding{}, err
	}
	return out, nil
}

func (s *PostgresStore) GetWorkspaceBillingBinding(workspaceID string) (domain.WorkspaceBillingBinding, error) {
	workspaceID = normalizeTenantID(workspaceID)
	if workspaceID == "" {
		return domain.WorkspaceBillingBinding{}, ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.WorkspaceBillingBinding{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`SELECT id, workspace_id, billing_provider_connection_id, backend, backend_organization_id, backend_provider_code,
			isolation_mode, status, provisioning_error, last_verified_at, connected_at, disabled_at,
			created_by_type, created_by_id, created_at, updated_at
		FROM workspace_billing_bindings
		WHERE workspace_id = $1`,
		workspaceID,
	)
	out, err := scanWorkspaceBillingBinding(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.WorkspaceBillingBinding{}, ErrNotFound
		}
		return domain.WorkspaceBillingBinding{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.WorkspaceBillingBinding{}, err
	}
	return out, nil
}

func (s *PostgresStore) ListWorkspaceBillingBindings(filter WorkspaceBillingBindingListFilter) ([]domain.WorkspaceBillingBinding, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return nil, err
	}
	defer rollbackSilently(tx)

	clauses := []string{"1=1"}
	args := []any{}
	nextArg := 1
	if workspaceID := normalizeTenantID(filter.WorkspaceID); workspaceID != "" {
		clauses = append(clauses, fmt.Sprintf("workspace_id = $%d", nextArg))
		args = append(args, workspaceID)
		nextArg++
	}
	if connectionID := strings.TrimSpace(filter.BillingProviderConnectionID); connectionID != "" {
		clauses = append(clauses, fmt.Sprintf("billing_provider_connection_id = $%d", nextArg))
		args = append(args, connectionID)
		nextArg++
	}
	if backend := strings.ToLower(strings.TrimSpace(filter.Backend)); backend != "" {
		clauses = append(clauses, fmt.Sprintf("backend = $%d", nextArg))
		args = append(args, backend)
		nextArg++
	}
	if isolationMode := strings.ToLower(strings.TrimSpace(filter.IsolationMode)); isolationMode != "" {
		clauses = append(clauses, fmt.Sprintf("isolation_mode = $%d", nextArg))
		args = append(args, isolationMode)
		nextArg++
	}
	if status := strings.ToLower(strings.TrimSpace(filter.Status)); status != "" {
		clauses = append(clauses, fmt.Sprintf("status = $%d", nextArg))
		args = append(args, status)
		nextArg++
	}
	query := fmt.Sprintf(`SELECT id, workspace_id, billing_provider_connection_id, backend, backend_organization_id, backend_provider_code,
		isolation_mode, status, provisioning_error, last_verified_at, connected_at, disabled_at,
		created_by_type, created_by_id, created_at, updated_at
	FROM workspace_billing_bindings
	WHERE %s
	ORDER BY created_at DESC, id ASC
	LIMIT $%d OFFSET $%d`, strings.Join(clauses, " AND "), nextArg, nextArg+1)
	args = append(args, limit, offset)

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.WorkspaceBillingBinding, 0)
	for rows.Next() {
		item, scanErr := scanWorkspaceBillingBinding(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PostgresStore) UpdateWorkspaceBillingBinding(input domain.WorkspaceBillingBinding) (domain.WorkspaceBillingBinding, error) {
	input.ID = strings.TrimSpace(input.ID)
	input.WorkspaceID = normalizeTenantID(input.WorkspaceID)
	input.BillingProviderConnectionID = strings.TrimSpace(input.BillingProviderConnectionID)
	input.Backend = domain.WorkspaceBillingBackend(strings.ToLower(strings.TrimSpace(string(input.Backend))))
	input.BackendOrganizationID = normalizeOptionalText(input.BackendOrganizationID)
	input.BackendProviderCode = normalizeOptionalText(input.BackendProviderCode)
	input.IsolationMode = domain.WorkspaceBillingIsolationMode(strings.ToLower(strings.TrimSpace(string(input.IsolationMode))))
	input.Status = domain.WorkspaceBillingBindingStatus(strings.ToLower(strings.TrimSpace(string(input.Status))))
	input.ProvisioningError = normalizeOptionalText(input.ProvisioningError)
	input.CreatedByType = strings.ToLower(strings.TrimSpace(input.CreatedByType))
	input.CreatedByID = normalizeOptionalText(input.CreatedByID)
	if input.ID == "" || input.WorkspaceID == "" || input.BillingProviderConnectionID == "" || input.Backend == "" || input.IsolationMode == "" || input.Status == "" || input.CreatedByType == "" {
		return domain.WorkspaceBillingBinding{}, fmt.Errorf("validation failed: id, workspace_id, billing_provider_connection_id, backend, isolation_mode, status, and created_by_type are required")
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = time.Now().UTC()
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.WorkspaceBillingBinding{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`UPDATE workspace_billing_bindings
		SET workspace_id = $1,
		    billing_provider_connection_id = $2,
		    backend = $3,
		    backend_organization_id = NULLIF($4,''),
		    backend_provider_code = NULLIF($5,''),
		    isolation_mode = $6,
		    status = $7,
		    provisioning_error = $8,
		    last_verified_at = $9,
		    connected_at = $10,
		    disabled_at = $11,
		    created_by_type = $12,
		    created_by_id = NULLIF($13,''),
		    updated_at = $14
		WHERE id = $15
		RETURNING id, workspace_id, billing_provider_connection_id, backend, backend_organization_id, backend_provider_code,
			isolation_mode, status, provisioning_error, last_verified_at, connected_at, disabled_at,
			created_by_type, created_by_id, created_at, updated_at`,
		input.WorkspaceID,
		input.BillingProviderConnectionID,
		string(input.Backend),
		input.BackendOrganizationID,
		input.BackendProviderCode,
		string(input.IsolationMode),
		string(input.Status),
		input.ProvisioningError,
		input.LastVerifiedAt,
		input.ConnectedAt,
		input.DisabledAt,
		input.CreatedByType,
		input.CreatedByID,
		input.UpdatedAt,
		input.ID,
	)
	out, err := scanWorkspaceBillingBinding(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.WorkspaceBillingBinding{}, ErrNotFound
		}
		if isUniqueViolation(err) {
			return domain.WorkspaceBillingBinding{}, ErrAlreadyExists
		}
		return domain.WorkspaceBillingBinding{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.WorkspaceBillingBinding{}, err
	}
	return out, nil
}

func (s *PostgresStore) CreateUser(input domain.User) (domain.User, error) {
	input.ID = strings.TrimSpace(input.ID)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Status = domain.UserStatus(strings.ToLower(strings.TrimSpace(string(input.Status))))
	input.PlatformRole = domain.UserPlatformRole(strings.ToLower(strings.TrimSpace(string(input.PlatformRole))))
	if input.Email == "" || input.DisplayName == "" || input.Status == "" {
		return domain.User{}, fmt.Errorf("validation failed: email, display_name, and status are required")
	}
	if input.ID == "" {
		input.ID = newID("usr")
	}
	now := time.Now().UTC()
	if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = now
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.User{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `INSERT INTO users (id, email, display_name, status, platform_role, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, email, display_name, status, platform_role, created_at, updated_at`,
		input.ID, input.Email, input.DisplayName, string(input.Status), string(input.PlatformRole), input.CreatedAt, input.UpdatedAt)
	out, err := scanUser(row)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.User{}, ErrAlreadyExists
		}
		return domain.User{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.User{}, err
	}
	return out, nil
}

func (s *PostgresStore) GetUser(id string) (domain.User, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.User{}, ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.User{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `SELECT id, email, display_name, status, platform_role, created_at, updated_at FROM users WHERE id = $1`, id)
	out, err := scanUser(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.User{}, ErrNotFound
		}
		return domain.User{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.User{}, err
	}
	return out, nil
}

func (s *PostgresStore) GetUserByEmail(email string) (domain.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return domain.User{}, ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.User{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `SELECT id, email, display_name, status, platform_role, created_at, updated_at FROM users WHERE lower(email) = lower($1)`, email)
	out, err := scanUser(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.User{}, ErrNotFound
		}
		return domain.User{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.User{}, err
	}
	return out, nil
}

func (s *PostgresStore) UpdateUser(input domain.User) (domain.User, error) {
	input.ID = strings.TrimSpace(input.ID)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Status = domain.UserStatus(strings.ToLower(strings.TrimSpace(string(input.Status))))
	input.PlatformRole = domain.UserPlatformRole(strings.ToLower(strings.TrimSpace(string(input.PlatformRole))))
	if input.ID == "" || input.Email == "" || input.DisplayName == "" || input.Status == "" {
		return domain.User{}, fmt.Errorf("validation failed: id, email, display_name, and status are required")
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = time.Now().UTC()
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.User{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `UPDATE users SET email = $1, display_name = $2, status = $3, platform_role = $4, updated_at = $5
		WHERE id = $6
		RETURNING id, email, display_name, status, platform_role, created_at, updated_at`,
		input.Email, input.DisplayName, string(input.Status), string(input.PlatformRole), input.UpdatedAt, input.ID)
	out, err := scanUser(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.User{}, ErrNotFound
		}
		if isUniqueViolation(err) {
			return domain.User{}, ErrAlreadyExists
		}
		return domain.User{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.User{}, err
	}
	return out, nil
}

func (s *PostgresStore) UpsertUserPasswordCredential(input domain.UserPasswordCredential) (domain.UserPasswordCredential, error) {
	input.UserID = strings.TrimSpace(input.UserID)
	input.PasswordHash = strings.TrimSpace(input.PasswordHash)
	if input.UserID == "" || input.PasswordHash == "" {
		return domain.UserPasswordCredential{}, fmt.Errorf("validation failed: user_id and password_hash are required")
	}
	now := time.Now().UTC()
	if input.PasswordUpdatedAt.IsZero() {
		input.PasswordUpdatedAt = now
	}
	if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = now
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.UserPasswordCredential{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `INSERT INTO user_password_credentials (user_id, password_hash, password_updated_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id) DO UPDATE
		SET password_hash = EXCLUDED.password_hash,
		    password_updated_at = EXCLUDED.password_updated_at,
		    updated_at = EXCLUDED.updated_at
		RETURNING user_id, password_hash, password_updated_at, created_at, updated_at`,
		input.UserID, input.PasswordHash, input.PasswordUpdatedAt, input.CreatedAt, input.UpdatedAt)
	out, err := scanUserPasswordCredential(row)
	if err != nil {
		return domain.UserPasswordCredential{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.UserPasswordCredential{}, err
	}
	return out, nil
}

func (s *PostgresStore) GetUserPasswordCredential(userID string) (domain.UserPasswordCredential, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return domain.UserPasswordCredential{}, ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.UserPasswordCredential{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `SELECT user_id, password_hash, password_updated_at, created_at, updated_at FROM user_password_credentials WHERE user_id = $1`, userID)
	out, err := scanUserPasswordCredential(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.UserPasswordCredential{}, ErrNotFound
		}
		return domain.UserPasswordCredential{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.UserPasswordCredential{}, err
	}
	return out, nil
}

func (s *PostgresStore) CreatePasswordResetToken(input domain.PasswordResetToken) (domain.PasswordResetToken, error) {
	input.UserID = strings.TrimSpace(input.UserID)
	input.TokenHash = strings.TrimSpace(input.TokenHash)
	if input.UserID == "" || input.TokenHash == "" || input.ExpiresAt.IsZero() {
		return domain.PasswordResetToken{}, fmt.Errorf("validation failed: user_id, token_hash, and expires_at are required")
	}
	now := time.Now().UTC()
	if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = now
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.PasswordResetToken{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `INSERT INTO password_reset_tokens (user_id, token_hash, expires_at, used_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, user_id, token_hash, expires_at, used_at, created_at, updated_at`,
		input.UserID, input.TokenHash, input.ExpiresAt, input.UsedAt, input.CreatedAt, input.UpdatedAt)
	out, err := scanPasswordResetToken(row)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.PasswordResetToken{}, ErrAlreadyExists
		}
		return domain.PasswordResetToken{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.PasswordResetToken{}, err
	}
	return out, nil
}

func (s *PostgresStore) GetPasswordResetTokenByTokenHash(tokenHash string) (domain.PasswordResetToken, error) {
	tokenHash = strings.TrimSpace(tokenHash)
	if tokenHash == "" {
		return domain.PasswordResetToken{}, ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.PasswordResetToken{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `SELECT id, user_id, token_hash, expires_at, used_at, created_at, updated_at FROM password_reset_tokens WHERE token_hash = $1`, tokenHash)
	out, err := scanPasswordResetToken(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.PasswordResetToken{}, ErrNotFound
		}
		return domain.PasswordResetToken{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.PasswordResetToken{}, err
	}
	return out, nil
}

func (s *PostgresStore) UpdatePasswordResetToken(input domain.PasswordResetToken) (domain.PasswordResetToken, error) {
	input.ID = strings.TrimSpace(input.ID)
	input.UserID = strings.TrimSpace(input.UserID)
	input.TokenHash = strings.TrimSpace(input.TokenHash)
	if input.ID == "" || input.UserID == "" || input.TokenHash == "" || input.ExpiresAt.IsZero() {
		return domain.PasswordResetToken{}, fmt.Errorf("validation failed: id, user_id, token_hash, and expires_at are required")
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = time.Now().UTC()
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.PasswordResetToken{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `UPDATE password_reset_tokens SET user_id = $1, token_hash = $2, expires_at = $3, used_at = $4, updated_at = $5
		WHERE id = $6
		RETURNING id, user_id, token_hash, expires_at, used_at, created_at, updated_at`,
		input.UserID, input.TokenHash, input.ExpiresAt, input.UsedAt, input.UpdatedAt, input.ID)
	out, err := scanPasswordResetToken(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.PasswordResetToken{}, ErrNotFound
		}
		if isUniqueViolation(err) {
			return domain.PasswordResetToken{}, ErrAlreadyExists
		}
		return domain.PasswordResetToken{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.PasswordResetToken{}, err
	}
	return out, nil
}

func (s *PostgresStore) UpsertUserTenantMembership(input domain.UserTenantMembership) (domain.UserTenantMembership, error) {
	input.UserID = strings.TrimSpace(input.UserID)
	input.TenantID = normalizeTenantID(input.TenantID)
	input.Role = strings.ToLower(strings.TrimSpace(input.Role))
	input.Status = domain.UserTenantMembershipStatus(strings.ToLower(strings.TrimSpace(string(input.Status))))
	if input.UserID == "" || input.TenantID == "" || input.Role == "" || input.Status == "" {
		return domain.UserTenantMembership{}, fmt.Errorf("validation failed: user_id, tenant_id, role, and status are required")
	}
	now := time.Now().UTC()
	if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = now
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.UserTenantMembership{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `INSERT INTO user_tenant_memberships (user_id, tenant_id, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, tenant_id) DO UPDATE
		SET role = EXCLUDED.role,
		    status = EXCLUDED.status,
		    updated_at = EXCLUDED.updated_at
		RETURNING user_id, tenant_id, role, status, created_at, updated_at`,
		input.UserID, input.TenantID, input.Role, string(input.Status), input.CreatedAt, input.UpdatedAt)
	out, err := scanUserTenantMembership(row)
	if err != nil {
		return domain.UserTenantMembership{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.UserTenantMembership{}, err
	}
	return out, nil
}

func (s *PostgresStore) GetUserTenantMembership(userID, tenantID string) (domain.UserTenantMembership, error) {
	userID = strings.TrimSpace(userID)
	tenantID = normalizeTenantID(tenantID)
	if userID == "" || tenantID == "" {
		return domain.UserTenantMembership{}, ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.UserTenantMembership{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `SELECT user_id, tenant_id, role, status, created_at, updated_at FROM user_tenant_memberships WHERE user_id = $1 AND tenant_id = $2`, userID, tenantID)
	out, err := scanUserTenantMembership(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.UserTenantMembership{}, ErrNotFound
		}
		return domain.UserTenantMembership{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.UserTenantMembership{}, err
	}
	return out, nil
}

func (s *PostgresStore) ListUserTenantMemberships(userID string) ([]domain.UserTenantMembership, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, nil
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return nil, err
	}
	defer rollbackSilently(tx)

	rows, err := tx.QueryContext(ctx, `SELECT user_id, tenant_id, role, status, created_at, updated_at FROM user_tenant_memberships WHERE user_id = $1 ORDER BY created_at ASC, tenant_id ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.UserTenantMembership, 0)
	for rows.Next() {
		item, scanErr := scanUserTenantMembership(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PostgresStore) ListTenantMemberships(tenantID string) ([]domain.UserTenantMembership, error) {
	tenantID = normalizeTenantID(tenantID)
	if tenantID == "" {
		return nil, nil
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return nil, err
	}
	defer rollbackSilently(tx)

	rows, err := tx.QueryContext(ctx, `SELECT user_id, tenant_id, role, status, created_at, updated_at FROM user_tenant_memberships WHERE tenant_id = $1 ORDER BY created_at ASC, user_id ASC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.UserTenantMembership, 0)
	for rows.Next() {
		item, scanErr := scanUserTenantMembership(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PostgresStore) CreateWorkspaceInvitation(input domain.WorkspaceInvitation) (domain.WorkspaceInvitation, error) {
	input.ID = strings.TrimSpace(input.ID)
	input.WorkspaceID = normalizeTenantID(input.WorkspaceID)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Role = strings.ToLower(strings.TrimSpace(input.Role))
	input.Status = domain.WorkspaceInvitationStatus(strings.ToLower(strings.TrimSpace(string(input.Status))))
	input.TokenHash = strings.TrimSpace(input.TokenHash)
	input.AcceptedByUserID = strings.TrimSpace(input.AcceptedByUserID)
	input.InvitedByUserID = strings.TrimSpace(input.InvitedByUserID)
	if input.WorkspaceID == "" || input.Email == "" || input.Role == "" || input.Status == "" || input.TokenHash == "" || input.ExpiresAt.IsZero() {
		return domain.WorkspaceInvitation{}, fmt.Errorf("validation failed: workspace_id, email, role, status, token_hash, and expires_at are required")
	}
	if input.ID == "" {
		input.ID = newID("wsi")
	}
	now := time.Now().UTC()
	if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = now
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.WorkspaceInvitation{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `INSERT INTO workspace_invitations (
		id, workspace_id, email, role, status, token_hash, expires_at, accepted_at, accepted_by_user_id,
		invited_by_user_id, invited_by_platform_user, revoked_at, created_at, updated_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9,''),NULLIF($10,''),$11,$12,$13,$14)
	RETURNING id, workspace_id, email, role, status, token_hash, expires_at, accepted_at, accepted_by_user_id,
		invited_by_user_id, invited_by_platform_user, revoked_at, created_at, updated_at`,
		input.ID,
		input.WorkspaceID,
		input.Email,
		input.Role,
		string(input.Status),
		input.TokenHash,
		input.ExpiresAt,
		input.AcceptedAt,
		input.AcceptedByUserID,
		input.InvitedByUserID,
		input.InvitedByPlatformUser,
		input.RevokedAt,
		input.CreatedAt,
		input.UpdatedAt,
	)
	out, err := scanWorkspaceInvitation(row)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.WorkspaceInvitation{}, ErrAlreadyExists
		}
		return domain.WorkspaceInvitation{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.WorkspaceInvitation{}, err
	}
	return out, nil
}

func (s *PostgresStore) GetWorkspaceInvitation(id string) (domain.WorkspaceInvitation, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.WorkspaceInvitation{}, ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.WorkspaceInvitation{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `SELECT id, workspace_id, email, role, status, token_hash, expires_at, accepted_at, accepted_by_user_id,
		invited_by_user_id, invited_by_platform_user, revoked_at, created_at, updated_at
		FROM workspace_invitations
		WHERE id = $1`, id)
	out, err := scanWorkspaceInvitation(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.WorkspaceInvitation{}, ErrNotFound
		}
		return domain.WorkspaceInvitation{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.WorkspaceInvitation{}, err
	}
	return out, nil
}

func (s *PostgresStore) GetWorkspaceInvitationByTokenHash(tokenHash string) (domain.WorkspaceInvitation, error) {
	tokenHash = strings.TrimSpace(tokenHash)
	if tokenHash == "" {
		return domain.WorkspaceInvitation{}, ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.WorkspaceInvitation{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `SELECT id, workspace_id, email, role, status, token_hash, expires_at, accepted_at, accepted_by_user_id,
		invited_by_user_id, invited_by_platform_user, revoked_at, created_at, updated_at
		FROM workspace_invitations
		WHERE token_hash = $1`, tokenHash)
	out, err := scanWorkspaceInvitation(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.WorkspaceInvitation{}, ErrNotFound
		}
		return domain.WorkspaceInvitation{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.WorkspaceInvitation{}, err
	}
	return out, nil
}

func (s *PostgresStore) ListWorkspaceInvitations(filter WorkspaceInvitationListFilter) ([]domain.WorkspaceInvitation, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return nil, err
	}
	defer rollbackSilently(tx)

	clauses := []string{"1=1"}
	args := []any{}
	nextArg := 1
	if workspaceID := normalizeTenantID(filter.WorkspaceID); workspaceID != "" {
		clauses = append(clauses, fmt.Sprintf("workspace_id = $%d", nextArg))
		args = append(args, workspaceID)
		nextArg++
	}
	if status := strings.ToLower(strings.TrimSpace(filter.Status)); status != "" {
		clauses = append(clauses, fmt.Sprintf("status = $%d", nextArg))
		args = append(args, status)
		nextArg++
	}
	if email := strings.ToLower(strings.TrimSpace(filter.Email)); email != "" {
		clauses = append(clauses, fmt.Sprintf("lower(email) = lower($%d)", nextArg))
		args = append(args, email)
		nextArg++
	}
	query := fmt.Sprintf(`SELECT id, workspace_id, email, role, status, token_hash, expires_at, accepted_at, accepted_by_user_id,
		invited_by_user_id, invited_by_platform_user, revoked_at, created_at, updated_at
		FROM workspace_invitations
		WHERE %s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d OFFSET $%d`, strings.Join(clauses, " AND "), nextArg, nextArg+1)
	args = append(args, limit, offset)

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.WorkspaceInvitation, 0)
	for rows.Next() {
		item, scanErr := scanWorkspaceInvitation(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PostgresStore) UpdateWorkspaceInvitation(input domain.WorkspaceInvitation) (domain.WorkspaceInvitation, error) {
	input.ID = strings.TrimSpace(input.ID)
	input.WorkspaceID = normalizeTenantID(input.WorkspaceID)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Role = strings.ToLower(strings.TrimSpace(input.Role))
	input.Status = domain.WorkspaceInvitationStatus(strings.ToLower(strings.TrimSpace(string(input.Status))))
	input.TokenHash = strings.TrimSpace(input.TokenHash)
	input.AcceptedByUserID = strings.TrimSpace(input.AcceptedByUserID)
	input.InvitedByUserID = strings.TrimSpace(input.InvitedByUserID)
	if input.ID == "" || input.WorkspaceID == "" || input.Email == "" || input.Role == "" || input.Status == "" || input.TokenHash == "" || input.ExpiresAt.IsZero() {
		return domain.WorkspaceInvitation{}, fmt.Errorf("validation failed: id, workspace_id, email, role, status, token_hash, and expires_at are required")
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = time.Now().UTC()
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.WorkspaceInvitation{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `UPDATE workspace_invitations
		SET workspace_id = $1,
		    email = $2,
		    role = $3,
		    status = $4,
		    token_hash = $5,
		    expires_at = $6,
		    accepted_at = $7,
		    accepted_by_user_id = NULLIF($8,''),
		    invited_by_user_id = NULLIF($9,''),
		    invited_by_platform_user = $10,
		    revoked_at = $11,
		    updated_at = $12
		WHERE id = $13
		RETURNING id, workspace_id, email, role, status, token_hash, expires_at, accepted_at, accepted_by_user_id,
			invited_by_user_id, invited_by_platform_user, revoked_at, created_at, updated_at`,
		input.WorkspaceID,
		input.Email,
		input.Role,
		string(input.Status),
		input.TokenHash,
		input.ExpiresAt,
		input.AcceptedAt,
		input.AcceptedByUserID,
		input.InvitedByUserID,
		input.InvitedByPlatformUser,
		input.RevokedAt,
		input.UpdatedAt,
		input.ID,
	)
	out, err := scanWorkspaceInvitation(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.WorkspaceInvitation{}, ErrNotFound
		}
		if isUniqueViolation(err) {
			return domain.WorkspaceInvitation{}, ErrAlreadyExists
		}
		return domain.WorkspaceInvitation{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.WorkspaceInvitation{}, err
	}
	return out, nil
}

func (s *PostgresStore) GetUserFederatedIdentity(providerKey, subject string) (domain.UserFederatedIdentity, error) {
	providerKey = strings.ToLower(strings.TrimSpace(providerKey))
	subject = strings.TrimSpace(subject)
	if providerKey == "" || subject == "" {
		return domain.UserFederatedIdentity{}, ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.UserFederatedIdentity{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `SELECT id, user_id, provider_key, provider_type, subject, email, email_verified, last_login_at, created_at, updated_at
		FROM user_federated_identities
		WHERE provider_key = $1 AND subject = $2`,
		providerKey, subject)
	out, err := scanUserFederatedIdentity(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.UserFederatedIdentity{}, ErrNotFound
		}
		return domain.UserFederatedIdentity{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.UserFederatedIdentity{}, err
	}
	return out, nil
}

func (s *PostgresStore) UpsertUserFederatedIdentity(input domain.UserFederatedIdentity) (domain.UserFederatedIdentity, error) {
	input.ID = strings.TrimSpace(input.ID)
	input.UserID = strings.TrimSpace(input.UserID)
	input.ProviderKey = strings.ToLower(strings.TrimSpace(input.ProviderKey))
	input.ProviderType = domain.BrowserSSOProviderType(strings.ToLower(strings.TrimSpace(string(input.ProviderType))))
	input.Subject = strings.TrimSpace(input.Subject)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if input.UserID == "" || input.ProviderKey == "" || input.ProviderType == "" || input.Subject == "" {
		return domain.UserFederatedIdentity{}, fmt.Errorf("validation failed: user_id, provider_key, provider_type, and subject are required")
	}
	if input.ID == "" {
		input.ID = newID("ufi")
	}
	now := time.Now().UTC()
	if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = now
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.UserFederatedIdentity{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `INSERT INTO user_federated_identities (
			id, user_id, provider_key, provider_type, subject, email, email_verified, last_login_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (provider_key, subject) DO UPDATE
		SET user_id = EXCLUDED.user_id,
		    provider_type = EXCLUDED.provider_type,
		    email = EXCLUDED.email,
		    email_verified = EXCLUDED.email_verified,
		    last_login_at = EXCLUDED.last_login_at,
		    updated_at = EXCLUDED.updated_at
		RETURNING id, user_id, provider_key, provider_type, subject, email, email_verified, last_login_at, created_at, updated_at`,
		input.ID, input.UserID, input.ProviderKey, string(input.ProviderType), input.Subject, input.Email, input.EmailVerified, input.LastLoginAt, input.CreatedAt, input.UpdatedAt)
	out, err := scanUserFederatedIdentity(row)
	if err != nil {
		return domain.UserFederatedIdentity{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.UserFederatedIdentity{}, err
	}
	return out, nil
}

func (s *PostgresStore) CreateCustomer(input domain.Customer) (domain.Customer, error) {
	input.TenantID = normalizeTenantID(input.TenantID)
	input.ExternalID = strings.TrimSpace(input.ExternalID)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Email = normalizeOptionalText(input.Email)
	input.Status = normalizeCustomerStatus(input.Status)
	input.LagoCustomerID = normalizeOptionalText(input.LagoCustomerID)
	if input.ExternalID == "" {
		return domain.Customer{}, fmt.Errorf("validation failed: external_id is required")
	}
	if input.DisplayName == "" {
		input.DisplayName = input.ExternalID
	}
	if input.ID == "" {
		input.ID = newID("cust")
	}
	now := time.Now().UTC()
	if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = now
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, input.TenantID)
	if err != nil {
		return domain.Customer{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `INSERT INTO customers (id, tenant_id, external_id, display_name, email, status, lago_customer_id, created_at, updated_at)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,NULLIF($7,''),$8,$9)
		RETURNING id, tenant_id, external_id, display_name, email, status, lago_customer_id, created_at, updated_at`,
		input.ID, input.TenantID, input.ExternalID, input.DisplayName, input.Email, string(input.Status), input.LagoCustomerID, input.CreatedAt, input.UpdatedAt)
	out, err := scanCustomer(row)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Customer{}, ErrAlreadyExists
		}
		return domain.Customer{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Customer{}, err
	}
	return out, nil
}

func (s *PostgresStore) GetCustomer(tenantID, id string) (domain.Customer, error) {
	tenantID = normalizeTenantID(tenantID)
	id = strings.TrimSpace(id)

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.Customer{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `SELECT id, tenant_id, external_id, display_name, email, status, lago_customer_id, created_at, updated_at FROM customers WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	out, err := scanCustomer(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Customer{}, ErrNotFound
		}
		return domain.Customer{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Customer{}, err
	}
	return out, nil
}

func (s *PostgresStore) GetCustomerByExternalID(tenantID, externalID string) (domain.Customer, error) {
	tenantID = normalizeTenantID(tenantID)
	externalID = strings.TrimSpace(externalID)

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.Customer{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `SELECT id, tenant_id, external_id, display_name, email, status, lago_customer_id, created_at, updated_at FROM customers WHERE tenant_id = $1 AND external_id = $2`, tenantID, externalID)
	out, err := scanCustomer(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Customer{}, ErrNotFound
		}
		return domain.Customer{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Customer{}, err
	}
	return out, nil
}

func (s *PostgresStore) ListCustomers(filter CustomerListFilter) ([]domain.Customer, error) {
	filter.TenantID = normalizeTenantID(filter.TenantID)
	filter.Status = strings.TrimSpace(strings.ToLower(filter.Status))
	filter.ExternalID = strings.TrimSpace(filter.ExternalID)
	if filter.Limit <= 0 {
		filter.Limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, filter.TenantID)
	if err != nil {
		return nil, err
	}
	defer rollbackSilently(tx)

	clauses := []string{"tenant_id = $1"}
	args := []any{filter.TenantID}
	if filter.Status != "" {
		args = append(args, filter.Status)
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	if filter.ExternalID != "" {
		args = append(args, filter.ExternalID)
		clauses = append(clauses, fmt.Sprintf("external_id = $%d", len(args)))
	}
	args = append(args, filter.Limit, filter.Offset)
	query := `SELECT id, tenant_id, external_id, display_name, email, status, lago_customer_id, created_at, updated_at FROM customers WHERE ` + strings.Join(clauses, " AND ") + fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.Customer, 0)
	for rows.Next() {
		item, err := scanCustomer(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *PostgresStore) UpdateCustomer(input domain.Customer) (domain.Customer, error) {
	input.TenantID = normalizeTenantID(input.TenantID)
	input.ID = strings.TrimSpace(input.ID)
	input.ExternalID = strings.TrimSpace(input.ExternalID)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Email = normalizeOptionalText(input.Email)
	input.Status = normalizeCustomerStatus(input.Status)
	input.LagoCustomerID = normalizeOptionalText(input.LagoCustomerID)
	if input.ID == "" {
		return domain.Customer{}, fmt.Errorf("validation failed: customer id is required")
	}
	if input.ExternalID == "" {
		return domain.Customer{}, fmt.Errorf("validation failed: external_id is required")
	}
	if input.DisplayName == "" {
		input.DisplayName = input.ExternalID
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = time.Now().UTC()
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, input.TenantID)
	if err != nil {
		return domain.Customer{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `UPDATE customers SET external_id = $1, display_name = $2, email = NULLIF($3,''), status = $4, lago_customer_id = NULLIF($5,''), updated_at = $6 WHERE tenant_id = $7 AND id = $8 RETURNING id, tenant_id, external_id, display_name, email, status, lago_customer_id, created_at, updated_at`,
		input.ExternalID, input.DisplayName, input.Email, string(input.Status), input.LagoCustomerID, input.UpdatedAt, input.TenantID, input.ID)
	out, err := scanCustomer(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Customer{}, ErrNotFound
		}
		if isUniqueViolation(err) {
			return domain.Customer{}, ErrAlreadyExists
		}
		return domain.Customer{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Customer{}, err
	}
	return out, nil
}

func (s *PostgresStore) UpsertCustomerBillingProfile(input domain.CustomerBillingProfile) (domain.CustomerBillingProfile, error) {
	input.CustomerID = strings.TrimSpace(input.CustomerID)
	input.TenantID = normalizeTenantID(input.TenantID)
	input.LegalName = normalizeOptionalText(input.LegalName)
	input.Email = normalizeOptionalText(input.Email)
	input.Phone = normalizeOptionalText(input.Phone)
	input.AddressLine1 = normalizeOptionalText(input.AddressLine1)
	input.AddressLine2 = normalizeOptionalText(input.AddressLine2)
	input.City = normalizeOptionalText(input.City)
	input.State = normalizeOptionalText(input.State)
	input.PostalCode = normalizeOptionalText(input.PostalCode)
	input.Country = normalizeOptionalText(input.Country)
	input.Currency = normalizeOptionalText(input.Currency)
	input.TaxIdentifier = normalizeOptionalText(input.TaxIdentifier)
	input.ProviderCode = normalizeOptionalText(input.ProviderCode)
	input.ProfileStatus = normalizeBillingProfileStatus(input.ProfileStatus)
	input.LastSyncError = normalizeOptionalText(input.LastSyncError)
	if input.CustomerID == "" {
		return domain.CustomerBillingProfile{}, fmt.Errorf("validation failed: customer_id is required")
	}
	now := time.Now().UTC()
	if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = now
	}

	ctx, cancel := s.withTimeout()
	defer cancel()
	tx, err := s.beginTxWithSession(ctx, txSessionTenant, input.TenantID)
	if err != nil {
		return domain.CustomerBillingProfile{}, err
	}
	defer rollbackSilently(tx)
	row := tx.QueryRowContext(ctx, `INSERT INTO customer_billing_profiles (customer_id, tenant_id, legal_name, email, phone, billing_address_line1, billing_address_line2, billing_city, billing_state, billing_postal_code, billing_country, currency, tax_identifier, provider_code, profile_status, last_synced_at, last_sync_error, created_at, updated_at)
	VALUES ($1,$2,NULLIF($3,''),NULLIF($4,''),NULLIF($5,''),NULLIF($6,''),NULLIF($7,''),NULLIF($8,''),NULLIF($9,''),NULLIF($10,''),NULLIF($11,''),NULLIF($12,''),NULLIF($13,''),NULLIF($14,''),$15,$16,NULLIF($17,''),$18,$19)
	ON CONFLICT (customer_id) DO UPDATE SET
	 legal_name = EXCLUDED.legal_name,
	 email = EXCLUDED.email,
	 phone = EXCLUDED.phone,
	 billing_address_line1 = EXCLUDED.billing_address_line1,
	 billing_address_line2 = EXCLUDED.billing_address_line2,
	 billing_city = EXCLUDED.billing_city,
	 billing_state = EXCLUDED.billing_state,
	 billing_postal_code = EXCLUDED.billing_postal_code,
	 billing_country = EXCLUDED.billing_country,
	 currency = EXCLUDED.currency,
	 tax_identifier = EXCLUDED.tax_identifier,
	 provider_code = EXCLUDED.provider_code,
	 profile_status = EXCLUDED.profile_status,
	 last_synced_at = EXCLUDED.last_synced_at,
	 last_sync_error = EXCLUDED.last_sync_error,
	 updated_at = EXCLUDED.updated_at
	RETURNING customer_id, tenant_id, legal_name, email, phone, billing_address_line1, billing_address_line2, billing_city, billing_state, billing_postal_code, billing_country, currency, tax_identifier, provider_code, profile_status, last_synced_at, last_sync_error, created_at, updated_at`,
		input.CustomerID, input.TenantID, input.LegalName, input.Email, input.Phone, input.AddressLine1, input.AddressLine2, input.City, input.State, input.PostalCode, input.Country, input.Currency, input.TaxIdentifier, input.ProviderCode, string(input.ProfileStatus), input.LastSyncedAt, input.LastSyncError, input.CreatedAt, input.UpdatedAt)
	out, err := scanCustomerBillingProfile(row)
	if err != nil {
		if isForeignKeyViolation(err) {
			return domain.CustomerBillingProfile{}, ErrNotFound
		}
		return domain.CustomerBillingProfile{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.CustomerBillingProfile{}, err
	}
	return out, nil
}

func (s *PostgresStore) GetCustomerBillingProfile(tenantID, customerID string) (domain.CustomerBillingProfile, error) {
	tenantID = normalizeTenantID(tenantID)
	customerID = strings.TrimSpace(customerID)
	ctx, cancel := s.withTimeout()
	defer cancel()
	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.CustomerBillingProfile{}, err
	}
	defer rollbackSilently(tx)
	row := tx.QueryRowContext(ctx, `SELECT customer_id, tenant_id, legal_name, email, phone, billing_address_line1, billing_address_line2, billing_city, billing_state, billing_postal_code, billing_country, currency, tax_identifier, provider_code, profile_status, last_synced_at, last_sync_error, created_at, updated_at FROM customer_billing_profiles WHERE tenant_id = $1 AND customer_id = $2`, tenantID, customerID)
	out, err := scanCustomerBillingProfile(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.CustomerBillingProfile{}, ErrNotFound
		}
		return domain.CustomerBillingProfile{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.CustomerBillingProfile{}, err
	}
	return out, nil
}

func (s *PostgresStore) UpsertCustomerPaymentSetup(input domain.CustomerPaymentSetup) (domain.CustomerPaymentSetup, error) {
	input.CustomerID = strings.TrimSpace(input.CustomerID)
	input.TenantID = normalizeTenantID(input.TenantID)
	input.SetupStatus = normalizePaymentSetupStatus(input.SetupStatus)
	input.PaymentMethodType = normalizeOptionalText(input.PaymentMethodType)
	input.ProviderCustomerReference = normalizeOptionalText(input.ProviderCustomerReference)
	input.ProviderPaymentMethodReference = normalizeOptionalText(input.ProviderPaymentMethodReference)
	input.LastVerificationResult = normalizeOptionalText(input.LastVerificationResult)
	input.LastVerificationError = normalizeOptionalText(input.LastVerificationError)
	if input.CustomerID == "" {
		return domain.CustomerPaymentSetup{}, fmt.Errorf("validation failed: customer_id is required")
	}
	now := time.Now().UTC()
	if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = now
	}
	ctx, cancel := s.withTimeout()
	defer cancel()
	tx, err := s.beginTxWithSession(ctx, txSessionTenant, input.TenantID)
	if err != nil {
		return domain.CustomerPaymentSetup{}, err
	}
	defer rollbackSilently(tx)
	row := tx.QueryRowContext(ctx, `INSERT INTO customer_payment_setup (customer_id, tenant_id, setup_status, default_payment_method_present, payment_method_type, provider_customer_reference, provider_payment_method_reference, last_verified_at, last_verification_result, last_verification_error, created_at, updated_at)
	VALUES ($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),NULLIF($7,''),$8,NULLIF($9,''),NULLIF($10,''),$11,$12)
	ON CONFLICT (customer_id) DO UPDATE SET
	 setup_status = EXCLUDED.setup_status,
	 default_payment_method_present = EXCLUDED.default_payment_method_present,
	 payment_method_type = EXCLUDED.payment_method_type,
	 provider_customer_reference = EXCLUDED.provider_customer_reference,
	 provider_payment_method_reference = EXCLUDED.provider_payment_method_reference,
	 last_verified_at = EXCLUDED.last_verified_at,
	 last_verification_result = EXCLUDED.last_verification_result,
	 last_verification_error = EXCLUDED.last_verification_error,
	 updated_at = EXCLUDED.updated_at
	RETURNING customer_id, tenant_id, setup_status, default_payment_method_present, payment_method_type, provider_customer_reference, provider_payment_method_reference, last_verified_at, last_verification_result, last_verification_error, created_at, updated_at`,
		input.CustomerID, input.TenantID, string(input.SetupStatus), input.DefaultPaymentMethodPresent, input.PaymentMethodType, input.ProviderCustomerReference, input.ProviderPaymentMethodReference, input.LastVerifiedAt, input.LastVerificationResult, input.LastVerificationError, input.CreatedAt, input.UpdatedAt)
	out, err := scanCustomerPaymentSetup(row)
	if err != nil {
		if isForeignKeyViolation(err) {
			return domain.CustomerPaymentSetup{}, ErrNotFound
		}
		return domain.CustomerPaymentSetup{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.CustomerPaymentSetup{}, err
	}
	return out, nil
}

func (s *PostgresStore) GetCustomerPaymentSetup(tenantID, customerID string) (domain.CustomerPaymentSetup, error) {
	tenantID = normalizeTenantID(tenantID)
	customerID = strings.TrimSpace(customerID)
	ctx, cancel := s.withTimeout()
	defer cancel()
	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.CustomerPaymentSetup{}, err
	}
	defer rollbackSilently(tx)
	row := tx.QueryRowContext(ctx, `SELECT customer_id, tenant_id, setup_status, default_payment_method_present, payment_method_type, provider_customer_reference, provider_payment_method_reference, last_verified_at, last_verification_result, last_verification_error, created_at, updated_at FROM customer_payment_setup WHERE tenant_id = $1 AND customer_id = $2`, tenantID, customerID)
	out, err := scanCustomerPaymentSetup(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.CustomerPaymentSetup{}, ErrNotFound
		}
		return domain.CustomerPaymentSetup{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.CustomerPaymentSetup{}, err
	}
	return out, nil
}

func (s *PostgresStore) CreateRatingRuleVersion(input domain.RatingRuleVersion) (domain.RatingRuleVersion, error) {
	if input.ID == "" {
		input.ID = newID("rrv")
	}
	if input.CreatedAt.IsZero() {
		input.CreatedAt = time.Now().UTC()
	}
	if input.GraduatedTiers == nil {
		input.GraduatedTiers = []domain.RatingTier{}
	}
	input.TenantID = normalizeTenantID(input.TenantID)
	input.RuleKey = strings.ToLower(strings.TrimSpace(input.RuleKey))
	input.LifecycleState = normalizeRatingRuleLifecycleState(input.LifecycleState)

	tiers, err := json.Marshal(input.GraduatedTiers)
	if err != nil {
		return domain.RatingRuleVersion{}, err
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, input.TenantID)
	if err != nil {
		return domain.RatingRuleVersion{}, err
	}
	defer rollbackSilently(tx)

	var maxVersion int
	if err := tx.QueryRowContext(
		ctx,
		`SELECT COALESCE(MAX(version), 0) FROM rating_rule_versions WHERE tenant_id = $1 AND rule_key = $2`,
		input.TenantID,
		input.RuleKey,
	).Scan(&maxVersion); err != nil {
		return domain.RatingRuleVersion{}, err
	}
	if maxVersion > 0 && input.Version <= maxVersion {
		return domain.RatingRuleVersion{}, ErrDuplicateKey
	}

	if input.LifecycleState == domain.RatingRuleLifecycleActive {
		if _, err := tx.ExecContext(
			ctx,
			`UPDATE rating_rule_versions
			SET lifecycle_state = $1
			WHERE tenant_id = $2 AND rule_key = $3 AND lifecycle_state = $4`,
			string(domain.RatingRuleLifecycleArchived),
			input.TenantID,
			input.RuleKey,
			string(domain.RatingRuleLifecycleActive),
		); err != nil {
			return domain.RatingRuleVersion{}, err
		}
	}

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO rating_rule_versions (
			id, tenant_id, rule_key, name, version, lifecycle_state, mode, currency, flat_amount_cents, graduated_tiers,
			package_size, package_amount_cents, overage_unit_amount_cents, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb,$11,$12,$13,$14)`,
		input.ID,
		input.TenantID,
		input.RuleKey,
		input.Name,
		input.Version,
		string(input.LifecycleState),
		string(input.Mode),
		input.Currency,
		input.FlatAmountCents,
		string(tiers),
		input.PackageSize,
		input.PackageAmountCents,
		input.OverageUnitAmountCents,
		input.CreatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.RatingRuleVersion{}, ErrDuplicateKey
		}
		return domain.RatingRuleVersion{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.RatingRuleVersion{}, err
	}

	return input, nil
}

func (s *PostgresStore) ListRatingRuleVersions(filter RatingRuleListFilter) ([]domain.RatingRuleVersion, error) {
	tenantID := normalizeTenantID(filter.TenantID)
	ruleKey := strings.ToLower(strings.TrimSpace(filter.RuleKey))
	lifecycleState := strings.ToLower(strings.TrimSpace(filter.LifecycleState))

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return nil, err
	}
	defer rollbackSilently(tx)

	clauses := []string{"tenant_id = $1"}
	args := []any{tenantID}
	nextArg := 2
	if ruleKey != "" {
		clauses = append(clauses, fmt.Sprintf("rule_key = $%d", nextArg))
		args = append(args, ruleKey)
		nextArg++
	}
	if lifecycleState != "" {
		clauses = append(clauses, fmt.Sprintf("lifecycle_state = $%d", nextArg))
		args = append(args, lifecycleState)
		nextArg++
	}
	where := strings.Join(clauses, " AND ")

	query := `SELECT id, tenant_id, rule_key, name, version, lifecycle_state, mode, currency, flat_amount_cents, graduated_tiers, package_size, package_amount_cents, overage_unit_amount_cents, created_at FROM rating_rule_versions WHERE ` + where + ` ORDER BY rule_key ASC, version ASC, created_at ASC, id ASC`
	if filter.LatestOnly {
		query = `SELECT DISTINCT ON (rule_key) id, tenant_id, rule_key, name, version, lifecycle_state, mode, currency, flat_amount_cents, graduated_tiers, package_size, package_amount_cents, overage_unit_amount_cents, created_at FROM rating_rule_versions WHERE ` + where + ` ORDER BY rule_key ASC, version DESC, created_at DESC, id DESC`
	}

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.RatingRuleVersion, 0)
	for rows.Next() {
		rule, scanErr := scanRatingRule(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PostgresStore) GetRatingRuleVersion(tenantID, id string) (domain.RatingRuleVersion, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.RatingRuleVersion{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `SELECT id, tenant_id, rule_key, name, version, lifecycle_state, mode, currency, flat_amount_cents, graduated_tiers, package_size, package_amount_cents, overage_unit_amount_cents, created_at FROM rating_rule_versions WHERE tenant_id = $1 AND id = $2`, normalizeTenantID(tenantID), id)
	rule, err := scanRatingRule(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.RatingRuleVersion{}, ErrNotFound
		}
		return domain.RatingRuleVersion{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.RatingRuleVersion{}, err
	}
	return rule, nil
}

func (s *PostgresStore) CreateMeter(input domain.Meter) (domain.Meter, error) {
	if input.ID == "" {
		input.ID = newID("mtr")
	}
	now := time.Now().UTC()
	if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}
	input.TenantID = normalizeTenantID(input.TenantID)
	input.UpdatedAt = now

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, input.TenantID)
	if err != nil {
		return domain.Meter{}, err
	}
	defer rollbackSilently(tx)

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO meters (id, tenant_id, meter_key, name, unit, aggregation, rating_rule_version_id, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		input.ID,
		input.TenantID,
		input.Key,
		input.Name,
		input.Unit,
		input.Aggregation,
		input.RatingRuleVersionID,
		input.CreatedAt,
		input.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Meter{}, ErrDuplicateKey
		}
		return domain.Meter{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Meter{}, err
	}

	return input, nil
}

func (s *PostgresStore) ListMeters(tenantID string) ([]domain.Meter, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return nil, err
	}
	defer rollbackSilently(tx)

	rows, err := tx.QueryContext(ctx, `SELECT id, tenant_id, meter_key, name, unit, aggregation, rating_rule_version_id, created_at, updated_at FROM meters WHERE tenant_id = $1 ORDER BY created_at ASC, id ASC`, normalizeTenantID(tenantID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.Meter, 0)
	for rows.Next() {
		meter, scanErr := scanMeter(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, meter)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PostgresStore) GetMeter(tenantID, id string) (domain.Meter, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.Meter{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `SELECT id, tenant_id, meter_key, name, unit, aggregation, rating_rule_version_id, created_at, updated_at FROM meters WHERE tenant_id = $1 AND id = $2`, normalizeTenantID(tenantID), id)
	meter, err := scanMeter(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Meter{}, ErrNotFound
		}
		return domain.Meter{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Meter{}, err
	}
	return meter, nil
}

func (s *PostgresStore) CreatePlan(input domain.Plan) (domain.Plan, error) {
	if input.ID == "" {
		input.ID = newID("pln")
	}
	now := time.Now().UTC()
	if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}
	input.TenantID = normalizeTenantID(input.TenantID)
	input.UpdatedAt = now

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, input.TenantID)
	if err != nil {
		return domain.Plan{}, err
	}
	defer rollbackSilently(tx)

	_, err = tx.ExecContext(ctx, `INSERT INTO plans (id, tenant_id, plan_code, name, description, currency, billing_interval, status, base_amount_cents, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		input.ID, input.TenantID, input.Code, input.Name, input.Description, input.Currency, input.BillingInterval, input.Status, input.BaseAmountCents, input.CreatedAt, input.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Plan{}, ErrDuplicateKey
		}
		return domain.Plan{}, err
	}
	for idx, meterID := range input.MeterIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO plan_metrics (tenant_id, plan_id, meter_id, position, created_at) VALUES ($1,$2,$3,$4,$5)`, input.TenantID, input.ID, meterID, idx, now); err != nil {
			if isUniqueViolation(err) {
				return domain.Plan{}, ErrDuplicateKey
			}
			return domain.Plan{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return domain.Plan{}, err
	}
	return input, nil
}

func (s *PostgresStore) ListPlans(tenantID string) ([]domain.Plan, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return nil, err
	}
	defer rollbackSilently(tx)

	rows, err := tx.QueryContext(ctx, `SELECT id, tenant_id, plan_code, name, description, currency, billing_interval, status, base_amount_cents, created_at, updated_at FROM plans WHERE tenant_id = $1 ORDER BY created_at ASC, id ASC`, normalizeTenantID(tenantID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.Plan, 0)
	ids := make([]string, 0)
	for rows.Next() {
		plan, scanErr := scanPlan(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, plan)
		ids = append(ids, plan.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	metricIDsByPlan, err := loadPlanMetricIDs(ctx, tx, normalizeTenantID(tenantID), ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].MeterIDs = metricIDsByPlan[out[i].ID]
		if out[i].MeterIDs == nil {
			out[i].MeterIDs = []string{}
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PostgresStore) GetPlan(tenantID, id string) (domain.Plan, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.Plan{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `SELECT id, tenant_id, plan_code, name, description, currency, billing_interval, status, base_amount_cents, created_at, updated_at FROM plans WHERE tenant_id = $1 AND id = $2`, normalizeTenantID(tenantID), id)
	plan, err := scanPlan(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Plan{}, ErrNotFound
		}
		return domain.Plan{}, err
	}
	metricIDsByPlan, err := loadPlanMetricIDs(ctx, tx, normalizeTenantID(tenantID), []string{id})
	if err != nil {
		return domain.Plan{}, err
	}
	plan.MeterIDs = metricIDsByPlan[id]
	if plan.MeterIDs == nil {
		plan.MeterIDs = []string{}
	}
	if err := tx.Commit(); err != nil {
		return domain.Plan{}, err
	}
	return plan, nil
}

func (s *PostgresStore) CreateSubscription(input domain.Subscription) (domain.Subscription, error) {
	if input.ID == "" {
		input.ID = newID("sub")
	}
	now := time.Now().UTC()
	if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}
	input.TenantID = normalizeTenantID(input.TenantID)
	input.UpdatedAt = now

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, input.TenantID)
	if err != nil {
		return domain.Subscription{}, err
	}
	defer rollbackSilently(tx)

	_, err = tx.ExecContext(ctx, `INSERT INTO subscriptions (id, tenant_id, subscription_code, display_name, customer_id, plan_id, status, payment_setup_requested_at, activated_at, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		input.ID, input.TenantID, input.Code, input.DisplayName, input.CustomerID, input.PlanID, input.Status, input.PaymentSetupRequestedAt, input.ActivatedAt, input.CreatedAt, input.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Subscription{}, ErrDuplicateKey
		}
		return domain.Subscription{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Subscription{}, err
	}
	return input, nil
}

func (s *PostgresStore) ListSubscriptions(tenantID string) ([]domain.Subscription, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return nil, err
	}
	defer rollbackSilently(tx)

	rows, err := tx.QueryContext(ctx, `SELECT id, tenant_id, subscription_code, display_name, customer_id, plan_id, status, payment_setup_requested_at, activated_at, created_at, updated_at FROM subscriptions WHERE tenant_id = $1 ORDER BY created_at DESC, id DESC`, normalizeTenantID(tenantID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.Subscription, 0)
	for rows.Next() {
		subscription, scanErr := scanSubscription(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, subscription)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PostgresStore) GetSubscription(tenantID, id string) (domain.Subscription, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.Subscription{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `SELECT id, tenant_id, subscription_code, display_name, customer_id, plan_id, status, payment_setup_requested_at, activated_at, created_at, updated_at FROM subscriptions WHERE tenant_id = $1 AND id = $2`, normalizeTenantID(tenantID), strings.TrimSpace(id))
	subscription, err := scanSubscription(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Subscription{}, ErrNotFound
		}
		return domain.Subscription{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Subscription{}, err
	}
	return subscription, nil
}

func (s *PostgresStore) UpdateSubscription(input domain.Subscription) (domain.Subscription, error) {
	input.TenantID = normalizeTenantID(input.TenantID)
	input.UpdatedAt = time.Now().UTC()

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, input.TenantID)
	if err != nil {
		return domain.Subscription{}, err
	}
	defer rollbackSilently(tx)

	result, err := tx.ExecContext(ctx, `UPDATE subscriptions SET subscription_code = $3, display_name = $4, customer_id = $5, plan_id = $6, status = $7, payment_setup_requested_at = $8, activated_at = $9, updated_at = $10 WHERE tenant_id = $1 AND id = $2`,
		input.TenantID, input.ID, input.Code, input.DisplayName, input.CustomerID, input.PlanID, input.Status, input.PaymentSetupRequestedAt, input.ActivatedAt, input.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Subscription{}, ErrDuplicateKey
		}
		return domain.Subscription{}, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return domain.Subscription{}, err
	}
	if rowsAffected == 0 {
		return domain.Subscription{}, ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return domain.Subscription{}, err
	}
	return input, nil
}

func loadPlanMetricIDs(ctx context.Context, tx *sql.Tx, tenantID string, planIDs []string) (map[string][]string, error) {
	out := make(map[string][]string, len(planIDs))
	if len(planIDs) == 0 {
		return out, nil
	}
	rows, err := tx.QueryContext(ctx, `SELECT plan_id, meter_id FROM plan_metrics WHERE tenant_id = $1 AND plan_id = ANY($2) ORDER BY position ASC, meter_id ASC`, tenantID, pq.Array(planIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var planID string
		var meterID string
		if err := rows.Scan(&planID, &meterID); err != nil {
			return nil, err
		}
		out[planID] = append(out[planID], meterID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PostgresStore) UpdateMeter(input domain.Meter) (domain.Meter, error) {
	input.UpdatedAt = time.Now().UTC()

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, input.TenantID)
	if err != nil {
		return domain.Meter{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`UPDATE meters SET meter_key = $1, name = $2, unit = $3, aggregation = $4, rating_rule_version_id = $5, updated_at = $6 WHERE tenant_id = $7 AND id = $8 RETURNING id, tenant_id, meter_key, name, unit, aggregation, rating_rule_version_id, created_at, updated_at`,
		input.Key,
		input.Name,
		input.Unit,
		input.Aggregation,
		input.RatingRuleVersionID,
		input.UpdatedAt,
		normalizeTenantID(input.TenantID),
		input.ID,
	)

	meter, err := scanMeter(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Meter{}, ErrNotFound
		}
		if isUniqueViolation(err) {
			return domain.Meter{}, ErrDuplicateKey
		}
		return domain.Meter{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Meter{}, err
	}

	return meter, nil
}

func (s *PostgresStore) CreateUsageEvent(input domain.UsageEvent) (domain.UsageEvent, error) {
	if input.ID == "" {
		input.ID = newID("evt")
	}
	if input.Timestamp.IsZero() {
		input.Timestamp = time.Now().UTC()
	}
	input.TenantID = normalizeTenantID(input.TenantID)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, input.TenantID)
	if err != nil {
		return domain.UsageEvent{}, err
	}
	defer rollbackSilently(tx)

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO usage_events (id, tenant_id, customer_id, meter_id, quantity, idempotency_key, occurred_at) VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),$7)`,
		input.ID,
		input.TenantID,
		input.CustomerID,
		input.MeterID,
		input.Quantity,
		input.IdempotencyKey,
		input.Timestamp,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.UsageEvent{}, ErrAlreadyExists
		}
		return domain.UsageEvent{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.UsageEvent{}, err
	}
	return input, nil
}

func (s *PostgresStore) GetUsageEventByIdempotencyKey(tenantID, idempotencyKey string) (domain.UsageEvent, error) {
	tenantID = normalizeTenantID(tenantID)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return domain.UsageEvent{}, ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.UsageEvent{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`SELECT id, tenant_id, customer_id, meter_id, quantity, idempotency_key, occurred_at
		FROM usage_events
		WHERE tenant_id = $1 AND idempotency_key = $2
		ORDER BY occurred_at DESC, id DESC
		LIMIT 1`,
		tenantID,
		idempotencyKey,
	)
	var event domain.UsageEvent
	var eventIdempotencyKey sql.NullString
	if err := row.Scan(
		&event.ID,
		&event.TenantID,
		&event.CustomerID,
		&event.MeterID,
		&event.Quantity,
		&eventIdempotencyKey,
		&event.Timestamp,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.UsageEvent{}, ErrNotFound
		}
		return domain.UsageEvent{}, err
	}
	event.TenantID = normalizeTenantID(event.TenantID)
	if eventIdempotencyKey.Valid {
		event.IdempotencyKey = strings.TrimSpace(eventIdempotencyKey.String)
	}

	if err := tx.Commit(); err != nil {
		return domain.UsageEvent{}, err
	}
	return event, nil
}

func (s *PostgresStore) ListUsageEvents(filter Filter) ([]domain.UsageEvent, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, filter.TenantID)
	if err != nil {
		return nil, err
	}
	defer rollbackSilently(tx)

	query, args := buildUsageEventsFilteredQuery(`SELECT id, tenant_id, customer_id, meter_id, quantity, idempotency_key, occurred_at FROM usage_events`, filter, "occurred_at")
	if filter.SortDesc {
		query += ` ORDER BY occurred_at DESC, id DESC`
	} else {
		query += ` ORDER BY occurred_at ASC, id ASC`
	}
	if filter.Limit > 0 {
		args = append(args, filter.Limit)
		query += fmt.Sprintf(" LIMIT $%d", len(args))
	}
	if filter.Offset > 0 {
		args = append(args, filter.Offset)
		query += fmt.Sprintf(" OFFSET $%d", len(args))
	}

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.UsageEvent, 0)
	for rows.Next() {
		var event domain.UsageEvent
		var idempotencyKey sql.NullString
		if scanErr := rows.Scan(&event.ID, &event.TenantID, &event.CustomerID, &event.MeterID, &event.Quantity, &idempotencyKey, &event.Timestamp); scanErr != nil {
			return nil, scanErr
		}
		event.TenantID = normalizeTenantID(event.TenantID)
		if idempotencyKey.Valid {
			event.IdempotencyKey = strings.TrimSpace(idempotencyKey.String)
		}
		out = append(out, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PostgresStore) CreateBilledEntry(input domain.BilledEntry) (domain.BilledEntry, error) {
	if input.ID == "" {
		input.ID = newID("bil")
	}
	if input.Timestamp.IsZero() {
		input.Timestamp = time.Now().UTC()
	}
	input.TenantID = normalizeTenantID(input.TenantID)
	input.Source = normalizeBilledEntrySource(input.Source)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	input.ReplayJobID = strings.TrimSpace(input.ReplayJobID)

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, input.TenantID)
	if err != nil {
		return domain.BilledEntry{}, err
	}
	defer rollbackSilently(tx)

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO billed_entries (id, tenant_id, customer_id, meter_id, amount_cents, idempotency_key, source, replay_job_id, occurred_at) VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),$7,$8,$9)`,
		input.ID,
		input.TenantID,
		input.CustomerID,
		input.MeterID,
		input.AmountCents,
		input.IdempotencyKey,
		string(input.Source),
		nullableString(input.ReplayJobID),
		input.Timestamp,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.BilledEntry{}, ErrAlreadyExists
		}
		return domain.BilledEntry{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.BilledEntry{}, err
	}
	return input, nil
}

func (s *PostgresStore) GetBilledEntryByIdempotencyKey(tenantID, idempotencyKey string) (domain.BilledEntry, error) {
	tenantID = normalizeTenantID(tenantID)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return domain.BilledEntry{}, ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.BilledEntry{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`SELECT id, tenant_id, customer_id, meter_id, amount_cents, idempotency_key, source, replay_job_id, occurred_at
		FROM billed_entries
		WHERE tenant_id = $1 AND idempotency_key = $2
		ORDER BY occurred_at DESC, id DESC
		LIMIT 1`,
		tenantID,
		idempotencyKey,
	)
	var entry domain.BilledEntry
	var source string
	var entryIdempotencyKey sql.NullString
	var replayJobID sql.NullString
	if err := row.Scan(
		&entry.ID,
		&entry.TenantID,
		&entry.CustomerID,
		&entry.MeterID,
		&entry.AmountCents,
		&entryIdempotencyKey,
		&source,
		&replayJobID,
		&entry.Timestamp,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.BilledEntry{}, ErrNotFound
		}
		return domain.BilledEntry{}, err
	}
	entry.TenantID = normalizeTenantID(entry.TenantID)
	entry.Source = normalizeBilledEntrySource(domain.BilledEntrySource(source))
	if entryIdempotencyKey.Valid {
		entry.IdempotencyKey = strings.TrimSpace(entryIdempotencyKey.String)
	}
	if replayJobID.Valid {
		entry.ReplayJobID = strings.TrimSpace(replayJobID.String)
	}

	if err := tx.Commit(); err != nil {
		return domain.BilledEntry{}, err
	}
	return entry, nil
}

func (s *PostgresStore) ListBilledEntries(filter Filter) ([]domain.BilledEntry, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, filter.TenantID)
	if err != nil {
		return nil, err
	}
	defer rollbackSilently(tx)

	query, args := buildBilledEntriesFilteredQuery(`SELECT id, tenant_id, customer_id, meter_id, amount_cents, idempotency_key, source, replay_job_id, occurred_at FROM billed_entries`, filter, "occurred_at")
	if filter.SortDesc {
		query += ` ORDER BY occurred_at DESC, id DESC`
	} else {
		query += ` ORDER BY occurred_at ASC, id ASC`
	}
	if filter.Limit > 0 {
		args = append(args, filter.Limit)
		query += fmt.Sprintf(" LIMIT $%d", len(args))
	}
	if filter.Offset > 0 {
		args = append(args, filter.Offset)
		query += fmt.Sprintf(" OFFSET $%d", len(args))
	}

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.BilledEntry, 0)
	for rows.Next() {
		var entry domain.BilledEntry
		var idempotencyKey sql.NullString
		var source string
		var replayJobID sql.NullString
		if scanErr := rows.Scan(&entry.ID, &entry.TenantID, &entry.CustomerID, &entry.MeterID, &entry.AmountCents, &idempotencyKey, &source, &replayJobID, &entry.Timestamp); scanErr != nil {
			return nil, scanErr
		}
		entry.TenantID = normalizeTenantID(entry.TenantID)
		entry.Source = normalizeBilledEntrySource(domain.BilledEntrySource(source))
		if idempotencyKey.Valid {
			entry.IdempotencyKey = strings.TrimSpace(idempotencyKey.String)
		}
		if replayJobID.Valid {
			entry.ReplayJobID = strings.TrimSpace(replayJobID.String)
		}
		out = append(out, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PostgresStore) CreateReplayJob(input domain.ReplayJob) (domain.ReplayJob, error) {
	if input.ID == "" {
		input.ID = newID("rpl")
	}
	if input.CreatedAt.IsZero() {
		input.CreatedAt = time.Now().UTC()
	}
	input.TenantID = normalizeTenantID(input.TenantID)

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, input.TenantID)
	if err != nil {
		return domain.ReplayJob{}, err
	}
	defer rollbackSilently(tx)

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO replay_jobs (
			id, tenant_id, customer_id, meter_id, from_ts, to_ts, idempotency_key, status,
			attempt_count, last_attempt_at, processed_records, error, created_at, started_at, completed_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		input.ID,
		input.TenantID,
		nullableString(input.CustomerID),
		nullableString(input.MeterID),
		input.From,
		input.To,
		input.IdempotencyKey,
		string(input.Status),
		input.AttemptCount,
		input.LastAttemptAt,
		input.ProcessedRecords,
		input.Error,
		input.CreatedAt,
		input.StartedAt,
		input.CompletedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ReplayJob{}, ErrAlreadyExists
		}
		return domain.ReplayJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.ReplayJob{}, err
	}

	return input, nil
}

func (s *PostgresStore) GetReplayJob(tenantID, id string) (domain.ReplayJob, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.ReplayJob{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `SELECT id, tenant_id, customer_id, meter_id, from_ts, to_ts, idempotency_key, status, attempt_count, last_attempt_at, processed_records, error, created_at, started_at, completed_at FROM replay_jobs WHERE tenant_id = $1 AND id = $2`, normalizeTenantID(tenantID), id)
	job, err := scanReplayJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ReplayJob{}, ErrNotFound
		}
		return domain.ReplayJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.ReplayJob{}, err
	}
	return job, nil
}

func (s *PostgresStore) GetReplayJobByIdempotencyKey(tenantID, key string) (domain.ReplayJob, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tenantID = normalizeTenantID(tenantID)
	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.ReplayJob{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `SELECT id, tenant_id, customer_id, meter_id, from_ts, to_ts, idempotency_key, status, attempt_count, last_attempt_at, processed_records, error, created_at, started_at, completed_at FROM replay_jobs WHERE tenant_id = $1 AND idempotency_key = $2`, tenantID, key)
	job, err := scanReplayJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ReplayJob{}, ErrNotFound
		}
		return domain.ReplayJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.ReplayJob{}, err
	}
	return job, nil
}

func (s *PostgresStore) ListReplayJobs(filter ReplayJobListFilter) (ReplayJobListResult, error) {
	tenantID := normalizeTenantID(filter.TenantID)
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	cursorID := strings.TrimSpace(filter.CursorID)
	cursorCreated := filter.CursorCreated

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return ReplayJobListResult{}, err
	}
	defer rollbackSilently(tx)

	baseClauses := []string{"tenant_id = $1"}
	baseArgs := []any{tenantID}
	nextArg := 2

	if customerID := strings.TrimSpace(filter.CustomerID); customerID != "" {
		baseClauses = append(baseClauses, fmt.Sprintf("customer_id = $%d", nextArg))
		baseArgs = append(baseArgs, customerID)
		nextArg++
	}
	if meterID := strings.TrimSpace(filter.MeterID); meterID != "" {
		baseClauses = append(baseClauses, fmt.Sprintf("meter_id = $%d", nextArg))
		baseArgs = append(baseArgs, meterID)
		nextArg++
	}
	if status := strings.ToLower(strings.TrimSpace(filter.Status)); status != "" {
		baseClauses = append(baseClauses, fmt.Sprintf("status = $%d", nextArg))
		baseArgs = append(baseArgs, status)
		nextArg++
	}

	baseWhere := strings.Join(baseClauses, " AND ")

	var total int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM replay_jobs WHERE `+baseWhere, baseArgs...).Scan(&total); err != nil {
		return ReplayJobListResult{}, err
	}

	listClauses := append([]string{}, baseClauses...)
	listArgs := append([]any{}, baseArgs...)
	listArgPos := len(listArgs) + 1
	useCursor := cursorCreated != nil && cursorID != ""
	if useCursor {
		listClauses = append(listClauses, fmt.Sprintf("(created_at < $%d OR (created_at = $%d AND id < $%d))", listArgPos, listArgPos, listArgPos+1))
		listArgs = append(listArgs, *cursorCreated, cursorID)
		listArgPos += 2
	}
	listWhere := strings.Join(listClauses, " AND ")
	limitWithPeek := limit + 1

	var (
		query     string
		queryArgs []any
	)
	if useCursor {
		query = fmt.Sprintf(`SELECT id, tenant_id, customer_id, meter_id, from_ts, to_ts, idempotency_key, status, attempt_count, last_attempt_at, processed_records, error, created_at, started_at, completed_at
			FROM replay_jobs
			WHERE %s
			ORDER BY created_at DESC, id DESC
			LIMIT $%d`, listWhere, listArgPos)
		queryArgs = append(listArgs, limitWithPeek)
	} else {
		query = fmt.Sprintf(`SELECT id, tenant_id, customer_id, meter_id, from_ts, to_ts, idempotency_key, status, attempt_count, last_attempt_at, processed_records, error, created_at, started_at, completed_at
			FROM replay_jobs
			WHERE %s
			ORDER BY created_at DESC, id DESC
			LIMIT $%d OFFSET $%d`, listWhere, listArgPos, listArgPos+1)
		queryArgs = append(listArgs, limitWithPeek, offset)
	}

	rows, err := tx.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return ReplayJobListResult{}, err
	}
	defer rows.Close()

	items := make([]domain.ReplayJob, 0, limit)
	for rows.Next() {
		item, scanErr := scanReplayJob(rows)
		if scanErr != nil {
			return ReplayJobListResult{}, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return ReplayJobListResult{}, err
	}

	var (
		nextCursorCreated *time.Time
		nextCursorID      string
	)
	hasMore := len(items) > limit
	if len(items) > limit {
		items = items[:limit]
	}
	if len(items) == limit && hasMore {
		last := items[len(items)-1]
		t := last.CreatedAt.UTC()
		nextCursorCreated = &t
		nextCursorID = last.ID
	}

	if err := tx.Commit(); err != nil {
		return ReplayJobListResult{}, err
	}
	return ReplayJobListResult{
		Items:             items,
		Total:             total,
		Limit:             limit,
		Offset:            offset,
		NextCursorID:      nextCursorID,
		NextCursorCreated: nextCursorCreated,
	}, nil
}

func (s *PostgresStore) RetryReplayJob(tenantID, id string) (domain.ReplayJob, error) {
	tenantID = normalizeTenantID(tenantID)
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.ReplayJob{}, ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.ReplayJob{}, err
	}
	defer rollbackSilently(tx)

	currentRow := tx.QueryRowContext(
		ctx,
		`SELECT id, tenant_id, customer_id, meter_id, from_ts, to_ts, idempotency_key, status, attempt_count, last_attempt_at, processed_records, error, created_at, started_at, completed_at
		FROM replay_jobs
		WHERE tenant_id = $1 AND id = $2`,
		tenantID,
		id,
	)
	current, err := scanReplayJob(currentRow)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ReplayJob{}, ErrNotFound
		}
		return domain.ReplayJob{}, err
	}
	if current.Status != domain.ReplayFailed {
		return domain.ReplayJob{}, fmt.Errorf("validation error: replay job can be retried only when status=failed")
	}

	row := tx.QueryRowContext(
		ctx,
		`UPDATE replay_jobs
		SET status = $1, error = '', started_at = NULL, completed_at = NULL, processed_records = 0
		WHERE tenant_id = $2 AND id = $3
		RETURNING id, tenant_id, customer_id, meter_id, from_ts, to_ts, idempotency_key, status, attempt_count, last_attempt_at, processed_records, error, created_at, started_at, completed_at`,
		string(domain.ReplayQueued),
		tenantID,
		id,
	)
	job, err := scanReplayJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ReplayJob{}, ErrNotFound
		}
		return domain.ReplayJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.ReplayJob{}, err
	}
	return job, nil
}

func (s *PostgresStore) StartReplayJob(tenantID, id string) (domain.ReplayJob, error) {
	tenantID = normalizeTenantID(tenantID)
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.ReplayJob{}, ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.ReplayJob{}, err
	}
	defer rollbackSilently(tx)

	now := time.Now().UTC()
	row := tx.QueryRowContext(
		ctx,
		`UPDATE replay_jobs
		SET status = $1, started_at = $2, last_attempt_at = $2, attempt_count = attempt_count + 1, processed_records = 0, error = '', completed_at = NULL
		WHERE tenant_id = $3 AND id = $4 AND status = $5
		RETURNING id, tenant_id, customer_id, meter_id, from_ts, to_ts, idempotency_key, status, attempt_count, last_attempt_at, processed_records, error, created_at, started_at, completed_at`,
		string(domain.ReplayRunning),
		now,
		tenantID,
		id,
		string(domain.ReplayQueued),
	)
	job, err := scanReplayJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			var currentStatus string
			statusErr := tx.QueryRowContext(ctx, `SELECT status FROM replay_jobs WHERE tenant_id = $1 AND id = $2`, tenantID, id).Scan(&currentStatus)
			if errors.Is(statusErr, sql.ErrNoRows) {
				return domain.ReplayJob{}, ErrNotFound
			}
			if statusErr != nil {
				return domain.ReplayJob{}, statusErr
			}
			return domain.ReplayJob{}, fmt.Errorf("%w: replay job can be started only when status=queued", ErrInvalidState)
		}
		return domain.ReplayJob{}, err
	}

	if err := tx.Commit(); err != nil {
		return domain.ReplayJob{}, err
	}
	return job, nil
}

func (s *PostgresStore) ListQueuedReplayJobs(limit int) ([]domain.ReplayJob, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return nil, err
	}
	defer rollbackSilently(tx)

	rows, err := tx.QueryContext(
		ctx,
		`SELECT id, tenant_id, customer_id, meter_id, from_ts, to_ts, idempotency_key, status, attempt_count, last_attempt_at, processed_records, error, created_at, started_at, completed_at
		FROM replay_jobs
		WHERE status = $1
		ORDER BY created_at ASC, id ASC
		LIMIT $2`,
		string(domain.ReplayQueued),
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.ReplayJob, 0, limit)
	for rows.Next() {
		item, scanErr := scanReplayJob(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *PostgresStore) CompleteReplayJob(id string, processedRecords int64, completedAt time.Time) (domain.ReplayJob, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.ReplayJob{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`UPDATE replay_jobs SET status = $1, processed_records = $2, error = '', completed_at = $3 WHERE id = $4 RETURNING id, tenant_id, customer_id, meter_id, from_ts, to_ts, idempotency_key, status, attempt_count, last_attempt_at, processed_records, error, created_at, started_at, completed_at`,
		string(domain.ReplayDone),
		processedRecords,
		completedAt,
		id,
	)
	job, err := scanReplayJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ReplayJob{}, ErrNotFound
		}
		return domain.ReplayJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.ReplayJob{}, err
	}
	return job, nil
}

func (s *PostgresStore) FailReplayJob(id string, errMessage string, completedAt time.Time) (domain.ReplayJob, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.ReplayJob{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`UPDATE replay_jobs SET status = $1, error = $2, completed_at = $3 WHERE id = $4 RETURNING id, tenant_id, customer_id, meter_id, from_ts, to_ts, idempotency_key, status, attempt_count, last_attempt_at, processed_records, error, created_at, started_at, completed_at`,
		string(domain.ReplayFailed),
		errMessage,
		completedAt,
		id,
	)
	job, err := scanReplayJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ReplayJob{}, ErrNotFound
		}
		return domain.ReplayJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.ReplayJob{}, err
	}
	return job, nil
}

func (s *PostgresStore) IngestLagoWebhookEvent(input domain.LagoWebhookEvent) (domain.LagoWebhookEvent, bool, error) {
	if input.ID == "" {
		input.ID = newID("lwh")
	}
	input.TenantID = normalizeTenantID(input.TenantID)
	input.OrganizationID = strings.TrimSpace(input.OrganizationID)
	input.WebhookKey = strings.TrimSpace(input.WebhookKey)
	input.WebhookType = strings.TrimSpace(input.WebhookType)
	input.ObjectType = strings.TrimSpace(input.ObjectType)
	input.InvoiceID = strings.TrimSpace(input.InvoiceID)
	input.PaymentRequestID = strings.TrimSpace(input.PaymentRequestID)
	input.DunningCampaignCode = strings.TrimSpace(input.DunningCampaignCode)
	input.CustomerExternalID = strings.TrimSpace(input.CustomerExternalID)
	input.InvoiceNumber = strings.TrimSpace(input.InvoiceNumber)
	input.Currency = strings.TrimSpace(input.Currency)
	input.InvoiceStatus = strings.TrimSpace(input.InvoiceStatus)
	input.PaymentStatus = strings.TrimSpace(input.PaymentStatus)
	input.LastPaymentError = strings.TrimSpace(input.LastPaymentError)
	if input.Payload == nil {
		input.Payload = map[string]any{}
	}
	if input.ReceivedAt.IsZero() {
		input.ReceivedAt = time.Now().UTC()
	}
	if input.OccurredAt.IsZero() {
		input.OccurredAt = input.ReceivedAt
	}
	if input.WebhookKey == "" {
		input.WebhookKey = input.ID
	}
	if input.OrganizationID == "" || input.WebhookType == "" || input.ObjectType == "" {
		return domain.LagoWebhookEvent{}, false, fmt.Errorf("validation failed: organization_id, webhook_type, and object_type are required")
	}

	payloadJSON, err := json.Marshal(input.Payload)
	if err != nil {
		return domain.LagoWebhookEvent{}, false, err
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.LagoWebhookEvent{}, false, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`INSERT INTO lago_webhook_events (
			id, tenant_id, organization_id, webhook_key, webhook_type, object_type, invoice_id, payment_request_id,
			dunning_campaign_code, customer_external_id, invoice_number, currency, invoice_status, payment_status,
			payment_overdue, total_amount_cents, total_due_amount_cents, total_paid_amount_cents, last_payment_error,
			payload, received_at, occurred_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20::jsonb,$21,$22
		)
		ON CONFLICT (webhook_key) DO NOTHING
		RETURNING id, tenant_id, organization_id, webhook_key, webhook_type, object_type, invoice_id, payment_request_id,
			dunning_campaign_code, customer_external_id, invoice_number, currency, invoice_status, payment_status,
			payment_overdue, total_amount_cents, total_due_amount_cents, total_paid_amount_cents, last_payment_error,
			payload, received_at, occurred_at`,
		input.ID,
		input.TenantID,
		input.OrganizationID,
		input.WebhookKey,
		input.WebhookType,
		input.ObjectType,
		nullableString(input.InvoiceID),
		nullableString(input.PaymentRequestID),
		nullableString(input.DunningCampaignCode),
		nullableString(input.CustomerExternalID),
		nullableString(input.InvoiceNumber),
		nullableString(input.Currency),
		nullableString(input.InvoiceStatus),
		nullableString(input.PaymentStatus),
		nullableBoolPtr(input.PaymentOverdue),
		nullableInt64Ptr(input.TotalAmountCents),
		nullableInt64Ptr(input.TotalDueAmountCents),
		nullableInt64Ptr(input.TotalPaidAmountCents),
		nullableString(input.LastPaymentError),
		string(payloadJSON),
		input.ReceivedAt,
		input.OccurredAt,
	)

	created, scanErr := scanLagoWebhookEvent(row)
	if scanErr != nil {
		if !errors.Is(scanErr, sql.ErrNoRows) {
			return domain.LagoWebhookEvent{}, false, scanErr
		}
		existingRow := tx.QueryRowContext(
			ctx,
			`SELECT id, tenant_id, organization_id, webhook_key, webhook_type, object_type, invoice_id, payment_request_id,
				dunning_campaign_code, customer_external_id, invoice_number, currency, invoice_status, payment_status,
				payment_overdue, total_amount_cents, total_due_amount_cents, total_paid_amount_cents, last_payment_error,
				payload, received_at, occurred_at
			FROM lago_webhook_events
			WHERE webhook_key = $1`,
			input.WebhookKey,
		)
		existing, err := scanLagoWebhookEvent(existingRow)
		if err != nil {
			return domain.LagoWebhookEvent{}, false, err
		}
		if err := tx.Commit(); err != nil {
			return domain.LagoWebhookEvent{}, false, err
		}
		return existing, false, nil
	}

	if created.InvoiceID != "" {
		if err := s.upsertInvoicePaymentStatusViewTx(ctx, tx, created); err != nil {
			return domain.LagoWebhookEvent{}, false, err
		}
	}

	if err := tx.Commit(); err != nil {
		return domain.LagoWebhookEvent{}, false, err
	}
	return created, true, nil
}

func (s *PostgresStore) upsertInvoicePaymentStatusViewTx(ctx context.Context, tx *sql.Tx, event domain.LagoWebhookEvent) error {
	_, err := tx.ExecContext(
		ctx,
		`INSERT INTO invoice_payment_status_views (
			tenant_id, organization_id, invoice_id, customer_external_id, invoice_number, currency,
			invoice_status, payment_status, payment_overdue, total_amount_cents, total_due_amount_cents,
			total_paid_amount_cents, last_payment_error, last_event_type, last_event_at, last_webhook_key, updated_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17
		)
		ON CONFLICT (tenant_id, invoice_id) DO UPDATE SET
			organization_id = EXCLUDED.organization_id,
			customer_external_id = COALESCE(NULLIF(EXCLUDED.customer_external_id, ''), invoice_payment_status_views.customer_external_id),
			invoice_number = COALESCE(NULLIF(EXCLUDED.invoice_number, ''), invoice_payment_status_views.invoice_number),
			currency = COALESCE(NULLIF(EXCLUDED.currency, ''), invoice_payment_status_views.currency),
			invoice_status = COALESCE(NULLIF(EXCLUDED.invoice_status, ''), invoice_payment_status_views.invoice_status),
			payment_status = COALESCE(NULLIF(EXCLUDED.payment_status, ''), invoice_payment_status_views.payment_status),
			payment_overdue = COALESCE(EXCLUDED.payment_overdue, invoice_payment_status_views.payment_overdue),
			total_amount_cents = COALESCE(EXCLUDED.total_amount_cents, invoice_payment_status_views.total_amount_cents),
			total_due_amount_cents = COALESCE(EXCLUDED.total_due_amount_cents, invoice_payment_status_views.total_due_amount_cents),
			total_paid_amount_cents = COALESCE(EXCLUDED.total_paid_amount_cents, invoice_payment_status_views.total_paid_amount_cents),
			last_payment_error = CASE
				WHEN EXCLUDED.last_payment_error = '' THEN invoice_payment_status_views.last_payment_error
				ELSE EXCLUDED.last_payment_error
			END,
			last_event_type = EXCLUDED.last_event_type,
			last_event_at = EXCLUDED.last_event_at,
			last_webhook_key = EXCLUDED.last_webhook_key,
			updated_at = EXCLUDED.updated_at
		WHERE EXCLUDED.last_event_at >= invoice_payment_status_views.last_event_at`,
		event.TenantID,
		event.OrganizationID,
		event.InvoiceID,
		nullableString(event.CustomerExternalID),
		nullableString(event.InvoiceNumber),
		nullableString(event.Currency),
		nullableString(event.InvoiceStatus),
		nullableString(event.PaymentStatus),
		nullableBoolPtr(event.PaymentOverdue),
		nullableInt64Ptr(event.TotalAmountCents),
		nullableInt64Ptr(event.TotalDueAmountCents),
		nullableInt64Ptr(event.TotalPaidAmountCents),
		nullableString(event.LastPaymentError),
		event.WebhookType,
		event.OccurredAt,
		event.WebhookKey,
		time.Now().UTC(),
	)
	return err
}

func (s *PostgresStore) ListInvoicePaymentStatusViews(filter InvoicePaymentStatusListFilter) ([]domain.InvoicePaymentStatusView, error) {
	tenantID := normalizeTenantID(filter.TenantID)
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return nil, err
	}
	defer rollbackSilently(tx)

	clauses := []string{"tenant_id = $1"}
	args := []any{tenantID}
	argPos := 2

	if organizationID := strings.TrimSpace(filter.OrganizationID); organizationID != "" {
		clauses = append(clauses, fmt.Sprintf("organization_id = $%d", argPos))
		args = append(args, organizationID)
		argPos++
	}
	if paymentStatus := strings.TrimSpace(filter.PaymentStatus); paymentStatus != "" {
		clauses = append(clauses, fmt.Sprintf("payment_status = $%d", argPos))
		args = append(args, paymentStatus)
		argPos++
	}
	if invoiceStatus := strings.TrimSpace(filter.InvoiceStatus); invoiceStatus != "" {
		clauses = append(clauses, fmt.Sprintf("invoice_status = $%d", argPos))
		args = append(args, invoiceStatus)
		argPos++
	}
	if filter.PaymentOverdue != nil {
		clauses = append(clauses, fmt.Sprintf("payment_overdue = $%d", argPos))
		args = append(args, *filter.PaymentOverdue)
		argPos++
	}

	sortColumn := "last_event_at"
	switch strings.ToLower(strings.TrimSpace(filter.SortBy)) {
	case "updated_at":
		sortColumn = "updated_at"
	case "total_due_amount_cents":
		sortColumn = "COALESCE(total_due_amount_cents, 0)"
	case "total_amount_cents":
		sortColumn = "COALESCE(total_amount_cents, 0)"
	case "", "last_event_at":
		sortColumn = "last_event_at"
	default:
		sortColumn = "last_event_at"
	}
	sortDirection := "ASC"
	if filter.SortDesc {
		sortDirection = "DESC"
	}
	tieDirection := "ASC"
	if filter.SortDesc {
		tieDirection = "DESC"
	}

	query := `SELECT tenant_id, organization_id, invoice_id, customer_external_id, invoice_number, currency,
		invoice_status, payment_status, payment_overdue, total_amount_cents, total_due_amount_cents,
		total_paid_amount_cents, last_payment_error, last_event_type, last_event_at, last_webhook_key, updated_at
		FROM invoice_payment_status_views
		WHERE ` + strings.Join(clauses, " AND ") + `
		ORDER BY ` + sortColumn + ` ` + sortDirection + `, invoice_id ` + tieDirection + `
		LIMIT $` + fmt.Sprintf("%d", argPos) + ` OFFSET $` + fmt.Sprintf("%d", argPos+1)
	args = append(args, limit, offset)

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.InvoicePaymentStatusView, 0, limit)
	for rows.Next() {
		item, scanErr := scanInvoicePaymentStatusView(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PostgresStore) GetInvoicePaymentStatusView(tenantID, invoiceID string) (domain.InvoicePaymentStatusView, error) {
	tenantID = normalizeTenantID(tenantID)
	invoiceID = strings.TrimSpace(invoiceID)
	if invoiceID == "" {
		return domain.InvoicePaymentStatusView{}, ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.InvoicePaymentStatusView{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`SELECT tenant_id, organization_id, invoice_id, customer_external_id, invoice_number, currency,
			invoice_status, payment_status, payment_overdue, total_amount_cents, total_due_amount_cents,
			total_paid_amount_cents, last_payment_error, last_event_type, last_event_at, last_webhook_key, updated_at
		FROM invoice_payment_status_views
		WHERE tenant_id = $1 AND invoice_id = $2`,
		tenantID,
		invoiceID,
	)
	item, err := scanInvoicePaymentStatusView(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.InvoicePaymentStatusView{}, ErrNotFound
		}
		return domain.InvoicePaymentStatusView{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.InvoicePaymentStatusView{}, err
	}
	return item, nil
}

func (s *PostgresStore) GetInvoicePaymentStatusSummary(filter InvoicePaymentStatusSummaryFilter) (InvoicePaymentStatusSummary, error) {
	tenantID := normalizeTenantID(filter.TenantID)

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return InvoicePaymentStatusSummary{}, err
	}
	defer rollbackSilently(tx)

	clauses := []string{"tenant_id = $1"}
	args := []any{tenantID}
	argPos := 2
	if organizationID := strings.TrimSpace(filter.OrganizationID); organizationID != "" {
		clauses = append(clauses, fmt.Sprintf("organization_id = $%d", argPos))
		args = append(args, organizationID)
		argPos++
	}
	whereClause := strings.Join(clauses, " AND ")

	out := InvoicePaymentStatusSummary{
		PaymentStatusCounts: make(map[string]int64),
		InvoiceStatusCounts: make(map[string]int64),
	}

	var latestEventAt sql.NullTime
	totalsQuery := `SELECT
		COUNT(*) AS total_invoices,
		COALESCE(SUM(CASE WHEN payment_overdue IS TRUE THEN 1 ELSE 0 END), 0) AS overdue_count,
		COALESCE(SUM(CASE WHEN payment_overdue IS TRUE OR LOWER(COALESCE(payment_status, '')) IN ('failed', 'pending') THEN 1 ELSE 0 END), 0) AS attention_required_count,
		MAX(last_event_at) AS latest_event_at
	FROM invoice_payment_status_views
	WHERE ` + whereClause
	if err := tx.QueryRowContext(ctx, totalsQuery, args...).Scan(
		&out.TotalInvoices,
		&out.OverdueCount,
		&out.AttentionRequiredCount,
		&latestEventAt,
	); err != nil {
		return InvoicePaymentStatusSummary{}, err
	}
	if latestEventAt.Valid {
		t := latestEventAt.Time.UTC()
		out.LatestEventAt = &t
	}

	paymentStatusQuery := `SELECT
		COALESCE(NULLIF(TRIM(payment_status), ''), 'unknown') AS status_key,
		COUNT(*)
	FROM invoice_payment_status_views
	WHERE ` + whereClause + `
	GROUP BY 1`
	rows, err := tx.QueryContext(ctx, paymentStatusQuery, args...)
	if err != nil {
		return InvoicePaymentStatusSummary{}, err
	}
	for rows.Next() {
		var key string
		var count int64
		if err := rows.Scan(&key, &count); err != nil {
			rows.Close()
			return InvoicePaymentStatusSummary{}, err
		}
		out.PaymentStatusCounts[strings.ToLower(strings.TrimSpace(key))] = count
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return InvoicePaymentStatusSummary{}, err
	}
	rows.Close()

	invoiceStatusQuery := `SELECT
		COALESCE(NULLIF(TRIM(invoice_status), ''), 'unknown') AS status_key,
		COUNT(*)
	FROM invoice_payment_status_views
	WHERE ` + whereClause + `
	GROUP BY 1`
	rows, err = tx.QueryContext(ctx, invoiceStatusQuery, args...)
	if err != nil {
		return InvoicePaymentStatusSummary{}, err
	}
	for rows.Next() {
		var key string
		var count int64
		if err := rows.Scan(&key, &count); err != nil {
			rows.Close()
			return InvoicePaymentStatusSummary{}, err
		}
		out.InvoiceStatusCounts[strings.ToLower(strings.TrimSpace(key))] = count
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return InvoicePaymentStatusSummary{}, err
	}
	rows.Close()

	if filter.StaleBefore != nil {
		staleArgs := append([]any{}, args...)
		staleArgs = append(staleArgs, filter.StaleBefore.UTC())
		staleQuery := `SELECT
			COUNT(*)
		FROM invoice_payment_status_views
		WHERE ` + whereClause + `
		  AND last_event_at < $` + fmt.Sprintf("%d", len(staleArgs)) + `
		  AND (payment_overdue IS TRUE OR LOWER(COALESCE(payment_status, '')) IN ('failed', 'pending'))`
		if err := tx.QueryRowContext(ctx, staleQuery, staleArgs...).Scan(&out.StaleAttentionRequired); err != nil {
			return InvoicePaymentStatusSummary{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return InvoicePaymentStatusSummary{}, err
	}
	return out, nil
}

func (s *PostgresStore) ListLagoWebhookEvents(filter LagoWebhookEventListFilter) ([]domain.LagoWebhookEvent, error) {
	tenantID := normalizeTenantID(filter.TenantID)
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return nil, err
	}
	defer rollbackSilently(tx)

	clauses := []string{"tenant_id = $1"}
	args := []any{tenantID}
	argPos := 2
	if organizationID := strings.TrimSpace(filter.OrganizationID); organizationID != "" {
		clauses = append(clauses, fmt.Sprintf("organization_id = $%d", argPos))
		args = append(args, organizationID)
		argPos++
	}
	if invoiceID := strings.TrimSpace(filter.InvoiceID); invoiceID != "" {
		clauses = append(clauses, fmt.Sprintf("invoice_id = $%d", argPos))
		args = append(args, invoiceID)
		argPos++
	}
	if webhookType := strings.TrimSpace(filter.WebhookType); webhookType != "" {
		clauses = append(clauses, fmt.Sprintf("webhook_type = $%d", argPos))
		args = append(args, webhookType)
		argPos++
	}

	sortColumn := "received_at"
	switch strings.ToLower(strings.TrimSpace(filter.SortBy)) {
	case "occurred_at":
		sortColumn = "occurred_at"
	case "", "received_at":
		sortColumn = "received_at"
	default:
		sortColumn = "received_at"
	}
	sortDirection := "ASC"
	if filter.SortDesc {
		sortDirection = "DESC"
	}
	tieDirection := "ASC"
	if filter.SortDesc {
		tieDirection = "DESC"
	}

	query := `SELECT id, tenant_id, organization_id, webhook_key, webhook_type, object_type, invoice_id, payment_request_id,
		dunning_campaign_code, customer_external_id, invoice_number, currency, invoice_status, payment_status,
		payment_overdue, total_amount_cents, total_due_amount_cents, total_paid_amount_cents, last_payment_error,
		payload, received_at, occurred_at
		FROM lago_webhook_events
		WHERE ` + strings.Join(clauses, " AND ") + `
		ORDER BY ` + sortColumn + ` ` + sortDirection + `, id ` + tieDirection + `
		LIMIT $` + fmt.Sprintf("%d", argPos) + ` OFFSET $` + fmt.Sprintf("%d", argPos+1)
	args = append(args, limit, offset)

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.LagoWebhookEvent, 0, limit)
	for rows.Next() {
		item, scanErr := scanLagoWebhookEvent(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *PostgresStore) ListInvoicePaymentSyncCandidates(filter InvoicePaymentSyncCandidateFilter) ([]InvoicePaymentSyncCandidate, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	staleBefore := filter.StaleBefore
	if staleBefore.IsZero() {
		staleBefore = time.Now().UTC()
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return nil, err
	}
	defer rollbackSilently(tx)

	rows, err := tx.QueryContext(
		ctx,
		`SELECT tenant_id, organization_id, invoice_id, payment_status, payment_overdue, last_event_at, updated_at
		 FROM invoice_payment_status_views
		 WHERE updated_at <= $1
		   AND (payment_overdue IS TRUE OR payment_status IN ('failed', 'pending'))
		 ORDER BY updated_at ASC, invoice_id ASC
		 LIMIT $2`,
		staleBefore,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]InvoicePaymentSyncCandidate, 0, limit)
	for rows.Next() {
		var (
			item           InvoicePaymentSyncCandidate
			paymentStatus  sql.NullString
			paymentOverdue sql.NullBool
		)
		if err := rows.Scan(
			&item.TenantID,
			&item.OrganizationID,
			&item.InvoiceID,
			&paymentStatus,
			&paymentOverdue,
			&item.LastEventAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		item.TenantID = normalizeTenantID(item.TenantID)
		item.OrganizationID = strings.TrimSpace(item.OrganizationID)
		item.InvoiceID = strings.TrimSpace(item.InvoiceID)
		if paymentStatus.Valid {
			item.PaymentStatus = strings.TrimSpace(paymentStatus.String)
		}
		if paymentOverdue.Valid {
			v := paymentOverdue.Bool
			item.PaymentOverdue = &v
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *PostgresStore) CreateServiceAccount(input domain.ServiceAccount) (domain.ServiceAccount, error) {
	input.TenantID = normalizeTenantID(input.TenantID)
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.Role = strings.ToLower(strings.TrimSpace(input.Role))
	input.Purpose = strings.TrimSpace(input.Purpose)
	input.Environment = strings.TrimSpace(input.Environment)
	input.CreatedByUserID = strings.TrimSpace(input.CreatedByUserID)

	if input.TenantID == "" || input.Name == "" || input.Role == "" {
		return domain.ServiceAccount{}, fmt.Errorf("validation failed: tenant_id, name, and role are required")
	}
	if input.ID == "" {
		input.ID = newID("sa")
	}
	if input.CreatedAt.IsZero() {
		input.CreatedAt = time.Now().UTC()
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = input.CreatedAt
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, input.TenantID)
	if err != nil {
		return domain.ServiceAccount{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`INSERT INTO service_accounts (
			id, tenant_id, name, description, role, purpose, environment, created_by_user_id, created_by_platform_user, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF($8,''),$9,$10,$11)
		RETURNING id, tenant_id, name, description, role, purpose, environment, created_by_user_id, created_by_platform_user, created_at, updated_at`,
		input.ID,
		input.TenantID,
		input.Name,
		input.Description,
		input.Role,
		input.Purpose,
		input.Environment,
		input.CreatedByUserID,
		input.CreatedByPlatformUser,
		input.CreatedAt,
		input.UpdatedAt,
	)
	account, err := scanServiceAccount(row)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ServiceAccount{}, ErrAlreadyExists
		}
		return domain.ServiceAccount{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.ServiceAccount{}, err
	}
	return account, nil
}

func (s *PostgresStore) GetServiceAccount(tenantID, id string) (domain.ServiceAccount, error) {
	tenantID = normalizeTenantID(tenantID)
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.ServiceAccount{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`SELECT id, tenant_id, name, description, role, purpose, environment, created_by_user_id, created_by_platform_user, created_at, updated_at
		FROM service_accounts
		WHERE tenant_id = $1 AND id = $2`,
		tenantID,
		id,
	)
	account, err := scanServiceAccount(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ServiceAccount{}, ErrNotFound
		}
		return domain.ServiceAccount{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.ServiceAccount{}, err
	}
	return account, nil
}

func (s *PostgresStore) ListServiceAccounts(filter ServiceAccountListFilter) ([]domain.ServiceAccount, error) {
	tenantID := normalizeTenantID(filter.TenantID)
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return nil, err
	}
	defer rollbackSilently(tx)

	clauses := []string{"tenant_id = $1"}
	args := []any{tenantID}
	nextArg := 2
	if role := strings.TrimSpace(filter.Role); role != "" {
		clauses = append(clauses, fmt.Sprintf("role = $%d", nextArg))
		args = append(args, role)
		nextArg++
	}
	if nameContains := strings.TrimSpace(filter.NameContains); nameContains != "" {
		clauses = append(clauses, fmt.Sprintf("name ILIKE $%d", nextArg))
		args = append(args, "%"+nameContains+"%")
		nextArg++
	}
	query := fmt.Sprintf(`SELECT id, tenant_id, name, description, role, purpose, environment, created_by_user_id, created_by_platform_user, created_at, updated_at
		FROM service_accounts
		WHERE %s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d OFFSET $%d`, strings.Join(clauses, " AND "), nextArg, nextArg+1)
	args = append(args, limit, offset)

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.ServiceAccount, 0)
	for rows.Next() {
		account, scanErr := scanServiceAccount(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, account)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PostgresStore) CreateAPIKey(input domain.APIKey) (domain.APIKey, error) {
	input.KeyPrefix = strings.TrimSpace(input.KeyPrefix)
	input.KeyHash = strings.ToLower(strings.TrimSpace(input.KeyHash))
	input.Name = strings.TrimSpace(input.Name)
	input.Role = strings.ToLower(strings.TrimSpace(input.Role))
	input.TenantID = normalizeTenantID(input.TenantID)

	if input.KeyPrefix == "" || input.KeyHash == "" || input.Role == "" {
		return domain.APIKey{}, fmt.Errorf("validation failed: key prefix, key hash, and role are required")
	}
	if input.Name == "" {
		input.Name = input.KeyPrefix
	}

	if input.ID == "" {
		input.ID = newID("key")
	}
	if input.CreatedAt.IsZero() {
		input.CreatedAt = time.Now().UTC()
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, input.TenantID)
	if err != nil {
		return domain.APIKey{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`INSERT INTO api_keys (
			id, key_prefix, key_hash, name, role, tenant_id, owner_type, owner_id, purpose, environment, created_by_user_id, created_by_platform_user, created_at, expires_at, revoked_at, last_used_at, last_rotated_at, rotation_required_at, revocation_reason
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NULLIF($11,''),$12,$13,$14,$15,$16,$17,$18,$19)
		RETURNING id, key_prefix, key_hash, name, role, tenant_id, owner_type, owner_id, purpose, environment, created_by_user_id, created_by_platform_user, created_at, expires_at, revoked_at, last_used_at, last_rotated_at, rotation_required_at, revocation_reason`,
		input.ID,
		input.KeyPrefix,
		input.KeyHash,
		input.Name,
		input.Role,
		input.TenantID,
		input.OwnerType,
		input.OwnerID,
		input.Purpose,
		input.Environment,
		input.CreatedByUserID,
		input.CreatedAt,
		input.ExpiresAt,
		input.RevokedAt,
		input.LastUsedAt,
		input.LastRotatedAt,
		input.RotationRequiredAt,
		input.RevocationReason,
	)
	key, err := scanAPIKey(row)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.APIKey{}, ErrAlreadyExists
		}
		return domain.APIKey{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.APIKey{}, err
	}
	return key, nil
}

func (s *PostgresStore) CreatePlatformAPIKey(input domain.PlatformAPIKey) (domain.PlatformAPIKey, error) {
	input.KeyPrefix = strings.TrimSpace(input.KeyPrefix)
	input.KeyHash = strings.ToLower(strings.TrimSpace(input.KeyHash))
	input.Name = strings.TrimSpace(input.Name)
	input.Role = strings.ToLower(strings.TrimSpace(input.Role))

	if input.KeyPrefix == "" || input.KeyHash == "" || input.Role == "" {
		return domain.PlatformAPIKey{}, fmt.Errorf("validation failed: key prefix, key hash, and role are required")
	}
	if input.Name == "" {
		input.Name = input.KeyPrefix
	}
	if input.ID == "" {
		input.ID = newID("pkey")
	}
	if input.CreatedAt.IsZero() {
		input.CreatedAt = time.Now().UTC()
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.PlatformAPIKey{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`INSERT INTO platform_api_keys (
			id, key_prefix, key_hash, name, role, owner_type, owner_id, purpose, environment, created_by_user_id, created_at, expires_at, revoked_at, last_used_at, last_rotated_at, rotation_required_at, revocation_reason
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NULLIF($10,''),$11,$12,$13,$14,$15,$16,$17)
		RETURNING id, key_prefix, key_hash, name, role, owner_type, owner_id, purpose, environment, created_by_user_id, created_at, expires_at, revoked_at, last_used_at, last_rotated_at, rotation_required_at, revocation_reason`,
		input.ID,
		input.KeyPrefix,
		input.KeyHash,
		input.Name,
		input.Role,
		input.OwnerType,
		input.OwnerID,
		input.Purpose,
		input.Environment,
		input.CreatedByUserID,
		input.CreatedAt,
		input.ExpiresAt,
		input.RevokedAt,
		input.LastUsedAt,
		input.LastRotatedAt,
		input.RotationRequiredAt,
		input.RevocationReason,
	)
	key, err := scanPlatformAPIKey(row)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.PlatformAPIKey{}, ErrAlreadyExists
		}
		return domain.PlatformAPIKey{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.PlatformAPIKey{}, err
	}
	return key, nil
}

func (s *PostgresStore) GetAPIKeyByID(tenantID, id string) (domain.APIKey, error) {
	tenantID = normalizeTenantID(tenantID)
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.APIKey{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`SELECT id, key_prefix, key_hash, name, role, tenant_id, owner_type, owner_id, purpose, environment, created_by_user_id, created_by_platform_user, created_at, expires_at, revoked_at, last_used_at, last_rotated_at, rotation_required_at, revocation_reason
		FROM api_keys
		WHERE tenant_id = $1 AND id = $2`,
		tenantID,
		id,
	)
	key, err := scanAPIKey(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.APIKey{}, ErrNotFound
		}
		return domain.APIKey{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.APIKey{}, err
	}
	return key, nil
}

func (s *PostgresStore) ListAPIKeys(filter APIKeyListFilter) (APIKeyListResult, error) {
	tenantID := normalizeTenantID(filter.TenantID)
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	cursorID := strings.TrimSpace(filter.CursorID)
	cursorCreated := filter.CursorCreated
	refTime := filter.ReferenceTime
	if refTime.IsZero() {
		refTime = time.Now().UTC()
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return APIKeyListResult{}, err
	}
	defer rollbackSilently(tx)

	baseClauses := []string{"tenant_id = $1"}
	baseArgs := []any{tenantID}
	nextArg := 2

	if role := strings.TrimSpace(filter.Role); role != "" {
		baseClauses = append(baseClauses, fmt.Sprintf("role = $%d", nextArg))
		baseArgs = append(baseArgs, role)
		nextArg++
	}
	if nameContains := strings.TrimSpace(filter.NameContains); nameContains != "" {
		baseClauses = append(baseClauses, fmt.Sprintf("name ILIKE $%d", nextArg))
		baseArgs = append(baseArgs, "%"+nameContains+"%")
		nextArg++
	}
	if ownerType := strings.TrimSpace(filter.OwnerType); ownerType != "" {
		baseClauses = append(baseClauses, fmt.Sprintf("owner_type = $%d", nextArg))
		baseArgs = append(baseArgs, ownerType)
		nextArg++
	}
	if ownerID := strings.TrimSpace(filter.OwnerID); ownerID != "" {
		baseClauses = append(baseClauses, fmt.Sprintf("owner_id = $%d", nextArg))
		baseArgs = append(baseArgs, ownerID)
		nextArg++
	}
	switch strings.ToLower(strings.TrimSpace(filter.State)) {
	case "active":
		baseClauses = append(baseClauses, fmt.Sprintf("revoked_at IS NULL AND (expires_at IS NULL OR expires_at > $%d)", nextArg))
		baseArgs = append(baseArgs, refTime)
		nextArg++
	case "revoked":
		baseClauses = append(baseClauses, "revoked_at IS NOT NULL")
	case "expired":
		baseClauses = append(baseClauses, fmt.Sprintf("revoked_at IS NULL AND expires_at IS NOT NULL AND expires_at <= $%d", nextArg))
		baseArgs = append(baseArgs, refTime)
		nextArg++
	}
	baseWhere := strings.Join(baseClauses, " AND ")

	countQuery := `SELECT COUNT(*) FROM api_keys WHERE ` + baseWhere
	var total int
	if err := tx.QueryRowContext(ctx, countQuery, baseArgs...).Scan(&total); err != nil {
		return APIKeyListResult{}, err
	}

	listClauses := append([]string{}, baseClauses...)
	listArgs := append([]any{}, baseArgs...)
	listArgPos := len(listArgs) + 1
	useCursor := cursorCreated != nil && cursorID != ""
	if useCursor {
		listClauses = append(listClauses, fmt.Sprintf("(created_at < $%d OR (created_at = $%d AND id < $%d))", listArgPos, listArgPos, listArgPos+1))
		listArgs = append(listArgs, *cursorCreated, cursorID)
		listArgPos += 2
	}
	listWhere := strings.Join(listClauses, " AND ")
	limitWithPeek := limit + 1

	var (
		listQuery string
		queryArgs []any
	)
	if useCursor {
		listQuery = fmt.Sprintf(`SELECT id, key_prefix, key_hash, name, role, tenant_id, owner_type, owner_id, purpose, environment, created_by_user_id, created_by_platform_user, created_at, expires_at, revoked_at, last_used_at, last_rotated_at, rotation_required_at, revocation_reason
			FROM api_keys
			WHERE %s
			ORDER BY created_at DESC, id DESC
			LIMIT $%d`, listWhere, listArgPos)
		queryArgs = append(listArgs, limitWithPeek)
	} else {
		listQuery = fmt.Sprintf(`SELECT id, key_prefix, key_hash, name, role, tenant_id, owner_type, owner_id, purpose, environment, created_by_user_id, created_by_platform_user, created_at, expires_at, revoked_at, last_used_at, last_rotated_at, rotation_required_at, revocation_reason
			FROM api_keys
			WHERE %s
			ORDER BY created_at DESC, id DESC
			LIMIT $%d OFFSET $%d`, listWhere, listArgPos, listArgPos+1)
		queryArgs = append(listArgs, limitWithPeek, offset)
	}

	rows, err := tx.QueryContext(ctx, listQuery, queryArgs...)
	if err != nil {
		return APIKeyListResult{}, err
	}
	defer rows.Close()

	out := make([]domain.APIKey, 0)
	for rows.Next() {
		key, scanErr := scanAPIKey(rows)
		if scanErr != nil {
			return APIKeyListResult{}, scanErr
		}
		out = append(out, key)
	}
	if err := rows.Err(); err != nil {
		return APIKeyListResult{}, err
	}

	var (
		nextCursorCreated *time.Time
		nextCursorID      string
	)
	hasMore := len(out) > limit
	if len(out) > limit {
		out = out[:limit]
	}
	if len(out) == limit && hasMore {
		last := out[len(out)-1]
		t := last.CreatedAt.UTC()
		nextCursorCreated = &t
		nextCursorID = last.ID
	}

	if err := tx.Commit(); err != nil {
		return APIKeyListResult{}, err
	}
	return APIKeyListResult{
		Items:             out,
		Total:             total,
		Limit:             limit,
		Offset:            offset,
		NextCursorID:      nextCursorID,
		NextCursorCreated: nextCursorCreated,
	}, nil
}

func (s *PostgresStore) RevokeAPIKey(tenantID, id string, revokedAt time.Time) (domain.APIKey, error) {
	tenantID = normalizeTenantID(tenantID)
	if revokedAt.IsZero() {
		revokedAt = time.Now().UTC()
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.APIKey{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`UPDATE api_keys
		SET revoked_at = COALESCE(revoked_at, $1)
		WHERE tenant_id = $2 AND id = $3
		RETURNING id, key_prefix, key_hash, name, role, tenant_id, owner_type, owner_id, purpose, environment, created_by_user_id, created_by_platform_user, created_at, expires_at, revoked_at, last_used_at, last_rotated_at, rotation_required_at, revocation_reason`,
		revokedAt,
		tenantID,
		id,
	)
	key, err := scanAPIKey(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.APIKey{}, ErrNotFound
		}
		return domain.APIKey{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.APIKey{}, err
	}
	return key, nil
}

func (s *PostgresStore) CreateAPIKeyAuditEvent(input domain.APIKeyAuditEvent) (domain.APIKeyAuditEvent, error) {
	input.TenantID = normalizeTenantID(input.TenantID)
	input.APIKeyID = strings.TrimSpace(input.APIKeyID)
	input.ActorAPIKeyID = strings.TrimSpace(input.ActorAPIKeyID)
	input.Action = strings.ToLower(strings.TrimSpace(input.Action))
	if input.ID == "" {
		input.ID = newID("ake")
	}
	if input.CreatedAt.IsZero() {
		input.CreatedAt = time.Now().UTC()
	}
	if input.APIKeyID == "" || input.Action == "" {
		return domain.APIKeyAuditEvent{}, fmt.Errorf("validation failed: api_key_id and action are required")
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}

	metadataRaw, err := json.Marshal(input.Metadata)
	if err != nil {
		return domain.APIKeyAuditEvent{}, err
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, input.TenantID)
	if err != nil {
		return domain.APIKeyAuditEvent{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`INSERT INTO api_key_audit_events (id, tenant_id, api_key_id, actor_api_key_id, action, metadata, created_at)
		VALUES ($1,$2,$3,NULLIF($4,''),$5,$6::jsonb,$7)
		RETURNING id, tenant_id, api_key_id, actor_api_key_id, action, metadata, created_at`,
		input.ID,
		input.TenantID,
		input.APIKeyID,
		input.ActorAPIKeyID,
		input.Action,
		metadataRaw,
		input.CreatedAt,
	)
	event, err := scanAPIKeyAuditEvent(row)
	if err != nil {
		return domain.APIKeyAuditEvent{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.APIKeyAuditEvent{}, err
	}
	return event, nil
}

func (s *PostgresStore) ListAPIKeyAuditEvents(filter APIKeyAuditFilter) (APIKeyAuditResult, error) {
	tenantID := normalizeTenantID(filter.TenantID)
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	cursorID := strings.TrimSpace(filter.CursorID)
	cursorCreated := filter.CursorCreated

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return APIKeyAuditResult{}, err
	}
	defer rollbackSilently(tx)

	baseClauses := []string{"tenant_id = $1"}
	baseArgs := []any{tenantID}
	nextArg := 2

	if apiKeyID := strings.TrimSpace(filter.APIKeyID); apiKeyID != "" {
		baseClauses = append(baseClauses, fmt.Sprintf("api_key_id = $%d", nextArg))
		baseArgs = append(baseArgs, apiKeyID)
		nextArg++
	}
	if actorAPIKeyID := strings.TrimSpace(filter.ActorAPIKeyID); actorAPIKeyID != "" {
		baseClauses = append(baseClauses, fmt.Sprintf("actor_api_key_id = $%d", nextArg))
		baseArgs = append(baseArgs, actorAPIKeyID)
		nextArg++
	}
	if action := strings.ToLower(strings.TrimSpace(filter.Action)); action != "" {
		baseClauses = append(baseClauses, fmt.Sprintf("action = $%d", nextArg))
		baseArgs = append(baseArgs, action)
		nextArg++
	}

	baseWhere := strings.Join(baseClauses, " AND ")

	var total int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM api_key_audit_events WHERE `+baseWhere, baseArgs...).Scan(&total); err != nil {
		return APIKeyAuditResult{}, err
	}

	listClauses := append([]string{}, baseClauses...)
	listArgs := append([]any{}, baseArgs...)
	listArgPos := len(listArgs) + 1
	useCursor := cursorCreated != nil && cursorID != ""
	if useCursor {
		listClauses = append(listClauses, fmt.Sprintf("(created_at < $%d OR (created_at = $%d AND id < $%d))", listArgPos, listArgPos, listArgPos+1))
		listArgs = append(listArgs, *cursorCreated, cursorID)
		listArgPos += 2
	}
	listWhere := strings.Join(listClauses, " AND ")
	limitWithPeek := limit + 1

	var (
		query     string
		queryArgs []any
	)
	if useCursor {
		query = fmt.Sprintf(`SELECT id, tenant_id, api_key_id, actor_api_key_id, action, metadata, created_at
			FROM api_key_audit_events
			WHERE %s
			ORDER BY created_at DESC, id DESC
			LIMIT $%d`, listWhere, listArgPos)
		queryArgs = append(listArgs, limitWithPeek)
	} else {
		query = fmt.Sprintf(`SELECT id, tenant_id, api_key_id, actor_api_key_id, action, metadata, created_at
			FROM api_key_audit_events
			WHERE %s
			ORDER BY created_at DESC, id DESC
			LIMIT $%d OFFSET $%d`, listWhere, listArgPos, listArgPos+1)
		queryArgs = append(listArgs, limitWithPeek, offset)
	}

	rows, err := tx.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return APIKeyAuditResult{}, err
	}
	defer rows.Close()

	items := make([]domain.APIKeyAuditEvent, 0, limit)
	for rows.Next() {
		item, scanErr := scanAPIKeyAuditEvent(rows)
		if scanErr != nil {
			return APIKeyAuditResult{}, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return APIKeyAuditResult{}, err
	}

	var (
		nextCursorCreated *time.Time
		nextCursorID      string
	)
	hasMore := len(items) > limit
	if len(items) > limit {
		items = items[:limit]
	}
	if len(items) == limit && hasMore {
		last := items[len(items)-1]
		t := last.CreatedAt.UTC()
		nextCursorCreated = &t
		nextCursorID = last.ID
	}

	if err := tx.Commit(); err != nil {
		return APIKeyAuditResult{}, err
	}

	return APIKeyAuditResult{
		Items:             items,
		Total:             total,
		Limit:             limit,
		Offset:            offset,
		NextCursorID:      nextCursorID,
		NextCursorCreated: nextCursorCreated,
	}, nil
}

func (s *PostgresStore) CreateAPIKeyAuditExportJob(input domain.APIKeyAuditExportJob) (domain.APIKeyAuditExportJob, error) {
	input.TenantID = normalizeTenantID(input.TenantID)
	input.RequestedByAPIKeyID = strings.TrimSpace(input.RequestedByAPIKeyID)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if input.ID == "" {
		input.ID = newID("akx")
	}
	if input.CreatedAt.IsZero() {
		input.CreatedAt = time.Now().UTC()
	}
	if strings.TrimSpace(string(input.Status)) == "" {
		input.Status = domain.APIKeyAuditExportQueued
	}
	if input.IdempotencyKey == "" || input.RequestedByAPIKeyID == "" {
		return domain.APIKeyAuditExportJob{}, fmt.Errorf("validation failed: idempotency_key and requested_by_api_key_id are required")
	}

	filtersRaw, err := json.Marshal(input.Filters)
	if err != nil {
		return domain.APIKeyAuditExportJob{}, err
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, input.TenantID)
	if err != nil {
		return domain.APIKeyAuditExportJob{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`INSERT INTO api_key_audit_export_jobs (
			id, tenant_id, requested_by_api_key_id, idempotency_key, status, filters, object_key, row_count, error, attempt_count, created_at, started_at, completed_at, expires_at
		) VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7,$8,$9,$10,$11,$12,$13,$14)
		RETURNING id, tenant_id, requested_by_api_key_id, idempotency_key, status, filters, object_key, row_count, error, attempt_count, created_at, started_at, completed_at, expires_at`,
		input.ID,
		input.TenantID,
		input.RequestedByAPIKeyID,
		input.IdempotencyKey,
		string(input.Status),
		filtersRaw,
		input.ObjectKey,
		input.RowCount,
		input.Error,
		input.AttemptCount,
		input.CreatedAt,
		input.StartedAt,
		input.CompletedAt,
		input.ExpiresAt,
	)
	job, err := scanAPIKeyAuditExportJob(row)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.APIKeyAuditExportJob{}, ErrAlreadyExists
		}
		return domain.APIKeyAuditExportJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.APIKeyAuditExportJob{}, err
	}
	return job, nil
}

func (s *PostgresStore) GetAPIKeyAuditExportJob(tenantID, id string) (domain.APIKeyAuditExportJob, error) {
	tenantID = normalizeTenantID(tenantID)
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.APIKeyAuditExportJob{}, ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.APIKeyAuditExportJob{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`SELECT id, tenant_id, requested_by_api_key_id, idempotency_key, status, filters, object_key, row_count, error, attempt_count, created_at, started_at, completed_at, expires_at
		FROM api_key_audit_export_jobs
		WHERE tenant_id = $1 AND id = $2`,
		tenantID,
		id,
	)
	job, err := scanAPIKeyAuditExportJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.APIKeyAuditExportJob{}, ErrNotFound
		}
		return domain.APIKeyAuditExportJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.APIKeyAuditExportJob{}, err
	}
	return job, nil
}

func (s *PostgresStore) GetAPIKeyAuditExportJobByIdempotencyKey(tenantID, idempotencyKey string) (domain.APIKeyAuditExportJob, error) {
	tenantID = normalizeTenantID(tenantID)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return domain.APIKeyAuditExportJob{}, ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.APIKeyAuditExportJob{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`SELECT id, tenant_id, requested_by_api_key_id, idempotency_key, status, filters, object_key, row_count, error, attempt_count, created_at, started_at, completed_at, expires_at
		FROM api_key_audit_export_jobs
		WHERE tenant_id = $1 AND idempotency_key = $2`,
		tenantID,
		idempotencyKey,
	)
	job, err := scanAPIKeyAuditExportJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.APIKeyAuditExportJob{}, ErrNotFound
		}
		return domain.APIKeyAuditExportJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.APIKeyAuditExportJob{}, err
	}
	return job, nil
}

func (s *PostgresStore) ListAPIKeyAuditExportJobs(filter APIKeyAuditExportFilter) (APIKeyAuditExportResult, error) {
	tenantID := normalizeTenantID(filter.TenantID)
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	cursorID := strings.TrimSpace(filter.CursorID)
	cursorCreated := filter.CursorCreated

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return APIKeyAuditExportResult{}, err
	}
	defer rollbackSilently(tx)

	baseClauses := []string{"tenant_id = $1"}
	baseArgs := []any{tenantID}
	nextArg := 2

	if status := strings.ToLower(strings.TrimSpace(filter.Status)); status != "" {
		baseClauses = append(baseClauses, fmt.Sprintf("status = $%d", nextArg))
		baseArgs = append(baseArgs, status)
		nextArg++
	}
	if requestedBy := strings.TrimSpace(filter.RequestedByAPIKeyID); requestedBy != "" {
		baseClauses = append(baseClauses, fmt.Sprintf("requested_by_api_key_id = $%d", nextArg))
		baseArgs = append(baseArgs, requestedBy)
		nextArg++
	}

	baseWhere := strings.Join(baseClauses, " AND ")

	var total int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM api_key_audit_export_jobs WHERE `+baseWhere, baseArgs...).Scan(&total); err != nil {
		return APIKeyAuditExportResult{}, err
	}

	listClauses := append([]string{}, baseClauses...)
	listArgs := append([]any{}, baseArgs...)
	listArgPos := len(listArgs) + 1
	useCursor := cursorCreated != nil && cursorID != ""
	if useCursor {
		listClauses = append(listClauses, fmt.Sprintf("(created_at < $%d OR (created_at = $%d AND id < $%d))", listArgPos, listArgPos, listArgPos+1))
		listArgs = append(listArgs, *cursorCreated, cursorID)
		listArgPos += 2
	}
	listWhere := strings.Join(listClauses, " AND ")
	limitWithPeek := limit + 1

	var (
		query     string
		queryArgs []any
	)
	if useCursor {
		query = fmt.Sprintf(`SELECT id, tenant_id, requested_by_api_key_id, idempotency_key, status, filters, object_key, row_count, error, attempt_count, created_at, started_at, completed_at, expires_at
			FROM api_key_audit_export_jobs
			WHERE %s
			ORDER BY created_at DESC, id DESC
			LIMIT $%d`, listWhere, listArgPos)
		queryArgs = append(listArgs, limitWithPeek)
	} else {
		query = fmt.Sprintf(`SELECT id, tenant_id, requested_by_api_key_id, idempotency_key, status, filters, object_key, row_count, error, attempt_count, created_at, started_at, completed_at, expires_at
			FROM api_key_audit_export_jobs
			WHERE %s
			ORDER BY created_at DESC, id DESC
			LIMIT $%d OFFSET $%d`, listWhere, listArgPos, listArgPos+1)
		queryArgs = append(listArgs, limitWithPeek, offset)
	}

	rows, err := tx.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return APIKeyAuditExportResult{}, err
	}
	defer rows.Close()

	items := make([]domain.APIKeyAuditExportJob, 0, limit)
	for rows.Next() {
		item, scanErr := scanAPIKeyAuditExportJob(rows)
		if scanErr != nil {
			return APIKeyAuditExportResult{}, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return APIKeyAuditExportResult{}, err
	}

	var (
		nextCursorCreated *time.Time
		nextCursorID      string
	)
	hasMore := len(items) > limit
	if len(items) > limit {
		items = items[:limit]
	}
	if len(items) == limit && hasMore {
		last := items[len(items)-1]
		t := last.CreatedAt.UTC()
		nextCursorCreated = &t
		nextCursorID = last.ID
	}

	if err := tx.Commit(); err != nil {
		return APIKeyAuditExportResult{}, err
	}
	return APIKeyAuditExportResult{
		Items:             items,
		Total:             total,
		Limit:             limit,
		Offset:            offset,
		NextCursorID:      nextCursorID,
		NextCursorCreated: nextCursorCreated,
	}, nil
}

func (s *PostgresStore) DequeueAPIKeyAuditExportJob() (domain.APIKeyAuditExportJob, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.APIKeyAuditExportJob{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`SELECT id, tenant_id, requested_by_api_key_id, idempotency_key, status, filters, object_key, row_count, error, attempt_count, created_at, started_at, completed_at, expires_at
		FROM api_key_audit_export_jobs
		WHERE status = $1
		ORDER BY created_at ASC
		FOR UPDATE SKIP LOCKED
		LIMIT 1`,
		string(domain.APIKeyAuditExportQueued),
	)
	job, err := scanAPIKeyAuditExportJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.APIKeyAuditExportJob{}, ErrNotFound
		}
		return domain.APIKeyAuditExportJob{}, err
	}

	now := time.Now().UTC()
	if _, err := tx.ExecContext(
		ctx,
		`UPDATE api_key_audit_export_jobs
		SET status = $1, started_at = $2, attempt_count = attempt_count + 1
		WHERE id = $3`,
		string(domain.APIKeyAuditExportRunning),
		now,
		job.ID,
	); err != nil {
		return domain.APIKeyAuditExportJob{}, err
	}

	job.Status = domain.APIKeyAuditExportRunning
	job.StartedAt = &now
	job.AttemptCount++

	if err := tx.Commit(); err != nil {
		return domain.APIKeyAuditExportJob{}, err
	}
	return job, nil
}

func (s *PostgresStore) CompleteAPIKeyAuditExportJob(id, objectKey string, rowCount int64, completedAt, expiresAt time.Time) (domain.APIKeyAuditExportJob, error) {
	id = strings.TrimSpace(id)
	objectKey = strings.TrimSpace(objectKey)
	if id == "" {
		return domain.APIKeyAuditExportJob{}, ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.APIKeyAuditExportJob{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`UPDATE api_key_audit_export_jobs
		SET status = $1, object_key = $2, row_count = $3, error = '', completed_at = $4, expires_at = $5
		WHERE id = $6
		RETURNING id, tenant_id, requested_by_api_key_id, idempotency_key, status, filters, object_key, row_count, error, attempt_count, created_at, started_at, completed_at, expires_at`,
		string(domain.APIKeyAuditExportDone),
		objectKey,
		rowCount,
		completedAt,
		expiresAt,
		id,
	)
	job, err := scanAPIKeyAuditExportJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.APIKeyAuditExportJob{}, ErrNotFound
		}
		return domain.APIKeyAuditExportJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.APIKeyAuditExportJob{}, err
	}
	return job, nil
}

func (s *PostgresStore) FailAPIKeyAuditExportJob(id, errMessage string, completedAt time.Time) (domain.APIKeyAuditExportJob, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.APIKeyAuditExportJob{}, ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.APIKeyAuditExportJob{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`UPDATE api_key_audit_export_jobs
		SET status = $1, error = $2, completed_at = $3
		WHERE id = $4
		RETURNING id, tenant_id, requested_by_api_key_id, idempotency_key, status, filters, object_key, row_count, error, attempt_count, created_at, started_at, completed_at, expires_at`,
		string(domain.APIKeyAuditExportFailed),
		strings.TrimSpace(errMessage),
		completedAt,
		id,
	)
	job, err := scanAPIKeyAuditExportJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.APIKeyAuditExportJob{}, ErrNotFound
		}
		return domain.APIKeyAuditExportJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.APIKeyAuditExportJob{}, err
	}
	return job, nil
}

func (s *PostgresStore) GetAPIKeyByPrefix(prefix string) (domain.APIKey, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return domain.APIKey{}, ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.APIKey{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`SELECT id, key_prefix, key_hash, name, role, tenant_id, owner_type, owner_id, purpose, environment, created_by_user_id, created_by_platform_user, created_at, expires_at, revoked_at, last_used_at, last_rotated_at, rotation_required_at, revocation_reason
		FROM api_keys
		WHERE key_prefix = $1`,
		prefix,
	)
	key, err := scanAPIKey(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.APIKey{}, ErrNotFound
		}
		return domain.APIKey{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.APIKey{}, err
	}
	return key, nil
}

func (s *PostgresStore) GetPlatformAPIKeyByPrefix(prefix string) (domain.PlatformAPIKey, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return domain.PlatformAPIKey{}, ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.PlatformAPIKey{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`SELECT id, key_prefix, key_hash, name, role, owner_type, owner_id, purpose, environment, created_by_user_id, created_at, expires_at, revoked_at, last_used_at, last_rotated_at, rotation_required_at, revocation_reason
		FROM platform_api_keys
		WHERE key_prefix = $1`,
		prefix,
	)
	key, err := scanPlatformAPIKey(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.PlatformAPIKey{}, ErrNotFound
		}
		return domain.PlatformAPIKey{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.PlatformAPIKey{}, err
	}
	return key, nil
}

func (s *PostgresStore) GetActiveAPIKeyByPrefix(prefix string, at time.Time) (domain.APIKey, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return domain.APIKey{}, ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	if at.IsZero() {
		at = time.Now().UTC()
	}

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.APIKey{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`SELECT id, key_prefix, key_hash, name, role, tenant_id, owner_type, owner_id, purpose, environment, created_by_user_id, created_by_platform_user, created_at, expires_at, revoked_at, last_used_at, last_rotated_at, rotation_required_at, revocation_reason
		FROM api_keys
		WHERE key_prefix = $1
		  AND revoked_at IS NULL
		  AND (expires_at IS NULL OR expires_at > $2)`,
		prefix,
		at,
	)
	key, err := scanAPIKey(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.APIKey{}, ErrNotFound
		}
		return domain.APIKey{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.APIKey{}, err
	}
	return key, nil
}

func (s *PostgresStore) GetActivePlatformAPIKeyByPrefix(prefix string, at time.Time) (domain.PlatformAPIKey, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return domain.PlatformAPIKey{}, ErrNotFound
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.PlatformAPIKey{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`SELECT id, key_prefix, key_hash, name, role, owner_type, owner_id, purpose, environment, created_by_user_id, created_at, expires_at, revoked_at, last_used_at, last_rotated_at, rotation_required_at, revocation_reason
		FROM platform_api_keys
		WHERE key_prefix = $1
		  AND revoked_at IS NULL
		  AND (expires_at IS NULL OR expires_at > $2)`,
		prefix,
		at,
	)
	key, err := scanPlatformAPIKey(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.PlatformAPIKey{}, ErrNotFound
		}
		return domain.PlatformAPIKey{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.PlatformAPIKey{}, err
	}
	return key, nil
}

func (s *PostgresStore) TouchAPIKeyLastUsed(id string, usedAt time.Time) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	if usedAt.IsZero() {
		usedAt = time.Now().UTC()
	}

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return err
	}
	defer rollbackSilently(tx)

	res, err := tx.ExecContext(ctx, `UPDATE api_keys SET last_used_at = $1 WHERE id = $2`, usedAt, id)
	if err != nil {
		return err
	}
	updated, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if updated == 0 {
		return ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (s *PostgresStore) TouchPlatformAPIKeyLastUsed(id string, usedAt time.Time) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	if usedAt.IsZero() {
		usedAt = time.Now().UTC()
	}

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return err
	}
	defer rollbackSilently(tx)

	res, err := tx.ExecContext(ctx, `UPDATE platform_api_keys SET last_used_at = $1 WHERE id = $2`, usedAt, id)
	if err != nil {
		return err
	}
	updated, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if updated == 0 {
		return ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (s *PostgresStore) CountActivePlatformAPIKeys(at time.Time) (int, error) {
	if at.IsZero() {
		at = time.Now().UTC()
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return 0, err
	}
	defer rollbackSilently(tx)

	var count int
	if err := tx.QueryRowContext(
		ctx,
		`SELECT COUNT(*)
		FROM platform_api_keys
		WHERE revoked_at IS NULL
		  AND (expires_at IS NULL OR expires_at > $1)`,
		at,
	).Scan(&count); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *PostgresStore) RevokeActivePlatformAPIKeysByName(name string, revokedAt time.Time) (int, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, ErrNotFound
	}
	if revokedAt.IsZero() {
		revokedAt = time.Now().UTC()
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return 0, err
	}
	defer rollbackSilently(tx)

	res, err := tx.ExecContext(
		ctx,
		`UPDATE platform_api_keys
		SET revoked_at = COALESCE(revoked_at, $1)
		WHERE name = $2
		  AND revoked_at IS NULL
		  AND (expires_at IS NULL OR expires_at > $1)`,
		revokedAt,
		name,
	)
	if err != nil {
		return 0, err
	}
	updated, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return int(updated), nil
}

func (s *PostgresStore) withTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), s.queryTimeout)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRatingRule(s rowScanner) (domain.RatingRuleVersion, error) {
	var out domain.RatingRuleVersion
	var ruleKey string
	var lifecycleState string
	var mode string
	var tiersRaw []byte
	if err := s.Scan(
		&out.ID,
		&out.TenantID,
		&ruleKey,
		&out.Name,
		&out.Version,
		&lifecycleState,
		&mode,
		&out.Currency,
		&out.FlatAmountCents,
		&tiersRaw,
		&out.PackageSize,
		&out.PackageAmountCents,
		&out.OverageUnitAmountCents,
		&out.CreatedAt,
	); err != nil {
		return domain.RatingRuleVersion{}, err
	}
	out.TenantID = normalizeTenantID(out.TenantID)
	out.RuleKey = strings.TrimSpace(ruleKey)
	out.LifecycleState = normalizeRatingRuleLifecycleState(domain.RatingRuleLifecycleState(lifecycleState))
	out.Mode = domain.PricingMode(mode)
	if len(tiersRaw) == 0 {
		out.GraduatedTiers = []domain.RatingTier{}
		return out, nil
	}
	if err := json.Unmarshal(tiersRaw, &out.GraduatedTiers); err != nil {
		return domain.RatingRuleVersion{}, err
	}
	return out, nil
}

func scanPlan(s rowScanner) (domain.Plan, error) {
	var out domain.Plan
	var description sql.NullString
	var billingInterval string
	var status string
	if err := s.Scan(&out.ID, &out.TenantID, &out.Code, &out.Name, &description, &out.Currency, &billingInterval, &status, &out.BaseAmountCents, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return domain.Plan{}, err
	}
	out.TenantID = normalizeTenantID(out.TenantID)
	out.BillingInterval = domain.BillingInterval(strings.TrimSpace(billingInterval))
	out.Status = domain.PlanStatus(strings.TrimSpace(status))
	if description.Valid {
		out.Description = normalizeOptionalText(description.String)
	}
	return out, nil
}

func scanSubscription(s rowScanner) (domain.Subscription, error) {
	var out domain.Subscription
	var status string
	var paymentSetupRequestedAt sql.NullTime
	var activatedAt sql.NullTime
	if err := s.Scan(
		&out.ID,
		&out.TenantID,
		&out.Code,
		&out.DisplayName,
		&out.CustomerID,
		&out.PlanID,
		&status,
		&paymentSetupRequestedAt,
		&activatedAt,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		return domain.Subscription{}, err
	}
	out.TenantID = normalizeTenantID(out.TenantID)
	out.Status = normalizeSubscriptionStatus(domain.SubscriptionStatus(status))
	if paymentSetupRequestedAt.Valid {
		value := paymentSetupRequestedAt.Time.UTC()
		out.PaymentSetupRequestedAt = &value
	}
	if activatedAt.Valid {
		value := activatedAt.Time.UTC()
		out.ActivatedAt = &value
	}
	return out, nil
}

func scanMeter(s rowScanner) (domain.Meter, error) {
	var out domain.Meter
	if err := s.Scan(
		&out.ID,
		&out.TenantID,
		&out.Key,
		&out.Name,
		&out.Unit,
		&out.Aggregation,
		&out.RatingRuleVersionID,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		return domain.Meter{}, err
	}
	out.TenantID = normalizeTenantID(out.TenantID)
	return out, nil
}

func scanCustomer(s rowScanner) (domain.Customer, error) {
	var out domain.Customer
	var email sql.NullString
	var status string
	var lagoCustomerID sql.NullString
	if err := s.Scan(&out.ID, &out.TenantID, &out.ExternalID, &out.DisplayName, &email, &status, &lagoCustomerID, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return domain.Customer{}, err
	}
	out.TenantID = normalizeTenantID(out.TenantID)
	out.ExternalID = strings.TrimSpace(out.ExternalID)
	out.DisplayName = strings.TrimSpace(out.DisplayName)
	if email.Valid {
		out.Email = normalizeOptionalText(email.String)
	}
	if lagoCustomerID.Valid {
		out.LagoCustomerID = normalizeOptionalText(lagoCustomerID.String)
	}
	out.Status = normalizeCustomerStatus(domain.CustomerStatus(status))
	return out, nil
}

func scanCustomerBillingProfile(s rowScanner) (domain.CustomerBillingProfile, error) {
	var out domain.CustomerBillingProfile
	var legalName, email, phone, line1, line2, city, state, postal, country, currency, taxID, providerCode, lastSyncError sql.NullString
	var status string
	var lastSyncedAt sql.NullTime
	if err := s.Scan(&out.CustomerID, &out.TenantID, &legalName, &email, &phone, &line1, &line2, &city, &state, &postal, &country, &currency, &taxID, &providerCode, &status, &lastSyncedAt, &lastSyncError, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return domain.CustomerBillingProfile{}, err
	}
	out.TenantID = normalizeTenantID(out.TenantID)
	out.CustomerID = strings.TrimSpace(out.CustomerID)
	return finalizeCustomerBillingProfile(out, legalName, email, phone, line1, line2, city, state, postal, country, currency, taxID, providerCode, status, lastSyncedAt, lastSyncError), nil
}

func finalizeCustomerBillingProfile(out domain.CustomerBillingProfile, legalName, email, phone, line1, line2, city, state, postal, country, currency, taxID, providerCode sql.NullString, status string, lastSyncedAt sql.NullTime, lastSyncError sql.NullString) domain.CustomerBillingProfile {
	if legalName.Valid {
		out.LegalName = normalizeOptionalText(legalName.String)
	}
	if email.Valid {
		out.Email = normalizeOptionalText(email.String)
	}
	if phone.Valid {
		out.Phone = normalizeOptionalText(phone.String)
	}
	if line1.Valid {
		out.AddressLine1 = normalizeOptionalText(line1.String)
	}
	if line2.Valid {
		out.AddressLine2 = normalizeOptionalText(line2.String)
	}
	if city.Valid {
		out.City = normalizeOptionalText(city.String)
	}
	if state.Valid {
		out.State = normalizeOptionalText(state.String)
	}
	if postal.Valid {
		out.PostalCode = normalizeOptionalText(postal.String)
	}
	if country.Valid {
		out.Country = normalizeOptionalText(country.String)
	}
	if currency.Valid {
		out.Currency = normalizeOptionalText(currency.String)
	}
	if taxID.Valid {
		out.TaxIdentifier = normalizeOptionalText(taxID.String)
	}
	if providerCode.Valid {
		out.ProviderCode = normalizeOptionalText(providerCode.String)
	}
	if lastSyncError.Valid {
		out.LastSyncError = normalizeOptionalText(lastSyncError.String)
	}
	if lastSyncedAt.Valid {
		t := lastSyncedAt.Time.UTC()
		out.LastSyncedAt = &t
	}
	out.ProfileStatus = normalizeBillingProfileStatus(domain.BillingProfileStatus(status))
	return out
}

func scanCustomerPaymentSetup(s rowScanner) (domain.CustomerPaymentSetup, error) {
	var out domain.CustomerPaymentSetup
	var status string
	var paymentMethodType, providerCustomerRef, providerPaymentMethodRef, lastVerificationResult, lastVerificationError sql.NullString
	var lastVerifiedAt sql.NullTime
	if err := s.Scan(&out.CustomerID, &out.TenantID, &status, &out.DefaultPaymentMethodPresent, &paymentMethodType, &providerCustomerRef, &providerPaymentMethodRef, &lastVerifiedAt, &lastVerificationResult, &lastVerificationError, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return domain.CustomerPaymentSetup{}, err
	}
	out.TenantID = normalizeTenantID(out.TenantID)
	out.CustomerID = strings.TrimSpace(out.CustomerID)
	if paymentMethodType.Valid {
		out.PaymentMethodType = normalizeOptionalText(paymentMethodType.String)
	}
	if providerCustomerRef.Valid {
		out.ProviderCustomerReference = normalizeOptionalText(providerCustomerRef.String)
	}
	if providerPaymentMethodRef.Valid {
		out.ProviderPaymentMethodReference = normalizeOptionalText(providerPaymentMethodRef.String)
	}
	if lastVerificationResult.Valid {
		out.LastVerificationResult = normalizeOptionalText(lastVerificationResult.String)
	}
	if lastVerificationError.Valid {
		out.LastVerificationError = normalizeOptionalText(lastVerificationError.String)
	}
	if lastVerifiedAt.Valid {
		t := lastVerifiedAt.Time.UTC()
		out.LastVerifiedAt = &t
	}
	out.SetupStatus = normalizePaymentSetupStatus(domain.PaymentSetupStatus(status))
	return out, nil
}

func scanReplayJob(s rowScanner) (domain.ReplayJob, error) {
	var out domain.ReplayJob
	var tenantID string
	var customerID sql.NullString
	var meterID sql.NullString
	var from sql.NullTime
	var to sql.NullTime
	var status string
	var lastAttemptAt sql.NullTime
	var startedAt sql.NullTime
	var completedAt sql.NullTime

	if err := s.Scan(
		&out.ID,
		&tenantID,
		&customerID,
		&meterID,
		&from,
		&to,
		&out.IdempotencyKey,
		&status,
		&out.AttemptCount,
		&lastAttemptAt,
		&out.ProcessedRecords,
		&out.Error,
		&out.CreatedAt,
		&startedAt,
		&completedAt,
	); err != nil {
		return domain.ReplayJob{}, err
	}

	out.TenantID = normalizeTenantID(tenantID)
	if customerID.Valid {
		out.CustomerID = customerID.String
	}
	if meterID.Valid {
		out.MeterID = meterID.String
	}
	if from.Valid {
		t := from.Time.UTC()
		out.From = &t
	}
	if to.Valid {
		t := to.Time.UTC()
		out.To = &t
	}
	if startedAt.Valid {
		t := startedAt.Time.UTC()
		out.StartedAt = &t
	}
	if lastAttemptAt.Valid {
		t := lastAttemptAt.Time.UTC()
		out.LastAttemptAt = &t
	}
	if completedAt.Valid {
		t := completedAt.Time.UTC()
		out.CompletedAt = &t
	}
	out.Status = domain.ReplayJobStatus(status)
	return out, nil
}

func scanTenant(s rowScanner) (domain.Tenant, error) {
	var out domain.Tenant
	var status string
	var billingProviderConnectionID sql.NullString
	var lagoOrganizationID sql.NullString
	var lagoBillingProviderCode sql.NullString
	if err := s.Scan(&out.ID, &out.Name, &status, &billingProviderConnectionID, &lagoOrganizationID, &lagoBillingProviderCode, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return domain.Tenant{}, err
	}
	out.ID = normalizeTenantID(out.ID)
	out.Name = strings.TrimSpace(out.Name)
	if billingProviderConnectionID.Valid {
		out.BillingProviderConnectionID = normalizeOptionalText(billingProviderConnectionID.String)
	}
	if lagoOrganizationID.Valid {
		out.LagoOrganizationID = normalizeOptionalText(lagoOrganizationID.String)
	}
	if lagoBillingProviderCode.Valid {
		out.LagoBillingProviderCode = normalizeOptionalText(lagoBillingProviderCode.String)
	}
	out.Status = normalizeTenantStatus(domain.TenantStatus(status))
	return out, nil
}

func scanWorkspaceBillingBinding(s rowScanner) (domain.WorkspaceBillingBinding, error) {
	var out domain.WorkspaceBillingBinding
	var backend string
	var backendOrganizationID sql.NullString
	var backendProviderCode sql.NullString
	var isolationMode string
	var status string
	var provisioningError sql.NullString
	var lastVerifiedAt sql.NullTime
	var connectedAt sql.NullTime
	var disabledAt sql.NullTime
	var createdByID sql.NullString
	if err := s.Scan(
		&out.ID,
		&out.WorkspaceID,
		&out.BillingProviderConnectionID,
		&backend,
		&backendOrganizationID,
		&backendProviderCode,
		&isolationMode,
		&status,
		&provisioningError,
		&lastVerifiedAt,
		&connectedAt,
		&disabledAt,
		&out.CreatedByType,
		&createdByID,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		return domain.WorkspaceBillingBinding{}, err
	}
	out.ID = strings.TrimSpace(out.ID)
	out.WorkspaceID = normalizeTenantID(out.WorkspaceID)
	out.BillingProviderConnectionID = strings.TrimSpace(out.BillingProviderConnectionID)
	out.Backend = domain.WorkspaceBillingBackend(strings.ToLower(strings.TrimSpace(backend)))
	if backendOrganizationID.Valid {
		out.BackendOrganizationID = normalizeOptionalText(backendOrganizationID.String)
	}
	if backendProviderCode.Valid {
		out.BackendProviderCode = normalizeOptionalText(backendProviderCode.String)
	}
	out.IsolationMode = domain.WorkspaceBillingIsolationMode(strings.ToLower(strings.TrimSpace(isolationMode)))
	out.Status = domain.WorkspaceBillingBindingStatus(strings.ToLower(strings.TrimSpace(status)))
	if provisioningError.Valid {
		out.ProvisioningError = normalizeOptionalText(provisioningError.String)
	}
	out.CreatedByType = strings.ToLower(strings.TrimSpace(out.CreatedByType))
	if createdByID.Valid {
		out.CreatedByID = normalizeOptionalText(createdByID.String)
	}
	if lastVerifiedAt.Valid {
		t := lastVerifiedAt.Time.UTC()
		out.LastVerifiedAt = &t
	}
	if connectedAt.Valid {
		t := connectedAt.Time.UTC()
		out.ConnectedAt = &t
	}
	if disabledAt.Valid {
		t := disabledAt.Time.UTC()
		out.DisabledAt = &t
	}
	return out, nil
}

func scanWorkspaceInvitation(s rowScanner) (domain.WorkspaceInvitation, error) {
	var out domain.WorkspaceInvitation
	var status string
	var acceptedAt sql.NullTime
	var acceptedByUserID sql.NullString
	var invitedByUserID sql.NullString
	var revokedAt sql.NullTime
	if err := s.Scan(
		&out.ID,
		&out.WorkspaceID,
		&out.Email,
		&out.Role,
		&status,
		&out.TokenHash,
		&out.ExpiresAt,
		&acceptedAt,
		&acceptedByUserID,
		&invitedByUserID,
		&out.InvitedByPlatformUser,
		&revokedAt,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		return domain.WorkspaceInvitation{}, err
	}
	out.ID = strings.TrimSpace(out.ID)
	out.WorkspaceID = normalizeTenantID(out.WorkspaceID)
	out.Email = strings.ToLower(strings.TrimSpace(out.Email))
	out.Role = strings.ToLower(strings.TrimSpace(out.Role))
	out.Status = domain.WorkspaceInvitationStatus(strings.ToLower(strings.TrimSpace(status)))
	out.TokenHash = strings.TrimSpace(out.TokenHash)
	if acceptedAt.Valid {
		t := acceptedAt.Time.UTC()
		out.AcceptedAt = &t
	}
	if acceptedByUserID.Valid {
		out.AcceptedByUserID = strings.TrimSpace(acceptedByUserID.String)
	}
	if invitedByUserID.Valid {
		out.InvitedByUserID = strings.TrimSpace(invitedByUserID.String)
	}
	if revokedAt.Valid {
		t := revokedAt.Time.UTC()
		out.RevokedAt = &t
	}
	return out, nil
}

func scanBillingProviderConnection(s rowScanner) (domain.BillingProviderConnection, error) {
	var out domain.BillingProviderConnection
	var providerType string
	var scope string
	var status string
	var ownerTenantID sql.NullString
	var lagoOrganizationID sql.NullString
	var lagoProviderCode sql.NullString
	var secretRef sql.NullString
	var lastSyncedAt sql.NullTime
	var lastSyncError sql.NullString
	var connectedAt sql.NullTime
	var disabledAt sql.NullTime
	var createdByID sql.NullString

	if err := s.Scan(
		&out.ID,
		&providerType,
		&out.Environment,
		&out.DisplayName,
		&scope,
		&ownerTenantID,
		&status,
		&lagoOrganizationID,
		&lagoProviderCode,
		&secretRef,
		&lastSyncedAt,
		&lastSyncError,
		&connectedAt,
		&disabledAt,
		&out.CreatedByType,
		&createdByID,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		return domain.BillingProviderConnection{}, err
	}

	out.ProviderType = domain.BillingProviderType(strings.ToLower(strings.TrimSpace(providerType)))
	out.Environment = strings.ToLower(strings.TrimSpace(out.Environment))
	out.DisplayName = strings.TrimSpace(out.DisplayName)
	out.Scope = domain.BillingProviderConnectionScope(strings.ToLower(strings.TrimSpace(scope)))
	out.Status = domain.BillingProviderConnectionStatus(strings.ToLower(strings.TrimSpace(status)))
	out.CreatedByType = strings.ToLower(strings.TrimSpace(out.CreatedByType))
	if ownerTenantID.Valid {
		out.OwnerTenantID = normalizeOptionalText(ownerTenantID.String)
	}
	if lagoOrganizationID.Valid {
		out.LagoOrganizationID = normalizeOptionalText(lagoOrganizationID.String)
	}
	if lagoProviderCode.Valid {
		out.LagoProviderCode = normalizeOptionalText(lagoProviderCode.String)
	}
	if secretRef.Valid {
		out.SecretRef = normalizeOptionalText(secretRef.String)
	}
	if lastSyncedAt.Valid {
		t := lastSyncedAt.Time.UTC()
		out.LastSyncedAt = &t
	}
	if lastSyncError.Valid {
		out.LastSyncError = normalizeOptionalText(lastSyncError.String)
	}
	if connectedAt.Valid {
		t := connectedAt.Time.UTC()
		out.ConnectedAt = &t
	}
	if disabledAt.Valid {
		t := disabledAt.Time.UTC()
		out.DisabledAt = &t
	}
	if createdByID.Valid {
		out.CreatedByID = normalizeOptionalText(createdByID.String)
	}
	return out, nil
}

func scanUser(s rowScanner) (domain.User, error) {
	var out domain.User
	var status string
	var platformRole string
	if err := s.Scan(&out.ID, &out.Email, &out.DisplayName, &status, &platformRole, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return domain.User{}, err
	}
	out.ID = strings.TrimSpace(out.ID)
	out.Email = strings.ToLower(strings.TrimSpace(out.Email))
	out.DisplayName = strings.TrimSpace(out.DisplayName)
	out.Status = domain.UserStatus(strings.ToLower(strings.TrimSpace(status)))
	out.PlatformRole = domain.UserPlatformRole(strings.ToLower(strings.TrimSpace(platformRole)))
	return out, nil
}

func scanUserPasswordCredential(s rowScanner) (domain.UserPasswordCredential, error) {
	var out domain.UserPasswordCredential
	if err := s.Scan(&out.UserID, &out.PasswordHash, &out.PasswordUpdatedAt, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return domain.UserPasswordCredential{}, err
	}
	out.UserID = strings.TrimSpace(out.UserID)
	out.PasswordHash = strings.TrimSpace(out.PasswordHash)
	return out, nil
}

func scanPasswordResetToken(s rowScanner) (domain.PasswordResetToken, error) {
	var out domain.PasswordResetToken
	if err := s.Scan(&out.ID, &out.UserID, &out.TokenHash, &out.ExpiresAt, &out.UsedAt, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return domain.PasswordResetToken{}, err
	}
	out.ID = strings.TrimSpace(out.ID)
	out.UserID = strings.TrimSpace(out.UserID)
	out.TokenHash = strings.TrimSpace(out.TokenHash)
	return out, nil
}

func scanUserTenantMembership(s rowScanner) (domain.UserTenantMembership, error) {
	var out domain.UserTenantMembership
	var status string
	if err := s.Scan(&out.UserID, &out.TenantID, &out.Role, &status, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return domain.UserTenantMembership{}, err
	}
	out.UserID = strings.TrimSpace(out.UserID)
	out.TenantID = normalizeTenantID(out.TenantID)
	out.Role = strings.ToLower(strings.TrimSpace(out.Role))
	out.Status = domain.UserTenantMembershipStatus(strings.ToLower(strings.TrimSpace(status)))
	return out, nil
}

func scanUserFederatedIdentity(s rowScanner) (domain.UserFederatedIdentity, error) {
	var out domain.UserFederatedIdentity
	var providerType string
	if err := s.Scan(&out.ID, &out.UserID, &out.ProviderKey, &providerType, &out.Subject, &out.Email, &out.EmailVerified, &out.LastLoginAt, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return domain.UserFederatedIdentity{}, err
	}
	out.ID = strings.TrimSpace(out.ID)
	out.UserID = strings.TrimSpace(out.UserID)
	out.ProviderKey = strings.ToLower(strings.TrimSpace(out.ProviderKey))
	out.ProviderType = domain.BrowserSSOProviderType(strings.ToLower(strings.TrimSpace(providerType)))
	out.Subject = strings.TrimSpace(out.Subject)
	out.Email = strings.ToLower(strings.TrimSpace(out.Email))
	return out, nil
}

func scanServiceAccount(s rowScanner) (domain.ServiceAccount, error) {
	var out domain.ServiceAccount
	var tenantID sql.NullString
	var description sql.NullString
	var purpose sql.NullString
	var environment sql.NullString
	var createdByUserID sql.NullString
	var createdByPlatformUser sql.NullBool

	if err := s.Scan(
		&out.ID,
		&tenantID,
		&out.Name,
		&description,
		&out.Role,
		&purpose,
		&environment,
		&createdByUserID,
		&createdByPlatformUser,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		return domain.ServiceAccount{}, err
	}
	if tenantID.Valid {
		out.TenantID = normalizeTenantID(tenantID.String)
	} else {
		out.TenantID = defaultTenantID
	}
	out.Description = strings.TrimSpace(description.String)
	out.Purpose = strings.TrimSpace(purpose.String)
	out.Environment = strings.TrimSpace(environment.String)
	out.CreatedByUserID = strings.TrimSpace(createdByUserID.String)
	out.CreatedByPlatformUser = createdByPlatformUser.Valid && createdByPlatformUser.Bool
	return out, nil
}

func scanAPIKey(s rowScanner) (domain.APIKey, error) {
	var out domain.APIKey
	var tenantID sql.NullString
	var ownerType sql.NullString
	var ownerID sql.NullString
	var purpose sql.NullString
	var environment sql.NullString
	var createdByUserID sql.NullString
	var createdByPlatformUser sql.NullBool
	var expiresAt sql.NullTime
	var revokedAt sql.NullTime
	var lastUsedAt sql.NullTime
	var lastRotatedAt sql.NullTime
	var rotationRequiredAt sql.NullTime
	var revocationReason sql.NullString

	if err := s.Scan(
		&out.ID,
		&out.KeyPrefix,
		&out.KeyHash,
		&out.Name,
		&out.Role,
		&tenantID,
		&ownerType,
		&ownerID,
		&purpose,
		&environment,
		&createdByUserID,
		&createdByPlatformUser,
		&out.CreatedAt,
		&expiresAt,
		&revokedAt,
		&lastUsedAt,
		&lastRotatedAt,
		&rotationRequiredAt,
		&revocationReason,
	); err != nil {
		return domain.APIKey{}, err
	}

	if tenantID.Valid {
		out.TenantID = normalizeTenantID(tenantID.String)
	} else {
		out.TenantID = defaultTenantID
	}
	out.OwnerType = strings.TrimSpace(ownerType.String)
	out.OwnerID = strings.TrimSpace(ownerID.String)
	out.Purpose = strings.TrimSpace(purpose.String)
	out.Environment = strings.TrimSpace(environment.String)
	out.CreatedByUserID = strings.TrimSpace(createdByUserID.String)
	out.CreatedByPlatformUser = createdByPlatformUser.Valid && createdByPlatformUser.Bool
	if expiresAt.Valid {
		t := expiresAt.Time.UTC()
		out.ExpiresAt = &t
	}
	if revokedAt.Valid {
		t := revokedAt.Time.UTC()
		out.RevokedAt = &t
	}
	if lastUsedAt.Valid {
		t := lastUsedAt.Time.UTC()
		out.LastUsedAt = &t
	}
	if lastRotatedAt.Valid {
		t := lastRotatedAt.Time.UTC()
		out.LastRotatedAt = &t
	}
	if rotationRequiredAt.Valid {
		t := rotationRequiredAt.Time.UTC()
		out.RotationRequiredAt = &t
	}
	out.RevocationReason = strings.TrimSpace(revocationReason.String)
	return out, nil
}

func scanPlatformAPIKey(s rowScanner) (domain.PlatformAPIKey, error) {
	var out domain.PlatformAPIKey
	var ownerType sql.NullString
	var ownerID sql.NullString
	var purpose sql.NullString
	var environment sql.NullString
	var createdByUserID sql.NullString
	var expiresAt sql.NullTime
	var revokedAt sql.NullTime
	var lastUsedAt sql.NullTime
	var lastRotatedAt sql.NullTime
	var rotationRequiredAt sql.NullTime
	var revocationReason sql.NullString

	if err := s.Scan(
		&out.ID,
		&out.KeyPrefix,
		&out.KeyHash,
		&out.Name,
		&out.Role,
		&ownerType,
		&ownerID,
		&purpose,
		&environment,
		&createdByUserID,
		&out.CreatedAt,
		&expiresAt,
		&revokedAt,
		&lastUsedAt,
		&lastRotatedAt,
		&rotationRequiredAt,
		&revocationReason,
	); err != nil {
		return domain.PlatformAPIKey{}, err
	}

	out.OwnerType = strings.TrimSpace(ownerType.String)
	out.OwnerID = strings.TrimSpace(ownerID.String)
	out.Purpose = strings.TrimSpace(purpose.String)
	out.Environment = strings.TrimSpace(environment.String)
	out.CreatedByUserID = strings.TrimSpace(createdByUserID.String)
	if expiresAt.Valid {
		t := expiresAt.Time.UTC()
		out.ExpiresAt = &t
	}
	if revokedAt.Valid {
		t := revokedAt.Time.UTC()
		out.RevokedAt = &t
	}
	if lastUsedAt.Valid {
		t := lastUsedAt.Time.UTC()
		out.LastUsedAt = &t
	}
	if lastRotatedAt.Valid {
		t := lastRotatedAt.Time.UTC()
		out.LastRotatedAt = &t
	}
	if rotationRequiredAt.Valid {
		t := rotationRequiredAt.Time.UTC()
		out.RotationRequiredAt = &t
	}
	out.RevocationReason = strings.TrimSpace(revocationReason.String)
	return out, nil
}

func scanAPIKeyAuditEvent(s rowScanner) (domain.APIKeyAuditEvent, error) {
	var out domain.APIKeyAuditEvent
	var actorAPIKeyID sql.NullString
	var metadataRaw []byte

	if err := s.Scan(
		&out.ID,
		&out.TenantID,
		&out.APIKeyID,
		&actorAPIKeyID,
		&out.Action,
		&metadataRaw,
		&out.CreatedAt,
	); err != nil {
		return domain.APIKeyAuditEvent{}, err
	}

	out.TenantID = normalizeTenantID(out.TenantID)
	if actorAPIKeyID.Valid {
		out.ActorAPIKeyID = actorAPIKeyID.String
	}
	if len(metadataRaw) == 0 {
		out.Metadata = map[string]any{}
		return out, nil
	}
	if err := json.Unmarshal(metadataRaw, &out.Metadata); err != nil {
		return domain.APIKeyAuditEvent{}, err
	}
	if out.Metadata == nil {
		out.Metadata = map[string]any{}
	}
	return out, nil
}

func scanTenantAuditEvent(s rowScanner) (domain.TenantAuditEvent, error) {
	var out domain.TenantAuditEvent
	var actorAPIKeyID sql.NullString
	var metadataRaw []byte

	if err := s.Scan(
		&out.ID,
		&out.TenantID,
		&actorAPIKeyID,
		&out.Action,
		&metadataRaw,
		&out.CreatedAt,
	); err != nil {
		return domain.TenantAuditEvent{}, err
	}

	out.TenantID = normalizeTenantID(out.TenantID)
	if actorAPIKeyID.Valid {
		out.ActorAPIKeyID = actorAPIKeyID.String
	}
	if len(metadataRaw) == 0 {
		out.Metadata = map[string]any{}
		return out, nil
	}
	if err := json.Unmarshal(metadataRaw, &out.Metadata); err != nil {
		return domain.TenantAuditEvent{}, err
	}
	if out.Metadata == nil {
		out.Metadata = map[string]any{}
	}
	return out, nil
}

func scanAPIKeyAuditExportJob(s rowScanner) (domain.APIKeyAuditExportJob, error) {
	var out domain.APIKeyAuditExportJob
	var filtersRaw []byte
	var startedAt sql.NullTime
	var completedAt sql.NullTime
	var expiresAt sql.NullTime

	if err := s.Scan(
		&out.ID,
		&out.TenantID,
		&out.RequestedByAPIKeyID,
		&out.IdempotencyKey,
		&out.Status,
		&filtersRaw,
		&out.ObjectKey,
		&out.RowCount,
		&out.Error,
		&out.AttemptCount,
		&out.CreatedAt,
		&startedAt,
		&completedAt,
		&expiresAt,
	); err != nil {
		return domain.APIKeyAuditExportJob{}, err
	}

	out.TenantID = normalizeTenantID(out.TenantID)
	if len(filtersRaw) == 0 {
		out.Filters = domain.APIKeyAuditExportFilters{}
	} else if err := json.Unmarshal(filtersRaw, &out.Filters); err != nil {
		return domain.APIKeyAuditExportJob{}, err
	}

	if startedAt.Valid {
		t := startedAt.Time.UTC()
		out.StartedAt = &t
	}
	if completedAt.Valid {
		t := completedAt.Time.UTC()
		out.CompletedAt = &t
	}
	if expiresAt.Valid {
		t := expiresAt.Time.UTC()
		out.ExpiresAt = &t
	}
	return out, nil
}

func scanInvoicePaymentStatusView(s rowScanner) (domain.InvoicePaymentStatusView, error) {
	var out domain.InvoicePaymentStatusView
	var customerExternalID sql.NullString
	var invoiceNumber sql.NullString
	var currency sql.NullString
	var invoiceStatus sql.NullString
	var paymentStatus sql.NullString
	var paymentOverdue sql.NullBool
	var totalAmount sql.NullInt64
	var totalDueAmount sql.NullInt64
	var totalPaidAmount sql.NullInt64
	var lastPaymentError sql.NullString

	if err := s.Scan(
		&out.TenantID,
		&out.OrganizationID,
		&out.InvoiceID,
		&customerExternalID,
		&invoiceNumber,
		&currency,
		&invoiceStatus,
		&paymentStatus,
		&paymentOverdue,
		&totalAmount,
		&totalDueAmount,
		&totalPaidAmount,
		&lastPaymentError,
		&out.LastEventType,
		&out.LastEventAt,
		&out.LastWebhookKey,
		&out.UpdatedAt,
	); err != nil {
		return domain.InvoicePaymentStatusView{}, err
	}
	out.TenantID = normalizeTenantID(out.TenantID)
	if customerExternalID.Valid {
		out.CustomerExternalID = strings.TrimSpace(customerExternalID.String)
	}
	if invoiceNumber.Valid {
		out.InvoiceNumber = strings.TrimSpace(invoiceNumber.String)
	}
	if currency.Valid {
		out.Currency = strings.TrimSpace(currency.String)
	}
	if invoiceStatus.Valid {
		out.InvoiceStatus = strings.TrimSpace(invoiceStatus.String)
	}
	if paymentStatus.Valid {
		out.PaymentStatus = strings.TrimSpace(paymentStatus.String)
	}
	if paymentOverdue.Valid {
		v := paymentOverdue.Bool
		out.PaymentOverdue = &v
	}
	if totalAmount.Valid {
		v := totalAmount.Int64
		out.TotalAmountCents = &v
	}
	if totalDueAmount.Valid {
		v := totalDueAmount.Int64
		out.TotalDueAmountCents = &v
	}
	if totalPaidAmount.Valid {
		v := totalPaidAmount.Int64
		out.TotalPaidAmountCents = &v
	}
	if lastPaymentError.Valid {
		out.LastPaymentError = strings.TrimSpace(lastPaymentError.String)
	}
	return out, nil
}

func scanLagoWebhookEvent(s rowScanner) (domain.LagoWebhookEvent, error) {
	var out domain.LagoWebhookEvent
	var invoiceID sql.NullString
	var paymentRequestID sql.NullString
	var dunningCampaignCode sql.NullString
	var customerExternalID sql.NullString
	var invoiceNumber sql.NullString
	var currency sql.NullString
	var invoiceStatus sql.NullString
	var paymentStatus sql.NullString
	var paymentOverdue sql.NullBool
	var totalAmount sql.NullInt64
	var totalDueAmount sql.NullInt64
	var totalPaidAmount sql.NullInt64
	var lastPaymentError sql.NullString
	var payloadRaw []byte

	if err := s.Scan(
		&out.ID,
		&out.TenantID,
		&out.OrganizationID,
		&out.WebhookKey,
		&out.WebhookType,
		&out.ObjectType,
		&invoiceID,
		&paymentRequestID,
		&dunningCampaignCode,
		&customerExternalID,
		&invoiceNumber,
		&currency,
		&invoiceStatus,
		&paymentStatus,
		&paymentOverdue,
		&totalAmount,
		&totalDueAmount,
		&totalPaidAmount,
		&lastPaymentError,
		&payloadRaw,
		&out.ReceivedAt,
		&out.OccurredAt,
	); err != nil {
		return domain.LagoWebhookEvent{}, err
	}

	out.TenantID = normalizeTenantID(out.TenantID)
	if invoiceID.Valid {
		out.InvoiceID = strings.TrimSpace(invoiceID.String)
	}
	if paymentRequestID.Valid {
		out.PaymentRequestID = strings.TrimSpace(paymentRequestID.String)
	}
	if dunningCampaignCode.Valid {
		out.DunningCampaignCode = strings.TrimSpace(dunningCampaignCode.String)
	}
	if customerExternalID.Valid {
		out.CustomerExternalID = strings.TrimSpace(customerExternalID.String)
	}
	if invoiceNumber.Valid {
		out.InvoiceNumber = strings.TrimSpace(invoiceNumber.String)
	}
	if currency.Valid {
		out.Currency = strings.TrimSpace(currency.String)
	}
	if invoiceStatus.Valid {
		out.InvoiceStatus = strings.TrimSpace(invoiceStatus.String)
	}
	if paymentStatus.Valid {
		out.PaymentStatus = strings.TrimSpace(paymentStatus.String)
	}
	if paymentOverdue.Valid {
		v := paymentOverdue.Bool
		out.PaymentOverdue = &v
	}
	if totalAmount.Valid {
		v := totalAmount.Int64
		out.TotalAmountCents = &v
	}
	if totalDueAmount.Valid {
		v := totalDueAmount.Int64
		out.TotalDueAmountCents = &v
	}
	if totalPaidAmount.Valid {
		v := totalPaidAmount.Int64
		out.TotalPaidAmountCents = &v
	}
	if lastPaymentError.Valid {
		out.LastPaymentError = strings.TrimSpace(lastPaymentError.String)
	}
	if len(payloadRaw) == 0 {
		out.Payload = map[string]any{}
		return out, nil
	}
	if err := json.Unmarshal(payloadRaw, &out.Payload); err != nil {
		return domain.LagoWebhookEvent{}, err
	}
	if out.Payload == nil {
		out.Payload = map[string]any{}
	}
	return out, nil
}

func buildFilteredQuery(base string, filter Filter, timeColumn string) (string, []any) {
	clauses := make([]string, 0, 4)
	args := make([]any, 0, 4)

	add := func(format string, val any) {
		args = append(args, val)
		clauses = append(clauses, fmt.Sprintf(format, len(args)))
	}

	if filter.CustomerID != "" {
		add("customer_id = $%d", filter.CustomerID)
	}
	if filter.MeterID != "" {
		add("meter_id = $%d", filter.MeterID)
	}
	if strings.TrimSpace(filter.TenantID) != "" {
		add("tenant_id = $%d", normalizeTenantID(filter.TenantID))
	}
	if filter.From != nil {
		add(timeColumn+" >= $%d", *filter.From)
	}
	if filter.To != nil {
		add(timeColumn+" <= $%d", *filter.To)
	}

	if len(clauses) > 0 {
		base += " WHERE " + strings.Join(clauses, " AND ")
	}

	return base, args
}

func buildUsageEventsFilteredQuery(base string, filter Filter, timeColumn string) (string, []any) {
	clauses := make([]string, 0, 7)
	args := make([]any, 0, 7)

	add := func(format string, val any) {
		args = append(args, val)
		clauses = append(clauses, fmt.Sprintf(format, len(args)))
	}

	if filter.CustomerID != "" {
		add("customer_id = $%d", filter.CustomerID)
	}
	if filter.MeterID != "" {
		add("meter_id = $%d", filter.MeterID)
	}
	if strings.TrimSpace(filter.TenantID) != "" {
		add("tenant_id = $%d", normalizeTenantID(filter.TenantID))
	}
	if filter.CursorOccurredAt != nil && strings.TrimSpace(filter.CursorID) != "" {
		args = append(args, filter.CursorOccurredAt.UTC(), strings.TrimSpace(filter.CursorID))
		posOccurredAt := len(args) - 1
		posID := len(args)
		if filter.SortDesc {
			clauses = append(clauses, fmt.Sprintf("(%s < $%d OR (%s = $%d AND id < $%d))", timeColumn, posOccurredAt, timeColumn, posOccurredAt, posID))
		} else {
			clauses = append(clauses, fmt.Sprintf("(%s > $%d OR (%s = $%d AND id > $%d))", timeColumn, posOccurredAt, timeColumn, posOccurredAt, posID))
		}
	}
	if filter.From != nil {
		add(timeColumn+" >= $%d", *filter.From)
	}
	if filter.To != nil {
		add(timeColumn+" <= $%d", *filter.To)
	}

	if len(clauses) > 0 {
		base += " WHERE " + strings.Join(clauses, " AND ")
	}

	return base, args
}

func buildBilledEntriesFilteredQuery(base string, filter Filter, timeColumn string) (string, []any) {
	clauses := make([]string, 0, 8)
	args := make([]any, 0, 8)

	add := func(format string, val any) {
		args = append(args, val)
		clauses = append(clauses, fmt.Sprintf(format, len(args)))
	}

	if filter.CustomerID != "" {
		add("customer_id = $%d", filter.CustomerID)
	}
	if filter.MeterID != "" {
		add("meter_id = $%d", filter.MeterID)
	}
	if strings.TrimSpace(filter.TenantID) != "" {
		add("tenant_id = $%d", normalizeTenantID(filter.TenantID))
	}
	if strings.TrimSpace(string(filter.BilledSource)) != "" {
		add("source = $%d", strings.TrimSpace(string(filter.BilledSource)))
	}
	if strings.TrimSpace(filter.BilledReplayJobID) != "" {
		add("replay_job_id = $%d", strings.TrimSpace(filter.BilledReplayJobID))
	}
	if filter.CursorOccurredAt != nil && strings.TrimSpace(filter.CursorID) != "" {
		args = append(args, filter.CursorOccurredAt.UTC(), strings.TrimSpace(filter.CursorID))
		posOccurredAt := len(args) - 1
		posID := len(args)
		if filter.SortDesc {
			clauses = append(clauses, fmt.Sprintf("(%s < $%d OR (%s = $%d AND id < $%d))", timeColumn, posOccurredAt, timeColumn, posOccurredAt, posID))
		} else {
			clauses = append(clauses, fmt.Sprintf("(%s > $%d OR (%s = $%d AND id > $%d))", timeColumn, posOccurredAt, timeColumn, posOccurredAt, posID))
		}
	}
	if filter.From != nil {
		add(timeColumn+" >= $%d", *filter.From)
	}
	if filter.To != nil {
		add(timeColumn+" <= $%d", *filter.To)
	}

	if len(clauses) > 0 {
		base += " WHERE " + strings.Join(clauses, " AND ")
	}

	return base, args
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

func normalizeCustomerStatus(v domain.CustomerStatus) domain.CustomerStatus {
	switch strings.ToLower(strings.TrimSpace(string(v))) {
	case string(domain.CustomerStatusSuspended):
		return domain.CustomerStatusSuspended
	case string(domain.CustomerStatusArchived):
		return domain.CustomerStatusArchived
	default:
		return domain.CustomerStatusActive
	}
}

func normalizeBillingProfileStatus(v domain.BillingProfileStatus) domain.BillingProfileStatus {
	switch strings.ToLower(strings.TrimSpace(string(v))) {
	case string(domain.BillingProfileStatusIncomplete):
		return domain.BillingProfileStatusIncomplete
	case string(domain.BillingProfileStatusReady):
		return domain.BillingProfileStatusReady
	case string(domain.BillingProfileStatusSyncError):
		return domain.BillingProfileStatusSyncError
	default:
		return domain.BillingProfileStatusMissing
	}
}

func normalizePaymentSetupStatus(v domain.PaymentSetupStatus) domain.PaymentSetupStatus {
	switch strings.ToLower(strings.TrimSpace(string(v))) {
	case string(domain.PaymentSetupStatusPending):
		return domain.PaymentSetupStatusPending
	case string(domain.PaymentSetupStatusReady):
		return domain.PaymentSetupStatusReady
	case string(domain.PaymentSetupStatusError):
		return domain.PaymentSetupStatusError
	default:
		return domain.PaymentSetupStatusMissing
	}
}

func normalizeSubscriptionStatus(v domain.SubscriptionStatus) domain.SubscriptionStatus {
	switch strings.ToLower(strings.TrimSpace(string(v))) {
	case string(domain.SubscriptionStatusPendingPaymentSetup):
		return domain.SubscriptionStatusPendingPaymentSetup
	case string(domain.SubscriptionStatusActive):
		return domain.SubscriptionStatusActive
	case string(domain.SubscriptionStatusActionRequired):
		return domain.SubscriptionStatusActionRequired
	case string(domain.SubscriptionStatusArchived):
		return domain.SubscriptionStatusArchived
	default:
		return domain.SubscriptionStatusDraft
	}
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23503"
	}
	return false
}

func normalizeTenantStatus(v domain.TenantStatus) domain.TenantStatus {
	switch domain.TenantStatus(strings.ToLower(strings.TrimSpace(string(v)))) {
	case domain.TenantStatusSuspended:
		return domain.TenantStatusSuspended
	case domain.TenantStatusDeleted:
		return domain.TenantStatusDeleted
	default:
		return domain.TenantStatusActive
	}
}

func normalizeBilledEntrySource(v domain.BilledEntrySource) domain.BilledEntrySource {
	normalized := domain.BilledEntrySource(strings.TrimSpace(string(v)))
	if normalized == "" {
		return domain.BilledEntrySourceAPI
	}
	return normalized
}

func normalizeRatingRuleLifecycleState(v domain.RatingRuleLifecycleState) domain.RatingRuleLifecycleState {
	state := domain.RatingRuleLifecycleState(strings.ToLower(strings.TrimSpace(string(v))))
	switch state {
	case domain.RatingRuleLifecycleDraft, domain.RatingRuleLifecycleArchived:
		return state
	case domain.RatingRuleLifecycleActive:
		return domain.RatingRuleLifecycleActive
	default:
		return domain.RatingRuleLifecycleActive
	}
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
