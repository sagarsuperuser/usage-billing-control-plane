package service

import (
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

func NewTenantService(s store.Repository) *TenantService {
	return &TenantService{store: s}
}

func (s *TenantService) EnsureTenant(req EnsureTenantRequest) (domain.Tenant, bool, error) {
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
