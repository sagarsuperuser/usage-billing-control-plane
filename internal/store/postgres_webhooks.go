package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
)

func (s *PostgresStore) IngestBillingEvent(input domain.BillingEvent) (domain.BillingEvent, bool, error) {
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
		return domain.BillingEvent{}, false, fmt.Errorf("validation failed: organization_id, webhook_type, and object_type are required")
	}

	payloadJSON, err := json.Marshal(input.Payload)
	if err != nil {
		return domain.BillingEvent{}, false, err
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.BillingEvent{}, false, err
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

	created, scanErr := scanBillingEvent(row)
	if scanErr != nil {
		if !errors.Is(scanErr, sql.ErrNoRows) {
			return domain.BillingEvent{}, false, scanErr
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
		existing, err := scanBillingEvent(existingRow)
		if err != nil {
			return domain.BillingEvent{}, false, err
		}
		if err := tx.Commit(); err != nil {
			return domain.BillingEvent{}, false, err
		}
		return existing, false, nil
	}

	if created.InvoiceID != "" {
		if err := s.upsertInvoicePaymentStatusViewTx(ctx, tx, created); err != nil {
			return domain.BillingEvent{}, false, err
		}
	}

	if err := tx.Commit(); err != nil {
		return domain.BillingEvent{}, false, err
	}
	return created, true, nil
}

func (s *PostgresStore) upsertInvoicePaymentStatusViewTx(ctx context.Context, tx *sql.Tx, event domain.BillingEvent) error {
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
	if err != nil {
		return fmt.Errorf("upsert invoice payment status view: %w", err)
	}
	return nil
}

func (s *PostgresStore) UpsertInvoicePaymentStatusView(input domain.InvoicePaymentStatusView) (domain.InvoicePaymentStatusView, error) {
	tenantID := normalizeTenantID(input.TenantID)
	if tenantID == "" {
		return domain.InvoicePaymentStatusView{}, fmt.Errorf("tenant_id is required")
	}
	invoiceID := strings.TrimSpace(input.InvoiceID)
	if invoiceID == "" {
		return domain.InvoicePaymentStatusView{}, fmt.Errorf("invoice_id is required")
	}
	now := time.Now().UTC()
	lastEventAt := input.LastEventAt.UTC()
	if lastEventAt.IsZero() {
		lastEventAt = now
	}
	updatedAt := input.UpdatedAt.UTC()
	if updatedAt.IsZero() {
		updatedAt = lastEventAt
	}
	lastEventType := strings.TrimSpace(input.LastEventType)
	if lastEventType == "" {
		lastEventType = "invoice.payment_status_observed"
	}
	lastWebhookKey := strings.TrimSpace(input.LastWebhookKey)
	if lastWebhookKey == "" {
		lastWebhookKey = fmt.Sprintf("synthetic:%s:%d", invoiceID, lastEventAt.UnixNano())
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.InvoicePaymentStatusView{}, err
	}
	defer rollbackSilently(tx)

	event := domain.BillingEvent{
		TenantID:             tenantID,
		OrganizationID:       strings.TrimSpace(input.OrganizationID),
		InvoiceID:            invoiceID,
		CustomerExternalID:   strings.TrimSpace(input.CustomerExternalID),
		InvoiceNumber:        strings.TrimSpace(input.InvoiceNumber),
		Currency:             strings.TrimSpace(input.Currency),
		InvoiceStatus:        strings.TrimSpace(input.InvoiceStatus),
		PaymentStatus:        strings.TrimSpace(input.PaymentStatus),
		PaymentOverdue:       input.PaymentOverdue,
		TotalAmountCents:     input.TotalAmountCents,
		TotalDueAmountCents:  input.TotalDueAmountCents,
		TotalPaidAmountCents: input.TotalPaidAmountCents,
		LastPaymentError:     strings.TrimSpace(input.LastPaymentError),
		WebhookType:          lastEventType,
		WebhookKey:           lastWebhookKey,
		OccurredAt:           lastEventAt,
		ReceivedAt:           updatedAt,
	}
	if err := s.upsertInvoicePaymentStatusViewTx(ctx, tx, event); err != nil {
		return domain.InvoicePaymentStatusView{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.InvoicePaymentStatusView{}, err
	}

	input.TenantID = tenantID
	input.InvoiceID = invoiceID
	input.OrganizationID = strings.TrimSpace(input.OrganizationID)
	input.CustomerExternalID = strings.TrimSpace(input.CustomerExternalID)
	input.InvoiceNumber = strings.TrimSpace(input.InvoiceNumber)
	input.Currency = strings.TrimSpace(input.Currency)
	input.InvoiceStatus = strings.TrimSpace(input.InvoiceStatus)
	input.PaymentStatus = strings.TrimSpace(input.PaymentStatus)
	input.LastPaymentError = strings.TrimSpace(input.LastPaymentError)
	input.LastEventType = lastEventType
	input.LastWebhookKey = lastWebhookKey
	input.LastEventAt = lastEventAt
	input.UpdatedAt = updatedAt
	return input, nil
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
	if customerExternalID := strings.TrimSpace(filter.CustomerExternalID); customerExternalID != "" {
		clauses = append(clauses, fmt.Sprintf("customer_external_id = $%d", argPos))
		args = append(args, customerExternalID)
		argPos++
	}
	if invoiceID := strings.TrimSpace(filter.InvoiceID); invoiceID != "" {
		clauses = append(clauses, fmt.Sprintf("invoice_id = $%d", argPos))
		args = append(args, invoiceID)
		argPos++
	}
	if invoiceNumber := strings.TrimSpace(filter.InvoiceNumber); invoiceNumber != "" {
		clauses = append(clauses, fmt.Sprintf("invoice_number ILIKE $%d", argPos))
		args = append(args, "%"+invoiceNumber+"%")
		argPos++
	}
	if lastEventType := strings.TrimSpace(filter.LastEventType); lastEventType != "" {
		clauses = append(clauses, fmt.Sprintf("last_event_type = $%d", argPos))
		args = append(args, lastEventType)
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

func (s *PostgresStore) ListBillingEvents(filter BillingEventListFilter) ([]domain.BillingEvent, error) {
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

	items := make([]domain.BillingEvent, 0, limit)
	for rows.Next() {
		item, scanErr := scanBillingEvent(rows)
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


func (s *PostgresStore) IngestStripeWebhookEvent(input domain.StripeWebhookEvent) (domain.StripeWebhookEvent, bool, error) {
	if input.ID == "" {
		input.ID = newID("swe")
	}
	now := time.Now().UTC()
	if input.ReceivedAt.IsZero() {
		input.ReceivedAt = now
	}
	if input.OccurredAt.IsZero() {
		input.OccurredAt = now
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.StripeWebhookEvent{}, false, err
	}
	defer rollbackSilently(tx)

	payloadJSON, _ := json.Marshal(input.Payload)

	row := tx.QueryRowContext(ctx, `
		INSERT INTO stripe_webhook_events (
			id, tenant_id, stripe_event_id, event_type, object_type,
			invoice_id, customer_external_id, payment_intent_id,
			payment_status, amount_cents, currency, failure_message,
			payload, received_at, occurred_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		ON CONFLICT (stripe_event_id) DO NOTHING
		RETURNING id, tenant_id, stripe_event_id, event_type, object_type,
			invoice_id, customer_external_id, payment_intent_id,
			payment_status, amount_cents, currency, failure_message,
			payload, received_at, occurred_at`,
		input.ID, input.TenantID, input.StripeEventID, input.EventType, input.ObjectType,
		nullIfEmpty(input.InvoiceID), nullIfEmpty(input.CustomerExternalID), nullIfEmpty(input.PaymentIntentID),
		nullIfEmpty(input.PaymentStatus), input.AmountCents, nullIfEmpty(input.Currency), nullIfEmpty(input.FailureMessage),
		payloadJSON, input.ReceivedAt, input.OccurredAt,
	)

	event, err := scanStripeWebhookEvent(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Idempotent: event already existed.
			if commitErr := tx.Commit(); commitErr != nil {
				return domain.StripeWebhookEvent{}, false, commitErr
			}
			return input, true, nil
		}
		return domain.StripeWebhookEvent{}, false, err
	}

	if err := tx.Commit(); err != nil {
		return domain.StripeWebhookEvent{}, false, err
	}
	return event, false, nil
}

func (s *PostgresStore) ListStripeWebhookEvents(filter StripeWebhookEventListFilter) ([]domain.StripeWebhookEvent, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, filter.TenantID)
	if err != nil {
		return nil, err
	}
	defer rollbackSilently(tx)

	var conditions []string
	var args []any
	argN := 1

	if filter.InvoiceID != "" {
		conditions = append(conditions, fmt.Sprintf("invoice_id = $%d", argN))
		args = append(args, filter.InvoiceID)
		argN++
	}
	if filter.EventType != "" {
		conditions = append(conditions, fmt.Sprintf("event_type = $%d", argN))
		args = append(args, filter.EventType)
		argN++
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	sortBy := "received_at"
	order := "DESC"
	limit := 50
	if filter.Limit > 0 && filter.Limit <= 200 {
		limit = filter.Limit
	}
	offset := 0
	if filter.Offset > 0 {
		offset = filter.Offset
	}

	query := fmt.Sprintf(`
		SELECT id, tenant_id, stripe_event_id, event_type, object_type,
			invoice_id, customer_external_id, payment_intent_id,
			payment_status, amount_cents, currency, failure_message,
			payload, received_at, occurred_at
		FROM stripe_webhook_events %s ORDER BY %s %s LIMIT %d OFFSET %d`,
		where, sortBy, order, limit, offset)

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []domain.StripeWebhookEvent
	for rows.Next() {
		event, err := scanStripeWebhookEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
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


func scanBillingEvent(s rowScanner) (domain.BillingEvent, error) {
	var out domain.BillingEvent
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
		return domain.BillingEvent{}, err
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
		return domain.BillingEvent{}, err
	}
	if out.Payload == nil {
		out.Payload = map[string]any{}
	}
	return out, nil
}


func scanStripeWebhookEvent(s rowScanner) (domain.StripeWebhookEvent, error) {
	var out domain.StripeWebhookEvent
	var invoiceID, customerExternalID, paymentIntentID, paymentStatus, currency, failureMessage sql.NullString
	var amountCents sql.NullInt64
	var payloadRaw []byte
	if err := s.Scan(
		&out.ID, &out.TenantID, &out.StripeEventID, &out.EventType, &out.ObjectType,
		&invoiceID, &customerExternalID, &paymentIntentID,
		&paymentStatus, &amountCents, &currency, &failureMessage,
		&payloadRaw, &out.ReceivedAt, &out.OccurredAt,
	); err != nil {
		return domain.StripeWebhookEvent{}, err
	}
	if invoiceID.Valid {
		out.InvoiceID = invoiceID.String
	}
	if customerExternalID.Valid {
		out.CustomerExternalID = customerExternalID.String
	}
	if paymentIntentID.Valid {
		out.PaymentIntentID = paymentIntentID.String
	}
	if paymentStatus.Valid {
		out.PaymentStatus = paymentStatus.String
	}
	if amountCents.Valid {
		v := amountCents.Int64
		out.AmountCents = &v
	}
	if currency.Valid {
		out.Currency = currency.String
	}
	if failureMessage.Valid {
		out.FailureMessage = failureMessage.String
	}
	if len(payloadRaw) > 0 {
		_ = json.Unmarshal(payloadRaw, &out.Payload)
	}
	return out, nil
}

