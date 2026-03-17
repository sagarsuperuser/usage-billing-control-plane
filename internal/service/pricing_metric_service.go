package service

import (
	"fmt"
	"regexp"
	"strings"

	"usage-billing-control-plane/internal/domain"
)

type PricingMetricService struct {
	ratingService *RatingService
	meterService  *MeterService
}

type CreatePricingMetricInput struct {
	TenantID    string `json:"tenant_id,omitempty"`
	Key         string `json:"key"`
	Name        string `json:"name"`
	Unit        string `json:"unit"`
	Aggregation string `json:"aggregation"`
	Currency    string `json:"currency"`
}

var (
	metricKeyInvalidRE = regexp.MustCompile(`[^a-z0-9_-]+`)
	metricKeyMultiRE   = regexp.MustCompile(`_+`)
)

func NewPricingMetricService(ratingService *RatingService, meterService *MeterService) *PricingMetricService {
	return &PricingMetricService{ratingService: ratingService, meterService: meterService}
}

func (s *PricingMetricService) CreateMetric(input CreatePricingMetricInput) (domain.Meter, error) {
	input.TenantID = normalizeTenantID(input.TenantID)
	input.Key = normalizeMetricKey(input.Key)
	input.Name = strings.TrimSpace(input.Name)
	input.Unit = strings.TrimSpace(input.Unit)
	input.Aggregation = strings.ToLower(strings.TrimSpace(input.Aggregation))
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	if input.Currency == "" {
		input.Currency = "USD"
	}
	if input.TenantID == "" {
		return domain.Meter{}, fmt.Errorf("%w: tenant_id is required", ErrValidation)
	}
	if input.Key == "" {
		return domain.Meter{}, fmt.Errorf("%w: key is required", ErrValidation)
	}
	if input.Name == "" {
		return domain.Meter{}, fmt.Errorf("%w: name is required", ErrValidation)
	}
	if input.Unit == "" {
		return domain.Meter{}, fmt.Errorf("%w: unit is required", ErrValidation)
	}
	if input.Aggregation == "" {
		return domain.Meter{}, fmt.Errorf("%w: aggregation is required", ErrValidation)
	}
	if meters, err := s.meterService.ListMeters(input.TenantID); err == nil {
		for _, meter := range meters {
			if strings.EqualFold(strings.TrimSpace(meter.Key), input.Key) {
				return domain.Meter{}, fmt.Errorf("%w: metric key already exists", ErrValidation)
			}
		}
	}

	rule, err := s.ratingService.CreateRuleVersion(domain.RatingRuleVersion{
		TenantID:        input.TenantID,
		RuleKey:         input.Key + "_default",
		Name:            input.Name + " default rule",
		Version:         1,
		LifecycleState:  domain.RatingRuleLifecycleDraft,
		Mode:            domain.PricingModeFlat,
		Currency:        input.Currency,
		FlatAmountCents: 0,
	})
	if err != nil {
		return domain.Meter{}, err
	}

	return s.meterService.CreateMeter(domain.Meter{
		TenantID:            input.TenantID,
		Key:                 input.Key,
		Name:                input.Name,
		Unit:                input.Unit,
		Aggregation:         input.Aggregation,
		RatingRuleVersionID: rule.ID,
	})
}

func (s *PricingMetricService) ListMetrics(tenantID string) ([]domain.Meter, error) {
	return s.meterService.ListMeters(tenantID)
}

func (s *PricingMetricService) GetMetric(tenantID, id string) (domain.Meter, error) {
	return s.meterService.GetMeter(tenantID, id)
}

func normalizeMetricKey(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}
	raw = metricKeyInvalidRE.ReplaceAllString(raw, "_")
	raw = metricKeyMultiRE.ReplaceAllString(raw, "_")
	return strings.Trim(raw, "_")
}
