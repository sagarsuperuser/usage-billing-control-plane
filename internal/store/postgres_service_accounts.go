package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
)

func (s *PostgresStore) CreateServiceAccount(input domain.ServiceAccount) (domain.ServiceAccount, error) {
	input.TenantID = normalizeTenantID(input.TenantID)
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.Role = strings.ToLower(strings.TrimSpace(input.Role))
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	input.Purpose = strings.TrimSpace(input.Purpose)
	input.Environment = strings.TrimSpace(input.Environment)
	input.CreatedByUserID = strings.TrimSpace(input.CreatedByUserID)

	if input.TenantID == "" || input.Name == "" || input.Role == "" {
		return domain.ServiceAccount{}, fmt.Errorf("validation failed: tenant_id, name, and role are required")
	}
	if input.Status == "" {
		input.Status = domain.ServiceAccountStatusActive
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
			id, tenant_id, name, description, role, status, purpose, environment, created_by_user_id, created_by_platform_user, created_at, updated_at, disabled_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9,''),$10,$11,$12,$13)
		RETURNING id, tenant_id, name, description, role, status, purpose, environment, created_by_user_id, created_by_platform_user, created_at, updated_at, disabled_at`,
		input.ID,
		input.TenantID,
		input.Name,
		input.Description,
		input.Role,
		input.Status,
		input.Purpose,
		input.Environment,
		input.CreatedByUserID,
		input.CreatedByPlatformUser,
		input.CreatedAt,
		input.UpdatedAt,
		input.DisabledAt,
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
		`SELECT id, tenant_id, name, description, role, status, purpose, environment, created_by_user_id, created_by_platform_user, created_at, updated_at, disabled_at
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

func (s *PostgresStore) GetServiceAccountByName(tenantID, name string) (domain.ServiceAccount, error) {
	tenantID = normalizeTenantID(tenantID)
	name = strings.TrimSpace(name)
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.ServiceAccount{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`SELECT id, tenant_id, name, description, role, status, purpose, environment, created_by_user_id, created_by_platform_user, created_at, updated_at, disabled_at
		FROM service_accounts
		WHERE tenant_id = $1 AND lower(name) = lower($2)`,
		tenantID,
		name,
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
	query := fmt.Sprintf(`SELECT id, tenant_id, name, description, role, status, purpose, environment, created_by_user_id, created_by_platform_user, created_at, updated_at, disabled_at
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

func (s *PostgresStore) UpdateServiceAccount(input domain.ServiceAccount) (domain.ServiceAccount, error) {
	input.TenantID = normalizeTenantID(input.TenantID)
	input.ID = strings.TrimSpace(input.ID)
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.Role = strings.ToLower(strings.TrimSpace(input.Role))
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	input.Purpose = strings.TrimSpace(input.Purpose)
	input.Environment = strings.TrimSpace(input.Environment)
	input.CreatedByUserID = strings.TrimSpace(input.CreatedByUserID)
	if input.TenantID == "" || input.ID == "" || input.Name == "" || input.Role == "" {
		return domain.ServiceAccount{}, fmt.Errorf("validation failed: tenant_id, id, name, and role are required")
	}
	if input.Status == "" {
		input.Status = domain.ServiceAccountStatusActive
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = time.Now().UTC()
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
		`UPDATE service_accounts
		SET name = $3,
			description = $4,
			role = $5,
			status = $6,
			purpose = $7,
			environment = $8,
			created_by_user_id = NULLIF($9,''),
			created_by_platform_user = $10,
			updated_at = $11,
			disabled_at = $12
		WHERE tenant_id = $1 AND id = $2
		RETURNING id, tenant_id, name, description, role, status, purpose, environment, created_by_user_id, created_by_platform_user, created_at, updated_at, disabled_at`,
		input.TenantID,
		input.ID,
		input.Name,
		input.Description,
		input.Role,
		input.Status,
		input.Purpose,
		input.Environment,
		input.CreatedByUserID,
		input.CreatedByPlatformUser,
		input.UpdatedAt,
		input.DisabledAt,
	)
	account, err := scanServiceAccount(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ServiceAccount{}, ErrNotFound
		}
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


func scanServiceAccount(s rowScanner) (domain.ServiceAccount, error) {
	var out domain.ServiceAccount
	var tenantID sql.NullString
	var description sql.NullString
	var status sql.NullString
	var purpose sql.NullString
	var environment sql.NullString
	var createdByUserID sql.NullString
	var createdByPlatformUser sql.NullBool
	var disabledAt sql.NullTime

	if err := s.Scan(
		&out.ID,
		&tenantID,
		&out.Name,
		&description,
		&out.Role,
		&status,
		&purpose,
		&environment,
		&createdByUserID,
		&createdByPlatformUser,
		&out.CreatedAt,
		&out.UpdatedAt,
		&disabledAt,
	); err != nil {
		return domain.ServiceAccount{}, err
	}
	if tenantID.Valid {
		out.TenantID = normalizeTenantID(tenantID.String)
	} else {
		out.TenantID = defaultTenantID
	}
	out.Description = strings.TrimSpace(description.String)
	out.Status = strings.TrimSpace(status.String)
	if out.Status == "" {
		out.Status = domain.ServiceAccountStatusActive
	}
	out.Purpose = strings.TrimSpace(purpose.String)
	out.Environment = strings.TrimSpace(environment.String)
	out.CreatedByUserID = strings.TrimSpace(createdByUserID.String)
	out.CreatedByPlatformUser = createdByPlatformUser.Valid && createdByPlatformUser.Bool
	if disabledAt.Valid {
		value := disabledAt.Time.UTC()
		out.DisabledAt = &value
	}
	return out, nil
}

