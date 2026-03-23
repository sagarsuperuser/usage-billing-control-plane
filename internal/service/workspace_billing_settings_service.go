package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

type WorkspaceBillingSettingsService struct {
	store                    store.Repository
	billingEntitySyncAdapter BillingEntitySettingsSyncAdapter
}

type UpdateWorkspaceBillingSettingsRequest struct {
	BillingEntityCode      string   `json:"billing_entity_code,omitempty"`
	NetPaymentTermDays     *int     `json:"net_payment_term_days,omitempty"`
	TaxCodes               []string `json:"tax_codes,omitempty"`
	InvoiceMemo            string   `json:"invoice_memo,omitempty"`
	InvoiceFooter          string   `json:"invoice_footer,omitempty"`
	DocumentLocale         string   `json:"document_locale,omitempty"`
	InvoiceGracePeriodDays *int     `json:"invoice_grace_period_days,omitempty"`
	DocumentNumbering      string   `json:"document_numbering,omitempty"`
	DocumentNumberPrefix   string   `json:"document_number_prefix,omitempty"`
}

func NewWorkspaceBillingSettingsService(s store.Repository) *WorkspaceBillingSettingsService {
	return &WorkspaceBillingSettingsService{store: s}
}

func (s *WorkspaceBillingSettingsService) WithBillingEntitySyncAdapter(adapter BillingEntitySettingsSyncAdapter) *WorkspaceBillingSettingsService {
	s.billingEntitySyncAdapter = adapter
	return s
}

func (s *WorkspaceBillingSettingsService) GetWorkspaceBillingSettings(workspaceID string) (domain.WorkspaceBillingSettings, error) {
	if s == nil || s.store == nil {
		return domain.WorkspaceBillingSettings{}, fmt.Errorf("%w: workspace billing settings repository is required", ErrValidation)
	}
	workspaceID = normalizeTenantID(workspaceID)
	if _, err := s.store.GetTenant(workspaceID); err != nil {
		return domain.WorkspaceBillingSettings{}, err
	}
	settings, err := s.store.GetWorkspaceBillingSettings(workspaceID)
	if err != nil {
		if err == store.ErrNotFound {
			return defaultWorkspaceBillingSettings(workspaceID), nil
		}
		return domain.WorkspaceBillingSettings{}, err
	}
	return settings, nil
}

func (s *WorkspaceBillingSettingsService) UpsertWorkspaceBillingSettings(workspaceID string, req UpdateWorkspaceBillingSettingsRequest) (domain.WorkspaceBillingSettings, error) {
	if s == nil || s.store == nil {
		return domain.WorkspaceBillingSettings{}, fmt.Errorf("%w: workspace billing settings repository is required", ErrValidation)
	}
	workspaceID = normalizeTenantID(workspaceID)
	if _, err := s.store.GetTenant(workspaceID); err != nil {
		return domain.WorkspaceBillingSettings{}, err
	}
	current, err := s.store.GetWorkspaceBillingSettings(workspaceID)
	if err != nil {
		if err != store.ErrNotFound {
			return domain.WorkspaceBillingSettings{}, err
		}
		current = defaultWorkspaceBillingSettings(workspaceID)
	}
	if req.NetPaymentTermDays != nil && *req.NetPaymentTermDays < 0 {
		return domain.WorkspaceBillingSettings{}, fmt.Errorf("%w: net_payment_term_days must be non-negative", ErrValidation)
	}
	if req.InvoiceGracePeriodDays != nil && *req.InvoiceGracePeriodDays < 0 {
		return domain.WorkspaceBillingSettings{}, fmt.Errorf("%w: invoice_grace_period_days must be non-negative", ErrValidation)
	}
	documentNumbering := strings.TrimSpace(req.DocumentNumbering)
	if documentNumbering != "" && documentNumbering != "per_customer" && documentNumbering != "per_billing_entity" {
		return domain.WorkspaceBillingSettings{}, fmt.Errorf("%w: document_numbering must be per_customer or per_billing_entity", ErrValidation)
	}
	current.BillingEntityCode = strings.TrimSpace(req.BillingEntityCode)
	current.NetPaymentTermDays = req.NetPaymentTermDays
	current.TaxCodes = normalizeTaxCodes(req.TaxCodes)
	current.InvoiceMemo = strings.TrimSpace(req.InvoiceMemo)
	current.InvoiceFooter = strings.TrimSpace(req.InvoiceFooter)
	current.DocumentLocale = strings.TrimSpace(req.DocumentLocale)
	current.InvoiceGracePeriodDays = req.InvoiceGracePeriodDays
	current.DocumentNumbering = documentNumbering
	current.DocumentNumberPrefix = strings.TrimSpace(req.DocumentNumberPrefix)
	current.UpdatedAt = time.Now().UTC()
	if err := s.syncWorkspaceBillingSettings(current); err != nil {
		return domain.WorkspaceBillingSettings{}, err
	}
	return s.store.UpsertWorkspaceBillingSettings(current)
}

func defaultWorkspaceBillingSettings(workspaceID string) domain.WorkspaceBillingSettings {
	return domain.WorkspaceBillingSettings{
		WorkspaceID: normalizeTenantID(workspaceID),
	}
}

func (s *WorkspaceBillingSettingsService) syncWorkspaceBillingSettings(settings domain.WorkspaceBillingSettings) error {
	if s == nil || s.billingEntitySyncAdapter == nil {
		return nil
	}
	if strings.TrimSpace(settings.BillingEntityCode) == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.billingEntitySyncAdapter.SyncBillingEntitySettings(ctx, settings); err != nil {
		return fmt.Errorf("%w: sync workspace billing settings: %v", ErrDependency, err)
	}
	return nil
}
