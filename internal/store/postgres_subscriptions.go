package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
)

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

	_, err = tx.ExecContext(ctx, `INSERT INTO subscriptions (id, tenant_id, subscription_code, display_name, customer_id, plan_id, status, billing_time, started_at, payment_setup_requested_at, activated_at, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		input.ID, input.TenantID, input.Code, input.DisplayName, input.CustomerID, input.PlanID, input.Status, input.BillingTime, input.StartedAt, input.PaymentSetupRequestedAt, input.ActivatedAt, input.CreatedAt, input.UpdatedAt,
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

	rows, err := tx.QueryContext(ctx, `SELECT id, tenant_id, subscription_code, display_name, customer_id, plan_id, status, billing_time, started_at, payment_setup_requested_at, activated_at, created_at, updated_at FROM subscriptions WHERE tenant_id = $1 ORDER BY created_at DESC, id DESC`, normalizeTenantID(tenantID))
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

	row := tx.QueryRowContext(ctx, `SELECT id, tenant_id, subscription_code, display_name, customer_id, plan_id, status, billing_time, started_at, payment_setup_requested_at, activated_at, created_at, updated_at FROM subscriptions WHERE tenant_id = $1 AND id = $2`, normalizeTenantID(tenantID), strings.TrimSpace(id))
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
	if strings.TrimSpace(string(input.BillingTime)) == "" {
		input.BillingTime = domain.SubscriptionBillingTimeCalendar
	}
	input.UpdatedAt = time.Now().UTC()

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, input.TenantID)
	if err != nil {
		return domain.Subscription{}, err
	}
	defer rollbackSilently(tx)

	result, err := tx.ExecContext(ctx, `UPDATE subscriptions SET subscription_code = $3, display_name = $4, customer_id = $5, plan_id = $6, status = $7, billing_time = $8, started_at = $9, payment_setup_requested_at = $10, activated_at = $11, updated_at = $12 WHERE tenant_id = $1 AND id = $2`,
		input.TenantID, input.ID, input.Code, input.DisplayName, input.CustomerID, input.PlanID, input.Status, input.BillingTime, input.StartedAt, input.PaymentSetupRequestedAt, input.ActivatedAt, input.UpdatedAt,
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


func (s *PostgresStore) GetSubscriptionsDueBilling(before time.Time, limit int) ([]domain.Subscription, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return nil, err
	}
	defer rollbackSilently(tx)

	if limit <= 0 {
		limit = 50
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT id, tenant_id, subscription_code, display_name, customer_id, plan_id,
			status, billing_time, started_at, payment_setup_requested_at, activated_at,
			current_billing_period_start, current_billing_period_end, next_billing_at,
			created_at, updated_at
		FROM subscriptions
		WHERE status = 'active' AND next_billing_at IS NOT NULL AND next_billing_at <= $1
		ORDER BY next_billing_at ASC LIMIT $2`, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []domain.Subscription
	for rows.Next() {
		sub, err := scanSubscriptionFull(rows)
		if err != nil {
			return nil, err
		}
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}

func (s *PostgresStore) UpdateSubscriptionBillingCycle(tenantID, id string, periodStart, periodEnd time.Time, nextBillingAt time.Time) error {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer rollbackSilently(tx)

	_, err = tx.ExecContext(ctx, `
		UPDATE subscriptions SET
			current_billing_period_start = $1,
			current_billing_period_end = $2,
			next_billing_at = $3,
			updated_at = NOW()
		WHERE id = $4`, periodStart, periodEnd, nextBillingAt, id)
	if err != nil {
		return fmt.Errorf("update subscription billing cycle: %w", err)
	}
	return tx.Commit()
}

// ---------------------------------------------------------------------------
// Usage aggregation for billing
// ---------------------------------------------------------------------------


func scanSubscription(s rowScanner) (domain.Subscription, error) {
	var out domain.Subscription
	var status string
	var billingTime string
	var startedAt sql.NullTime
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
		&billingTime,
		&startedAt,
		&paymentSetupRequestedAt,
		&activatedAt,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		return domain.Subscription{}, err
	}
	out.TenantID = normalizeTenantID(out.TenantID)
	out.Status = normalizeSubscriptionStatus(domain.SubscriptionStatus(status))
	switch strings.ToLower(strings.TrimSpace(billingTime)) {
	case string(domain.SubscriptionBillingTimeAnniversary):
		out.BillingTime = domain.SubscriptionBillingTimeAnniversary
	default:
		out.BillingTime = domain.SubscriptionBillingTimeCalendar
	}
	if startedAt.Valid {
		value := startedAt.Time.UTC()
		out.StartedAt = &value
	}
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


func scanSubscriptionFull(s rowScanner) (domain.Subscription, error) {
	var out domain.Subscription
	var status, billingTime string
	if err := s.Scan(
		&out.ID, &out.TenantID, &out.Code, &out.DisplayName, &out.CustomerID, &out.PlanID,
		&status, &billingTime, &out.StartedAt, &out.PaymentSetupRequestedAt, &out.ActivatedAt,
		&out.CurrentBillingPeriodStart, &out.CurrentBillingPeriodEnd, &out.NextBillingAt,
		&out.CreatedAt, &out.UpdatedAt,
	); err != nil {
		return domain.Subscription{}, err
	}
	out.Status = domain.SubscriptionStatus(status)
	out.BillingTime = domain.SubscriptionBillingTime(billingTime)
	return out, nil
}

