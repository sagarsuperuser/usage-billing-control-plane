package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lib/pq"

	"usage-billing-control-plane/internal/domain"
)

func (s *PostgresStore) GetDunningPolicy(tenantID string) (domain.DunningPolicy, error) {
	tenantID = normalizeTenantID(tenantID)
	ctx, cancel := s.withTimeout()
	defer cancel()
	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.DunningPolicy{}, err
	}
	defer rollbackSilently(tx)
	row := tx.QueryRowContext(ctx, `SELECT id, tenant_id, name, enabled, retry_schedule, max_retry_attempts, collect_payment_reminder_schedule, final_action, grace_period_days, created_at, updated_at
		FROM dunning_policies WHERE tenant_id = $1`, tenantID)
	out, err := scanDunningPolicy(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.DunningPolicy{}, ErrNotFound
		}
		return domain.DunningPolicy{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.DunningPolicy{}, err
	}
	return out, nil
}

func (s *PostgresStore) UpsertDunningPolicy(input domain.DunningPolicy) (domain.DunningPolicy, error) {
	input.TenantID = normalizeTenantID(input.TenantID)
	input.Name = strings.TrimSpace(input.Name)
	input.RetrySchedule = normalizeDunningSchedule(input.RetrySchedule)
	input.CollectPaymentReminderSchedule = normalizeDunningSchedule(input.CollectPaymentReminderSchedule)
	input.FinalAction = normalizeDunningFinalAction(input.FinalAction)
	if input.TenantID == "" {
		return domain.DunningPolicy{}, fmt.Errorf("validation failed: tenant_id is required")
	}
	if input.ID == "" {
		input.ID = newID("dpo")
	}
	if input.Name == "" {
		input.Name = "Default dunning policy"
	}
	if input.MaxRetryAttempts < 0 {
		return domain.DunningPolicy{}, fmt.Errorf("validation failed: max_retry_attempts must be >= 0")
	}
	if input.GracePeriodDays < 0 {
		return domain.DunningPolicy{}, fmt.Errorf("validation failed: grace_period_days must be >= 0")
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
		return domain.DunningPolicy{}, err
	}
	defer rollbackSilently(tx)
	row := tx.QueryRowContext(ctx, `INSERT INTO dunning_policies (
			id, tenant_id, name, enabled, retry_schedule, max_retry_attempts, collect_payment_reminder_schedule, final_action, grace_period_days, created_at, updated_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11
		)
		ON CONFLICT (tenant_id) DO UPDATE SET
			name = EXCLUDED.name,
			enabled = EXCLUDED.enabled,
			retry_schedule = EXCLUDED.retry_schedule,
			max_retry_attempts = EXCLUDED.max_retry_attempts,
			collect_payment_reminder_schedule = EXCLUDED.collect_payment_reminder_schedule,
			final_action = EXCLUDED.final_action,
			grace_period_days = EXCLUDED.grace_period_days,
			updated_at = EXCLUDED.updated_at
		RETURNING id, tenant_id, name, enabled, retry_schedule, max_retry_attempts, collect_payment_reminder_schedule, final_action, grace_period_days, created_at, updated_at`,
		input.ID, input.TenantID, input.Name, input.Enabled, pq.Array(input.RetrySchedule), input.MaxRetryAttempts, pq.Array(input.CollectPaymentReminderSchedule), string(input.FinalAction), input.GracePeriodDays, input.CreatedAt, input.UpdatedAt)
	out, err := scanDunningPolicy(row)
	if err != nil {
		return domain.DunningPolicy{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.DunningPolicy{}, err
	}
	return out, nil
}

func (s *PostgresStore) CreateInvoiceDunningRun(input domain.InvoiceDunningRun) (domain.InvoiceDunningRun, error) {
	input.TenantID = normalizeTenantID(input.TenantID)
	input.InvoiceID = strings.TrimSpace(input.InvoiceID)
	input.CustomerExternalID = normalizeOptionalText(input.CustomerExternalID)
	input.PolicyID = strings.TrimSpace(input.PolicyID)
	input.State = normalizeDunningRunState(input.State)
	input.Reason = normalizeOptionalText(input.Reason)
	input.NextActionType = normalizeDunningActionType(input.NextActionType)
	input.Resolution = normalizeDunningResolution(input.Resolution)
	if input.TenantID == "" {
		return domain.InvoiceDunningRun{}, fmt.Errorf("validation failed: tenant_id is required")
	}
	if input.InvoiceID == "" {
		return domain.InvoiceDunningRun{}, fmt.Errorf("validation failed: invoice_id is required")
	}
	if input.PolicyID == "" {
		return domain.InvoiceDunningRun{}, fmt.Errorf("validation failed: policy_id is required")
	}
	if input.AttemptCount < 0 {
		return domain.InvoiceDunningRun{}, fmt.Errorf("validation failed: attempt_count must be >= 0")
	}
	if input.ID == "" {
		input.ID = newID("dru")
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
		return domain.InvoiceDunningRun{}, err
	}
	defer rollbackSilently(tx)
	row := tx.QueryRowContext(ctx, `INSERT INTO invoice_dunning_runs (
			id, tenant_id, invoice_id, customer_external_id, policy_id, state, reason, attempt_count,
			last_attempt_at, next_action_at, next_action_type, paused, resolved_at, resolution, created_at, updated_at
		) VALUES (
			$1,$2,$3,NULLIF($4,''),$5,$6,NULLIF($7,''),$8,$9,$10,NULLIF($11,''),$12,$13,NULLIF($14,''),$15,$16
		)
		RETURNING id, tenant_id, invoice_id, customer_external_id, policy_id, state, reason, attempt_count, last_attempt_at, next_action_at, next_action_type, paused, resolved_at, resolution, created_at, updated_at`,
		input.ID, input.TenantID, input.InvoiceID, input.CustomerExternalID, input.PolicyID, string(input.State), input.Reason, input.AttemptCount, input.LastAttemptAt, input.NextActionAt, string(input.NextActionType), input.Paused, input.ResolvedAt, string(input.Resolution), input.CreatedAt, input.UpdatedAt)
	out, err := scanInvoiceDunningRun(row)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.InvoiceDunningRun{}, ErrAlreadyExists
		}
		return domain.InvoiceDunningRun{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.InvoiceDunningRun{}, err
	}
	return out, nil
}

func (s *PostgresStore) UpdateInvoiceDunningRun(input domain.InvoiceDunningRun) (domain.InvoiceDunningRun, error) {
	input.ID = strings.TrimSpace(input.ID)
	input.TenantID = normalizeTenantID(input.TenantID)
	input.InvoiceID = strings.TrimSpace(input.InvoiceID)
	input.CustomerExternalID = normalizeOptionalText(input.CustomerExternalID)
	input.PolicyID = strings.TrimSpace(input.PolicyID)
	input.State = normalizeDunningRunState(input.State)
	input.Reason = normalizeOptionalText(input.Reason)
	input.NextActionType = normalizeDunningActionType(input.NextActionType)
	input.Resolution = normalizeDunningResolution(input.Resolution)
	if input.ID == "" {
		return domain.InvoiceDunningRun{}, fmt.Errorf("validation failed: id is required")
	}
	if input.TenantID == "" {
		return domain.InvoiceDunningRun{}, fmt.Errorf("validation failed: tenant_id is required")
	}
	if input.InvoiceID == "" {
		return domain.InvoiceDunningRun{}, fmt.Errorf("validation failed: invoice_id is required")
	}
	if input.PolicyID == "" {
		return domain.InvoiceDunningRun{}, fmt.Errorf("validation failed: policy_id is required")
	}
	if input.AttemptCount < 0 {
		return domain.InvoiceDunningRun{}, fmt.Errorf("validation failed: attempt_count must be >= 0")
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = time.Now().UTC()
	}

	ctx, cancel := s.withTimeout()
	defer cancel()
	tx, err := s.beginTxWithSession(ctx, txSessionTenant, input.TenantID)
	if err != nil {
		return domain.InvoiceDunningRun{}, err
	}
	defer rollbackSilently(tx)
	row := tx.QueryRowContext(ctx, `UPDATE invoice_dunning_runs SET
			customer_external_id = NULLIF($3,''),
			policy_id = $4,
			state = $5,
			reason = NULLIF($6,''),
			attempt_count = $7,
			last_attempt_at = $8,
			next_action_at = $9,
			next_action_type = NULLIF($10,''),
			paused = $11,
			resolved_at = $12,
			resolution = NULLIF($13,''),
			updated_at = $14
		WHERE tenant_id = $1 AND id = $2
		RETURNING id, tenant_id, invoice_id, customer_external_id, policy_id, state, reason, attempt_count, last_attempt_at, next_action_at, next_action_type, paused, resolved_at, resolution, created_at, updated_at`,
		input.TenantID, input.ID, input.CustomerExternalID, input.PolicyID, string(input.State), input.Reason, input.AttemptCount, input.LastAttemptAt, input.NextActionAt, string(input.NextActionType), input.Paused, input.ResolvedAt, string(input.Resolution), input.UpdatedAt)
	out, err := scanInvoiceDunningRun(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.InvoiceDunningRun{}, ErrNotFound
		}
		return domain.InvoiceDunningRun{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.InvoiceDunningRun{}, err
	}
	return out, nil
}

func (s *PostgresStore) GetInvoiceDunningRun(tenantID, id string) (domain.InvoiceDunningRun, error) {
	tenantID = normalizeTenantID(tenantID)
	id = strings.TrimSpace(id)
	ctx, cancel := s.withTimeout()
	defer cancel()
	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.InvoiceDunningRun{}, err
	}
	defer rollbackSilently(tx)
	row := tx.QueryRowContext(ctx, `SELECT id, tenant_id, invoice_id, customer_external_id, policy_id, state, reason, attempt_count, last_attempt_at, next_action_at, next_action_type, paused, resolved_at, resolution, created_at, updated_at
		FROM invoice_dunning_runs
		WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	out, err := scanInvoiceDunningRun(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.InvoiceDunningRun{}, ErrNotFound
		}
		return domain.InvoiceDunningRun{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.InvoiceDunningRun{}, err
	}
	return out, nil
}

func (s *PostgresStore) GetActiveInvoiceDunningRunByInvoiceID(tenantID, invoiceID string) (domain.InvoiceDunningRun, error) {
	tenantID = normalizeTenantID(tenantID)
	invoiceID = strings.TrimSpace(invoiceID)
	ctx, cancel := s.withTimeout()
	defer cancel()
	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.InvoiceDunningRun{}, err
	}
	defer rollbackSilently(tx)
	row := tx.QueryRowContext(ctx, `SELECT id, tenant_id, invoice_id, customer_external_id, policy_id, state, reason, attempt_count, last_attempt_at, next_action_at, next_action_type, paused, resolved_at, resolution, created_at, updated_at
		FROM invoice_dunning_runs
		WHERE tenant_id = $1 AND invoice_id = $2 AND resolved_at IS NULL
		ORDER BY created_at DESC
		LIMIT 1`, tenantID, invoiceID)
	out, err := scanInvoiceDunningRun(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.InvoiceDunningRun{}, ErrNotFound
		}
		return domain.InvoiceDunningRun{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.InvoiceDunningRun{}, err
	}
	return out, nil
}

func (s *PostgresStore) ListInvoiceDunningRuns(filter InvoiceDunningRunListFilter) ([]domain.InvoiceDunningRun, error) {
	tenantID := normalizeTenantID(filter.TenantID)
	limit := filter.Limit
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > 100 {
		return nil, fmt.Errorf("validation failed: limit must be between 1 and 100")
	}
	offset := filter.Offset
	if offset < 0 {
		return nil, fmt.Errorf("validation failed: offset must be >= 0")
	}
	ctx, cancel := s.withTimeout()
	defer cancel()
	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return nil, err
	}
	defer rollbackSilently(tx)

	base := `SELECT id, tenant_id, invoice_id, customer_external_id, policy_id, state, reason, attempt_count, last_attempt_at, next_action_at, next_action_type, paused, resolved_at, resolution, created_at, updated_at
		FROM invoice_dunning_runs`
	clauses := []string{"tenant_id = $1"}
	args := []any{tenantID}
	argPos := 2

	if invoiceID := strings.TrimSpace(filter.InvoiceID); invoiceID != "" {
		clauses = append(clauses, fmt.Sprintf("invoice_id = $%d", argPos))
		args = append(args, invoiceID)
		argPos++
	}
	if customerExternalID := strings.TrimSpace(filter.CustomerExternalID); customerExternalID != "" {
		clauses = append(clauses, fmt.Sprintf("customer_external_id = $%d", argPos))
		args = append(args, customerExternalID)
		argPos++
	}
	if state := strings.TrimSpace(filter.State); state != "" {
		clauses = append(clauses, fmt.Sprintf("state = $%d", argPos))
		args = append(args, strings.ToLower(state))
		argPos++
	}
	if filter.ActiveOnly {
		clauses = append(clauses, "resolved_at IS NULL")
	}
	query := base + " WHERE " + strings.Join(clauses, " AND ") + fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argPos, argPos+1)
	args = append(args, limit, offset)

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.InvoiceDunningRun, 0)
	for rows.Next() {
		item, scanErr := scanInvoiceDunningRun(rows)
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

func (s *PostgresStore) ListDueInvoiceDunningRuns(filter DueInvoiceDunningRunFilter) ([]domain.InvoiceDunningRun, error) {
	tenantID := normalizeTenantID(filter.TenantID)
	limit := filter.Limit
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > 100 {
		return nil, fmt.Errorf("validation failed: limit must be between 1 and 100")
	}
	dueBefore := filter.DueBefore
	if dueBefore.IsZero() {
		dueBefore = time.Now().UTC()
	}
	actionType := strings.ToLower(strings.TrimSpace(filter.ActionType))
	ctx, cancel := s.withTimeout()
	defer cancel()
	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return nil, err
	}
	defer rollbackSilently(tx)

	query := `SELECT id, tenant_id, invoice_id, customer_external_id, policy_id, state, reason, attempt_count, last_attempt_at, next_action_at, next_action_type, paused, resolved_at, resolution, created_at, updated_at
		FROM invoice_dunning_runs
		WHERE tenant_id = $1
		  AND resolved_at IS NULL
		  AND paused IS FALSE
		  AND next_action_at IS NOT NULL
		  AND next_action_at <= $2`
	args := []any{tenantID, dueBefore.UTC()}
	if actionType != "" {
		query += ` AND next_action_type = $3`
		args = append(args, actionType)
		query += ` ORDER BY next_action_at ASC, created_at ASC LIMIT $4`
		args = append(args, limit)
	} else {
		query += ` ORDER BY next_action_at ASC, created_at ASC LIMIT $3`
		args = append(args, limit)
	}
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.InvoiceDunningRun, 0)
	for rows.Next() {
		item, scanErr := scanInvoiceDunningRun(rows)
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

func (s *PostgresStore) CreateInvoiceDunningEvent(input domain.InvoiceDunningEvent) (domain.InvoiceDunningEvent, error) {
	input.ID = strings.TrimSpace(input.ID)
	input.RunID = strings.TrimSpace(input.RunID)
	input.TenantID = normalizeTenantID(input.TenantID)
	input.InvoiceID = strings.TrimSpace(input.InvoiceID)
	input.CustomerExternalID = normalizeOptionalText(input.CustomerExternalID)
	input.EventType = normalizeDunningEventType(input.EventType)
	input.State = normalizeDunningRunState(input.State)
	input.ActionType = normalizeDunningActionType(input.ActionType)
	input.Reason = normalizeOptionalText(input.Reason)
	if input.RunID == "" {
		return domain.InvoiceDunningEvent{}, fmt.Errorf("validation failed: run_id is required")
	}
	if input.TenantID == "" {
		return domain.InvoiceDunningEvent{}, fmt.Errorf("validation failed: tenant_id is required")
	}
	if input.InvoiceID == "" {
		return domain.InvoiceDunningEvent{}, fmt.Errorf("validation failed: invoice_id is required")
	}
	if input.AttemptCount < 0 {
		return domain.InvoiceDunningEvent{}, fmt.Errorf("validation failed: attempt_count must be >= 0")
	}
	if input.ID == "" {
		input.ID = newID("dne")
	}
	if input.CreatedAt.IsZero() {
		input.CreatedAt = time.Now().UTC()
	}
	metadataRaw, err := json.Marshal(input.Metadata)
	if err != nil {
		return domain.InvoiceDunningEvent{}, err
	}

	ctx, cancel := s.withTimeout()
	defer cancel()
	tx, err := s.beginTxWithSession(ctx, txSessionTenant, input.TenantID)
	if err != nil {
		return domain.InvoiceDunningEvent{}, err
	}
	defer rollbackSilently(tx)
	row := tx.QueryRowContext(ctx, `INSERT INTO invoice_dunning_events (
			id, run_id, tenant_id, invoice_id, customer_external_id, event_type, state, action_type, reason, attempt_count, metadata, created_at
		) VALUES (
			$1,$2,$3,$4,NULLIF($5,''),$6,$7,NULLIF($8,''),NULLIF($9,''),$10,$11::jsonb,$12
		)
		RETURNING id, run_id, tenant_id, invoice_id, customer_external_id, event_type, state, action_type, reason, attempt_count, metadata, created_at`,
		input.ID, input.RunID, input.TenantID, input.InvoiceID, input.CustomerExternalID, string(input.EventType), string(input.State), string(input.ActionType), input.Reason, input.AttemptCount, metadataRaw, input.CreatedAt)
	out, err := scanInvoiceDunningEvent(row)
	if err != nil {
		if isForeignKeyViolation(err) {
			return domain.InvoiceDunningEvent{}, ErrNotFound
		}
		return domain.InvoiceDunningEvent{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.InvoiceDunningEvent{}, err
	}
	return out, nil
}

func (s *PostgresStore) ListInvoiceDunningEvents(tenantID, runID string) ([]domain.InvoiceDunningEvent, error) {
	tenantID = normalizeTenantID(tenantID)
	runID = strings.TrimSpace(runID)
	ctx, cancel := s.withTimeout()
	defer cancel()
	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return nil, err
	}
	defer rollbackSilently(tx)
	rows, err := tx.QueryContext(ctx, `SELECT id, run_id, tenant_id, invoice_id, customer_external_id, event_type, state, action_type, reason, attempt_count, metadata, created_at
		FROM invoice_dunning_events
		WHERE tenant_id = $1 AND run_id = $2
		ORDER BY created_at ASC`, tenantID, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.InvoiceDunningEvent, 0)
	for rows.Next() {
		item, scanErr := scanInvoiceDunningEvent(rows)
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

func (s *PostgresStore) CreateDunningNotificationIntent(input domain.DunningNotificationIntent) (domain.DunningNotificationIntent, error) {
	input.ID = strings.TrimSpace(input.ID)
	input.RunID = strings.TrimSpace(input.RunID)
	input.TenantID = normalizeTenantID(input.TenantID)
	input.InvoiceID = strings.TrimSpace(input.InvoiceID)
	input.CustomerExternalID = normalizeOptionalText(input.CustomerExternalID)
	input.IntentType = normalizeDunningNotificationIntentType(input.IntentType)
	input.ActionType = normalizeDunningActionType(input.ActionType)
	input.Status = normalizeDunningNotificationIntentStatus(input.Status)
	input.DeliveryBackend = normalizeOptionalText(input.DeliveryBackend)
	input.RecipientEmail = strings.ToLower(normalizeOptionalText(input.RecipientEmail))
	input.LastError = normalizeOptionalText(input.LastError)
	if input.RunID == "" {
		return domain.DunningNotificationIntent{}, fmt.Errorf("validation failed: run_id is required")
	}
	if input.TenantID == "" {
		return domain.DunningNotificationIntent{}, fmt.Errorf("validation failed: tenant_id is required")
	}
	if input.InvoiceID == "" {
		return domain.DunningNotificationIntent{}, fmt.Errorf("validation failed: invoice_id is required")
	}
	if input.ID == "" {
		input.ID = newID("dni")
	}
	if input.CreatedAt.IsZero() {
		input.CreatedAt = time.Now().UTC()
	}
	payloadRaw, err := json.Marshal(input.Payload)
	if err != nil {
		return domain.DunningNotificationIntent{}, err
	}

	ctx, cancel := s.withTimeout()
	defer cancel()
	tx, err := s.beginTxWithSession(ctx, txSessionTenant, input.TenantID)
	if err != nil {
		return domain.DunningNotificationIntent{}, err
	}
	defer rollbackSilently(tx)
	row := tx.QueryRowContext(ctx, `INSERT INTO dunning_notification_intents (
			id, run_id, tenant_id, invoice_id, customer_external_id, intent_type, action_type, status, delivery_backend, recipient_email, payload, last_error, created_at, dispatched_at
		) VALUES (
			$1,$2,$3,$4,NULLIF($5,''),$6,NULLIF($7,''),$8,NULLIF($9,''),NULLIF($10,''),$11::jsonb,NULLIF($12,''),$13,$14
		)
		RETURNING id, run_id, tenant_id, invoice_id, customer_external_id, intent_type, action_type, status, delivery_backend, recipient_email, payload, last_error, created_at, dispatched_at`,
		input.ID, input.RunID, input.TenantID, input.InvoiceID, input.CustomerExternalID, string(input.IntentType), string(input.ActionType), string(input.Status), input.DeliveryBackend, input.RecipientEmail, payloadRaw, input.LastError, input.CreatedAt, input.DispatchedAt)
	out, err := scanDunningNotificationIntent(row)
	if err != nil {
		if isForeignKeyViolation(err) {
			return domain.DunningNotificationIntent{}, ErrNotFound
		}
		return domain.DunningNotificationIntent{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.DunningNotificationIntent{}, err
	}
	return out, nil
}

func (s *PostgresStore) UpdateDunningNotificationIntent(input domain.DunningNotificationIntent) (domain.DunningNotificationIntent, error) {
	input.ID = strings.TrimSpace(input.ID)
	input.RunID = strings.TrimSpace(input.RunID)
	input.TenantID = normalizeTenantID(input.TenantID)
	input.InvoiceID = strings.TrimSpace(input.InvoiceID)
	input.CustomerExternalID = normalizeOptionalText(input.CustomerExternalID)
	input.IntentType = normalizeDunningNotificationIntentType(input.IntentType)
	input.ActionType = normalizeDunningActionType(input.ActionType)
	input.Status = normalizeDunningNotificationIntentStatus(input.Status)
	input.DeliveryBackend = normalizeOptionalText(input.DeliveryBackend)
	input.RecipientEmail = strings.ToLower(normalizeOptionalText(input.RecipientEmail))
	input.LastError = normalizeOptionalText(input.LastError)
	if input.ID == "" {
		return domain.DunningNotificationIntent{}, fmt.Errorf("validation failed: id is required")
	}
	if input.RunID == "" {
		return domain.DunningNotificationIntent{}, fmt.Errorf("validation failed: run_id is required")
	}
	if input.TenantID == "" {
		return domain.DunningNotificationIntent{}, fmt.Errorf("validation failed: tenant_id is required")
	}
	if input.InvoiceID == "" {
		return domain.DunningNotificationIntent{}, fmt.Errorf("validation failed: invoice_id is required")
	}
	if input.CreatedAt.IsZero() {
		return domain.DunningNotificationIntent{}, fmt.Errorf("validation failed: created_at is required")
	}
	payloadRaw, err := json.Marshal(input.Payload)
	if err != nil {
		return domain.DunningNotificationIntent{}, err
	}

	ctx, cancel := s.withTimeout()
	defer cancel()
	tx, err := s.beginTxWithSession(ctx, txSessionTenant, input.TenantID)
	if err != nil {
		return domain.DunningNotificationIntent{}, err
	}
	defer rollbackSilently(tx)
	row := tx.QueryRowContext(ctx, `UPDATE dunning_notification_intents SET
			customer_external_id = NULLIF($3,''),
			intent_type = $4,
			action_type = NULLIF($5,''),
			status = $6,
			delivery_backend = NULLIF($7,''),
			recipient_email = NULLIF($8,''),
			payload = $9::jsonb,
			last_error = NULLIF($10,''),
			dispatched_at = $11
		WHERE tenant_id = $1 AND id = $2
		RETURNING id, run_id, tenant_id, invoice_id, customer_external_id, intent_type, action_type, status, delivery_backend, recipient_email, payload, last_error, created_at, dispatched_at`,
		input.TenantID, input.ID, input.CustomerExternalID, string(input.IntentType), string(input.ActionType), string(input.Status), input.DeliveryBackend, input.RecipientEmail, payloadRaw, input.LastError, input.DispatchedAt)
	out, err := scanDunningNotificationIntent(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.DunningNotificationIntent{}, ErrNotFound
		}
		return domain.DunningNotificationIntent{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.DunningNotificationIntent{}, err
	}
	return out, nil
}

func (s *PostgresStore) ListDunningNotificationIntents(filter DunningNotificationIntentListFilter) ([]domain.DunningNotificationIntent, error) {
	tenantID := normalizeTenantID(filter.TenantID)
	limit := filter.Limit
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > 100 {
		return nil, fmt.Errorf("validation failed: limit must be between 1 and 100")
	}
	offset := filter.Offset
	if offset < 0 {
		return nil, fmt.Errorf("validation failed: offset must be >= 0")
	}
	ctx, cancel := s.withTimeout()
	defer cancel()
	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return nil, err
	}
	defer rollbackSilently(tx)
	query := `SELECT id, run_id, tenant_id, invoice_id, customer_external_id, intent_type, action_type, status, delivery_backend, recipient_email, payload, last_error, created_at, dispatched_at
		FROM dunning_notification_intents WHERE tenant_id = $1`
	args := []any{tenantID}
	argPos := 2
	if runID := strings.TrimSpace(filter.RunID); runID != "" {
		query += fmt.Sprintf(" AND run_id = $%d", argPos)
		args = append(args, runID)
		argPos++
	}
	if invoiceID := strings.TrimSpace(filter.InvoiceID); invoiceID != "" {
		query += fmt.Sprintf(" AND invoice_id = $%d", argPos)
		args = append(args, invoiceID)
		argPos++
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		query += fmt.Sprintf(" AND status = $%d", argPos)
		args = append(args, strings.ToLower(status))
		argPos++
	}
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argPos, argPos+1)
	args = append(args, limit, offset)
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.DunningNotificationIntent, 0)
	for rows.Next() {
		item, scanErr := scanDunningNotificationIntent(rows)
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


func scanDunningPolicy(s rowScanner) (domain.DunningPolicy, error) {
	var out domain.DunningPolicy
	var retrySchedule pq.StringArray
	var reminderSchedule pq.StringArray
	var finalAction string
	if err := s.Scan(
		&out.ID,
		&out.TenantID,
		&out.Name,
		&out.Enabled,
		&retrySchedule,
		&out.MaxRetryAttempts,
		&reminderSchedule,
		&finalAction,
		&out.GracePeriodDays,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		return domain.DunningPolicy{}, err
	}
	out.TenantID = normalizeTenantID(out.TenantID)
	out.Name = strings.TrimSpace(out.Name)
	out.RetrySchedule = normalizeDunningSchedule([]string(retrySchedule))
	out.CollectPaymentReminderSchedule = normalizeDunningSchedule([]string(reminderSchedule))
	out.FinalAction = normalizeDunningFinalAction(domain.DunningFinalAction(finalAction))
	return out, nil
}

func scanInvoiceDunningRun(s rowScanner) (domain.InvoiceDunningRun, error) {
	var out domain.InvoiceDunningRun
	var customerExternalID sql.NullString
	var reason sql.NullString
	var lastAttemptAt sql.NullTime
	var nextActionAt sql.NullTime
	var nextActionType sql.NullString
	var resolvedAt sql.NullTime
	var resolution sql.NullString
	var state string
	if err := s.Scan(
		&out.ID,
		&out.TenantID,
		&out.InvoiceID,
		&customerExternalID,
		&out.PolicyID,
		&state,
		&reason,
		&out.AttemptCount,
		&lastAttemptAt,
		&nextActionAt,
		&nextActionType,
		&out.Paused,
		&resolvedAt,
		&resolution,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		return domain.InvoiceDunningRun{}, err
	}
	out.TenantID = normalizeTenantID(out.TenantID)
	out.InvoiceID = strings.TrimSpace(out.InvoiceID)
	out.PolicyID = strings.TrimSpace(out.PolicyID)
	out.State = normalizeDunningRunState(domain.DunningRunState(state))
	if customerExternalID.Valid {
		out.CustomerExternalID = strings.TrimSpace(customerExternalID.String)
	}
	if reason.Valid {
		out.Reason = strings.TrimSpace(reason.String)
	}
	if lastAttemptAt.Valid {
		v := lastAttemptAt.Time.UTC()
		out.LastAttemptAt = &v
	}
	if nextActionAt.Valid {
		v := nextActionAt.Time.UTC()
		out.NextActionAt = &v
	}
	if nextActionType.Valid {
		out.NextActionType = normalizeDunningActionType(domain.DunningActionType(nextActionType.String))
	}
	if resolvedAt.Valid {
		v := resolvedAt.Time.UTC()
		out.ResolvedAt = &v
	}
	if resolution.Valid {
		out.Resolution = normalizeDunningResolution(domain.DunningResolution(resolution.String))
	}
	return out, nil
}

func scanInvoiceDunningEvent(s rowScanner) (domain.InvoiceDunningEvent, error) {
	var out domain.InvoiceDunningEvent
	var customerExternalID sql.NullString
	var eventType string
	var state string
	var actionType sql.NullString
	var reason sql.NullString
	var metadataRaw []byte
	if err := s.Scan(
		&out.ID,
		&out.RunID,
		&out.TenantID,
		&out.InvoiceID,
		&customerExternalID,
		&eventType,
		&state,
		&actionType,
		&reason,
		&out.AttemptCount,
		&metadataRaw,
		&out.CreatedAt,
	); err != nil {
		return domain.InvoiceDunningEvent{}, err
	}
	out.TenantID = normalizeTenantID(out.TenantID)
	out.RunID = strings.TrimSpace(out.RunID)
	out.InvoiceID = strings.TrimSpace(out.InvoiceID)
	out.EventType = normalizeDunningEventType(domain.DunningEventType(eventType))
	out.State = normalizeDunningRunState(domain.DunningRunState(state))
	if customerExternalID.Valid {
		out.CustomerExternalID = strings.TrimSpace(customerExternalID.String)
	}
	if actionType.Valid {
		out.ActionType = normalizeDunningActionType(domain.DunningActionType(actionType.String))
	}
	if reason.Valid {
		out.Reason = strings.TrimSpace(reason.String)
	}
	if len(metadataRaw) == 0 {
		out.Metadata = map[string]any{}
		return out, nil
	}
	if err := json.Unmarshal(metadataRaw, &out.Metadata); err != nil {
		return domain.InvoiceDunningEvent{}, err
	}
	if out.Metadata == nil {
		out.Metadata = map[string]any{}
	}
	return out, nil
}

func scanDunningNotificationIntent(s rowScanner) (domain.DunningNotificationIntent, error) {
	var out domain.DunningNotificationIntent
	var customerExternalID sql.NullString
	var intentType string
	var actionType sql.NullString
	var status string
	var deliveryBackend sql.NullString
	var recipientEmail sql.NullString
	var payloadRaw []byte
	var lastError sql.NullString
	var dispatchedAt sql.NullTime
	if err := s.Scan(
		&out.ID,
		&out.RunID,
		&out.TenantID,
		&out.InvoiceID,
		&customerExternalID,
		&intentType,
		&actionType,
		&status,
		&deliveryBackend,
		&recipientEmail,
		&payloadRaw,
		&lastError,
		&out.CreatedAt,
		&dispatchedAt,
	); err != nil {
		return domain.DunningNotificationIntent{}, err
	}
	out.TenantID = normalizeTenantID(out.TenantID)
	out.RunID = strings.TrimSpace(out.RunID)
	out.InvoiceID = strings.TrimSpace(out.InvoiceID)
	out.IntentType = normalizeDunningNotificationIntentType(domain.DunningNotificationIntentType(intentType))
	out.Status = normalizeDunningNotificationIntentStatus(domain.DunningNotificationIntentStatus(status))
	if customerExternalID.Valid {
		out.CustomerExternalID = strings.TrimSpace(customerExternalID.String)
	}
	if actionType.Valid {
		out.ActionType = normalizeDunningActionType(domain.DunningActionType(actionType.String))
	}
	if deliveryBackend.Valid {
		out.DeliveryBackend = strings.TrimSpace(deliveryBackend.String)
	}
	if recipientEmail.Valid {
		out.RecipientEmail = strings.ToLower(strings.TrimSpace(recipientEmail.String))
	}
	if lastError.Valid {
		out.LastError = strings.TrimSpace(lastError.String)
	}
	if len(payloadRaw) == 0 {
		out.Payload = map[string]any{}
	} else if err := json.Unmarshal(payloadRaw, &out.Payload); err != nil {
		return domain.DunningNotificationIntent{}, err
	}
	if out.Payload == nil {
		out.Payload = map[string]any{}
	}
	if dispatchedAt.Valid {
		value := dispatchedAt.Time.UTC()
		out.DispatchedAt = &value
	}
	return out, nil
}


func normalizeDunningFinalAction(v domain.DunningFinalAction) domain.DunningFinalAction {
	switch strings.ToLower(strings.TrimSpace(string(v))) {
	case string(domain.DunningFinalActionPause):
		return domain.DunningFinalActionPause
	case string(domain.DunningFinalActionWriteOff):
		return domain.DunningFinalActionWriteOff
	default:
		return domain.DunningFinalActionManualReview
	}
}

func normalizeDunningRunState(v domain.DunningRunState) domain.DunningRunState {
	switch strings.ToLower(strings.TrimSpace(string(v))) {
	case string(domain.DunningRunStateScheduled):
		return domain.DunningRunStateScheduled
	case string(domain.DunningRunStateAwaitingPaymentSetup):
		return domain.DunningRunStateAwaitingPaymentSetup
	case string(domain.DunningRunStateAwaitingRetryResult):
		return domain.DunningRunStateAwaitingRetryResult
	case string(domain.DunningRunStateResolved):
		return domain.DunningRunStateResolved
	case string(domain.DunningRunStatePaused):
		return domain.DunningRunStatePaused
	case string(domain.DunningRunStateEscalated):
		return domain.DunningRunStateEscalated
	case string(domain.DunningRunStateExhausted):
		return domain.DunningRunStateExhausted
	default:
		return domain.DunningRunStateRetryDue
	}
}

func normalizeDunningActionType(v domain.DunningActionType) domain.DunningActionType {
	switch strings.ToLower(strings.TrimSpace(string(v))) {
	case string(domain.DunningActionTypeCollectPaymentReminder):
		return domain.DunningActionTypeCollectPaymentReminder
	case "":
		return ""
	default:
		return domain.DunningActionTypeRetryPayment
	}
}

func normalizeDunningResolution(v domain.DunningResolution) domain.DunningResolution {
	switch strings.ToLower(strings.TrimSpace(string(v))) {
	case string(domain.DunningResolutionPaymentSucceeded):
		return domain.DunningResolutionPaymentSucceeded
	case string(domain.DunningResolutionInvoiceNotCollectible):
		return domain.DunningResolutionInvoiceNotCollectible
	case string(domain.DunningResolutionOperatorResolved):
		return domain.DunningResolutionOperatorResolved
	case string(domain.DunningResolutionEscalated):
		return domain.DunningResolutionEscalated
	default:
		return ""
	}
}

func normalizeDunningEventType(v domain.DunningEventType) domain.DunningEventType {
	switch strings.ToLower(strings.TrimSpace(string(v))) {
	case string(domain.DunningEventTypeRetryScheduled):
		return domain.DunningEventTypeRetryScheduled
	case string(domain.DunningEventTypeRetryAttempted):
		return domain.DunningEventTypeRetryAttempted
	case string(domain.DunningEventTypeRetrySucceeded):
		return domain.DunningEventTypeRetrySucceeded
	case string(domain.DunningEventTypeRetryFailed):
		return domain.DunningEventTypeRetryFailed
	case string(domain.DunningEventTypePaymentSetupPending):
		return domain.DunningEventTypePaymentSetupPending
	case string(domain.DunningEventTypePaymentSetupReady):
		return domain.DunningEventTypePaymentSetupReady
	case string(domain.DunningEventTypeNotificationSent):
		return domain.DunningEventTypeNotificationSent
	case string(domain.DunningEventTypeNotificationFailed):
		return domain.DunningEventTypeNotificationFailed
	case string(domain.DunningEventTypePaused):
		return domain.DunningEventTypePaused
	case string(domain.DunningEventTypeResumed):
		return domain.DunningEventTypeResumed
	case string(domain.DunningEventTypeEscalated):
		return domain.DunningEventTypeEscalated
	case string(domain.DunningEventTypeResolved):
		return domain.DunningEventTypeResolved
	default:
		return domain.DunningEventTypeStarted
	}
}

func normalizeDunningNotificationIntentType(v domain.DunningNotificationIntentType) domain.DunningNotificationIntentType {
	switch strings.ToLower(strings.TrimSpace(string(v))) {
	case string(domain.DunningNotificationIntentTypePaymentFailed):
		return domain.DunningNotificationIntentTypePaymentFailed
	case string(domain.DunningNotificationIntentTypeRetryScheduled):
		return domain.DunningNotificationIntentTypeRetryScheduled
	case string(domain.DunningNotificationIntentTypeFinalAttempt):
		return domain.DunningNotificationIntentTypeFinalAttempt
	case string(domain.DunningNotificationIntentTypeEscalated):
		return domain.DunningNotificationIntentTypeEscalated
	default:
		return domain.DunningNotificationIntentTypePaymentMethodRequired
	}
}

func normalizeDunningNotificationIntentStatus(v domain.DunningNotificationIntentStatus) domain.DunningNotificationIntentStatus {
	switch strings.ToLower(strings.TrimSpace(string(v))) {
	case string(domain.DunningNotificationIntentStatusDispatched):
		return domain.DunningNotificationIntentStatusDispatched
	case string(domain.DunningNotificationIntentStatusFailed):
		return domain.DunningNotificationIntentStatusFailed
	default:
		return domain.DunningNotificationIntentStatusQueued
	}
}

func normalizeDunningSchedule(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.ToLower(strings.TrimSpace(value))
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	if out == nil {
		return []string{}
	}
	return out
}


func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23503"
	}
	return false
}

