package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"

	"usage-billing-control-plane/internal/domain"
)

func (s *PostgresStore) CreateBillingProviderConnection(input domain.BillingProviderConnection) (domain.BillingProviderConnection, error) {
	input.ProviderType = domain.BillingProviderType(strings.ToLower(strings.TrimSpace(string(input.ProviderType))))
	input.Environment = strings.ToLower(strings.TrimSpace(input.Environment))
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Scope = domain.BillingProviderConnectionScope(strings.ToLower(strings.TrimSpace(string(input.Scope))))
	input.OwnerTenantID = normalizeOptionalText(input.OwnerTenantID)
	input.Status = domain.BillingProviderConnectionStatus(strings.ToLower(strings.TrimSpace(string(input.Status))))
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
			secret_ref, last_synced_at, last_sync_error,
			connected_at, disabled_at, created_by_type, created_by_id, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),$7,NULLIF($8,''),$9,NULLIF($10,''),$11,$12,$13,NULLIF($14,''),$15,$16)
		RETURNING id, provider_type, environment, display_name, scope, owner_tenant_id, status,
			secret_ref, last_synced_at, last_sync_error,
			connected_at, disabled_at, created_by_type, created_by_id, created_at, updated_at`,
		input.ID,
		string(input.ProviderType),
		input.Environment,
		input.DisplayName,
		string(input.Scope),
		input.OwnerTenantID,
		string(input.Status),
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
			secret_ref, last_synced_at, last_sync_error,
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
		secret_ref, last_synced_at, last_sync_error,
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
		    secret_ref = NULLIF($7,''),
		    last_synced_at = $8,
		    last_sync_error = NULLIF($9,''),
		    connected_at = $10,
		    disabled_at = $11,
		    created_by_type = $12,
		    created_by_id = NULLIF($13,''),
		    updated_at = $14
		WHERE id = $15
		RETURNING id, provider_type, environment, display_name, scope, owner_tenant_id, status,
			secret_ref, last_synced_at, last_sync_error,
			connected_at, disabled_at, created_by_type, created_by_id, created_at, updated_at`,
		string(input.ProviderType),
		input.Environment,
		input.DisplayName,
		string(input.Scope),
		input.OwnerTenantID,
		string(input.Status),
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

func (s *PostgresStore) GetWorkspaceBillingSettings(workspaceID string) (domain.WorkspaceBillingSettings, error) {
	workspaceID = normalizeTenantID(workspaceID)
	if workspaceID == "" {
		return domain.WorkspaceBillingSettings{}, ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.WorkspaceBillingSettings{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`SELECT workspace_id, billing_entity_code, net_payment_term_days, tax_codes, invoice_memo, invoice_footer, document_locale, invoice_grace_period_days, document_numbering, document_number_prefix, created_at, updated_at
		FROM workspace_billing_settings
		WHERE workspace_id = $1`,
		workspaceID,
	)
	out, err := scanWorkspaceBillingSettings(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.WorkspaceBillingSettings{}, ErrNotFound
		}
		return domain.WorkspaceBillingSettings{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.WorkspaceBillingSettings{}, err
	}
	return out, nil
}

func (s *PostgresStore) UpsertWorkspaceBillingSettings(input domain.WorkspaceBillingSettings) (domain.WorkspaceBillingSettings, error) {
	input.WorkspaceID = normalizeTenantID(input.WorkspaceID)
	input.BillingEntityCode = normalizeOptionalText(input.BillingEntityCode)
	input.InvoiceMemo = normalizeOptionalText(input.InvoiceMemo)
	input.InvoiceFooter = normalizeOptionalText(input.InvoiceFooter)
	input.DocumentLocale = strings.TrimSpace(input.DocumentLocale)
	input.DocumentNumbering = strings.TrimSpace(input.DocumentNumbering)
	input.DocumentNumberPrefix = strings.TrimSpace(input.DocumentNumberPrefix)
	if input.WorkspaceID == "" {
		return domain.WorkspaceBillingSettings{}, fmt.Errorf("validation failed: workspace_id is required")
	}
	if input.NetPaymentTermDays != nil && *input.NetPaymentTermDays < 0 {
		return domain.WorkspaceBillingSettings{}, fmt.Errorf("validation failed: net_payment_term_days must be non-negative")
	}
	if input.InvoiceGracePeriodDays != nil && *input.InvoiceGracePeriodDays < 0 {
		return domain.WorkspaceBillingSettings{}, fmt.Errorf("validation failed: invoice_grace_period_days must be non-negative")
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
		return domain.WorkspaceBillingSettings{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`INSERT INTO workspace_billing_settings (
			workspace_id, billing_entity_code, net_payment_term_days, tax_codes, invoice_memo, invoice_footer, document_locale, invoice_grace_period_days, document_numbering, document_number_prefix, created_at, updated_at
		) VALUES ($1,NULLIF($2,''),$3,$4,NULLIF($5,''),NULLIF($6,''),$7,$8,$9,$10,$11,$12)
		ON CONFLICT (workspace_id) DO UPDATE SET
			billing_entity_code = EXCLUDED.billing_entity_code,
			net_payment_term_days = EXCLUDED.net_payment_term_days,
			tax_codes = EXCLUDED.tax_codes,
			invoice_memo = EXCLUDED.invoice_memo,
			invoice_footer = EXCLUDED.invoice_footer,
			document_locale = EXCLUDED.document_locale,
			invoice_grace_period_days = EXCLUDED.invoice_grace_period_days,
			document_numbering = EXCLUDED.document_numbering,
			document_number_prefix = EXCLUDED.document_number_prefix,
			updated_at = EXCLUDED.updated_at
		RETURNING workspace_id, billing_entity_code, net_payment_term_days, tax_codes, invoice_memo, invoice_footer, document_locale, invoice_grace_period_days, document_numbering, document_number_prefix, created_at, updated_at`,
		input.WorkspaceID,
		input.BillingEntityCode,
		input.NetPaymentTermDays,
		pq.Array(normalizeStringList(input.TaxCodes)),
		input.InvoiceMemo,
		input.InvoiceFooter,
		input.DocumentLocale,
		input.InvoiceGracePeriodDays,
		input.DocumentNumbering,
		input.DocumentNumberPrefix,
		input.CreatedAt,
		input.UpdatedAt,
	)
	out, err := scanWorkspaceBillingSettings(row)
	if err != nil {
		if isForeignKeyViolation(err) {
			return domain.WorkspaceBillingSettings{}, ErrNotFound
		}
		return domain.WorkspaceBillingSettings{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.WorkspaceBillingSettings{}, err
	}
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

func scanWorkspaceBillingSettings(s rowScanner) (domain.WorkspaceBillingSettings, error) {
	var out domain.WorkspaceBillingSettings
	var billingEntityCode sql.NullString
	var netPaymentTermDays sql.NullInt64
	var taxCodes pq.StringArray
	var invoiceMemo sql.NullString
	var invoiceFooter sql.NullString
	var documentLocale sql.NullString
	var invoiceGracePeriodDays sql.NullInt64
	var documentNumbering sql.NullString
	var documentNumberPrefix sql.NullString
	if err := s.Scan(
		&out.WorkspaceID,
		&billingEntityCode,
		&netPaymentTermDays,
		&taxCodes,
		&invoiceMemo,
		&invoiceFooter,
		&documentLocale,
		&invoiceGracePeriodDays,
		&documentNumbering,
		&documentNumberPrefix,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		return domain.WorkspaceBillingSettings{}, err
	}
	out.WorkspaceID = normalizeTenantID(out.WorkspaceID)
	if billingEntityCode.Valid {
		out.BillingEntityCode = normalizeOptionalText(billingEntityCode.String)
	}
	if netPaymentTermDays.Valid {
		value := int(netPaymentTermDays.Int64)
		out.NetPaymentTermDays = &value
	}
	out.TaxCodes = normalizeStringList([]string(taxCodes))
	if invoiceMemo.Valid {
		out.InvoiceMemo = normalizeOptionalText(invoiceMemo.String)
	}
	if invoiceFooter.Valid {
		out.InvoiceFooter = normalizeOptionalText(invoiceFooter.String)
	}
	if documentLocale.Valid {
		out.DocumentLocale = normalizeOptionalText(documentLocale.String)
	}
	if invoiceGracePeriodDays.Valid {
		value := int(invoiceGracePeriodDays.Int64)
		out.InvoiceGracePeriodDays = &value
	}
	if documentNumbering.Valid {
		out.DocumentNumbering = normalizeOptionalText(documentNumbering.String)
	}
	if documentNumberPrefix.Valid {
		out.DocumentNumberPrefix = normalizeOptionalText(documentNumberPrefix.String)
	}
	return out, nil
}


func scanBillingProviderConnection(s rowScanner) (domain.BillingProviderConnection, error) {
	var out domain.BillingProviderConnection
	var providerType string
	var scope string
	var status string
	var ownerTenantID sql.NullString
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

