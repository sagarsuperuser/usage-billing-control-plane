package service

import (
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
	billingProviderConnectionSvc   *BillingProviderConnectionService
}

type EnsureTenantRequest struct {
	ID                          string `json:"id"`
	Name                        string `json:"name"`
	BillingProviderConnectionID string `json:"billing_provider_connection_id,omitempty"`
}

type UpdateTenantRequest struct {
	Name                        *string              `json:"name,omitempty"`
	Status                      *domain.TenantStatus `json:"status,omitempty"`
	BillingProviderConnectionID *string              `json:"billing_provider_connection_id,omitempty"`
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

func (s *TenantService) WithBillingProviderConnectionService(connectionSvc *BillingProviderConnectionService) *TenantService {
	if s == nil {
		return nil
	}
	s.billingProviderConnectionSvc = connectionSvc
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
	if initialConnectionID != "" {
		if _, err := s.store.GetBillingProviderConnection(initialConnectionID); err != nil {
			if err == store.ErrNotFound {
				return domain.Tenant{}, fmt.Errorf("%w: billing provider connection not found", ErrValidation)
			}
			return domain.Tenant{}, err
		}
	}
	created, err := s.store.CreateTenant(domain.Tenant{
		ID:                          id,
		Name:                        name,
		Status:                      domain.TenantStatusActive,
		BillingProviderConnectionID: initialConnectionID,
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
		synced, err := s.syncTenantBillingBinding(created, req.BillingProviderConnectionID, actorAPIKeyID)
		if err != nil {
			return domain.Tenant{}, err
		}
		created = synced
	}

	if _, auditErr := s.store.CreateTenantAuditEvent(domain.TenantAuditEvent{
		TenantID:      id,
		ActorAPIKeyID: tenantAuditActorAPIKeyID(actorAPIKeyID),
		Action:        "workspace.created",
		Metadata: tenantAuditMetadata(strings.TrimSpace(actorAPIKeyID), map[string]any{
			"name":                           created.Name,
			"status":                         created.Status,
			"billing_provider_connection_id": created.BillingProviderConnectionID,
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
	nextConnectionID, err := s.resolveTenantWriteBillingConnection(existing, &rawConnectionID, actorAPIKeyID)
	if err != nil {
		return domain.Tenant{}, false, err
	}
	if nextConnectionID != existing.BillingProviderConnectionID {
		updated.BillingProviderConnectionID = nextConnectionID
		metadata["previous_billing_provider_connection_id"] = existing.BillingProviderConnectionID
		metadata["new_billing_provider_connection_id"] = nextConnectionID
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
		Action:        classifyTenantAuditChangeAction(metadata, false),
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
	nextConnectionID, err := s.resolveTenantWriteBillingConnection(current, req.BillingProviderConnectionID, actorAPIKeyID)
	if err != nil {
		return domain.Tenant{}, err
	}
	if nextConnectionID != current.BillingProviderConnectionID {
		updated.BillingProviderConnectionID = nextConnectionID
		metadata["previous_billing_provider_connection_id"] = current.BillingProviderConnectionID
		metadata["new_billing_provider_connection_id"] = nextConnectionID
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

	if _, auditErr := s.store.CreateTenantAuditEvent(domain.TenantAuditEvent{
		TenantID:      id,
		ActorAPIKeyID: tenantAuditActorAPIKeyID(actorAPIKeyID),
		Action:        classifyTenantAuditChangeAction(metadata, statusChanged && len(metadata) == 2),
		Metadata:      tenantAuditMetadata(strings.TrimSpace(actorAPIKeyID), metadata),
		CreatedAt:     updated.UpdatedAt,
	}); auditErr != nil {
		return domain.Tenant{}, fmt.Errorf("create tenant audit event: %w", auditErr)
	}
	return out, nil
}


func (s *TenantService) resolveTenantWriteBillingConnection(current domain.Tenant, connectionID *string, actorAPIKeyID string) (string, error) {
	rawConnectionID := current.BillingProviderConnectionID
	if connectionID != nil {
		rawConnectionID = strings.TrimSpace(*connectionID)
	}
	if rawConnectionID == "" {
		return "", nil
	}
	connection, err := s.store.GetBillingProviderConnection(rawConnectionID)
	if err != nil {
		if err == store.ErrNotFound {
			return "", fmt.Errorf("%w: billing provider connection not found", ErrValidation)
		}
		return "", err
	}
	if connection.Status != domain.BillingProviderConnectionStatusConnected {
		return "", fmt.Errorf("%w: billing provider connection must be checked before workspace assignment", ErrValidation)
	}
	if s.workspaceBillingBindingService != nil {
		actorType := "platform_api_key"
		actorID := strings.TrimSpace(actorAPIKeyID)
		if actorID == "" {
			actorType = "system_migration"
			actorID = normalizeTenantID(current.ID)
		}
		binding, _, bindErr := s.workspaceBillingBindingService.EnsureWorkspaceBillingBinding(EnsureWorkspaceBillingBindingRequest{
			WorkspaceID:                 current.ID,
			BillingProviderConnectionID: rawConnectionID,
			Backend:                     string(domain.WorkspaceBillingBackendStripe),
			IsolationMode:               string(domain.WorkspaceBillingIsolationModeShared),
			CreatedByType:               actorType,
			CreatedByID:                 actorID,
		})
		if bindErr != nil {
			return "", bindErr
		}
		effective, effErr := effectiveWorkspaceBillingContextFromBinding(binding)
		if effErr != nil {
			return "", effErr
		}
		return effective.BillingProviderConnectionID, nil
	}
	return rawConnectionID, nil
}

func (s *TenantService) syncTenantBillingBinding(current domain.Tenant, connectionID, actorAPIKeyID string) (domain.Tenant, error) {
	if s.workspaceBillingBindingService == nil || strings.TrimSpace(connectionID) == "" {
		return current, nil
	}
	resolvedConnectionID, err := s.resolveTenantWriteBillingConnection(current, &connectionID, actorAPIKeyID)
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
	actions, err := normalizeTenantAuditActions(req.Action)
	if err != nil {
		return ListTenantAuditEventsResult{}, err
	}
	out, err := s.store.ListTenantAuditEvents(store.TenantAuditFilter{
		TenantID:      strings.TrimSpace(req.TenantID),
		ActorAPIKeyID: strings.TrimSpace(req.ActorAPIKeyID),
		Actions:       actions,
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

func normalizeTenantAuditActions(v string) ([]string, error) {
	value := strings.ToLower(strings.TrimSpace(v))
	if value == "" {
		return nil, nil
	}
	switch value {
	case "created",
		"status_changed",
		"updated",
		"workspace.created":
		return []string{"workspace.created", "created"}, nil
	case "workspace.updated":
		return []string{"workspace.updated", "updated"}, nil
	case "workspace.renamed":
		return []string{"workspace.renamed", "updated"}, nil
	case "workspace.status_changed":
		return []string{"workspace.status_changed", "status_changed", "updated"}, nil
	case "workspace.billing_connection_changed":
		return []string{"workspace.billing_connection_changed", "workspace_billing_binding_updated", "updated"}, nil
	case "workspace.billing_configuration_updated":
		return []string{"workspace.billing_configuration_updated", "updated"}, nil
	case "customer.payment_setup_requested",
		"payment_setup_requested":
		return []string{"customer.payment_setup_requested", "payment_setup_requested"}, nil
	case "customer.payment_setup_resent",
		"payment_setup_resent":
		return []string{"customer.payment_setup_resent", "payment_setup_resent"}, nil
	case "workspace.member_role_changed",
		"workspace_member_role_changed":
		return []string{"workspace.member_role_changed", "workspace_member_role_changed"}, nil
	case "workspace.member_disabled",
		"workspace_member_disabled":
		return []string{"workspace.member_disabled", "workspace_member_disabled"}, nil
	case "workspace.member_reactivated",
		"workspace_member_reactivated":
		return []string{"workspace.member_reactivated", "workspace_member_reactivated"}, nil
	case "workspace.invitation_revoked",
		"workspace_invitation_revoked":
		return []string{"workspace.invitation_revoked", "workspace_invitation_revoked"}, nil
	default:
		return nil, fmt.Errorf("%w: unsupported tenant audit action", ErrValidation)
	}
}

func classifyTenantAuditChangeAction(metadata map[string]any, statusOnly bool) string {
	if statusOnly {
		return "workspace.status_changed"
	}
	if metadataHasTenantAuditKey(metadata, "previous_billing_provider_connection_id", "new_billing_provider_connection_id", "previous_lago_organization_id", "new_lago_organization_id", "previous_lago_billing_provider_code", "new_lago_billing_provider_code") {
		return "workspace.billing_connection_changed"
	}
	if metadataHasTenantAuditKey(metadata, "previous_name", "new_name") {
		return "workspace.renamed"
	}
	return "workspace.updated"
}

func metadataHasTenantAuditKey(metadata map[string]any, keys ...string) bool {
	for _, key := range keys {
		if value, ok := metadata[key]; ok && strings.TrimSpace(fmt.Sprint(value)) != "" {
			return true
		}
	}
	return false
}
