package service

import (
	"fmt"
	"strings"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

type MeterService struct {
	store store.Repository
}

func NewMeterService(s store.Repository) *MeterService {
	return &MeterService{store: s}
}

func (s *MeterService) CreateMeter(input domain.Meter) (domain.Meter, error) {
	input.TenantID = normalizeTenantID(input.TenantID)
	if err := s.validateMeterInput(input, false); err != nil {
		return domain.Meter{}, err
	}
	rule, err := s.store.GetRatingRuleVersion(input.TenantID, input.RatingRuleVersionID)
	if err != nil {
		return domain.Meter{}, fmt.Errorf("%w: rating_rule_version_id not found", ErrValidation)
	}
	if normalizeTenantID(rule.TenantID) != input.TenantID {
		return domain.Meter{}, fmt.Errorf("%w: rating rule tenant mismatch", ErrValidation)
	}
	meter, err := s.store.CreateMeter(input)
	if err != nil {
		if err == store.ErrDuplicateKey {
			return domain.Meter{}, fmt.Errorf("%w: meter key already exists", ErrValidation)
		}
		return domain.Meter{}, err
	}
	return meter, nil
}

func (s *MeterService) ListMeters(tenantID string) ([]domain.Meter, error) {
	return s.store.ListMeters(normalizeTenantID(tenantID))
}

func (s *MeterService) UpdateMeter(tenantID, id string, patch domain.Meter) (domain.Meter, error) {
	tenantID = normalizeTenantID(tenantID)
	existing, err := s.store.GetMeter(tenantID, id)
	if err != nil {
		return domain.Meter{}, err
	}
	existing.TenantID = normalizeTenantID(existing.TenantID)
	if strings.TrimSpace(patch.TenantID) != "" && normalizeTenantID(patch.TenantID) != existing.TenantID {
		return domain.Meter{}, fmt.Errorf("%w: tenant mismatch", ErrValidation)
	}

	if strings.TrimSpace(patch.Key) != "" {
		existing.Key = patch.Key
	}
	if strings.TrimSpace(patch.Name) != "" {
		existing.Name = patch.Name
	}
	if strings.TrimSpace(patch.Unit) != "" {
		existing.Unit = patch.Unit
	}
	if strings.TrimSpace(patch.Aggregation) != "" {
		existing.Aggregation = patch.Aggregation
	}
	if strings.TrimSpace(patch.RatingRuleVersionID) != "" {
		rule, err := s.store.GetRatingRuleVersion(existing.TenantID, patch.RatingRuleVersionID)
		if err != nil {
			return domain.Meter{}, fmt.Errorf("%w: rating_rule_version_id not found", ErrValidation)
		}
		if normalizeTenantID(rule.TenantID) != existing.TenantID {
			return domain.Meter{}, fmt.Errorf("%w: rating rule tenant mismatch", ErrValidation)
		}
		existing.RatingRuleVersionID = patch.RatingRuleVersionID
	}

	if err := s.validateMeterInput(existing, true); err != nil {
		return domain.Meter{}, err
	}

	meter, err := s.store.UpdateMeter(existing)
	if err != nil {
		if err == store.ErrDuplicateKey {
			return domain.Meter{}, fmt.Errorf("%w: meter key already exists", ErrValidation)
		}
		return domain.Meter{}, err
	}
	return meter, nil
}

func (s *MeterService) GetMeter(tenantID, id string) (domain.Meter, error) {
	return s.store.GetMeter(normalizeTenantID(tenantID), id)
}

func (s *MeterService) validateMeterInput(input domain.Meter, isUpdate bool) error {
	if strings.TrimSpace(input.Key) == "" {
		return fmt.Errorf("%w: key is required", ErrValidation)
	}
	if strings.TrimSpace(input.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrValidation)
	}
	if strings.TrimSpace(input.Unit) == "" {
		return fmt.Errorf("%w: unit is required", ErrValidation)
	}
	switch input.Aggregation {
	case "sum", "count", "max":
	default:
		return fmt.Errorf("%w: aggregation must be one of sum,count,max", ErrValidation)
	}
	if !isUpdate && strings.TrimSpace(input.RatingRuleVersionID) == "" {
		return fmt.Errorf("%w: rating_rule_version_id is required", ErrValidation)
	}
	return nil
}
