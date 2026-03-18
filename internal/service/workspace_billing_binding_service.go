package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

type workspaceBillingBindingStore interface {
	GetTenant(id string) (domain.Tenant, error)
	GetBillingProviderConnection(id string) (domain.BillingProviderConnection, error)
	CreateWorkspaceBillingBinding(input domain.WorkspaceBillingBinding) (domain.WorkspaceBillingBinding, error)
	GetWorkspaceBillingBinding(workspaceID string) (domain.WorkspaceBillingBinding, error)
	ListWorkspaceBillingBindings(filter store.WorkspaceBillingBindingListFilter) ([]domain.WorkspaceBillingBinding, error)
	UpdateWorkspaceBillingBinding(input domain.WorkspaceBillingBinding) (domain.WorkspaceBillingBinding, error)
}

type WorkspaceBillingBindingService struct {
	store workspaceBillingBindingStore
}

type EnsureWorkspaceBillingBindingRequest struct {
	WorkspaceID                 string `json:"workspace_id"`
	BillingProviderConnectionID string `json:"billing_provider_connection_id"`
	Backend                     string `json:"backend,omitempty"`
	BackendOrganizationID       string `json:"backend_organization_id,omitempty"`
	BackendProviderCode         string `json:"backend_provider_code,omitempty"`
	IsolationMode               string `json:"isolation_mode,omitempty"`
	CreatedByType               string `json:"created_by_type"`
	CreatedByID                 string `json:"created_by_id,omitempty"`
}

type ListWorkspaceBillingBindingsRequest struct {
	WorkspaceID                 string `json:"workspace_id,omitempty"`
	BillingProviderConnectionID string `json:"billing_provider_connection_id,omitempty"`
	Backend                     string `json:"backend,omitempty"`
	IsolationMode               string `json:"isolation_mode,omitempty"`
	Status                      string `json:"status,omitempty"`
	Limit                       int    `json:"limit,omitempty"`
	Offset                      int    `json:"offset,omitempty"`
}

type EffectiveWorkspaceBillingContext struct {
	WorkspaceID                 string                               `json:"workspace_id"`
	BillingProviderConnectionID string                               `json:"billing_provider_connection_id"`
	Backend                     domain.WorkspaceBillingBackend       `json:"backend"`
	BackendOrganizationID       string                               `json:"backend_organization_id"`
	BackendProviderCode         string                               `json:"backend_provider_code"`
	IsolationMode               domain.WorkspaceBillingIsolationMode `json:"isolation_mode"`
	Status                      string                               `json:"status"`
	Source                      string                               `json:"source"`
}

func NewWorkspaceBillingBindingService(repo workspaceBillingBindingStore) *WorkspaceBillingBindingService {
	return &WorkspaceBillingBindingService{store: repo}
}

func (s *WorkspaceBillingBindingService) EnsureWorkspaceBillingBinding(req EnsureWorkspaceBillingBindingRequest) (domain.WorkspaceBillingBinding, bool, error) {
	if s == nil || s.store == nil {
		return domain.WorkspaceBillingBinding{}, false, fmt.Errorf("%w: workspace billing binding repository is required", ErrValidation)
	}

	workspaceID := normalizeTenantID(req.WorkspaceID)
	if workspaceID == "" {
		return domain.WorkspaceBillingBinding{}, false, fmt.Errorf("%w: workspace_id is required", ErrValidation)
	}
	if _, err := s.store.GetTenant(workspaceID); err != nil {
		return domain.WorkspaceBillingBinding{}, false, err
	}

	connectionID := strings.TrimSpace(req.BillingProviderConnectionID)
	if connectionID == "" {
		return domain.WorkspaceBillingBinding{}, false, fmt.Errorf("%w: billing_provider_connection_id is required", ErrValidation)
	}
	connection, err := s.store.GetBillingProviderConnection(connectionID)
	if err != nil {
		return domain.WorkspaceBillingBinding{}, false, err
	}

	backend, err := normalizeWorkspaceBillingBackend(req.Backend)
	if err != nil {
		return domain.WorkspaceBillingBinding{}, false, err
	}
	isolationMode, err := normalizeWorkspaceBillingIsolationMode(req.IsolationMode)
	if err != nil {
		return domain.WorkspaceBillingBinding{}, false, err
	}
	createdByType := strings.ToLower(strings.TrimSpace(req.CreatedByType))
	if createdByType == "" {
		return domain.WorkspaceBillingBinding{}, false, fmt.Errorf("%w: created_by_type is required", ErrValidation)
	}
	createdByID := strings.TrimSpace(req.CreatedByID)

	now := time.Now().UTC()
	candidate := domain.WorkspaceBillingBinding{
		WorkspaceID:                 workspaceID,
		BillingProviderConnectionID: connection.ID,
		Backend:                     backend,
		BackendOrganizationID:       strings.TrimSpace(req.BackendOrganizationID),
		BackendProviderCode:         strings.TrimSpace(req.BackendProviderCode),
		IsolationMode:               isolationMode,
		Status:                      domain.WorkspaceBillingBindingStatusPending,
		CreatedByType:               createdByType,
		CreatedByID:                 createdByID,
		CreatedAt:                   now,
		UpdatedAt:                   now,
	}
	if candidate.BackendOrganizationID != "" && candidate.BackendProviderCode != "" && connection.Status == domain.BillingProviderConnectionStatusConnected {
		candidate.Status = domain.WorkspaceBillingBindingStatusConnected
		candidate.ConnectedAt = &now
	}

	current, err := s.store.GetWorkspaceBillingBinding(workspaceID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			return domain.WorkspaceBillingBinding{}, false, err
		}
		created, createErr := s.store.CreateWorkspaceBillingBinding(candidate)
		return created, true, createErr
	}

	updated := current
	updated.BillingProviderConnectionID = candidate.BillingProviderConnectionID
	updated.Backend = candidate.Backend
	updated.BackendOrganizationID = candidate.BackendOrganizationID
	updated.BackendProviderCode = candidate.BackendProviderCode
	updated.IsolationMode = candidate.IsolationMode
	updated.Status = candidate.Status
	updated.ProvisioningError = ""
	updated.ConnectedAt = candidate.ConnectedAt
	updated.CreatedByType = candidate.CreatedByType
	updated.CreatedByID = candidate.CreatedByID
	updated.UpdatedAt = now
	if unchangedWorkspaceBillingBinding(current, updated) {
		return current, false, nil
	}
	out, updateErr := s.store.UpdateWorkspaceBillingBinding(updated)
	return out, false, updateErr
}

func (s *WorkspaceBillingBindingService) GetWorkspaceBillingBinding(workspaceID string) (domain.WorkspaceBillingBinding, error) {
	if s == nil || s.store == nil {
		return domain.WorkspaceBillingBinding{}, fmt.Errorf("%w: workspace billing binding repository is required", ErrValidation)
	}
	workspaceID = normalizeTenantID(workspaceID)
	if workspaceID == "" {
		return domain.WorkspaceBillingBinding{}, fmt.Errorf("%w: workspace_id is required", ErrValidation)
	}
	return s.store.GetWorkspaceBillingBinding(workspaceID)
}

func (s *WorkspaceBillingBindingService) ListWorkspaceBillingBindings(req ListWorkspaceBillingBindingsRequest) ([]domain.WorkspaceBillingBinding, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("%w: workspace billing binding repository is required", ErrValidation)
	}
	limit, offset, err := normalizeListWindow(req.Limit, req.Offset)
	if err != nil {
		return nil, err
	}
	backend, err := normalizeWorkspaceBillingBackendFilter(req.Backend)
	if err != nil {
		return nil, err
	}
	isolationMode, err := normalizeWorkspaceBillingIsolationModeFilter(req.IsolationMode)
	if err != nil {
		return nil, err
	}
	status, err := normalizeWorkspaceBillingBindingStatusFilter(req.Status)
	if err != nil {
		return nil, err
	}
	return s.store.ListWorkspaceBillingBindings(store.WorkspaceBillingBindingListFilter{
		WorkspaceID:                 normalizeOptionalTenantID(req.WorkspaceID),
		BillingProviderConnectionID: strings.TrimSpace(req.BillingProviderConnectionID),
		Backend:                     backend,
		IsolationMode:               isolationMode,
		Status:                      status,
		Limit:                       limit,
		Offset:                      offset,
	})
}

func (s *WorkspaceBillingBindingService) ResolveEffectiveWorkspaceBillingContext(workspaceID string) (EffectiveWorkspaceBillingContext, error) {
	if s == nil || s.store == nil {
		return EffectiveWorkspaceBillingContext{}, fmt.Errorf("%w: workspace billing binding repository is required", ErrValidation)
	}
	workspaceID = normalizeTenantID(workspaceID)
	if workspaceID == "" {
		return EffectiveWorkspaceBillingContext{}, fmt.Errorf("%w: workspace_id is required", ErrValidation)
	}

	if binding, err := s.store.GetWorkspaceBillingBinding(workspaceID); err == nil {
		return effectiveWorkspaceBillingContextFromBinding(binding)
	} else if !errors.Is(err, store.ErrNotFound) {
		return EffectiveWorkspaceBillingContext{}, err
	}

	tenant, err := s.store.GetTenant(workspaceID)
	if err != nil {
		return EffectiveWorkspaceBillingContext{}, err
	}
	if tenant.BillingProviderConnectionID == "" || tenant.LagoOrganizationID == "" || tenant.LagoBillingProviderCode == "" {
		return EffectiveWorkspaceBillingContext{}, fmt.Errorf("%w: workspace has no billing execution context", ErrValidation)
	}
	binding, _, err := s.ensureBindingFromTenantLegacy(tenant)
	if err != nil {
		return EffectiveWorkspaceBillingContext{}, err
	}
	return effectiveWorkspaceBillingContextFromBinding(binding)
}

func (s *WorkspaceBillingBindingService) ensureBindingFromTenantLegacy(tenant domain.Tenant) (domain.WorkspaceBillingBinding, bool, error) {
	if s == nil || s.store == nil {
		return domain.WorkspaceBillingBinding{}, false, fmt.Errorf("%w: workspace billing binding repository is required", ErrValidation)
	}
	if strings.TrimSpace(tenant.BillingProviderConnectionID) == "" || strings.TrimSpace(tenant.LagoOrganizationID) == "" || strings.TrimSpace(tenant.LagoBillingProviderCode) == "" {
		return domain.WorkspaceBillingBinding{}, false, fmt.Errorf("%w: workspace has no billing execution context", ErrValidation)
	}
	if _, err := s.store.GetBillingProviderConnection(tenant.BillingProviderConnectionID); err != nil {
		return domain.WorkspaceBillingBinding{}, false, err
	}
	return s.EnsureWorkspaceBillingBinding(EnsureWorkspaceBillingBindingRequest{
		WorkspaceID:                 tenant.ID,
		BillingProviderConnectionID: tenant.BillingProviderConnectionID,
		Backend:                     string(domain.WorkspaceBillingBackendLago),
		BackendOrganizationID:       tenant.LagoOrganizationID,
		BackendProviderCode:         tenant.LagoBillingProviderCode,
		IsolationMode:               string(domain.WorkspaceBillingIsolationModeShared),
		CreatedByType:               "system_migration",
		CreatedByID:                 tenant.ID,
	})
}

func effectiveWorkspaceBillingContextFromBinding(binding domain.WorkspaceBillingBinding) (EffectiveWorkspaceBillingContext, error) {
	if binding.Status == domain.WorkspaceBillingBindingStatusDisabled {
		return EffectiveWorkspaceBillingContext{}, fmt.Errorf("%w: workspace billing binding is disabled", ErrValidation)
	}
	if binding.BackendOrganizationID == "" || binding.BackendProviderCode == "" {
		return EffectiveWorkspaceBillingContext{}, fmt.Errorf("%w: workspace billing binding exists but is not ready", ErrValidation)
	}
	return EffectiveWorkspaceBillingContext{
		WorkspaceID:                 binding.WorkspaceID,
		BillingProviderConnectionID: binding.BillingProviderConnectionID,
		Backend:                     binding.Backend,
		BackendOrganizationID:       binding.BackendOrganizationID,
		BackendProviderCode:         binding.BackendProviderCode,
		IsolationMode:               binding.IsolationMode,
		Status:                      string(binding.Status),
		Source:                      "binding",
	}, nil
}

func unchangedWorkspaceBillingBinding(current, updated domain.WorkspaceBillingBinding) bool {
	return current.WorkspaceID == updated.WorkspaceID &&
		current.BillingProviderConnectionID == updated.BillingProviderConnectionID &&
		current.Backend == updated.Backend &&
		current.BackendOrganizationID == updated.BackendOrganizationID &&
		current.BackendProviderCode == updated.BackendProviderCode &&
		current.IsolationMode == updated.IsolationMode &&
		current.Status == updated.Status &&
		current.ProvisioningError == updated.ProvisioningError &&
		sameTimePointer(current.ConnectedAt, updated.ConnectedAt) &&
		current.CreatedByType == updated.CreatedByType &&
		current.CreatedByID == updated.CreatedByID
}

func sameTimePointer(a, b *time.Time) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Equal(*b)
}

func normalizeWorkspaceBillingBackend(value string) (domain.WorkspaceBillingBackend, error) {
	backend := domain.WorkspaceBillingBackend(strings.ToLower(strings.TrimSpace(value)))
	if backend == "" {
		return domain.WorkspaceBillingBackendLago, nil
	}
	switch backend {
	case domain.WorkspaceBillingBackendLago:
		return backend, nil
	default:
		return "", fmt.Errorf("%w: unsupported workspace billing backend %q", ErrValidation, value)
	}
}

func normalizeWorkspaceBillingBackendFilter(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	backend, err := normalizeWorkspaceBillingBackend(value)
	if err != nil {
		return "", err
	}
	return string(backend), nil
}

func normalizeWorkspaceBillingIsolationMode(value string) (domain.WorkspaceBillingIsolationMode, error) {
	isolationMode := domain.WorkspaceBillingIsolationMode(strings.ToLower(strings.TrimSpace(value)))
	if isolationMode == "" {
		return domain.WorkspaceBillingIsolationModeShared, nil
	}
	switch isolationMode {
	case domain.WorkspaceBillingIsolationModeShared, domain.WorkspaceBillingIsolationModeDedicated:
		return isolationMode, nil
	default:
		return "", fmt.Errorf("%w: unsupported workspace billing isolation mode %q", ErrValidation, value)
	}
}

func normalizeWorkspaceBillingIsolationModeFilter(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	isolationMode, err := normalizeWorkspaceBillingIsolationMode(value)
	if err != nil {
		return "", err
	}
	return string(isolationMode), nil
}

func normalizeWorkspaceBillingBindingStatusFilter(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "", nil
	}
	switch domain.WorkspaceBillingBindingStatus(value) {
	case domain.WorkspaceBillingBindingStatusPending,
		domain.WorkspaceBillingBindingStatusProvisioning,
		domain.WorkspaceBillingBindingStatusConnected,
		domain.WorkspaceBillingBindingStatusVerificationFailed,
		domain.WorkspaceBillingBindingStatusDisabled:
		return value, nil
	default:
		return "", fmt.Errorf("%w: unsupported workspace billing binding status %q", ErrValidation, value)
	}
}
