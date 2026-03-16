package service

import (
	"fmt"
	"strings"

	"usage-billing-control-plane/internal/store"
)

type LagoOrganizationTenantMapper interface {
	TenantIDForOrganization(organizationID string) string
}

type StaticLagoOrganizationTenantMapper struct {
	defaultTenantID string
	byOrganization  map[string]string
}

func NewStaticLagoOrganizationTenantMapper(defaultTenantID string, byOrganization map[string]string) *StaticLagoOrganizationTenantMapper {
	cleanDefault := normalizeTenantID(defaultTenantID)
	clean := make(map[string]string, len(byOrganization))
	for orgID, tenantID := range byOrganization {
		orgID = strings.TrimSpace(orgID)
		if orgID == "" {
			continue
		}
		clean[orgID] = normalizeTenantID(tenantID)
	}
	return &StaticLagoOrganizationTenantMapper{
		defaultTenantID: cleanDefault,
		byOrganization:  clean,
	}
}

func (m *StaticLagoOrganizationTenantMapper) TenantIDForOrganization(organizationID string) string {
	if m == nil {
		return defaultTenantID
	}
	orgID := strings.TrimSpace(organizationID)
	if orgID == "" {
		return m.defaultTenantID
	}
	if tenantID, ok := m.byOrganization[orgID]; ok {
		return normalizeTenantID(tenantID)
	}
	return m.defaultTenantID
}

type TenantBackedLagoOrganizationTenantMapper struct {
	repo store.Repository
}

func NewTenantBackedLagoOrganizationTenantMapper(repo store.Repository) *TenantBackedLagoOrganizationTenantMapper {
	return &TenantBackedLagoOrganizationTenantMapper{repo: repo}
}

func (m *TenantBackedLagoOrganizationTenantMapper) TenantIDForOrganization(organizationID string) string {
	if m == nil || m.repo == nil {
		return ""
	}
	tenant, err := m.repo.GetTenantByLagoOrganizationID(strings.TrimSpace(organizationID))
	if err != nil {
		return ""
	}
	return normalizeTenantID(tenant.ID)
}

func ParseLagoOrganizationTenantMap(raw string) (map[string]string, error) {
	out := make(map[string]string)
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		pair := strings.SplitN(item, ":", 2)
		if len(pair) != 2 {
			return nil, fmt.Errorf("invalid LAGO_ORG_TENANT_MAP entry %q: expected organization_id:tenant_id", item)
		}
		orgID := strings.TrimSpace(pair[0])
		tenantID := strings.TrimSpace(pair[1])
		if orgID == "" || tenantID == "" {
			return nil, fmt.Errorf("invalid LAGO_ORG_TENANT_MAP entry %q: organization_id and tenant_id are required", item)
		}
		out[orgID] = tenantID
	}
	return out, nil
}
