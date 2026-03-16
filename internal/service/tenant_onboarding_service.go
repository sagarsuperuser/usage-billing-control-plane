package service

import (
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
)

type TenantOnboardingService struct {
	tenantService *TenantService
	apiKeyService *APIKeyService
	ratingService *RatingService
	meterService  *MeterService
}

type TenantOnboardingRequest struct {
	ID                      string     `json:"id"`
	Name                    string     `json:"name"`
	LagoOrganizationID      string     `json:"lago_organization_id,omitempty"`
	LagoBillingProviderCode string     `json:"lago_billing_provider_code,omitempty"`
	AdminKeyName            string     `json:"admin_key_name,omitempty"`
	AdminKeyExpiresAt       *time.Time `json:"admin_key_expires_at,omitempty"`
	AllowExistingActiveKeys bool       `json:"allow_existing_active_keys,omitempty"`
	BootstrapAdminKey       *bool      `json:"bootstrap_admin_key,omitempty"`
}

type TenantOnboardingReadiness struct {
	Status              string   `json:"status"`
	TenantExists        bool     `json:"tenant_exists"`
	TenantActive        bool     `json:"tenant_active"`
	BillingMappingReady bool     `json:"billing_mapping_ready"`
	TenantAdminReady    bool     `json:"tenant_admin_ready"`
	PricingReady        bool     `json:"pricing_ready"`
	MissingSteps        []string `json:"missing_steps"`
}

type TenantAdminBootstrapResult struct {
	Created            bool           `json:"created"`
	ExistingActiveKeys int            `json:"existing_active_keys"`
	APIKey             *domain.APIKey `json:"api_key,omitempty"`
	Secret             string         `json:"secret,omitempty"`
}

type TenantOnboardingResult struct {
	Tenant               domain.Tenant              `json:"tenant"`
	TenantCreated        bool                       `json:"tenant_created"`
	TenantAdminBootstrap TenantAdminBootstrapResult `json:"tenant_admin_bootstrap"`
	Readiness            TenantOnboardingReadiness  `json:"readiness"`
}

func NewTenantOnboardingService(tenantSvc *TenantService, apiKeySvc *APIKeyService, ratingSvc *RatingService, meterSvc *MeterService) *TenantOnboardingService {
	return &TenantOnboardingService{
		tenantService: tenantSvc,
		apiKeyService: apiKeySvc,
		ratingService: ratingSvc,
		meterService:  meterSvc,
	}
}

func (s *TenantOnboardingService) OnboardTenant(req TenantOnboardingRequest, actorAPIKeyID string) (TenantOnboardingResult, error) {
	if s == nil || s.tenantService == nil || s.apiKeyService == nil || s.ratingService == nil || s.meterService == nil {
		return TenantOnboardingResult{}, ErrValidation
	}

	bootstrapAdminKey := true
	if req.BootstrapAdminKey != nil {
		bootstrapAdminKey = *req.BootstrapAdminKey
	}

	tenant, tenantCreated, err := s.tenantService.EnsureTenant(EnsureTenantRequest{
		ID:                      req.ID,
		Name:                    req.Name,
		LagoOrganizationID:      req.LagoOrganizationID,
		LagoBillingProviderCode: req.LagoBillingProviderCode,
	}, actorAPIKeyID)
	if err != nil {
		return TenantOnboardingResult{}, err
	}

	bootstrapResult := TenantAdminBootstrapResult{}
	if bootstrapAdminKey {
		activeKeys, err := s.apiKeyService.ListAPIKeys(tenant.ID, ListAPIKeysRequest{
			State: "active",
			Limit: 1,
		})
		if err != nil {
			return TenantOnboardingResult{}, err
		}
		bootstrapResult.ExistingActiveKeys = activeKeys.Total
		if activeKeys.Total == 0 || req.AllowExistingActiveKeys {
			name := strings.TrimSpace(req.AdminKeyName)
			if name == "" {
				name = "bootstrap-admin-" + normalizeTenantID(tenant.ID)
			}
			created, err := s.apiKeyService.CreateAPIKey(tenant.ID, actorAPIKeyID, CreateAPIKeyRequest{
				Name:      name,
				Role:      string(domainTenantAdminRole),
				ExpiresAt: req.AdminKeyExpiresAt,
			})
			if err != nil {
				return TenantOnboardingResult{}, err
			}
			bootstrapResult.Created = true
			bootstrapResult.APIKey = &created.APIKey
			bootstrapResult.Secret = created.Secret
			bootstrapResult.ExistingActiveKeys = activeKeys.Total
		}
	}

	readiness, err := s.GetTenantReadiness(tenant.ID)
	if err != nil {
		return TenantOnboardingResult{}, err
	}

	return TenantOnboardingResult{
		Tenant:               tenant,
		TenantCreated:        tenantCreated,
		TenantAdminBootstrap: bootstrapResult,
		Readiness:            readiness,
	}, nil
}

func (s *TenantOnboardingService) GetTenantReadiness(id string) (TenantOnboardingReadiness, error) {
	if s == nil || s.tenantService == nil || s.apiKeyService == nil || s.ratingService == nil || s.meterService == nil {
		return TenantOnboardingReadiness{}, ErrValidation
	}

	tenant, err := s.tenantService.GetTenant(id)
	if err != nil {
		return TenantOnboardingReadiness{}, err
	}

	activeKeys, err := s.apiKeyService.ListAPIKeys(tenant.ID, ListAPIKeysRequest{
		State: "active",
		Limit: 1,
	})
	if err != nil {
		return TenantOnboardingReadiness{}, err
	}

	rules, err := s.ratingService.ListRuleVersions(tenant.ID, ListRuleVersionsRequest{})
	if err != nil {
		return TenantOnboardingReadiness{}, err
	}
	meters, err := s.meterService.ListMeters(tenant.ID)
	if err != nil {
		return TenantOnboardingReadiness{}, err
	}

	readiness := TenantOnboardingReadiness{
		Status:              "pending",
		TenantExists:        true,
		TenantActive:        tenant.Status == domain.TenantStatusActive,
		BillingMappingReady: strings.TrimSpace(tenant.LagoOrganizationID) != "" && strings.TrimSpace(tenant.LagoBillingProviderCode) != "",
		TenantAdminReady:    activeKeys.Total > 0,
		PricingReady:        len(rules) > 0 && len(meters) > 0,
		MissingSteps:        make([]string, 0, 4),
	}

	if !readiness.TenantActive {
		readiness.MissingSteps = append(readiness.MissingSteps, "tenant_active")
	}
	if !readiness.BillingMappingReady {
		readiness.MissingSteps = append(readiness.MissingSteps, "billing_mapping")
	}
	if !readiness.TenantAdminReady {
		readiness.MissingSteps = append(readiness.MissingSteps, "tenant_admin_key")
	}
	if !readiness.PricingReady {
		readiness.MissingSteps = append(readiness.MissingSteps, "pricing")
	}
	if len(readiness.MissingSteps) == 0 {
		readiness.Status = "ready"
	}

	return readiness, nil
}

const domainTenantAdminRole = "admin"
