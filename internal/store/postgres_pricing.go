package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"

	"usage-billing-control-plane/internal/domain"
)

func (s *PostgresStore) CreateTax(input domain.Tax) (domain.Tax, error) {
	if input.ID == "" {
		input.ID = newID("tax")
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
		return domain.Tax{}, err
	}
	defer rollbackSilently(tx)

	_, err = tx.ExecContext(ctx, `INSERT INTO taxes (id, tenant_id, tax_code, name, description, status, rate, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		input.ID, input.TenantID, input.Code, input.Name, input.Description, input.Status, input.Rate, input.CreatedAt, input.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Tax{}, ErrDuplicateKey
		}
		return domain.Tax{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Tax{}, err
	}
	return input, nil
}

func (s *PostgresStore) ListTaxes(tenantID string) ([]domain.Tax, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return nil, err
	}
	defer rollbackSilently(tx)

	rows, err := tx.QueryContext(ctx, `SELECT id, tenant_id, tax_code, name, description, status, rate, created_at, updated_at FROM taxes WHERE tenant_id = $1 ORDER BY created_at ASC, id ASC`, normalizeTenantID(tenantID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.Tax, 0)
	for rows.Next() {
		item, scanErr := scanTax(rows)
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

func (s *PostgresStore) GetTax(tenantID, id string) (domain.Tax, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.Tax{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `SELECT id, tenant_id, tax_code, name, description, status, rate, created_at, updated_at FROM taxes WHERE tenant_id = $1 AND id = $2`, normalizeTenantID(tenantID), strings.TrimSpace(id))
	out, err := scanTax(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Tax{}, ErrNotFound
		}
		return domain.Tax{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Tax{}, err
	}
	return out, nil
}

func (s *PostgresStore) GetTaxByCode(tenantID, code string) (domain.Tax, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.Tax{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `SELECT id, tenant_id, tax_code, name, description, status, rate, created_at, updated_at FROM taxes WHERE tenant_id = $1 AND tax_code = $2`, normalizeTenantID(tenantID), strings.TrimSpace(code))
	out, err := scanTax(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Tax{}, ErrNotFound
		}
		return domain.Tax{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Tax{}, err
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
	for idx, addOnID := range input.AddOnIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO plan_add_ons (tenant_id, plan_id, add_on_id, position, created_at) VALUES ($1,$2,$3,$4,$5)`, input.TenantID, input.ID, addOnID, idx, now); err != nil {
			if isUniqueViolation(err) {
				return domain.Plan{}, ErrDuplicateKey
			}
			return domain.Plan{}, err
		}
	}
	for idx, couponID := range input.CouponIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO plan_coupons (tenant_id, plan_id, coupon_id, position, created_at) VALUES ($1,$2,$3,$4,$5)`, input.TenantID, input.ID, couponID, idx, now); err != nil {
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

func (s *PostgresStore) CreateAddOn(input domain.AddOn) (domain.AddOn, error) {
	if input.ID == "" {
		input.ID = newID("aon")
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
		return domain.AddOn{}, err
	}
	defer rollbackSilently(tx)

	_, err = tx.ExecContext(ctx, `INSERT INTO add_ons (id, tenant_id, add_on_code, name, description, currency, billing_interval, status, amount_cents, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		input.ID, input.TenantID, input.Code, input.Name, input.Description, input.Currency, input.BillingInterval, input.Status, input.AmountCents, input.CreatedAt, input.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.AddOn{}, ErrDuplicateKey
		}
		return domain.AddOn{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.AddOn{}, err
	}
	return input, nil
}

func (s *PostgresStore) CreateCoupon(input domain.Coupon) (domain.Coupon, error) {
	if input.ID == "" {
		input.ID = newID("cpn")
	}
	now := time.Now().UTC()
	if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}
	input.TenantID = normalizeTenantID(input.TenantID)
	if strings.TrimSpace(string(input.Frequency)) == "" {
		input.Frequency = domain.CouponFrequencyForever
	}
	input.UpdatedAt = now

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, input.TenantID)
	if err != nil {
		return domain.Coupon{}, err
	}
	defer rollbackSilently(tx)

	_, err = tx.ExecContext(ctx, `INSERT INTO coupons (id, tenant_id, coupon_code, name, description, status, discount_type, currency, amount_off_cents, percent_off, frequency, frequency_duration, expiration_at, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		input.ID, input.TenantID, input.Code, input.Name, input.Description, input.Status, input.DiscountType, input.Currency, input.AmountOffCents, input.PercentOff, input.Frequency, input.FrequencyDuration, input.ExpirationAt, input.CreatedAt, input.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Coupon{}, ErrDuplicateKey
		}
		return domain.Coupon{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Coupon{}, err
	}
	return input, nil
}

func (s *PostgresStore) ListAddOns(tenantID string) ([]domain.AddOn, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return nil, err
	}
	defer rollbackSilently(tx)

	rows, err := tx.QueryContext(ctx, `SELECT id, tenant_id, add_on_code, name, description, currency, billing_interval, status, amount_cents, created_at, updated_at FROM add_ons WHERE tenant_id = $1 ORDER BY created_at ASC, id ASC`, normalizeTenantID(tenantID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.AddOn, 0)
	for rows.Next() {
		addOn, scanErr := scanAddOn(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, addOn)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PostgresStore) GetAddOn(tenantID, id string) (domain.AddOn, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.AddOn{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `SELECT id, tenant_id, add_on_code, name, description, currency, billing_interval, status, amount_cents, created_at, updated_at FROM add_ons WHERE tenant_id = $1 AND id = $2`, normalizeTenantID(tenantID), id)
	addOn, err := scanAddOn(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.AddOn{}, ErrNotFound
		}
		return domain.AddOn{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.AddOn{}, err
	}
	return addOn, nil
}

func (s *PostgresStore) ListCoupons(tenantID string) ([]domain.Coupon, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return nil, err
	}
	defer rollbackSilently(tx)

	rows, err := tx.QueryContext(ctx, `SELECT id, tenant_id, coupon_code, name, description, status, discount_type, currency, amount_off_cents, percent_off, frequency, frequency_duration, expiration_at, created_at, updated_at FROM coupons WHERE tenant_id = $1 ORDER BY created_at ASC, id ASC`, normalizeTenantID(tenantID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.Coupon, 0)
	for rows.Next() {
		coupon, scanErr := scanCoupon(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, coupon)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PostgresStore) GetCoupon(tenantID, id string) (domain.Coupon, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.Coupon{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `SELECT id, tenant_id, coupon_code, name, description, status, discount_type, currency, amount_off_cents, percent_off, frequency, frequency_duration, expiration_at, created_at, updated_at FROM coupons WHERE tenant_id = $1 AND id = $2`, normalizeTenantID(tenantID), id)
	coupon, err := scanCoupon(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Coupon{}, ErrNotFound
		}
		return domain.Coupon{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Coupon{}, err
	}
	return coupon, nil
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
	addOnIDsByPlan, err := loadPlanAddOnIDs(ctx, tx, normalizeTenantID(tenantID), ids)
	if err != nil {
		return nil, err
	}
	couponIDsByPlan, err := loadPlanCouponIDs(ctx, tx, normalizeTenantID(tenantID), ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].MeterIDs = metricIDsByPlan[out[i].ID]
		if out[i].MeterIDs == nil {
			out[i].MeterIDs = []string{}
		}
		out[i].AddOnIDs = addOnIDsByPlan[out[i].ID]
		if out[i].AddOnIDs == nil {
			out[i].AddOnIDs = []string{}
		}
		out[i].CouponIDs = couponIDsByPlan[out[i].ID]
		if out[i].CouponIDs == nil {
			out[i].CouponIDs = []string{}
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
	addOnIDsByPlan, err := loadPlanAddOnIDs(ctx, tx, normalizeTenantID(tenantID), []string{id})
	if err != nil {
		return domain.Plan{}, err
	}
	plan.AddOnIDs = addOnIDsByPlan[id]
	if plan.AddOnIDs == nil {
		plan.AddOnIDs = []string{}
	}
	couponIDsByPlan, err := loadPlanCouponIDs(ctx, tx, normalizeTenantID(tenantID), []string{id})
	if err != nil {
		return domain.Plan{}, err
	}
	plan.CouponIDs = couponIDsByPlan[id]
	if plan.CouponIDs == nil {
		plan.CouponIDs = []string{}
	}
	if err := tx.Commit(); err != nil {
		return domain.Plan{}, err
	}
	return plan, nil
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

func loadPlanAddOnIDs(ctx context.Context, tx *sql.Tx, tenantID string, planIDs []string) (map[string][]string, error) {
	out := make(map[string][]string, len(planIDs))
	if len(planIDs) == 0 {
		return out, nil
	}
	rows, err := tx.QueryContext(ctx, `SELECT plan_id, add_on_id FROM plan_add_ons WHERE tenant_id = $1 AND plan_id = ANY($2) ORDER BY position ASC, add_on_id ASC`, tenantID, pq.Array(planIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var planID string
		var addOnID string
		if err := rows.Scan(&planID, &addOnID); err != nil {
			return nil, err
		}
		out[planID] = append(out[planID], addOnID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func loadPlanCouponIDs(ctx context.Context, tx *sql.Tx, tenantID string, planIDs []string) (map[string][]string, error) {
	out := make(map[string][]string, len(planIDs))
	if len(planIDs) == 0 {
		return out, nil
	}
	rows, err := tx.QueryContext(ctx, `SELECT plan_id, coupon_id FROM plan_coupons WHERE tenant_id = $1 AND plan_id = ANY($2) ORDER BY position ASC, coupon_id ASC`, tenantID, pq.Array(planIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var planID string
		var couponID string
		if err := rows.Scan(&planID, &couponID); err != nil {
			return nil, err
		}
		out[planID] = append(out[planID], couponID)
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

func scanAddOn(s rowScanner) (domain.AddOn, error) {
	var out domain.AddOn
	var description sql.NullString
	var billingInterval string
	var status string
	if err := s.Scan(&out.ID, &out.TenantID, &out.Code, &out.Name, &description, &out.Currency, &billingInterval, &status, &out.AmountCents, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return domain.AddOn{}, err
	}
	out.TenantID = normalizeTenantID(out.TenantID)
	out.BillingInterval = domain.BillingInterval(strings.TrimSpace(billingInterval))
	out.Status = domain.AddOnStatus(strings.TrimSpace(status))
	if description.Valid {
		out.Description = normalizeOptionalText(description.String)
	}
	return out, nil
}


func scanCoupon(s rowScanner) (domain.Coupon, error) {
	var out domain.Coupon
	var description sql.NullString
	var status string
	var discountType string
	var currency sql.NullString
	var frequency string
	var expirationAt sql.NullTime
	if err := s.Scan(&out.ID, &out.TenantID, &out.Code, &out.Name, &description, &status, &discountType, &currency, &out.AmountOffCents, &out.PercentOff, &frequency, &out.FrequencyDuration, &expirationAt, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return domain.Coupon{}, err
	}
	out.TenantID = normalizeTenantID(out.TenantID)
	out.Status = domain.CouponStatus(strings.TrimSpace(status))
	out.DiscountType = domain.CouponDiscountType(strings.TrimSpace(discountType))
	out.Frequency = domain.CouponFrequency(strings.TrimSpace(frequency))
	if out.Frequency == "" {
		out.Frequency = domain.CouponFrequencyForever
	}
	if description.Valid {
		out.Description = normalizeOptionalText(description.String)
	}
	if currency.Valid {
		out.Currency = normalizeOptionalText(strings.ToUpper(currency.String))
	}
	if expirationAt.Valid {
		ts := expirationAt.Time.UTC()
		out.ExpirationAt = &ts
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


func scanTax(s rowScanner) (domain.Tax, error) {
	var out domain.Tax
	var description sql.NullString
	var status string
	if err := s.Scan(&out.ID, &out.TenantID, &out.Code, &out.Name, &description, &status, &out.Rate, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return domain.Tax{}, err
	}
	out.ID = strings.TrimSpace(out.ID)
	out.TenantID = normalizeTenantID(out.TenantID)
	out.Code = strings.TrimSpace(out.Code)
	out.Name = strings.TrimSpace(out.Name)
	if description.Valid {
		out.Description = normalizeOptionalText(description.String)
	}
	out.Status = domain.TaxStatus(strings.ToLower(strings.TrimSpace(status)))
	return out, nil
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

