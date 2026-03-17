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
	store store.Repository
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

func (s *TenantService) CreateTenant(req EnsureTenantRequest, actorAPIKeyID string) (domain.Tenant, error) {
	if s == nil || s.store == nil {
		return domain.Tenant{}, fmt.Errorf("%w: tenant repository is required", ErrValidation)
	}

	id := normalizeTenantID(req.ID)
	name := strings.TrimSpace(req.Name)
	billingProviderConnectionID, lagoOrganizationID, lagoBillingProviderCode, err := s.resolveTenantBillingConfiguration(
		req.BillingProviderConnectionID,
		req.LagoOrganizationID,
		req.LagoBillingProviderCode,
	)
	if err != nil {
		return domain.Tenant{}, err
	}
	if name == "" {
		name = id
	}

	now := time.Now().UTC()
	created, err := s.store.CreateTenant(domain.Tenant{
		ID:                          id,
		Name:                        name,
		Status:                      domain.TenantStatusActive,
		BillingProviderConnectionID: billingProviderConnectionID,
		LagoOrganizationID:          lagoOrganizationID,
		LagoBillingProviderCode:     lagoBillingProviderCode,
		CreatedAt:                   now,
		UpdatedAt:                   now,
	})
	if err != nil {
		if err == store.ErrAlreadyExists || err == store.ErrDuplicateKey {
			return domain.Tenant{}, fmt.Errorf("%w: tenant already exists", store.ErrDuplicateKey)
		}
		return domain.Tenant{}, err
	}

	if _, auditErr := s.store.CreateTenantAuditEvent(domain.TenantAuditEvent{
		TenantID:      id,
		ActorAPIKeyID: strings.TrimSpace(actorAPIKeyID),
		Action:        "created",
		Metadata: map[string]any{
			"name":                           created.Name,
			"status":                         created.Status,
			"billing_provider_connection_id": created.BillingProviderConnectionID,
			"lago_organization_id":           created.LagoOrganizationID,
			"lago_billing_provider_code":     created.LagoBillingProviderCode,
		},
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
	billingProviderConnectionID, lagoOrganizationID, lagoBillingProviderCode, err := s.resolveTenantBillingConfiguration(
		req.BillingProviderConnectionID,
		req.LagoOrganizationID,
		req.LagoBillingProviderCode,
	)
	if err != nil {
		return domain.Tenant{}, false, err
	}
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
	if billingProviderConnectionID != existing.BillingProviderConnectionID {
		updated.BillingProviderConnectionID = billingProviderConnectionID
		metadata["previous_billing_provider_connection_id"] = existing.BillingProviderConnectionID
		metadata["new_billing_provider_connection_id"] = billingProviderConnectionID
		changed = true
	}
	if lagoOrganizationID != existing.LagoOrganizationID {
		updated.LagoOrganizationID = lagoOrganizationID
		metadata["previous_lago_organization_id"] = existing.LagoOrganizationID
		metadata["new_lago_organization_id"] = lagoOrganizationID
		changed = true
	}
	if lagoBillingProviderCode != existing.LagoBillingProviderCode {
		updated.LagoBillingProviderCode = lagoBillingProviderCode
		metadata["previous_lago_billing_provider_code"] = existing.LagoBillingProviderCode
		metadata["new_lago_billing_provider_code"] = lagoBillingProviderCode
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
		ActorAPIKeyID: strings.TrimSpace(actorAPIKeyID),
		Action:        "updated",
		Metadata:      metadata,
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
	if req.BillingProviderConnectionID != nil {
		resolvedConnectionID, resolvedOrg, resolvedCode, err := s.resolveTenantBillingConfiguration(
			*req.BillingProviderConnectionID,
			valueOrDefault(req.LagoOrganizationID, ""),
			valueOrDefault(req.LagoBillingProviderCode, ""),
		)
		if err != nil {
			return domain.Tenant{}, err
		}
		if resolvedConnectionID != current.BillingProviderConnectionID {
			updated.BillingProviderConnectionID = resolvedConnectionID
			metadata["previous_billing_provider_connection_id"] = current.BillingProviderConnectionID
			metadata["new_billing_provider_connection_id"] = resolvedConnectionID
			changed = true
		}
		if req.LagoOrganizationID == nil && resolvedOrg != current.LagoOrganizationID {
			updated.LagoOrganizationID = resolvedOrg
			metadata["previous_lago_organization_id"] = current.LagoOrganizationID
			metadata["new_lago_organization_id"] = resolvedOrg
			changed = true
		}
		if req.LagoBillingProviderCode == nil && resolvedCode != current.LagoBillingProviderCode {
			updated.LagoBillingProviderCode = resolvedCode
			metadata["previous_lago_billing_provider_code"] = current.LagoBillingProviderCode
			metadata["new_lago_billing_provider_code"] = resolvedCode
			changed = true
		}
	}
	if req.LagoOrganizationID != nil {
		value := strings.TrimSpace(*req.LagoOrganizationID)
		if value != current.LagoOrganizationID {
			updated.LagoOrganizationID = value
			metadata["previous_lago_organization_id"] = current.LagoOrganizationID
			metadata["new_lago_organization_id"] = value
			changed = true
		}
	}
	if req.LagoBillingProviderCode != nil {
		value := strings.TrimSpace(*req.LagoBillingProviderCode)
		if value != current.LagoBillingProviderCode {
			updated.LagoBillingProviderCode = value
			metadata["previous_lago_billing_provider_code"] = current.LagoBillingProviderCode
			metadata["new_lago_billing_provider_code"] = value
			changed = true
		}
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
		ActorAPIKeyID: strings.TrimSpace(actorAPIKeyID),
		Action:        action,
		Metadata:      metadata,
		CreatedAt:     updated.UpdatedAt,
	}); auditErr != nil {
		return domain.Tenant{}, fmt.Errorf("create tenant audit event: %w", auditErr)
	}
	return out, nil
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
	case "created", "status_changed", "updated":
		return value, nil
	default:
		return "", fmt.Errorf("%w: action must be one of created, status_changed, updated", ErrValidation)
	}
}
