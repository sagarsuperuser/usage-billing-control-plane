package service

import (
	"fmt"
	"regexp"
	"strings"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

type AddOnService struct {
	store store.Repository
}

var (
	addOnCodeInvalidRE = regexp.MustCompile(`[^a-z0-9_-]+`)
	addOnCodeMultiRE   = regexp.MustCompile(`_+`)
)

func NewAddOnService(s store.Repository) *AddOnService {
	return &AddOnService{store: s}
}

func (s *AddOnService) CreateAddOn(input domain.AddOn) (domain.AddOn, error) {
	input.TenantID = normalizeTenantID(input.TenantID)
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	input.Code = normalizeAddOnCode(input.Code)
	input.BillingInterval = normalizePlanBillingInterval(string(input.BillingInterval))
	input.Status = normalizeAddOnStatus(input.Status)

	if err := validateAddOn(input); err != nil {
		return domain.AddOn{}, err
	}
	addOn, err := s.store.CreateAddOn(input)
	if err != nil {
		if err == store.ErrDuplicateKey {
			return domain.AddOn{}, fmt.Errorf("%w: add-on code already exists", ErrValidation)
		}
		return domain.AddOn{}, err
	}
	return addOn, nil
}

func (s *AddOnService) ListAddOns(tenantID string) ([]domain.AddOn, error) {
	return s.store.ListAddOns(normalizeTenantID(tenantID))
}

func (s *AddOnService) GetAddOn(tenantID, id string) (domain.AddOn, error) {
	return s.store.GetAddOn(normalizeTenantID(tenantID), strings.TrimSpace(id))
}

func validateAddOn(input domain.AddOn) error {
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
	if input.AmountCents < 0 {
		return fmt.Errorf("%w: amount_cents must be >= 0", ErrValidation)
	}
	return nil
}

func normalizeAddOnCode(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}
	raw = addOnCodeInvalidRE.ReplaceAllString(raw, "_")
	raw = addOnCodeMultiRE.ReplaceAllString(raw, "_")
	return strings.Trim(raw, "_")
}

func normalizeAddOnStatus(raw domain.AddOnStatus) domain.AddOnStatus {
	switch domain.AddOnStatus(strings.ToLower(strings.TrimSpace(string(raw)))) {
	case domain.AddOnStatusActive:
		return domain.AddOnStatusActive
	case domain.AddOnStatusArchived:
		return domain.AddOnStatusArchived
	default:
		return domain.AddOnStatusDraft
	}
}
