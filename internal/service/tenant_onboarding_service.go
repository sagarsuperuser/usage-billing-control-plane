package service

import (
	"errors"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
)

type TenantOnboardingService struct {
	tenantService                  *TenantService
	workspaceBillingBindingService *WorkspaceBillingBindingService
	customerService                *CustomerService
	apiKeyService                  *APIKeyService
	serviceAccountService          *ServiceAccountService
	ratingService                  *RatingService
	meterService                   *MeterService
}

type TenantOnboardingRequest struct {
	ID                          string     `json:"id"`
	Name                        string     `json:"name"`
	BillingProviderConnectionID string     `json:"billing_provider_connection_id,omitempty"`
	LagoOrganizationID          string     `json:"lago_organization_id,omitempty"`
	LagoBillingProviderCode     string     `json:"lago_billing_provider_code,omitempty"`
	AdminKeyName                string     `json:"admin_key_name,omitempty"`
	AdminKeyExpiresAt           *time.Time `json:"admin_key_expires_at,omitempty"`
	AllowExistingActiveKeys     bool       `json:"allow_existing_active_keys,omitempty"`
	BootstrapAdminKey           *bool      `json:"bootstrap_admin_key,omitempty"`
}

type TenantOnboardingStageError struct {
	Stage string
	Err   error
}

func (e *TenantOnboardingStageError) Error() string {
	if e == nil || e.Err == nil {
		return "tenant onboarding failed"
	}
	return e.Err.Error()
}

func (e *TenantOnboardingStageError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type TenantOnboardingReadiness struct {
	Status             string                        `json:"status"`
	MissingSteps       []string                      `json:"missing_steps"`
	Tenant             TenantCoreReadiness           `json:"tenant"`
	BillingIntegration BillingIntegrationReadiness   `json:"billing_integration"`
	FirstCustomer      FirstCustomerBillingReadiness `json:"first_customer"`
}

type TenantCoreReadiness struct {
	Status           string   `json:"status"`
	TenantExists     bool     `json:"tenant_exists"`
	TenantActive     bool     `json:"tenant_active"`
	TenantAdminReady bool     `json:"tenant_admin_ready"`
	MissingSteps     []string `json:"missing_steps"`
}

type BillingIntegrationReadiness struct {
	Status                    string   `json:"status"`
	BillingMappingReady       bool     `json:"billing_mapping_ready"`
	BillingConnected          bool     `json:"billing_connected"`
	WorkspaceBillingStatus    string   `json:"workspace_billing_status,omitempty"`
	WorkspaceBillingSource    string   `json:"workspace_billing_source,omitempty"`
	ActiveBillingConnectionID string   `json:"active_billing_connection_id,omitempty"`
	IsolationMode             string   `json:"isolation_mode,omitempty"`
	PricingReady              bool     `json:"pricing_ready"`
	MissingSteps              []string `json:"missing_steps"`
}

type FirstCustomerBillingReadiness struct {
	Status               string                      `json:"status"`
	Managed              bool                        `json:"managed"`
	CustomerExists       bool                        `json:"customer_exists"`
	CustomerExternalID   string                      `json:"customer_external_id,omitempty"`
	CustomerActive       bool                        `json:"customer_active"`
	BillingProfileStatus domain.BillingProfileStatus `json:"billing_profile_status"`
	PaymentSetupStatus   domain.PaymentSetupStatus   `json:"payment_setup_status"`
	MissingSteps         []string                    `json:"missing_steps"`
	Note                 string                      `json:"note,omitempty"`
}

type TenantAdminBootstrapResult struct {
	Created            bool                   `json:"created"`
	ExistingActiveKeys int                    `json:"existing_active_keys"`
	ServiceAccount     *domain.ServiceAccount `json:"service_account,omitempty"`
	Credential         *domain.APIKey         `json:"credential,omitempty"`
	Secret             string                 `json:"secret,omitempty"`
}

type TenantOnboardingResult struct {
	Tenant               domain.Tenant              `json:"tenant"`
	TenantCreated        bool                       `json:"tenant_created"`
	TenantAdminBootstrap TenantAdminBootstrapResult `json:"tenant_admin_bootstrap"`
	Readiness            TenantOnboardingReadiness  `json:"readiness"`
}

func NewTenantOnboardingService(tenantSvc *TenantService, workspaceBindingSvc *WorkspaceBillingBindingService, customerSvc *CustomerService, apiKeySvc *APIKeyService, serviceAccountSvc *ServiceAccountService, ratingSvc *RatingService, meterSvc *MeterService) *TenantOnboardingService {
	return &TenantOnboardingService{
		tenantService:                  tenantSvc,
		workspaceBillingBindingService: workspaceBindingSvc,
		customerService:                customerSvc,
		apiKeyService:                  apiKeySvc,
		serviceAccountService:          serviceAccountSvc,
		ratingService:                  ratingSvc,
		meterService:                   meterSvc,
	}
}

func (s *TenantOnboardingService) OnboardTenant(req TenantOnboardingRequest, actorAPIKeyID string) (TenantOnboardingResult, error) {
	if s == nil || s.tenantService == nil || s.customerService == nil || s.apiKeyService == nil || s.serviceAccountService == nil || s.ratingService == nil || s.meterService == nil {
		return TenantOnboardingResult{}, ErrValidation
	}

	bootstrapAdminKey := true
	if req.BootstrapAdminKey != nil {
		bootstrapAdminKey = *req.BootstrapAdminKey
	}

	tenant, tenantCreated, err := s.tenantService.EnsureTenant(EnsureTenantRequest{
		ID:                          req.ID,
		Name:                        req.Name,
		BillingProviderConnectionID: req.BillingProviderConnectionID,
		LagoOrganizationID:          req.LagoOrganizationID,
		LagoBillingProviderCode:     req.LagoBillingProviderCode,
	}, actorAPIKeyID)
	if err != nil {
		return TenantOnboardingResult{}, wrapTenantOnboardingStage("ensure_tenant", err)
	}

	bootstrapResult := TenantAdminBootstrapResult{}
	if bootstrapAdminKey {
		activeKeys, err := s.apiKeyService.ListAPIKeys(tenant.ID, ListAPIKeysRequest{
			State: "active",
			Limit: 1,
		})
		if err != nil {
			return TenantOnboardingResult{}, wrapTenantOnboardingStage("list_active_api_keys", err)
		}
		bootstrapResult.ExistingActiveKeys = activeKeys.Total
		if activeKeys.Total == 0 || req.AllowExistingActiveKeys {
			name := strings.TrimSpace(req.AdminKeyName)
			if name == "" {
				name = "bootstrap-admin-" + normalizeTenantID(tenant.ID)
			}
			created, err := s.serviceAccountService.IssueBootstrapWorkspaceServiceAccountCredential(tenant.ID, name, APICredentialActor{
				PlatformAPIKeyID:  strings.TrimSpace(actorAPIKeyID),
				CreatedByPlatform: true,
			}, req.AdminKeyExpiresAt)
			if err != nil {
				return TenantOnboardingResult{}, wrapTenantOnboardingStage("bootstrap_admin_key", err)
			}
			bootstrapResult.Created = true
			bootstrapResult.ServiceAccount = &created.ServiceAccount
			bootstrapResult.Credential = created.Credential
			bootstrapResult.Secret = created.Secret
			bootstrapResult.ExistingActiveKeys = activeKeys.Total
		}
	}

	readiness, err := s.GetTenantReadiness(tenant.ID)
	if err != nil {
		return TenantOnboardingResult{}, wrapTenantOnboardingStage("build_readiness", err)
	}

	return TenantOnboardingResult{
		Tenant:               tenant,
		TenantCreated:        tenantCreated,
		TenantAdminBootstrap: bootstrapResult,
		Readiness:            readiness,
	}, nil
}

func wrapTenantOnboardingStage(stage string, err error) error {
	if err == nil {
		return nil
	}
	var staged *TenantOnboardingStageError
	if errors.As(err, &staged) {
		return err
	}
	return &TenantOnboardingStageError{
		Stage: strings.TrimSpace(stage),
		Err:   err,
	}
}

func (s *TenantOnboardingService) GetTenantReadiness(id string) (TenantOnboardingReadiness, error) {
	if s == nil || s.tenantService == nil || s.customerService == nil || s.apiKeyService == nil || s.serviceAccountService == nil || s.ratingService == nil || s.meterService == nil {
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
	firstCustomerReadiness, err := s.getFirstCustomerReadiness(tenant.ID)
	if err != nil {
		return TenantOnboardingReadiness{}, err
	}

	tenantReadiness := TenantCoreReadiness{
		Status:           "pending",
		TenantExists:     true,
		TenantActive:     tenant.Status == domain.TenantStatusActive,
		TenantAdminReady: activeKeys.Total > 0,
		MissingSteps:     make([]string, 0, 2),
	}
	if !tenantReadiness.TenantActive {
		tenantReadiness.MissingSteps = append(tenantReadiness.MissingSteps, "tenant_active")
	}
	if !tenantReadiness.TenantAdminReady {
		tenantReadiness.MissingSteps = append(tenantReadiness.MissingSteps, "tenant_admin_key")
	}
	if len(tenantReadiness.MissingSteps) == 0 {
		tenantReadiness.Status = "ready"
	}

	billingIntegrationReadiness := BillingIntegrationReadiness{
		Status:                    "pending",
		ActiveBillingConnectionID: strings.TrimSpace(tenant.BillingProviderConnectionID),
		PricingReady:              len(rules) > 0 && len(meters) > 0,
		MissingSteps:              make([]string, 0, 2),
	}
	if s.workspaceBillingBindingService != nil {
		if billingContext, err := s.workspaceBillingBindingService.ResolveEffectiveWorkspaceBillingContext(tenant.ID); err == nil {
			billingIntegrationReadiness.BillingMappingReady = true
			billingIntegrationReadiness.BillingConnected = true
			billingIntegrationReadiness.WorkspaceBillingStatus = billingContext.Status
			billingIntegrationReadiness.WorkspaceBillingSource = billingContext.Source
			billingIntegrationReadiness.ActiveBillingConnectionID = billingContext.BillingProviderConnectionID
			billingIntegrationReadiness.IsolationMode = string(billingContext.IsolationMode)
		} else {
			billingIntegrationReadiness.BillingMappingReady = false
		}
	} else {
		billingIntegrationReadiness.BillingMappingReady = strings.TrimSpace(tenant.LagoOrganizationID) != "" && strings.TrimSpace(tenant.LagoBillingProviderCode) != ""
		billingIntegrationReadiness.BillingConnected = billingIntegrationReadiness.BillingMappingReady
		if billingIntegrationReadiness.BillingMappingReady {
			billingIntegrationReadiness.WorkspaceBillingStatus = "connected"
			billingIntegrationReadiness.WorkspaceBillingSource = "tenant_fields"
			billingIntegrationReadiness.IsolationMode = string(domain.WorkspaceBillingIsolationModeShared)
		}
	}
	if !billingIntegrationReadiness.BillingMappingReady {
		billingIntegrationReadiness.MissingSteps = append(billingIntegrationReadiness.MissingSteps, "billing_mapping")
	}
	if !billingIntegrationReadiness.PricingReady {
		billingIntegrationReadiness.MissingSteps = append(billingIntegrationReadiness.MissingSteps, "pricing")
	}
	if len(billingIntegrationReadiness.MissingSteps) == 0 {
		billingIntegrationReadiness.Status = "ready"
	}

	readiness := TenantOnboardingReadiness{
		Status:             "pending",
		MissingSteps:       make([]string, 0, len(tenantReadiness.MissingSteps)+len(billingIntegrationReadiness.MissingSteps)+len(firstCustomerReadiness.MissingSteps)),
		Tenant:             tenantReadiness,
		BillingIntegration: billingIntegrationReadiness,
		FirstCustomer:      firstCustomerReadiness,
	}
	for _, step := range tenantReadiness.MissingSteps {
		readiness.MissingSteps = append(readiness.MissingSteps, "tenant."+step)
	}
	for _, step := range billingIntegrationReadiness.MissingSteps {
		readiness.MissingSteps = append(readiness.MissingSteps, "billing_integration."+step)
	}
	for _, step := range firstCustomerReadiness.MissingSteps {
		readiness.MissingSteps = append(readiness.MissingSteps, "first_customer."+step)
	}
	if tenantReadiness.Status == "ready" && billingIntegrationReadiness.Status == "ready" && firstCustomerReadiness.Status == "ready" {
		readiness.Status = "ready"
	}

	return readiness, nil
}

const tenantOnboardingCustomerPageSize = 100

func (s *TenantOnboardingService) getFirstCustomerReadiness(tenantID string) (FirstCustomerBillingReadiness, error) {
	customers := make([]domain.Customer, 0, tenantOnboardingCustomerPageSize)
	for offset := 0; ; offset += tenantOnboardingCustomerPageSize {
		batch, err := s.customerService.ListCustomers(tenantID, ListCustomersRequest{Limit: tenantOnboardingCustomerPageSize, Offset: offset})
		if err != nil {
			return FirstCustomerBillingReadiness{}, err
		}
		if len(batch) == 0 {
			break
		}
		customers = append(customers, batch...)
		for _, customer := range batch {
			readiness, err := s.customerService.GetCustomerReadiness(tenantID, customer.ExternalID)
			if err != nil {
				return FirstCustomerBillingReadiness{}, err
			}
			if readiness.Status == "ready" {
				return s.buildFirstCustomerReadiness(tenantID, []domain.Customer{customer})
			}
		}
		if len(batch) < tenantOnboardingCustomerPageSize {
			break
		}
	}
	return s.buildFirstCustomerReadiness(tenantID, customers)
}

func (s *TenantOnboardingService) buildFirstCustomerReadiness(tenantID string, customers []domain.Customer) (FirstCustomerBillingReadiness, error) {
	readiness := FirstCustomerBillingReadiness{
		Status:               "pending",
		Managed:              true,
		BillingProfileStatus: domain.BillingProfileStatusMissing,
		PaymentSetupStatus:   domain.PaymentSetupStatusMissing,
		MissingSteps:         []string{"customer_created"},
		Note:                 "tenant onboarding is not complete until at least one billing-ready customer exists",
	}
	if len(customers) == 0 {
		return readiness, nil
	}

	var selected CustomerReadiness
	var selectedCustomer domain.Customer
	foundReady := false
	for _, customer := range customers {
		customerReadiness, err := s.customerService.GetCustomerReadiness(tenantID, customer.ExternalID)
		if err != nil {
			return FirstCustomerBillingReadiness{}, err
		}
		if !selected.CustomerExists {
			selected = customerReadiness
			selectedCustomer = customer
		}
		if customerReadiness.Status == "ready" {
			selected = customerReadiness
			selectedCustomer = customer
			foundReady = true
			break
		}
	}

	readiness.CustomerExists = true
	readiness.CustomerExternalID = selectedCustomer.ExternalID
	readiness.CustomerActive = selected.CustomerActive
	readiness.BillingProfileStatus = selected.BillingProfileStatus
	readiness.PaymentSetupStatus = selected.PaymentSetupStatus
	readiness.MissingSteps = make([]string, 0, len(selected.MissingSteps))
	if !selected.CustomerActive {
		readiness.MissingSteps = append(readiness.MissingSteps, "customer_active")
	}
	for _, step := range selected.MissingSteps {
		switch step {
		case "customer_active":
			// already included above
		case "billing_profile_ready":
			readiness.MissingSteps = append(readiness.MissingSteps, "billing_profile_ready")
		case "payment_setup_ready":
			readiness.MissingSteps = append(readiness.MissingSteps, "payment_setup_ready")
		default:
			readiness.MissingSteps = append(readiness.MissingSteps, step)
		}
	}
	if foundReady {
		readiness.Status = "ready"
		readiness.MissingSteps = nil
		readiness.Note = ""
	}
	return readiness, nil
}

const domainTenantAdminRole = "admin"
