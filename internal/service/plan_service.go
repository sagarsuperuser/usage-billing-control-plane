package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

type PlanService struct {
	store           store.Repository
	planSyncAdapter PlanSyncAdapter
}

var (
	planCodeInvalidRE = regexp.MustCompile(`[^a-z0-9_-]+`)
	planCodeMultiRE   = regexp.MustCompile(`_+`)
)

func NewPlanService(s store.Repository) *PlanService {
	return &PlanService{store: s}
}

func (s *PlanService) WithPlanSyncAdapter(adapter PlanSyncAdapter) *PlanService {
	s.planSyncAdapter = adapter
	return s
}

func (s *PlanService) CreatePlan(ctx context.Context, input domain.Plan) (domain.Plan, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	input.TenantID = normalizeTenantID(input.TenantID)
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	input.Code = normalizePlanCode(input.Code)
	input.BillingInterval = normalizePlanBillingInterval(string(input.BillingInterval))
	input.Status = normalizePlanStatus(input.Status)
	input.MeterIDs = dedupeIDs(input.MeterIDs)
	input.AddOnIDs = dedupeIDs(input.AddOnIDs)
	input.CouponIDs = dedupeIDs(input.CouponIDs)

	if err := validatePlan(input); err != nil {
		return domain.Plan{}, err
	}
	components := make([]PlanSyncComponent, 0, len(input.MeterIDs))
	for _, meterID := range input.MeterIDs {
		meter, err := s.store.GetMeter(input.TenantID, meterID)
		if err != nil {
			if err == store.ErrNotFound {
				return domain.Plan{}, fmt.Errorf("%w: meter_id %s not found", ErrValidation, meterID)
			}
			return domain.Plan{}, err
		}
		rule, err := s.store.GetRatingRuleVersion(input.TenantID, meter.RatingRuleVersionID)
		if err != nil {
			if err == store.ErrNotFound {
				return domain.Plan{}, fmt.Errorf("%w: rating_rule_version_id %s not found", ErrValidation, meter.RatingRuleVersionID)
			}
			return domain.Plan{}, err
		}
		components = append(components, PlanSyncComponent{
			Meter:             meter,
			RatingRuleVersion: rule,
		})
	}
	for _, addOnID := range input.AddOnIDs {
		if _, err := s.store.GetAddOn(input.TenantID, addOnID); err != nil {
			if err == store.ErrNotFound {
				return domain.Plan{}, fmt.Errorf("%w: add_on_id %s not found", ErrValidation, addOnID)
			}
			return domain.Plan{}, err
		}
	}
	for _, couponID := range input.CouponIDs {
		if _, err := s.store.GetCoupon(input.TenantID, couponID); err != nil {
			if err == store.ErrNotFound {
				return domain.Plan{}, fmt.Errorf("%w: coupon_id %s not found", ErrValidation, couponID)
			}
			return domain.Plan{}, err
		}
	}

	plan, err := s.store.CreatePlan(input)
	if err != nil {
		if err == store.ErrDuplicateKey {
			return domain.Plan{}, fmt.Errorf("%w: plan code already exists", ErrValidation)
		}
		return domain.Plan{}, err
	}
	if s.planSyncAdapter != nil {
		if err := s.planSyncAdapter.SyncPlan(ctx, plan, components); err != nil {
			return domain.Plan{}, err
		}
	}
	return plan, nil
}

func (s *PlanService) ListPlans(tenantID string) ([]domain.Plan, error) {
	return s.store.ListPlans(normalizeTenantID(tenantID))
}

func (s *PlanService) GetPlan(tenantID, id string) (domain.Plan, error) {
	return s.store.GetPlan(normalizeTenantID(tenantID), strings.TrimSpace(id))
}

func validatePlan(input domain.Plan) error {
	if input.TenantID == "" {
		return fmt.Errorf("%w: tenant_id is required", ErrValidation)
	}
	if input.Code == "" {
		return fmt.Errorf("%w: code is required", ErrValidation)
	}
	if len(input.Code) > 64 {
		return fmt.Errorf("%w: code length must be <= 64", ErrValidation)
	}
	if input.Name == "" {
		return fmt.Errorf("%w: name is required", ErrValidation)
	}
	if input.Currency == "" || len(input.Currency) != 3 {
		return fmt.Errorf("%w: currency must be a 3-letter code", ErrValidation)
	}
	if input.BillingInterval == "" {
		return fmt.Errorf("%w: billing_interval is required", ErrValidation)
	}
	if input.Status == "" {
		return fmt.Errorf("%w: status is required", ErrValidation)
	}
	if input.BaseAmountCents < 0 {
		return fmt.Errorf("%w: base_amount_cents must be >= 0", ErrValidation)
	}
	if len(input.MeterIDs) == 0 {
		return fmt.Errorf("%w: at least one metric is required", ErrValidation)
	}
	return nil
}

func normalizePlanCode(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}
	raw = planCodeInvalidRE.ReplaceAllString(raw, "_")
	raw = planCodeMultiRE.ReplaceAllString(raw, "_")
	return strings.Trim(raw, "_")
}

func normalizePlanBillingInterval(raw string) domain.BillingInterval {
	switch domain.BillingInterval(strings.ToLower(strings.TrimSpace(string(raw)))) {
	case domain.BillingIntervalYearly:
		return domain.BillingIntervalYearly
	default:
		return domain.BillingIntervalMonthly
	}
}

func normalizePlanStatus(raw domain.PlanStatus) domain.PlanStatus {
	switch domain.PlanStatus(strings.ToLower(strings.TrimSpace(string(raw)))) {
	case domain.PlanStatusActive:
		return domain.PlanStatusActive
	case domain.PlanStatusArchived:
		return domain.PlanStatusArchived
	default:
		return domain.PlanStatusDraft
	}
}

func dedupeIDs(ids []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		normalized := strings.TrimSpace(id)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}
