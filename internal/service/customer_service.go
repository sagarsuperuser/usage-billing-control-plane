package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

type CustomerService struct {
	store          store.Repository
	billingAdapter CustomerBillingAdapter
}

type CreateCustomerRequest struct {
	ExternalID     string `json:"external_id"`
	DisplayName    string `json:"display_name,omitempty"`
	Email          string `json:"email,omitempty"`
	LagoCustomerID string `json:"lago_customer_id,omitempty"`
}

type UpdateCustomerRequest struct {
	DisplayName    *string                `json:"display_name,omitempty"`
	Email          *string                `json:"email,omitempty"`
	Status         *domain.CustomerStatus `json:"status,omitempty"`
	LagoCustomerID *string                `json:"lago_customer_id,omitempty"`
}

type ListCustomersRequest struct {
	Status     string `json:"status,omitempty"`
	ExternalID string `json:"external_id,omitempty"`
	Limit      int    `json:"limit,omitempty"`
	Offset     int    `json:"offset,omitempty"`
}

type UpsertCustomerBillingProfileRequest struct {
	LegalName     string `json:"legal_name,omitempty"`
	Email         string `json:"email,omitempty"`
	Phone         string `json:"phone,omitempty"`
	AddressLine1  string `json:"billing_address_line1,omitempty"`
	AddressLine2  string `json:"billing_address_line2,omitempty"`
	City          string `json:"billing_city,omitempty"`
	State         string `json:"billing_state,omitempty"`
	PostalCode    string `json:"billing_postal_code,omitempty"`
	Country       string `json:"billing_country,omitempty"`
	Currency      string `json:"currency,omitempty"`
	TaxIdentifier string `json:"tax_identifier,omitempty"`
	ProviderCode  string `json:"provider_code,omitempty"`
}

type BeginCustomerPaymentSetupRequest struct {
	PaymentMethodType string `json:"payment_method_type,omitempty"`
}

type BeginCustomerPaymentSetupResult struct {
	ExternalID   string                      `json:"external_id"`
	CheckoutURL  string                      `json:"checkout_url"`
	PaymentSetup domain.CustomerPaymentSetup `json:"payment_setup"`
}

type RefreshCustomerPaymentSetupResult struct {
	ExternalID   string                      `json:"external_id"`
	PaymentSetup domain.CustomerPaymentSetup `json:"payment_setup"`
	Readiness    CustomerReadiness           `json:"readiness"`
}

type CustomerReadiness struct {
	Status                       string                        `json:"status"`
	MissingSteps                 []string                      `json:"missing_steps"`
	CustomerExists               bool                          `json:"customer_exists"`
	CustomerActive               bool                          `json:"customer_active"`
	BillingProviderConfigured    bool                          `json:"billing_provider_configured"`
	LagoCustomerSynced           bool                          `json:"lago_customer_synced"`
	DefaultPaymentMethodVerified bool                          `json:"default_payment_method_verified"`
	BillingProfileStatus         domain.BillingProfileStatus   `json:"billing_profile_status"`
	PaymentSetupStatus           domain.PaymentSetupStatus     `json:"payment_setup_status"`
	BillingProfile               domain.CustomerBillingProfile `json:"billing_profile"`
	PaymentSetup                 domain.CustomerPaymentSetup   `json:"payment_setup"`
}

func NewCustomerService(s store.Repository, billingAdapter CustomerBillingAdapter) *CustomerService {
	return &CustomerService{store: s, billingAdapter: billingAdapter}
}

func (s *CustomerService) CreateCustomer(tenantID string, req CreateCustomerRequest) (domain.Customer, error) {
	if s == nil || s.store == nil {
		return domain.Customer{}, fmt.Errorf("%w: customer repository is required", ErrValidation)
	}
	tenantID = normalizeTenantID(tenantID)
	externalID := strings.TrimSpace(req.ExternalID)
	displayName := strings.TrimSpace(req.DisplayName)
	email := strings.TrimSpace(req.Email)
	lagoCustomerID := strings.TrimSpace(req.LagoCustomerID)
	if externalID == "" {
		return domain.Customer{}, fmt.Errorf("%w: external_id is required", ErrValidation)
	}
	if displayName == "" {
		displayName = externalID
	}

	customer, err := s.store.CreateCustomer(domain.Customer{
		TenantID:       tenantID,
		ExternalID:     externalID,
		DisplayName:    displayName,
		Email:          email,
		Status:         domain.CustomerStatusActive,
		LagoCustomerID: lagoCustomerID,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	})
	if err != nil {
		if err == store.ErrAlreadyExists || err == store.ErrDuplicateKey {
			return domain.Customer{}, fmt.Errorf("%w: customer external_id already exists", store.ErrDuplicateKey)
		}
		return domain.Customer{}, err
	}
	if strings.TrimSpace(customer.LagoCustomerID) == "" && strings.TrimSpace(lagoCustomerID) != "" {
		customer.LagoCustomerID = lagoCustomerID
	}
	return customer, nil
}

func (s *CustomerService) GetCustomerByExternalID(tenantID, externalID string) (domain.Customer, error) {
	if s == nil || s.store == nil {
		return domain.Customer{}, fmt.Errorf("%w: customer repository is required", ErrValidation)
	}
	externalID = strings.TrimSpace(externalID)
	if externalID == "" {
		return domain.Customer{}, fmt.Errorf("%w: external_id is required", ErrValidation)
	}
	return s.store.GetCustomerByExternalID(normalizeTenantID(tenantID), externalID)
}

func (s *CustomerService) ListCustomers(tenantID string, req ListCustomersRequest) ([]domain.Customer, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("%w: customer repository is required", ErrValidation)
	}
	limit, offset, err := normalizeListWindow(req.Limit, req.Offset)
	if err != nil {
		return nil, err
	}
	status, err := normalizeCustomerStatusFilter(req.Status)
	if err != nil {
		return nil, err
	}
	return s.store.ListCustomers(store.CustomerListFilter{
		TenantID:   normalizeTenantID(tenantID),
		Status:     status,
		ExternalID: strings.TrimSpace(req.ExternalID),
		Limit:      limit,
		Offset:     offset,
	})
}

func (s *CustomerService) UpdateCustomer(tenantID, externalID string, req UpdateCustomerRequest) (domain.Customer, error) {
	if s == nil || s.store == nil {
		return domain.Customer{}, fmt.Errorf("%w: customer repository is required", ErrValidation)
	}
	current, err := s.GetCustomerByExternalID(tenantID, externalID)
	if err != nil {
		return domain.Customer{}, err
	}

	updated := current
	if req.DisplayName != nil {
		displayName := strings.TrimSpace(*req.DisplayName)
		if displayName == "" {
			return domain.Customer{}, fmt.Errorf("%w: display_name is required", ErrValidation)
		}
		updated.DisplayName = displayName
	}
	if req.Email != nil {
		updated.Email = strings.TrimSpace(*req.Email)
	}
	if req.Status != nil {
		status, err := normalizeMutableCustomerStatus(*req.Status)
		if err != nil {
			return domain.Customer{}, err
		}
		updated.Status = status
	}
	if req.LagoCustomerID != nil {
		updated.LagoCustomerID = strings.TrimSpace(*req.LagoCustomerID)
	}
	updated.UpdatedAt = time.Now().UTC()

	out, err := s.store.UpdateCustomer(updated)
	if err != nil {
		if err == store.ErrAlreadyExists || err == store.ErrDuplicateKey {
			return domain.Customer{}, fmt.Errorf("%w: customer external_id already exists", store.ErrDuplicateKey)
		}
		return domain.Customer{}, err
	}
	return out, nil
}

func (s *CustomerService) UpsertCustomerBillingProfile(tenantID, externalID string, req UpsertCustomerBillingProfileRequest) (domain.CustomerBillingProfile, error) {
	if s == nil || s.store == nil {
		return domain.CustomerBillingProfile{}, fmt.Errorf("%w: customer repository is required", ErrValidation)
	}
	customer, err := s.GetCustomerByExternalID(tenantID, externalID)
	if err != nil {
		return domain.CustomerBillingProfile{}, err
	}
	current, err := s.store.GetCustomerBillingProfile(customer.TenantID, customer.ID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return domain.CustomerBillingProfile{}, err
	}
	if errors.Is(err, store.ErrNotFound) {
		current = defaultCustomerBillingProfile(customer)
	}

	current.LegalName = strings.TrimSpace(req.LegalName)
	current.Email = strings.TrimSpace(req.Email)
	current.Phone = strings.TrimSpace(req.Phone)
	current.AddressLine1 = strings.TrimSpace(req.AddressLine1)
	current.AddressLine2 = strings.TrimSpace(req.AddressLine2)
	current.City = strings.TrimSpace(req.City)
	current.State = strings.TrimSpace(req.State)
	current.PostalCode = strings.TrimSpace(req.PostalCode)
	current.Country = strings.TrimSpace(req.Country)
	current.Currency = strings.ToUpper(strings.TrimSpace(req.Currency))
	current.TaxIdentifier = strings.TrimSpace(req.TaxIdentifier)
	current.ProviderCode = strings.TrimSpace(req.ProviderCode)
	current.ProfileStatus = deriveBillingProfileStatus(current)
	current.LastSyncedAt = nil
	current.LastSyncError = ""
	current.UpdatedAt = time.Now().UTC()
	profile, err := s.store.UpsertCustomerBillingProfile(current)
	if err != nil {
		return domain.CustomerBillingProfile{}, err
	}
	setup, err := s.GetCustomerPaymentSetup(customer.TenantID, customer.ExternalID)
	if err != nil {
		return domain.CustomerBillingProfile{}, err
	}
	customer, profile, _, err = s.syncAndVerifyCustomerBilling(tenantID, customer, profile, setup)
	if err != nil {
		return domain.CustomerBillingProfile{}, err
	}
	return profile, nil
}

func (s *CustomerService) GetCustomerBillingProfile(tenantID, externalID string) (domain.CustomerBillingProfile, error) {
	if s == nil || s.store == nil {
		return domain.CustomerBillingProfile{}, fmt.Errorf("%w: customer repository is required", ErrValidation)
	}
	customer, err := s.GetCustomerByExternalID(tenantID, externalID)
	if err != nil {
		return domain.CustomerBillingProfile{}, err
	}
	profile, err := s.store.GetCustomerBillingProfile(customer.TenantID, customer.ID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return defaultCustomerBillingProfile(customer), nil
		}
		return domain.CustomerBillingProfile{}, err
	}
	return profile, nil
}

func (s *CustomerService) BeginCustomerPaymentSetup(tenantID, externalID string, req BeginCustomerPaymentSetupRequest) (BeginCustomerPaymentSetupResult, error) {
	if s == nil || s.store == nil {
		return BeginCustomerPaymentSetupResult{}, fmt.Errorf("%w: customer repository is required", ErrValidation)
	}
	if s.billingAdapter == nil {
		return BeginCustomerPaymentSetupResult{}, fmt.Errorf("%w: customer billing adapter is required", ErrValidation)
	}
	customer, err := s.GetCustomerByExternalID(tenantID, externalID)
	if err != nil {
		return BeginCustomerPaymentSetupResult{}, err
	}
	if customer.Status != domain.CustomerStatusActive {
		return BeginCustomerPaymentSetupResult{}, fmt.Errorf("%w: customer must be active before payment setup", ErrValidation)
	}
	current, err := s.store.GetCustomerPaymentSetup(customer.TenantID, customer.ID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return BeginCustomerPaymentSetupResult{}, err
	}
	if errors.Is(err, store.ErrNotFound) {
		current = defaultCustomerPaymentSetup(customer)
	}
	profile, err := s.GetCustomerBillingProfile(customer.TenantID, customer.ExternalID)
	if err != nil {
		return BeginCustomerPaymentSetupResult{}, err
	}
	if profile.ProfileStatus != domain.BillingProfileStatusReady {
		return BeginCustomerPaymentSetupResult{}, fmt.Errorf("%w: billing profile must be ready before payment setup", ErrValidation)
	}
	tenant, err := s.store.GetTenant(normalizeTenantID(tenantID))
	if err != nil {
		return BeginCustomerPaymentSetupResult{}, err
	}
	if strings.TrimSpace(tenant.LagoBillingProviderCode) == "" {
		return BeginCustomerPaymentSetupResult{}, fmt.Errorf("%w: tenant billing provider is not configured", ErrValidation)
	}

	paymentMethodType := strings.TrimSpace(req.PaymentMethodType)
	if paymentMethodType != "" {
		current.PaymentMethodType = paymentMethodType
	}
	if strings.TrimSpace(current.PaymentMethodType) == "" {
		current.PaymentMethodType = "card"
	}

	customer, profile, current, err = s.syncAndVerifyCustomerBilling(tenantID, customer, profile, current)
	if err != nil {
		return BeginCustomerPaymentSetupResult{}, err
	}
	if current.DefaultPaymentMethodPresent && current.SetupStatus == domain.PaymentSetupStatusReady {
		return BeginCustomerPaymentSetupResult{}, fmt.Errorf("%w: default payment method is already verified", ErrValidation)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	statusCode, body, err := s.billingAdapter.GenerateCustomerCheckoutURL(ctx, customer.ExternalID)
	if err != nil {
		return BeginCustomerPaymentSetupResult{}, fmt.Errorf("%w: generate payment setup checkout url: %v", ErrValidation, err)
	}
	if statusCode < 200 || statusCode >= 300 {
		return BeginCustomerPaymentSetupResult{}, fmt.Errorf("%w: lago checkout_url returned status=%d body=%s", ErrValidation, statusCode, abbrevForLog(body))
	}
	checkout, err := decodeLagoCustomerCheckoutResponse(body)
	if err != nil {
		return BeginCustomerPaymentSetupResult{}, err
	}

	now := time.Now().UTC()
	current.LastVerifiedAt = nil
	current.LastVerificationResult = "checkout_url_generated"
	current.LastVerificationError = ""
	current.SetupStatus = derivePaymentSetupStatus(current)
	current.UpdatedAt = now
	setup, err := s.store.UpsertCustomerPaymentSetup(current)
	if err != nil {
		return BeginCustomerPaymentSetupResult{}, err
	}
	return BeginCustomerPaymentSetupResult{
		ExternalID:   customer.ExternalID,
		CheckoutURL:  checkout.Customer.CheckoutURL,
		PaymentSetup: setup,
	}, nil
}

func (s *CustomerService) GetCustomerPaymentSetup(tenantID, externalID string) (domain.CustomerPaymentSetup, error) {
	if s == nil || s.store == nil {
		return domain.CustomerPaymentSetup{}, fmt.Errorf("%w: customer repository is required", ErrValidation)
	}
	customer, err := s.GetCustomerByExternalID(tenantID, externalID)
	if err != nil {
		return domain.CustomerPaymentSetup{}, err
	}
	setup, err := s.store.GetCustomerPaymentSetup(customer.TenantID, customer.ID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return defaultCustomerPaymentSetup(customer), nil
		}
		return domain.CustomerPaymentSetup{}, err
	}
	return setup, nil
}

func (s *CustomerService) GetCustomerReadiness(tenantID, externalID string) (CustomerReadiness, error) {
	if s == nil || s.store == nil {
		return CustomerReadiness{}, fmt.Errorf("%w: customer repository is required", ErrValidation)
	}
	customer, err := s.GetCustomerByExternalID(tenantID, externalID)
	if err != nil {
		return CustomerReadiness{}, err
	}
	tenant, err := s.store.GetTenant(normalizeTenantID(tenantID))
	if err != nil {
		return CustomerReadiness{}, err
	}
	profile, err := s.GetCustomerBillingProfile(customer.TenantID, customer.ExternalID)
	if err != nil {
		return CustomerReadiness{}, err
	}
	setup, err := s.GetCustomerPaymentSetup(customer.TenantID, customer.ExternalID)
	if err != nil {
		return CustomerReadiness{}, err
	}
	return buildCustomerReadiness(tenant, customer, profile, setup), nil
}

func (s *CustomerService) RefreshCustomerPaymentSetup(tenantID, externalID string) (RefreshCustomerPaymentSetupResult, error) {
	if s == nil || s.store == nil {
		return RefreshCustomerPaymentSetupResult{}, fmt.Errorf("%w: customer repository is required", ErrValidation)
	}
	customer, err := s.GetCustomerByExternalID(tenantID, externalID)
	if err != nil {
		return RefreshCustomerPaymentSetupResult{}, err
	}
	profile, err := s.GetCustomerBillingProfile(customer.TenantID, customer.ExternalID)
	if err != nil {
		return RefreshCustomerPaymentSetupResult{}, err
	}
	setup, err := s.GetCustomerPaymentSetup(customer.TenantID, customer.ExternalID)
	if err != nil {
		return RefreshCustomerPaymentSetupResult{}, err
	}
	customer, profile, setup, err = s.syncAndVerifyCustomerBilling(tenantID, customer, profile, setup)
	if err != nil {
		return RefreshCustomerPaymentSetupResult{}, err
	}
	tenant, err := s.store.GetTenant(normalizeTenantID(tenantID))
	if err != nil {
		return RefreshCustomerPaymentSetupResult{}, err
	}
	return RefreshCustomerPaymentSetupResult{
		ExternalID:   customer.ExternalID,
		PaymentSetup: setup,
		Readiness:    buildCustomerReadiness(tenant, customer, profile, setup),
	}, nil
}

func (s *CustomerService) syncAndVerifyCustomerBilling(tenantID string, customer domain.Customer, profile domain.CustomerBillingProfile, setup domain.CustomerPaymentSetup) (domain.Customer, domain.CustomerBillingProfile, domain.CustomerPaymentSetup, error) {
	if s == nil || s.store == nil || s.billingAdapter == nil {
		return customer, profile, setup, nil
	}
	tenant, err := s.store.GetTenant(normalizeTenantID(tenantID))
	if err != nil {
		return customer, profile, setup, err
	}
	if strings.TrimSpace(tenant.LagoOrganizationID) == "" || strings.TrimSpace(tenant.LagoBillingProviderCode) == "" {
		return customer, profile, setup, nil
	}
	if profile.CustomerID == "" {
		return customer, profile, setup, nil
	}
	if setup.CustomerID == "" {
		setup = defaultCustomerPaymentSetup(customer)
	}
	if profile.ProfileStatus != domain.BillingProfileStatusReady && profile.ProfileStatus != domain.BillingProfileStatusSyncError {
		return customer, profile, setup, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	payload, err := buildLagoCustomerPayload(tenant, customer, profile, setup)
	if err != nil {
		return s.recordCustomerSyncFailure(customer, profile, setup, err.Error())
	}
	statusCode, body, err := s.billingAdapter.UpsertCustomer(ctx, payload)
	if err != nil {
		return s.recordCustomerSyncFailure(customer, profile, setup, err.Error())
	}
	if statusCode < 200 || statusCode >= 300 {
		return s.recordCustomerSyncFailure(customer, profile, setup, fmt.Sprintf("lago customer upsert returned status=%d body=%s", statusCode, abbrevForLog(body)))
	}

	remoteCustomer, err := decodeLagoCustomerResponse(body)
	if err != nil {
		return s.recordCustomerSyncFailure(customer, profile, setup, err.Error())
	}

	now := time.Now().UTC()
	customerChanged := false
	setupChanged := false

	if strings.TrimSpace(remoteCustomer.Customer.LagoID) != "" && customer.LagoCustomerID != remoteCustomer.Customer.LagoID {
		customer.LagoCustomerID = remoteCustomer.Customer.LagoID
		customer.UpdatedAt = now
		customerChanged = true
	}
	profile.ProfileStatus = deriveBillingProfileStatus(profile)
	profile.LastSyncedAt = &now
	profile.LastSyncError = ""
	profile.UpdatedAt = now

	providerCustomerID := strings.TrimSpace(remoteCustomer.Customer.BillingConfiguration.ProviderCustomerID)
	if providerCustomerID != "" && setup.ProviderCustomerReference != providerCustomerID {
		setup.ProviderCustomerReference = providerCustomerID
		setupChanged = true
	}
	if setup.PaymentMethodType == "" && len(remoteCustomer.Customer.BillingConfiguration.ProviderPaymentMethods) > 0 {
		setup.PaymentMethodType = strings.TrimSpace(remoteCustomer.Customer.BillingConfiguration.ProviderPaymentMethods[0])
		setupChanged = true
	}

	if customerChanged {
		updatedCustomer, updateErr := s.store.UpdateCustomer(customer)
		if updateErr != nil {
			return customer, profile, setup, updateErr
		}
		customer = updatedCustomer
	}
	updatedProfile, updateErr := s.store.UpsertCustomerBillingProfile(profile)
	if updateErr != nil {
		return customer, profile, setup, updateErr
	}
	profile = updatedProfile

	paymentStatusCode, paymentBody, paymentErr := s.billingAdapter.ListCustomerPaymentMethods(ctx, customer.ExternalID)
	if paymentErr != nil {
		return s.recordCustomerSyncFailure(customer, profile, setup, paymentErr.Error())
	}
	if paymentStatusCode < 200 || paymentStatusCode >= 300 {
		return s.recordCustomerSyncFailure(customer, profile, setup, fmt.Sprintf("lago customer payment method lookup returned status=%d body=%s", paymentStatusCode, abbrevForLog(paymentBody)))
	}
	paymentMethods, err := decodeLagoPaymentMethodsResponse(paymentBody)
	if err != nil {
		return s.recordCustomerSyncFailure(customer, profile, setup, err.Error())
	}
	defaultFound := false
	defaultMethodID := ""
	for _, paymentMethod := range paymentMethods.PaymentMethods {
		if paymentMethod.IsDefault {
			defaultFound = true
			defaultMethodID = strings.TrimSpace(paymentMethod.ProviderMethodID)
			break
		}
	}
	setup.DefaultPaymentMethodPresent = defaultFound
	if defaultFound {
		setup.ProviderPaymentMethodReference = defaultMethodID
		setup.LastVerificationResult = "verified"
		setup.LastVerificationError = ""
	} else {
		setup.ProviderPaymentMethodReference = ""
		setup.LastVerificationResult = "no_default_payment_method"
		setup.LastVerificationError = ""
	}
	setup.LastVerifiedAt = &now
	setup.SetupStatus = derivePaymentSetupStatus(setup)
	setup.UpdatedAt = now
	setupChanged = true

	if setupChanged {
		updatedSetup, updateErr := s.store.UpsertCustomerPaymentSetup(setup)
		if updateErr != nil {
			return customer, profile, setup, updateErr
		}
		setup = updatedSetup
	}
	return customer, profile, setup, nil
}

func (s *CustomerService) recordCustomerSyncFailure(customer domain.Customer, profile domain.CustomerBillingProfile, setup domain.CustomerPaymentSetup, errMessage string) (domain.Customer, domain.CustomerBillingProfile, domain.CustomerPaymentSetup, error) {
	now := time.Now().UTC()
	profile.ProfileStatus = domain.BillingProfileStatusSyncError
	profile.LastSyncError = strings.TrimSpace(errMessage)
	profile.LastSyncedAt = nil
	profile.UpdatedAt = now
	updatedProfile, err := s.store.UpsertCustomerBillingProfile(profile)
	if err != nil {
		return customer, domain.CustomerBillingProfile{}, domain.CustomerPaymentSetup{}, err
	}
	setup.LastVerifiedAt = &now
	setup.LastVerificationResult = ""
	setup.LastVerificationError = strings.TrimSpace(errMessage)
	setup.SetupStatus = derivePaymentSetupStatus(setup)
	setup.UpdatedAt = now
	updatedSetup, err := s.store.UpsertCustomerPaymentSetup(setup)
	if err != nil {
		return customer, domain.CustomerBillingProfile{}, domain.CustomerPaymentSetup{}, err
	}
	return customer, updatedProfile, updatedSetup, nil
}

type lagoCustomerResponse struct {
	Customer struct {
		LagoID               string `json:"lago_id"`
		ExternalID           string `json:"external_id"`
		BillingConfiguration struct {
			PaymentProvider        string   `json:"payment_provider"`
			PaymentProviderCode    string   `json:"payment_provider_code"`
			ProviderCustomerID     string   `json:"provider_customer_id"`
			ProviderPaymentMethods []string `json:"provider_payment_methods"`
		} `json:"billing_configuration"`
	} `json:"customer"`
}

type lagoPaymentMethodsResponse struct {
	PaymentMethods []struct {
		LagoID           string `json:"lago_id"`
		IsDefault        bool   `json:"is_default"`
		ProviderMethodID string `json:"provider_method_id"`
	} `json:"payment_methods"`
}

type lagoCustomerCheckoutResponse struct {
	Customer struct {
		CheckoutURL string `json:"checkout_url"`
	} `json:"customer"`
}

func buildLagoCustomerPayload(tenant domain.Tenant, customer domain.Customer, profile domain.CustomerBillingProfile, setup domain.CustomerPaymentSetup) ([]byte, error) {
	providerCode := strings.TrimSpace(profile.ProviderCode)
	if providerCode == "" {
		providerCode = strings.TrimSpace(tenant.LagoBillingProviderCode)
	}
	paymentProvider, err := inferPaymentProviderFromCode(providerCode)
	if err != nil {
		return nil, err
	}
	paymentMethodType := strings.TrimSpace(setup.PaymentMethodType)
	if paymentMethodType == "" {
		paymentMethodType = "card"
	}
	payload := map[string]any{
		"customer": map[string]any{
			"external_id":   customer.ExternalID,
			"name":          customer.DisplayName,
			"legal_name":    profile.LegalName,
			"email":         profile.Email,
			"currency":      profile.Currency,
			"country":       profile.Country,
			"address_line1": profile.AddressLine1,
			"address_line2": profile.AddressLine2,
			"city":          profile.City,
			"state":         profile.State,
			"zipcode":       profile.PostalCode,
			"billing_configuration": map[string]any{
				"payment_provider":         paymentProvider,
				"payment_provider_code":    providerCode,
				"sync":                     true,
				"sync_with_provider":       true,
				"provider_payment_methods": []string{paymentMethodType},
			},
		},
	}
	if setup.ProviderCustomerReference != "" {
		payload["customer"].(map[string]any)["billing_configuration"].(map[string]any)["provider_customer_id"] = setup.ProviderCustomerReference
	}
	return json.Marshal(payload)
}

func decodeLagoCustomerResponse(body []byte) (lagoCustomerResponse, error) {
	var out lagoCustomerResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return lagoCustomerResponse{}, fmt.Errorf("%w: lago customer payload must be valid json", ErrValidation)
	}
	if strings.TrimSpace(out.Customer.ExternalID) == "" {
		return lagoCustomerResponse{}, fmt.Errorf("%w: lago customer payload missing customer.external_id", ErrValidation)
	}
	return out, nil
}

func decodeLagoPaymentMethodsResponse(body []byte) (lagoPaymentMethodsResponse, error) {
	var out lagoPaymentMethodsResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return lagoPaymentMethodsResponse{}, fmt.Errorf("%w: lago payment methods payload must be valid json", ErrValidation)
	}
	if out.PaymentMethods == nil {
		out.PaymentMethods = []struct {
			LagoID           string `json:"lago_id"`
			IsDefault        bool   `json:"is_default"`
			ProviderMethodID string `json:"provider_method_id"`
		}{}
	}
	return out, nil
}

func decodeLagoCustomerCheckoutResponse(body []byte) (lagoCustomerCheckoutResponse, error) {
	var out lagoCustomerCheckoutResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return lagoCustomerCheckoutResponse{}, fmt.Errorf("%w: lago customer checkout payload must be valid json", ErrValidation)
	}
	if strings.TrimSpace(out.Customer.CheckoutURL) == "" {
		return lagoCustomerCheckoutResponse{}, fmt.Errorf("%w: lago customer checkout payload missing customer.checkout_url", ErrValidation)
	}
	return out, nil
}

func buildCustomerReadiness(tenant domain.Tenant, customer domain.Customer, profile domain.CustomerBillingProfile, setup domain.CustomerPaymentSetup) CustomerReadiness {
	missing := make([]string, 0)
	status := "ready"
	billingProviderConfigured := strings.TrimSpace(tenant.LagoBillingProviderCode) != ""
	lagoCustomerSynced := strings.TrimSpace(customer.LagoCustomerID) != "" && profile.LastSyncedAt != nil && strings.TrimSpace(profile.LastSyncError) == ""
	defaultPaymentMethodVerified := setup.DefaultPaymentMethodPresent && strings.TrimSpace(setup.LastVerificationError) == ""
	if customer.Status != domain.CustomerStatusActive {
		missing = append(missing, "customer_active")
	}
	if !billingProviderConfigured {
		missing = append(missing, "billing_provider_configured")
	}
	if profile.ProfileStatus != domain.BillingProfileStatusReady {
		missing = append(missing, "billing_profile_ready")
	}
	if !lagoCustomerSynced {
		missing = append(missing, "lago_customer_synced")
	}
	if setup.SetupStatus != domain.PaymentSetupStatusReady {
		missing = append(missing, "payment_setup_ready")
	}
	if !defaultPaymentMethodVerified {
		missing = append(missing, "default_payment_method_verified")
	}
	if len(missing) > 0 {
		status = "pending"
	}

	return CustomerReadiness{
		Status:                       status,
		MissingSteps:                 missing,
		CustomerExists:               true,
		CustomerActive:               customer.Status == domain.CustomerStatusActive,
		BillingProviderConfigured:    billingProviderConfigured,
		LagoCustomerSynced:           lagoCustomerSynced,
		DefaultPaymentMethodVerified: defaultPaymentMethodVerified,
		BillingProfileStatus:         profile.ProfileStatus,
		PaymentSetupStatus:           setup.SetupStatus,
		BillingProfile:               profile,
		PaymentSetup:                 setup,
	}
}

func inferPaymentProviderFromCode(code string) (string, error) {
	raw := strings.ToLower(strings.TrimSpace(code))
	switch {
	case raw == "":
		return "", fmt.Errorf("%w: payment provider code is required", ErrValidation)
	case strings.HasPrefix(raw, "stripe"):
		return "stripe", nil
	case strings.HasPrefix(raw, "gocardless"):
		return "gocardless", nil
	case strings.HasPrefix(raw, "cashfree"):
		return "cashfree", nil
	case strings.HasPrefix(raw, "adyen"):
		return "adyen", nil
	case strings.HasPrefix(raw, "moneyhash"):
		return "moneyhash", nil
	case strings.HasPrefix(raw, "flutterwave"):
		return "flutterwave", nil
	default:
		return "", fmt.Errorf("%w: unsupported payment provider code %q", ErrValidation, code)
	}
}

func normalizeCustomerStatusFilter(v string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(v))
	if value == "" {
		return "", nil
	}
	switch domain.CustomerStatus(value) {
	case domain.CustomerStatusActive, domain.CustomerStatusSuspended, domain.CustomerStatusArchived:
		return value, nil
	default:
		return "", fmt.Errorf("%w: status must be one of active, suspended, archived", ErrValidation)
	}
}

func normalizeMutableCustomerStatus(v domain.CustomerStatus) (domain.CustomerStatus, error) {
	switch domain.CustomerStatus(strings.ToLower(strings.TrimSpace(string(v)))) {
	case domain.CustomerStatusActive:
		return domain.CustomerStatusActive, nil
	case domain.CustomerStatusSuspended:
		return domain.CustomerStatusSuspended, nil
	case domain.CustomerStatusArchived:
		return domain.CustomerStatusArchived, nil
	default:
		return "", fmt.Errorf("%w: status must be one of active, suspended, archived", ErrValidation)
	}
}

func defaultCustomerBillingProfile(customer domain.Customer) domain.CustomerBillingProfile {
	now := customer.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return domain.CustomerBillingProfile{
		CustomerID:    customer.ID,
		TenantID:      customer.TenantID,
		ProfileStatus: domain.BillingProfileStatusMissing,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func defaultCustomerPaymentSetup(customer domain.Customer) domain.CustomerPaymentSetup {
	now := customer.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return domain.CustomerPaymentSetup{
		CustomerID:     customer.ID,
		TenantID:       customer.TenantID,
		SetupStatus:    domain.PaymentSetupStatusMissing,
		CreatedAt:      now,
		UpdatedAt:      now,
		LastVerifiedAt: nil,
	}
}

func deriveBillingProfileStatus(profile domain.CustomerBillingProfile) domain.BillingProfileStatus {
	if hasAnyBillingProfileData(profile) {
		if strings.TrimSpace(profile.LegalName) != "" &&
			strings.TrimSpace(profile.Email) != "" &&
			strings.TrimSpace(profile.AddressLine1) != "" &&
			strings.TrimSpace(profile.City) != "" &&
			strings.TrimSpace(profile.PostalCode) != "" &&
			strings.TrimSpace(profile.Country) != "" &&
			strings.TrimSpace(profile.Currency) != "" {
			return domain.BillingProfileStatusReady
		}
		return domain.BillingProfileStatusIncomplete
	}
	return domain.BillingProfileStatusMissing
}

func hasAnyBillingProfileData(profile domain.CustomerBillingProfile) bool {
	return strings.TrimSpace(profile.LegalName) != "" ||
		strings.TrimSpace(profile.Email) != "" ||
		strings.TrimSpace(profile.Phone) != "" ||
		strings.TrimSpace(profile.AddressLine1) != "" ||
		strings.TrimSpace(profile.AddressLine2) != "" ||
		strings.TrimSpace(profile.City) != "" ||
		strings.TrimSpace(profile.State) != "" ||
		strings.TrimSpace(profile.PostalCode) != "" ||
		strings.TrimSpace(profile.Country) != "" ||
		strings.TrimSpace(profile.Currency) != "" ||
		strings.TrimSpace(profile.TaxIdentifier) != "" ||
		strings.TrimSpace(profile.ProviderCode) != ""
}

func derivePaymentSetupStatus(setup domain.CustomerPaymentSetup) domain.PaymentSetupStatus {
	if strings.TrimSpace(setup.LastVerificationError) != "" {
		return domain.PaymentSetupStatusError
	}
	if setup.DefaultPaymentMethodPresent {
		return domain.PaymentSetupStatusReady
	}
	if strings.TrimSpace(setup.PaymentMethodType) != "" ||
		strings.TrimSpace(setup.ProviderCustomerReference) != "" ||
		strings.TrimSpace(setup.ProviderPaymentMethodReference) != "" ||
		strings.TrimSpace(setup.LastVerificationResult) != "" ||
		setup.LastVerifiedAt != nil {
		return domain.PaymentSetupStatusPending
	}
	return domain.PaymentSetupStatusMissing
}
