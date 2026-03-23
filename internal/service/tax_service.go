package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

type TaxService struct {
	store          store.Repository
	taxSyncAdapter TaxSyncAdapter
}

var (
	taxCodeInvalidRE = regexp.MustCompile(`[^a-z0-9_-]+`)
	taxCodeMultiRE   = regexp.MustCompile(`_+`)
)

func NewTaxService(s store.Repository) *TaxService {
	return &TaxService{store: s}
}

func (s *TaxService) WithSyncAdapter(adapter TaxSyncAdapter) *TaxService {
	s.taxSyncAdapter = adapter
	return s
}

func (s *TaxService) CreateTax(ctx context.Context, input domain.Tax) (domain.Tax, error) {
	input.TenantID = normalizeTenantID(input.TenantID)
	input.Code = normalizeTaxCode(input.Code)
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.Status = normalizeTaxStatus(input.Status)
	if err := validateTax(input); err != nil {
		return domain.Tax{}, err
	}
	item, err := s.store.CreateTax(input)
	if err != nil {
		if err == store.ErrDuplicateKey {
			return domain.Tax{}, fmt.Errorf("%w: tax code already exists", ErrValidation)
		}
		return domain.Tax{}, err
	}
	if s.taxSyncAdapter != nil && item.Status == domain.TaxStatusActive {
		if err := s.taxSyncAdapter.SyncTax(ctx, item); err != nil {
			return domain.Tax{}, err
		}
	}
	return item, nil
}

func (s *TaxService) ListTaxes(tenantID string) ([]domain.Tax, error) {
	return s.store.ListTaxes(normalizeTenantID(tenantID))
}

func (s *TaxService) GetTax(tenantID, id string) (domain.Tax, error) {
	return s.store.GetTax(normalizeTenantID(tenantID), strings.TrimSpace(id))
}

func validateTax(input domain.Tax) error {
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
	if input.Status == "" {
		return fmt.Errorf("%w: status is required", ErrValidation)
	}
	if input.Rate < 0 {
		return fmt.Errorf("%w: rate must be >= 0", ErrValidation)
	}
	return nil
}

func normalizeTaxCode(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}
	raw = taxCodeInvalidRE.ReplaceAllString(raw, "_")
	raw = taxCodeMultiRE.ReplaceAllString(raw, "_")
	return strings.Trim(raw, "_")
}

func normalizeTaxStatus(raw domain.TaxStatus) domain.TaxStatus {
	switch domain.TaxStatus(strings.ToLower(strings.TrimSpace(string(raw)))) {
	case domain.TaxStatusActive:
		return domain.TaxStatusActive
	case domain.TaxStatusArchived:
		return domain.TaxStatusArchived
	default:
		return domain.TaxStatusDraft
	}
}
