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
	ID   string `json:"id"`
	Name string `json:"name"`
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
	created, err := s.store.CreateTenant(domain.Tenant{
		ID:        id,
		Name:      name,
		Status:    domain.TenantStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err == nil {
		if _, auditErr := s.store.CreateTenantAuditEvent(domain.TenantAuditEvent{
			TenantID:      id,
			ActorAPIKeyID: strings.TrimSpace(actorAPIKeyID),
			Action:        "created",
			Metadata: map[string]any{
				"name":   created.Name,
				"status": created.Status,
			},
			CreatedAt: now,
		}); auditErr != nil {
			return domain.Tenant{}, false, fmt.Errorf("create tenant audit event: %w", auditErr)
		}
		return created, true, nil
	}
	if err != store.ErrAlreadyExists && err != store.ErrDuplicateKey {
		return domain.Tenant{}, false, err
	}

	existing, err := s.store.GetTenant(id)
	if err != nil {
		return domain.Tenant{}, false, err
	}
	return existing, false, nil
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

func (s *TenantService) UpdateTenantStatus(id string, status domain.TenantStatus) (domain.Tenant, error) {
	return s.UpdateTenantStatusWithActor(id, status, "")
}

func (s *TenantService) UpdateTenantStatusWithActor(id string, status domain.TenantStatus, actorAPIKeyID string) (domain.Tenant, error) {
	if s == nil || s.store == nil {
		return domain.Tenant{}, fmt.Errorf("%w: tenant repository is required", ErrValidation)
	}
	id = normalizeTenantID(id)
	next, err := normalizeMutableTenantStatus(status)
	if err != nil {
		return domain.Tenant{}, err
	}
	if id == defaultTenantID {
		return domain.Tenant{}, fmt.Errorf("%w: default tenant status cannot be changed", ErrValidation)
	}
	current, err := s.store.GetTenant(id)
	if err != nil {
		return domain.Tenant{}, err
	}
	if current.Status == next {
		return current, nil
	}
	if current.Status == domain.TenantStatusDeleted {
		return domain.Tenant{}, fmt.Errorf("%w: deleted tenant status cannot be changed", ErrValidation)
	}
	now := time.Now().UTC()
	updated, err := s.store.UpdateTenantStatus(id, next, now)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return domain.Tenant{}, err
		}
		return domain.Tenant{}, err
	}
	if _, auditErr := s.store.CreateTenantAuditEvent(domain.TenantAuditEvent{
		TenantID:      id,
		ActorAPIKeyID: strings.TrimSpace(actorAPIKeyID),
		Action:        "status_changed",
		Metadata: map[string]any{
			"previous_status": current.Status,
			"new_status":      updated.Status,
		},
		CreatedAt: now,
	}); auditErr != nil {
		return domain.Tenant{}, fmt.Errorf("create tenant audit event: %w", auditErr)
	}
	return updated, nil
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
	case "created", "status_changed":
		return value, nil
	default:
		return "", fmt.Errorf("%w: action must be one of created, status_changed", ErrValidation)
	}
}
