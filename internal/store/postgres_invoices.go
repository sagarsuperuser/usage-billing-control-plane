package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
)

func (s *PostgresStore) CreateInvoice(input domain.Invoice) (domain.Invoice, error) {
	if input.ID == "" {
		input.ID = newID("inv")
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
		return domain.Invoice{}, err
	}
	defer rollbackSilently(tx)

	metadataJSON, _ := json.Marshal(input.Metadata)

	row := tx.QueryRowContext(ctx, `
		INSERT INTO invoices (
			id, tenant_id, customer_id, subscription_id, invoice_number,
			status, payment_status, currency,
			subtotal_cents, discount_cents, tax_amount_cents, total_amount_cents,
			amount_due_cents, amount_paid_cents,
			billing_period_start, billing_period_end,
			issued_at, due_at, paid_at, voided_at,
			stripe_payment_intent_id, last_payment_error, payment_overdue,
			pdf_object_key, net_payment_term_days, memo, footer, metadata,
			created_at, updated_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30
		) RETURNING
			id, tenant_id, customer_id, subscription_id, invoice_number,
			status, payment_status, currency,
			subtotal_cents, discount_cents, tax_amount_cents, total_amount_cents,
			amount_due_cents, amount_paid_cents,
			billing_period_start, billing_period_end,
			issued_at, due_at, paid_at, voided_at,
			stripe_payment_intent_id, last_payment_error, payment_overdue,
			pdf_object_key, net_payment_term_days, memo, footer, metadata,
			created_at, updated_at`,
		input.ID, input.TenantID, input.CustomerID, input.SubscriptionID, input.InvoiceNumber,
		string(input.Status), string(input.PaymentStatus), input.Currency,
		input.SubtotalCents, input.DiscountCents, input.TaxAmountCents, input.TotalAmountCents,
		input.AmountDueCents, input.AmountPaidCents,
		input.BillingPeriodStart, input.BillingPeriodEnd,
		input.IssuedAt, input.DueAt, input.PaidAt, input.VoidedAt,
		nullIfEmpty(input.StripePaymentIntentID), nullIfEmpty(input.LastPaymentError), input.PaymentOverdue,
		nullIfEmpty(input.PDFObjectKey), input.NetPaymentTermDays, nullIfEmpty(input.Memo), nullIfEmpty(input.Footer), metadataJSON,
		input.CreatedAt, input.UpdatedAt,
	)
	inv, err := scanInvoice(row)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Invoice{}, ErrAlreadyExists
		}
		return domain.Invoice{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Invoice{}, err
	}
	return inv, nil
}

func (s *PostgresStore) GetInvoice(tenantID, id string) (domain.Invoice, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.Invoice{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `
		SELECT id, tenant_id, customer_id, subscription_id, invoice_number,
			status, payment_status, currency,
			subtotal_cents, discount_cents, tax_amount_cents, total_amount_cents,
			amount_due_cents, amount_paid_cents,
			billing_period_start, billing_period_end,
			issued_at, due_at, paid_at, voided_at,
			stripe_payment_intent_id, last_payment_error, payment_overdue,
			pdf_object_key, net_payment_term_days, memo, footer, metadata,
			created_at, updated_at
		FROM invoices WHERE id = $1`, id)
	return scanInvoice(row)
}

func (s *PostgresStore) GetInvoiceByNumber(tenantID, number string) (domain.Invoice, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.Invoice{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `
		SELECT id, tenant_id, customer_id, subscription_id, invoice_number,
			status, payment_status, currency,
			subtotal_cents, discount_cents, tax_amount_cents, total_amount_cents,
			amount_due_cents, amount_paid_cents,
			billing_period_start, billing_period_end,
			issued_at, due_at, paid_at, voided_at,
			stripe_payment_intent_id, last_payment_error, payment_overdue,
			pdf_object_key, net_payment_term_days, memo, footer, metadata,
			created_at, updated_at
		FROM invoices WHERE invoice_number = $1`, number)
	return scanInvoice(row)
}

func (s *PostgresStore) GetInvoiceByStripePaymentIntentID(tenantID, stripePaymentIntentID string) (domain.Invoice, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.Invoice{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `
		SELECT id, tenant_id, customer_id, subscription_id, invoice_number,
			status, payment_status, currency,
			subtotal_cents, discount_cents, tax_amount_cents, total_amount_cents,
			amount_due_cents, amount_paid_cents,
			billing_period_start, billing_period_end,
			issued_at, due_at, paid_at, voided_at,
			stripe_payment_intent_id, last_payment_error, payment_overdue,
			pdf_object_key, net_payment_term_days, memo, footer, metadata,
			created_at, updated_at
		FROM invoices WHERE stripe_payment_intent_id = $1`, stripePaymentIntentID)
	return scanInvoice(row)
}

func (s *PostgresStore) ListInvoices(filter InvoiceListFilter) ([]domain.Invoice, int, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, filter.TenantID)
	if err != nil {
		return nil, 0, err
	}
	defer rollbackSilently(tx)

	var conditions []string
	var args []any
	argN := 1

	if filter.CustomerID != "" {
		conditions = append(conditions, fmt.Sprintf("customer_id = $%d", argN))
		args = append(args, filter.CustomerID)
		argN++
	}
	if filter.SubscriptionID != "" {
		conditions = append(conditions, fmt.Sprintf("subscription_id = $%d", argN))
		args = append(args, filter.SubscriptionID)
		argN++
	}
	if filter.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argN))
		args = append(args, filter.Status)
		argN++
	}
	if filter.PaymentStatus != "" {
		conditions = append(conditions, fmt.Sprintf("payment_status = $%d", argN))
		args = append(args, filter.PaymentStatus)
		argN++
	}
	if filter.PaymentOverdue != nil {
		conditions = append(conditions, fmt.Sprintf("payment_overdue = $%d", argN))
		args = append(args, *filter.PaymentOverdue)
		argN++
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	sortBy := "created_at"
	switch strings.ToLower(strings.TrimSpace(filter.SortBy)) {
	case "created_at", "":
		sortBy = "created_at"
	case "updated_at":
		sortBy = "updated_at"
	case "billing_period_start":
		sortBy = "billing_period_start"
	case "total_amount_cents":
		sortBy = "total_amount_cents"
	case "amount_due_cents":
		sortBy = "amount_due_cents"
	case "invoice_number":
		sortBy = "invoice_number"
	default:
		sortBy = "created_at"
	}
	order := "DESC"
	if filter.SortDesc {
		order = "DESC"
	} else if filter.SortBy != "" {
		order = "ASC"
	}

	limit := 50
	if filter.Limit > 0 && filter.Limit <= 200 {
		limit = filter.Limit
	}
	offset := 0
	if filter.Offset > 0 {
		offset = filter.Offset
	}

	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM invoices %s", where)
	if err := tx.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(`
		SELECT id, tenant_id, customer_id, subscription_id, invoice_number,
			status, payment_status, currency,
			subtotal_cents, discount_cents, tax_amount_cents, total_amount_cents,
			amount_due_cents, amount_paid_cents,
			billing_period_start, billing_period_end,
			issued_at, due_at, paid_at, voided_at,
			stripe_payment_intent_id, last_payment_error, payment_overdue,
			pdf_object_key, net_payment_term_days, memo, footer, metadata,
			created_at, updated_at
		FROM invoices %s ORDER BY %s %s LIMIT %d OFFSET %d`,
		where, sortBy, order, limit, offset)

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var invoices []domain.Invoice
	for rows.Next() {
		inv, err := scanInvoice(rows)
		if err != nil {
			return nil, 0, err
		}
		invoices = append(invoices, inv)
	}
	return invoices, total, rows.Err()
}

func (s *PostgresStore) UpdateInvoiceStatus(tenantID, id string, status domain.InvoiceStatus, updatedAt time.Time) (domain.Invoice, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.Invoice{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `
		UPDATE invoices SET status = $1, updated_at = $2
		WHERE id = $3
		RETURNING id, tenant_id, customer_id, subscription_id, invoice_number,
			status, payment_status, currency,
			subtotal_cents, discount_cents, tax_amount_cents, total_amount_cents,
			amount_due_cents, amount_paid_cents,
			billing_period_start, billing_period_end,
			issued_at, due_at, paid_at, voided_at,
			stripe_payment_intent_id, last_payment_error, payment_overdue,
			pdf_object_key, net_payment_term_days, memo, footer, metadata,
			created_at, updated_at`,
		string(status), updatedAt, id)
	inv, err := scanInvoice(row)
	if err != nil {
		return domain.Invoice{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Invoice{}, err
	}
	return inv, nil
}

func (s *PostgresStore) UpdateInvoicePayment(tenantID, id string, paymentStatus domain.InvoicePaymentStatus, stripePaymentIntentID string, lastPaymentError string, paidAt *time.Time, updatedAt time.Time) (domain.Invoice, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.Invoice{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `
		UPDATE invoices SET
			payment_status = $1,
			stripe_payment_intent_id = COALESCE(NULLIF($2,''), stripe_payment_intent_id),
			last_payment_error = $3,
			paid_at = $4,
			payment_overdue = CASE WHEN $1 = 'succeeded' THEN FALSE ELSE payment_overdue END,
			updated_at = $5
		WHERE id = $6
		RETURNING id, tenant_id, customer_id, subscription_id, invoice_number,
			status, payment_status, currency,
			subtotal_cents, discount_cents, tax_amount_cents, total_amount_cents,
			amount_due_cents, amount_paid_cents,
			billing_period_start, billing_period_end,
			issued_at, due_at, paid_at, voided_at,
			stripe_payment_intent_id, last_payment_error, payment_overdue,
			pdf_object_key, net_payment_term_days, memo, footer, metadata,
			created_at, updated_at`,
		string(paymentStatus), stripePaymentIntentID, nullIfEmpty(lastPaymentError), paidAt, updatedAt, id)
	inv, err := scanInvoice(row)
	if err != nil {
		return domain.Invoice{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Invoice{}, err
	}
	return inv, nil
}

func (s *PostgresStore) CreateInvoiceLineItem(input domain.InvoiceLineItem) (domain.InvoiceLineItem, error) {
	if input.ID == "" {
		input.ID = newID("ili")
	}
	now := time.Now().UTC()
	if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, input.TenantID)
	if err != nil {
		return domain.InvoiceLineItem{}, err
	}
	defer rollbackSilently(tx)

	metadataJSON, _ := json.Marshal(input.Metadata)

	row := tx.QueryRowContext(ctx, `
		INSERT INTO invoice_line_items (
			id, invoice_id, tenant_id, line_type,
			meter_id, add_on_id, coupon_id, tax_id,
			description, quantity, unit_amount_cents, amount_cents,
			tax_rate, tax_amount_cents, total_amount_cents,
			pricing_mode, rating_rule_version_id,
			billing_period_start, billing_period_end, metadata, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)
		RETURNING id, invoice_id, tenant_id, line_type,
			meter_id, add_on_id, coupon_id, tax_id,
			description, quantity, unit_amount_cents, amount_cents,
			tax_rate, tax_amount_cents, total_amount_cents,
			pricing_mode, rating_rule_version_id,
			billing_period_start, billing_period_end, metadata, created_at`,
		input.ID, input.InvoiceID, input.TenantID, string(input.LineType),
		nullIfEmpty(input.MeterID), nullIfEmpty(input.AddOnID), nullIfEmpty(input.CouponID), nullIfEmpty(input.TaxID),
		input.Description, input.Quantity, input.UnitAmountCents, input.AmountCents,
		input.TaxRate, input.TaxAmountCents, input.TotalAmountCents,
		nullIfEmpty(input.PricingMode), nullIfEmpty(input.RatingRuleVersionID),
		input.BillingPeriodStart, input.BillingPeriodEnd, metadataJSON, input.CreatedAt,
	)
	item, err := scanInvoiceLineItem(row)
	if err != nil {
		return domain.InvoiceLineItem{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.InvoiceLineItem{}, err
	}
	return item, nil
}

func (s *PostgresStore) ListInvoiceLineItems(tenantID, invoiceID string) ([]domain.InvoiceLineItem, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return nil, err
	}
	defer rollbackSilently(tx)

	rows, err := tx.QueryContext(ctx, `
		SELECT id, invoice_id, tenant_id, line_type,
			meter_id, add_on_id, coupon_id, tax_id,
			description, quantity, unit_amount_cents, amount_cents,
			tax_rate, tax_amount_cents, total_amount_cents,
			pricing_mode, rating_rule_version_id,
			billing_period_start, billing_period_end, metadata, created_at
		FROM invoice_line_items WHERE invoice_id = $1 ORDER BY created_at`, invoiceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.InvoiceLineItem
	for rows.Next() {
		item, err := scanInvoiceLineItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// ---------------------------------------------------------------------------
// Subscription billing cycle tracking
// ---------------------------------------------------------------------------


func scanInvoice(s rowScanner) (domain.Invoice, error) {
	var out domain.Invoice
	var status, paymentStatus string
	var stripePI, lastErr, pdfKey, memo, footer sql.NullString
	var metadataRaw []byte
	if err := s.Scan(
		&out.ID, &out.TenantID, &out.CustomerID, &out.SubscriptionID, &out.InvoiceNumber,
		&status, &paymentStatus, &out.Currency,
		&out.SubtotalCents, &out.DiscountCents, &out.TaxAmountCents, &out.TotalAmountCents,
		&out.AmountDueCents, &out.AmountPaidCents,
		&out.BillingPeriodStart, &out.BillingPeriodEnd,
		&out.IssuedAt, &out.DueAt, &out.PaidAt, &out.VoidedAt,
		&stripePI, &lastErr, &out.PaymentOverdue,
		&pdfKey, &out.NetPaymentTermDays, &memo, &footer, &metadataRaw,
		&out.CreatedAt, &out.UpdatedAt,
	); err != nil {
		return domain.Invoice{}, err
	}
	out.Status = domain.InvoiceStatus(status)
	out.PaymentStatus = domain.InvoicePaymentStatus(paymentStatus)
	if stripePI.Valid {
		out.StripePaymentIntentID = stripePI.String
	}
	if lastErr.Valid {
		out.LastPaymentError = lastErr.String
	}
	if pdfKey.Valid {
		out.PDFObjectKey = pdfKey.String
	}
	if memo.Valid {
		out.Memo = memo.String
	}
	if footer.Valid {
		out.Footer = footer.String
	}
	if len(metadataRaw) > 0 {
		_ = json.Unmarshal(metadataRaw, &out.Metadata)
	}
	return out, nil
}

func scanInvoiceLineItem(s rowScanner) (domain.InvoiceLineItem, error) {
	var out domain.InvoiceLineItem
	var lineType string
	var meterID, addOnID, couponID, taxID, pricingMode, ratingRuleVersionID sql.NullString
	var metadataRaw []byte
	if err := s.Scan(
		&out.ID, &out.InvoiceID, &out.TenantID, &lineType,
		&meterID, &addOnID, &couponID, &taxID,
		&out.Description, &out.Quantity, &out.UnitAmountCents, &out.AmountCents,
		&out.TaxRate, &out.TaxAmountCents, &out.TotalAmountCents,
		&pricingMode, &ratingRuleVersionID,
		&out.BillingPeriodStart, &out.BillingPeriodEnd, &metadataRaw, &out.CreatedAt,
	); err != nil {
		return domain.InvoiceLineItem{}, err
	}
	out.LineType = domain.InvoiceLineItemType(lineType)
	if meterID.Valid {
		out.MeterID = meterID.String
	}
	if addOnID.Valid {
		out.AddOnID = addOnID.String
	}
	if couponID.Valid {
		out.CouponID = couponID.String
	}
	if taxID.Valid {
		out.TaxID = taxID.String
	}
	if pricingMode.Valid {
		out.PricingMode = pricingMode.String
	}
	if ratingRuleVersionID.Valid {
		out.RatingRuleVersionID = ratingRuleVersionID.String
	}
	if len(metadataRaw) > 0 {
		_ = json.Unmarshal(metadataRaw, &out.Metadata)
	}
	return out, nil
}

