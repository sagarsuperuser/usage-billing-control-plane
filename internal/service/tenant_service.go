package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

type TenantService struct {
	store                          store.Repository
	workspaceBillingBindingService *WorkspaceBillingBindingService
	organizationBootstrapper       LagoOrganizationBootstrapper
}

type EnsureTenantRequest struct {
	ID                          string `json:"id"`
	Name                        string `json:"name"`
	BillingProviderConnectionID string `json:"billing_provider_connection_id,omitempty"`
	LagoOrganizationID          string `json:"lago_organization_id,omitempty"`
	LagoBillingProviderCode     string `json:"lago_billing_provider_code,omitempty"`
}

type UpdateTenantRequest struct {
	Name                        *string              `json:"name,omitempty"`
	Status                      *domain.TenantStatus `json:"status,omitempty"`
	BillingProviderConnectionID *string              `json:"billing_provider_connection_id,omitempty"`
	LagoOrganizationID          *string              `json:"lago_organization_id,omitempty"`
	LagoBillingProviderCode     *string              `json:"lago_billing_provider_code,omitempty"`
}

type ListTenantsRequest struct {
	Status string `json:"status,omitempty"`
}

type ListTenantAuditEventsRequest struct {
	TenantID      string `json:"tenant_id,omitempty"`
	ActorAPIKeyID string `json:"actor_api_key_id,omitempty"`
	Action        string `json:"action,omitempty"`
	Limit         int    `json:"limit,omitempty"`
	Offset        int    `json:"offset,omitempty"`
}

type ListTenantAuditEventsResult struct {
	Items  []domain.TenantAuditEvent `json:"items"`
	Total  int                       `json:"total"`
	Limit  int                       `json:"limit"`
	Offset int                       `json:"offset"`
}

func NewTenantService(s store.Repository) *TenantService {
	return &TenantService{store: s}
}

func (s *TenantService) WithWorkspaceBillingBindingService(bindingSvc *WorkspaceBillingBindingService) *TenantService {
	if s == nil {
		return nil
	}
	s.workspaceBillingBindingService = bindingSvc
	return s
}

func (s *TenantService) WithLagoOrganizationBootstrapper(bootstrapper LagoOrganizationBootstrapper) *TenantService {
	if s == nil {
		return nil
	}
	s.organizationBootstrapper = bootstrapper
	return s
}

func (s *TenantService) CreateTenant(req EnsureTenantRequest, actorAPIKeyID string) (domain.Tenant, error) {
	if s == nil || s.store == nil {
		return domain.Tenant{}, fmt.Errorf("%w: tenant repository is required", ErrValidation)
	}

	id := normalizeTenantID(req.ID)
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = id
	}

	now := time.Now().UTC()
	initialConnectionID := strings.TrimSpace(req.BillingProviderConnectionID)
	initialOrg := strings.TrimSpace(req.LagoOrganizationID)
	initialCode := strings.TrimSpace(req.LagoBillingProviderCode)
	initialAPIKey := ""
	if s.workspaceBillingBindingService == nil || initialConnectionID == "" {
		var err error
		initialConnectionID, initialOrg, initialCode, err = s.resolveTenantBillingConfiguration(
			req.BillingProviderConnectionID,
			req.LagoOrganizationID,
			req.LagoBillingProviderCode,
		)
		if err != nil {
			return domain.Tenant{}, err
		}
	}
	if initialConnectionID == "" && initialOrg == "" && s.organizationBootstrapper != nil {
		bootstrapped, err := s.bootstrapTenantOrganization(name)
		if err != nil {
			return domain.Tenant{}, err
		}
		initialOrg = bootstrapped.OrganizationID
		initialAPIKey = bootstrapped.APIKey
	}
	created, err := s.store.CreateTenant(domain.Tenant{
		ID:                          id,
		Name:                        name,
		Status:                      domain.TenantStatusActive,
		BillingProviderConnectionID: initialConnectionID,
		LagoOrganizationID:          initialOrg,
		LagoBillingProviderCode:     initialCode,
		LagoAPIKey:                  initialAPIKey,
		CreatedAt:                   now,
		UpdatedAt:                   now,
	})
	if err != nil {
		if err == store.ErrAlreadyExists || err == store.ErrDuplicateKey {
			return domain.Tenant{}, fmt.Errorf("%w: tenant already exists", store.ErrDuplicateKey)
		}
		return domain.Tenant{}, err
	}
	if s.workspaceBillingBindingService != nil && strings.TrimSpace(req.BillingProviderConnectionID) != "" {
		synced, err := s.syncTenantBillingBinding(created, req.BillingProviderConnectionID, req.LagoOrganizationID, req.LagoBillingProviderCode, actorAPIKeyID)
		if err != nil {
			return domain.Tenant{}, err
		}
		created = synced
	}

	if _, auditErr := s.store.CreateTenantAuditEvent(domain.TenantAuditEvent{
		TenantID:      id,
		ActorAPIKeyID: tenantAuditActorAPIKeyID(actorAPIKeyID),
		Action:        "created",
		Metadata: tenantAuditMetadata(strings.TrimSpace(actorAPIKeyID), map[string]any{
			"name":                           created.Name,
			"status":                         created.Status,
			"billing_provider_connection_id": created.BillingProviderConnectionID,
			"lago_organization_id":           created.LagoOrganizationID,
			"lago_billing_provider_code":     created.LagoBillingProviderCode,
		}),
		CreatedAt: now,
	}); auditErr != nil {
		return domain.Tenant{}, fmt.Errorf("create tenant audit event: %w", auditErr)
	}
	return created, nil
}

func (s *TenantService) EnsureTenant(req EnsureTenantRequest, actorAPIKeyID string) (domain.Tenant, bool, error) {
	if s == nil || s.store == nil {
		return domain.Tenant{}, false, fmt.Errorf("%w: tenant repository is required", ErrValidation)
	}

	id := normalizeTenantID(req.ID)
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = id
	}

	now := time.Now().UTC()
	created, err := s.CreateTenant(req, actorAPIKeyID)
	if err == nil {
		return created, true, nil
	}
	if !errors.Is(err, store.ErrDuplicateKey) && !errors.Is(err, store.ErrAlreadyExists) {
		return domain.Tenant{}, false, err
	}

	existing, err := s.store.GetTenant(id)
	if err != nil {
		return domain.Tenant{}, false, err
	}

	updated := existing
	metadata := map[string]any{}
	changed := false
	if name != "" && name != existing.Name {
		updated.Name = name
		metadata["previous_name"] = existing.Name
		metadata["new_name"] = name
		changed = true
	}
	rawConnectionID := req.BillingProviderConnectionID
	rawOrg := req.LagoOrganizationID
	rawCode := req.LagoBillingProviderCode
	if existing.BillingProviderConnectionID == "" && strings.TrimSpace(rawConnectionID) == "" && existing.LagoOrganizationID == "" && strings.TrimSpace(rawOrg) == "" && s.organizationBootstrapper != nil {
		bootstrapped, bootstrapErr := s.bootstrapTenantOrganization(name)
		if bootstrapErr != nil {
			return domain.Tenant{}, false, bootstrapErr
		}
		rawOrg = bootstrapped.OrganizationID
		if bootstrapped.APIKey != existing.LagoAPIKey {
			updated.LagoAPIKey = bootstrapped.APIKey
			metadata["lago_api_key_bootstrapped"] = true
			changed = true
		}
		metadata["new_lago_organization_id"] = rawOrg
	}
	nextConnectionID, nextOrg, nextCode, err := s.resolveTenantWriteBillingConfiguration(existing, &rawConnectionID, &rawOrg, &rawCode, actorAPIKeyID)
	if err != nil {
		return domain.Tenant{}, false, err
	}
	if nextConnectionID != existing.BillingProviderConnectionID {
		updated.BillingProviderConnectionID = nextConnectionID
		metadata["previous_billing_provider_connection_id"] = existing.BillingProviderConnectionID
		metadata["new_billing_provider_connection_id"] = nextConnectionID
		changed = true
	}
	if nextOrg != existing.LagoOrganizationID {
		updated.LagoOrganizationID = nextOrg
		metadata["previous_lago_organization_id"] = existing.LagoOrganizationID
		metadata["new_lago_organization_id"] = nextOrg
		changed = true
	}
	if nextCode != existing.LagoBillingProviderCode {
		updated.LagoBillingProviderCode = nextCode
		metadata["previous_lago_billing_provider_code"] = existing.LagoBillingProviderCode
		metadata["new_lago_billing_provider_code"] = nextCode
		changed = true
	}
	if updated.LagoAPIKey != existing.LagoAPIKey {
		changed = true
	}
	if !changed {
		return existing, false, nil
	}
	updated.UpdatedAt = now
	out, err := s.store.UpdateTenant(updated)
	if err != nil {
		return domain.Tenant{}, false, err
	}
	if _, auditErr := s.store.CreateTenantAuditEvent(domain.TenantAuditEvent{
		TenantID:      id,
		ActorAPIKeyID: tenantAuditActorAPIKeyID(actorAPIKeyID),
		Action:        "updated",
		Metadata:      tenantAuditMetadata(strings.TrimSpace(actorAPIKeyID), metadata),
		CreatedAt:     now,
	}); auditErr != nil {
		return domain.Tenant{}, false, fmt.Errorf("create tenant audit event: %w", auditErr)
	}
	return out, false, nil
}

func (s *TenantService) GetTenant(id string) (domain.Tenant, error) {
	if s == nil || s.store == nil {
		return domain.Tenant{}, fmt.Errorf("%w: tenant repository is required", ErrValidation)
	}
	return s.store.GetTenant(normalizeTenantID(id))
}

func (s *TenantService) ListTenants(req ListTenantsRequest) ([]domain.Tenant, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("%w: tenant repository is required", ErrValidation)
	}
	status, err := normalizeTenantStatusFilter(req.Status)
	if err != nil {
		return nil, err
	}
	return s.store.ListTenants(status)
}

func (s *TenantService) UpdateTenant(id string, req UpdateTenantRequest, actorAPIKeyID string) (domain.Tenant, error) {
	if s == nil || s.store == nil {
		return domain.Tenant{}, fmt.Errorf("%w: tenant repository is required", ErrValidation)
	}
	id = normalizeTenantID(id)
	current, err := s.store.GetTenant(id)
	if err != nil {
		return domain.Tenant{}, err
	}

	updated := current
	metadata := map[string]any{}
	changed := false
	statusChanged := false

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return domain.Tenant{}, fmt.Errorf("%w: tenant name is required", ErrValidation)
		}
		if name != current.Name {
			updated.Name = name
			metadata["previous_name"] = current.Name
			metadata["new_name"] = name
			changed = true
		}
	}
	if req.Status != nil {
		next, err := normalizeMutableTenantStatus(*req.Status)
		if err != nil {
			return domain.Tenant{}, err
		}
		if id == defaultTenantID && next != current.Status {
			return domain.Tenant{}, fmt.Errorf("%w: default tenant status cannot be changed", ErrValidation)
		}
		if current.Status == domain.TenantStatusDeleted && next != current.Status {
			return domain.Tenant{}, fmt.Errorf("%w: deleted tenant status cannot be changed", ErrValidation)
		}
		if next != current.Status {
			updated.Status = next
			metadata["previous_status"] = current.Status
			metadata["new_status"] = next
			changed = true
			statusChanged = true
		}
	}
	nextConnectionID, nextOrg, nextCode, err := s.resolveTenantWriteBillingConfiguration(current, req.BillingProviderConnectionID, req.LagoOrganizationID, req.LagoBillingProviderCode, actorAPIKeyID)
	if err != nil {
		return domain.Tenant{}, err
	}
	if nextConnectionID != current.BillingProviderConnectionID {
		updated.BillingProviderConnectionID = nextConnectionID
		metadata["previous_billing_provider_connection_id"] = current.BillingProviderConnectionID
		metadata["new_billing_provider_connection_id"] = nextConnectionID
		changed = true
	}
	if nextOrg != current.LagoOrganizationID {
		updated.LagoOrganizationID = nextOrg
		metadata["previous_lago_organization_id"] = current.LagoOrganizationID
		metadata["new_lago_organization_id"] = nextOrg
		changed = true
	}
	if nextCode != current.LagoBillingProviderCode {
		updated.LagoBillingProviderCode = nextCode
		metadata["previous_lago_billing_provider_code"] = current.LagoBillingProviderCode
		metadata["new_lago_billing_provider_code"] = nextCode
		changed = true
	}
	if !changed {
		return current, nil
	}

	updated.UpdatedAt = time.Now().UTC()
	out, err := s.store.UpdateTenant(updated)
	if err != nil {
		return domain.Tenant{}, err
	}

	action := "updated"
	if statusChanged && len(metadata) == 2 {
		action = "status_changed"
	}
	if _, auditErr := s.store.CreateTenantAuditEvent(domain.TenantAuditEvent{
		TenantID:      id,
		ActorAPIKeyID: tenantAuditActorAPIKeyID(actorAPIKeyID),
		Action:        action,
		Metadata:      tenantAuditMetadata(strings.TrimSpace(actorAPIKeyID), metadata),
		CreatedAt:     updated.UpdatedAt,
	}); auditErr != nil {
		return domain.Tenant{}, fmt.Errorf("create tenant audit event: %w", auditErr)
	}
	return out, nil
}

func (s *TenantService) bootstrapTenantOrganization(name string) (LagoOrganizationBootstrapResult, error) {
	if s == nil || s.organizationBootstrapper == nil {
		return LagoOrganizationBootstrapResult{}, fmt.Errorf("%w: lago organization bootstrapper is required", ErrValidation)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := s.organizationBootstrapper.BootstrapOrganization(ctx, name)
	if err != nil {
		return LagoOrganizationBootstrapResult{}, fmt.Errorf("bootstrap lago organization for tenant %q: %w", strings.TrimSpace(name), err)
	}
	return result, nil
}

func (s *TenantService) resolveTenantWriteBillingConfiguration(current domain.Tenant, connectionID, lagoOrganizationID, lagoBillingProviderCode *string, actorAPIKeyID string) (string, string, string, error) {
	rawConnectionID := current.BillingProviderConnectionID
	if connectionID != nil {
		rawConnectionID = strings.TrimSpace(*connectionID)
	}
	rawOrg := current.LagoOrganizationID
	if lagoOrganizationID != nil {
		rawOrg = strings.TrimSpace(*lagoOrganizationID)
	}
	rawCode := current.LagoBillingProviderCode
	if lagoBillingProviderCode != nil {
		rawCode = strings.TrimSpace(*lagoBillingProviderCode)
	}
	if s.workspaceBillingBindingService != nil && rawConnectionID != "" {
		resolvedConnectionID, _, _, err := s.ensureBindingBackedTenantBillingConfiguration(current.ID, rawConnectionID, rawOrg, rawCode, actorAPIKeyID)
		if err != nil {
			return "", "", "", err
		}
		// Keep tenant-level Lago fields as compatibility residue only. The binding is the source of truth.
		return resolvedConnectionID, current.LagoOrganizationID, current.LagoBillingProviderCode, nil
	}
	return s.resolveTenantBillingConfiguration(rawConnectionID, rawOrg, rawCode)
}

func (s *TenantService) syncTenantBillingBinding(current domain.Tenant, connectionID, rawLagoOrganizationID, rawLagoBillingProviderCode, actorAPIKeyID string) (domain.Tenant, error) {
	if s.workspaceBillingBindingService == nil || strings.TrimSpace(connectionID) == "" {
		return current, nil
	}
	resolvedConnectionID, _, _, err := s.ensureBindingBackedTenantBillingConfiguration(
		current.ID,
		connectionID,
		rawLagoOrganizationID,
		rawLagoBillingProviderCode,
		actorAPIKeyID,
	)
	if err != nil {
		return domain.Tenant{}, err
	}
	if current.BillingProviderConnectionID == resolvedConnectionID {
		return current, nil
	}
	current.BillingProviderConnectionID = resolvedConnectionID
	current.UpdatedAt = time.Now().UTC()
	return s.store.UpdateTenant(current)
}

func (s *TenantService) ensureBindingBackedTenantBillingConfiguration(workspaceID, connectionID, rawLagoOrganizationID, rawLagoBillingProviderCode, actorAPIKeyID string) (string, string, string, error) {
	if s.workspaceBillingBindingService == nil {
		return s.resolveTenantBillingConfiguration(connectionID, rawLagoOrganizationID, rawLagoBillingProviderCode)
	}
	resolvedConnectionID, resolvedOrg, resolvedCode, err := s.resolveTenantBillingConfiguration(connectionID, rawLagoOrganizationID, rawLagoBillingProviderCode)
	if err != nil {
		return "", "", "", err
	}
	actorType := "platform_api_key"
	actorID := strings.TrimSpace(actorAPIKeyID)
	if actorID == "" {
		actorType = "system_migration"
		actorID = normalizeTenantID(workspaceID)
	}
	binding, _, err := s.workspaceBillingBindingService.EnsureWorkspaceBillingBinding(EnsureWorkspaceBillingBindingRequest{
		WorkspaceID:                 workspaceID,
		BillingProviderConnectionID: resolvedConnectionID,
		Backend:                     string(domain.WorkspaceBillingBackendLago),
		BackendOrganizationID:       resolvedOrg,
		BackendProviderCode:         resolvedCode,
		IsolationMode:               string(domain.WorkspaceBillingIsolationModeShared),
		CreatedByType:               actorType,
		CreatedByID:                 actorID,
	})
	if err != nil {
		return "", "", "", err
	}
	effective, err := effectiveWorkspaceBillingContextFromBinding(binding)
	if err != nil {
		return "", "", "", err
	}
	return effective.BillingProviderConnectionID, effective.BackendOrganizationID, effective.BackendProviderCode, nil
}

func (s *TenantService) resolveTenantBillingConfiguration(connectionID, rawLagoOrganizationID, rawLagoBillingProviderCode string) (string, string, string, error) {
	connectionID = strings.TrimSpace(connectionID)
	lagoOrganizationID := strings.TrimSpace(rawLagoOrganizationID)
	lagoBillingProviderCode := strings.TrimSpace(rawLagoBillingProviderCode)
	if connectionID == "" {
		return "", lagoOrganizationID, lagoBillingProviderCode, nil
	}
	connection, err := s.store.GetBillingProviderConnection(connectionID)
	if err != nil {
		if err == store.ErrNotFound {
			return "", "", "", fmt.Errorf("%w: billing provider connection not found", ErrValidation)
		}
		return "", "", "", err
	}
	if connection.Status != domain.BillingProviderConnectionStatusConnected {
		return "", "", "", fmt.Errorf("%w: billing provider connection must be connected before workspace assignment", ErrValidation)
	}
	if strings.TrimSpace(connection.LagoOrganizationID) == "" || strings.TrimSpace(connection.LagoProviderCode) == "" {
		return "", "", "", fmt.Errorf("%w: billing provider connection is missing lago mapping", ErrValidation)
	}
	return connectionID, strings.TrimSpace(connection.LagoOrganizationID), strings.TrimSpace(connection.LagoProviderCode), nil
}

func valueOrDefault(input *string, fallback string) string {
	if input == nil {
		return fallback
	}
	return *input
}

func tenantAuditActorAPIKeyID(actorAPIKeyID string) string {
	actorAPIKeyID = strings.TrimSpace(actorAPIKeyID)
	if strings.HasPrefix(actorAPIKeyID, "pkey_") {
		return ""
	}
	return actorAPIKeyID
}

func tenantAuditMetadata(actorAPIKeyID string, metadata map[string]any) map[string]any {
	if metadata == nil {
		metadata = map[string]any{}
	}
	actorAPIKeyID = strings.TrimSpace(actorAPIKeyID)
	if strings.HasPrefix(actorAPIKeyID, "pkey_") {
		metadata["actor_platform_api_key_id"] = actorAPIKeyID
	}
	return metadata
}

func (s *TenantService) UpdateTenantStatus(id string, status domain.TenantStatus) (domain.Tenant, error) {
	return s.UpdateTenant(id, UpdateTenantRequest{Status: &status}, "")
}

func (s *TenantService) SetTenantStatus(id string, status domain.TenantStatus, actorAPIKeyID string) (domain.Tenant, error) {
	return s.UpdateTenant(id, UpdateTenantRequest{Status: &status}, actorAPIKeyID)
}

func (s *TenantService) ListTenantAuditEvents(req ListTenantAuditEventsRequest) (ListTenantAuditEventsResult, error) {
	if s == nil || s.store == nil {
		return ListTenantAuditEventsResult{}, fmt.Errorf("%w: tenant repository is required", ErrValidation)
	}
	limit, offset, err := normalizeListWindow(req.Limit, req.Offset)
	if err != nil {
		return ListTenantAuditEventsResult{}, err
	}
	action, err := normalizeTenantAuditAction(req.Action)
	if err != nil {
		return ListTenantAuditEventsResult{}, err
	}
	out, err := s.store.ListTenantAuditEvents(store.TenantAuditFilter{
		TenantID:      strings.TrimSpace(req.TenantID),
		ActorAPIKeyID: strings.TrimSpace(req.ActorAPIKeyID),
		Action:        action,
		Limit:         limit,
		Offset:        offset,
	})
	if err != nil {
		return ListTenantAuditEventsResult{}, err
	}
	return ListTenantAuditEventsResult{
		Items:  out.Items,
		Total:  out.Total,
		Limit:  out.Limit,
		Offset: out.Offset,
	}, nil
}

func normalizeTenantStatusFilter(v string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(v))
	if value == "" {
		return "", nil
	}
	switch domain.TenantStatus(value) {
	case domain.TenantStatusActive, domain.TenantStatusSuspended, domain.TenantStatusDeleted:
		return value, nil
	default:
		return "", fmt.Errorf("%w: status must be one of active, suspended, deleted", ErrValidation)
	}
}

func normalizeMutableTenantStatus(v domain.TenantStatus) (domain.TenantStatus, error) {
	switch domain.TenantStatus(strings.ToLower(strings.TrimSpace(string(v)))) {
	case domain.TenantStatusActive:
		return domain.TenantStatusActive, nil
	case domain.TenantStatusSuspended:
		return domain.TenantStatusSuspended, nil
	case domain.TenantStatusDeleted:
		return domain.TenantStatusDeleted, nil
	default:
		return "", fmt.Errorf("%w: status must be one of active, suspended, deleted", ErrValidation)
	}
}

func normalizeTenantAuditAction(v string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(v))
	if value == "" {
		return "", nil
	}
	switch value {
	case "created", "status_changed", "updated", "payment_setup_requested", "payment_setup_resent", "workspace_member_role_changed", "workspace_member_disabled", "workspace_member_reactivated", "workspace_invitation_revoked":
		return value, nil
	default:
		return "", fmt.Errorf("%w: unsupported tenant audit action", ErrValidation)
	}
}
