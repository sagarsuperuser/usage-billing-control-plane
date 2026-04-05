package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
)

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
		input.CreatedByPlatformUser,
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
	if ownerType := strings.TrimSpace(filter.OwnerType); ownerType != "" {
		baseClauses = append(baseClauses, fmt.Sprintf(`EXISTS (
			SELECT 1
			FROM api_keys k
			WHERE k.tenant_id = api_key_audit_events.tenant_id
				AND k.id = api_key_audit_events.api_key_id
				AND k.owner_type = $%d
		)`, nextArg))
		baseArgs = append(baseArgs, ownerType)
		nextArg++
	}
	if ownerID := strings.TrimSpace(filter.OwnerID); ownerID != "" {
		baseClauses = append(baseClauses, fmt.Sprintf(`EXISTS (
			SELECT 1
			FROM api_keys k
			WHERE k.tenant_id = api_key_audit_events.tenant_id
				AND k.id = api_key_audit_events.api_key_id
				AND k.owner_id = $%d
		)`, nextArg))
		baseArgs = append(baseArgs, ownerID)
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
	if ownerType := strings.TrimSpace(filter.OwnerType); ownerType != "" {
		baseClauses = append(baseClauses, fmt.Sprintf("filters->>'owner_type' = $%d", nextArg))
		baseArgs = append(baseArgs, ownerType)
		nextArg++
	}
	if ownerID := strings.TrimSpace(filter.OwnerID); ownerID != "" {
		baseClauses = append(baseClauses, fmt.Sprintf("filters->>'owner_id' = $%d", nextArg))
		baseArgs = append(baseArgs, ownerID)
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
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer rollbackSilently(tx)

	res, err := tx.ExecContext(ctx, `UPDATE api_keys SET last_used_at = $1 WHERE id = $2`, usedAt, id)
	if err != nil {
		return fmt.Errorf("update api key last_used_at: %w", err)
	}
	updated, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if updated == 0 {
		return ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
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

