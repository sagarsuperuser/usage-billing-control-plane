package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
)

func (s *PostgresStore) CreateTenant(input domain.Tenant) (domain.Tenant, error) {
	input.ID = normalizeTenantID(input.ID)
	input.Name = strings.TrimSpace(input.Name)
	input.Status = normalizeTenantStatus(input.Status)
	input.BillingProviderConnectionID = normalizeOptionalText(input.BillingProviderConnectionID)
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
		`INSERT INTO tenants (id, name, status, billing_provider_connection_id, created_at, updated_at)
		VALUES ($1, $2, $3, NULLIF($4,''), $5, $6)
		RETURNING id, name, status, billing_provider_connection_id, created_at, updated_at`,
		input.ID,
		input.Name,
		string(input.Status),
		input.BillingProviderConnectionID,
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
		`SELECT id, name, status, billing_provider_connection_id, created_at, updated_at
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

func (s *PostgresStore) UpdateTenant(input domain.Tenant) (domain.Tenant, error) {
	input.ID = normalizeTenantID(input.ID)
	input.Name = strings.TrimSpace(input.Name)
	input.Status = normalizeTenantStatus(input.Status)
	input.BillingProviderConnectionID = normalizeOptionalText(input.BillingProviderConnectionID)
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
		    updated_at = $4
		WHERE id = $5
		RETURNING id, name, status, billing_provider_connection_id, created_at, updated_at`,
		input.Name,
		string(input.Status),
		input.BillingProviderConnectionID,
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

	query := `SELECT id, name, status, billing_provider_connection_id, created_at, updated_at FROM tenants`
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
		RETURNING id, name, status, billing_provider_connection_id, created_at, updated_at`,
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
	actions := make([]string, 0, len(filter.Actions))
	for _, action := range filter.Actions {
		normalized := strings.TrimSpace(action)
		if normalized == "" {
			continue
		}
		actions = append(actions, normalized)
	}
	if len(actions) == 1 {
		add("action = $%d", actions[0])
	}
	if len(actions) > 1 {
		placeholders := make([]string, 0, len(actions))
		for _, action := range actions {
			args = append(args, action)
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
		}
		clauses = append(clauses, "action IN ("+strings.Join(placeholders, ", ")+")")
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


func scanTenant(s rowScanner) (domain.Tenant, error) {
	var out domain.Tenant
	var status string
	var billingProviderConnectionID sql.NullString
	if err := s.Scan(&out.ID, &out.Name, &status, &billingProviderConnectionID, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return domain.Tenant{}, err
	}
	out.ID = normalizeTenantID(out.ID)
	out.Name = strings.TrimSpace(out.Name)
	if billingProviderConnectionID.Valid {
		out.BillingProviderConnectionID = normalizeOptionalText(billingProviderConnectionID.String)
	}
	out.Status = normalizeTenantStatus(domain.TenantStatus(status))
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

