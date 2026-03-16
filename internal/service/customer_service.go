package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

type CustomerService struct {
	store store.Repository
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

type UpsertCustomerPaymentSetupRequest struct {
	DefaultPaymentMethodPresent    bool       `json:"default_payment_method_present"`
	PaymentMethodType              string     `json:"payment_method_type,omitempty"`
	ProviderCustomerReference      string     `json:"provider_customer_reference,omitempty"`
	ProviderPaymentMethodReference string     `json:"provider_payment_method_reference,omitempty"`
	LastVerifiedAt                 *time.Time `json:"last_verified_at,omitempty"`
	LastVerificationResult         string     `json:"last_verification_result,omitempty"`
	LastVerificationError          string     `json:"last_verification_error,omitempty"`
}

type CustomerReadiness struct {
	Status               string                        `json:"status"`
	MissingSteps         []string                      `json:"missing_steps"`
	CustomerExists       bool                          `json:"customer_exists"`
	CustomerActive       bool                          `json:"customer_active"`
	BillingProfileStatus domain.BillingProfileStatus   `json:"billing_profile_status"`
	PaymentSetupStatus   domain.PaymentSetupStatus     `json:"payment_setup_status"`
	BillingProfile       domain.CustomerBillingProfile `json:"billing_profile"`
	PaymentSetup         domain.CustomerPaymentSetup   `json:"payment_setup"`
}

func NewCustomerService(s store.Repository) *CustomerService {
	return &CustomerService{store: s}
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

	return s.store.UpsertCustomerBillingProfile(current)
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

func (s *CustomerService) UpsertCustomerPaymentSetup(tenantID, externalID string, req UpsertCustomerPaymentSetupRequest) (domain.CustomerPaymentSetup, error) {
	if s == nil || s.store == nil {
		return domain.CustomerPaymentSetup{}, fmt.Errorf("%w: customer repository is required", ErrValidation)
	}
	customer, err := s.GetCustomerByExternalID(tenantID, externalID)
	if err != nil {
		return domain.CustomerPaymentSetup{}, err
	}
	current, err := s.store.GetCustomerPaymentSetup(customer.TenantID, customer.ID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return domain.CustomerPaymentSetup{}, err
	}
	if errors.Is(err, store.ErrNotFound) {
		current = defaultCustomerPaymentSetup(customer)
	}

	current.DefaultPaymentMethodPresent = req.DefaultPaymentMethodPresent
	current.PaymentMethodType = strings.TrimSpace(req.PaymentMethodType)
	current.ProviderCustomerReference = strings.TrimSpace(req.ProviderCustomerReference)
	current.ProviderPaymentMethodReference = strings.TrimSpace(req.ProviderPaymentMethodReference)
	current.LastVerifiedAt = req.LastVerifiedAt
	current.LastVerificationResult = strings.TrimSpace(req.LastVerificationResult)
	current.LastVerificationError = strings.TrimSpace(req.LastVerificationError)
	current.SetupStatus = derivePaymentSetupStatus(current)
	current.UpdatedAt = time.Now().UTC()

	return s.store.UpsertCustomerPaymentSetup(current)
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
	profile, err := s.GetCustomerBillingProfile(customer.TenantID, customer.ExternalID)
	if err != nil {
		return CustomerReadiness{}, err
	}
	setup, err := s.GetCustomerPaymentSetup(customer.TenantID, customer.ExternalID)
	if err != nil {
		return CustomerReadiness{}, err
	}

	missing := make([]string, 0)
	status := "ready"
	if customer.Status != domain.CustomerStatusActive {
		missing = append(missing, "customer_active")
	}
	if profile.ProfileStatus != domain.BillingProfileStatusReady {
		missing = append(missing, "billing_profile_ready")
	}
	if setup.SetupStatus != domain.PaymentSetupStatusReady {
		missing = append(missing, "payment_setup_ready")
	}
	if len(missing) > 0 {
		status = "pending"
	}

	return CustomerReadiness{
		Status:               status,
		MissingSteps:         missing,
		CustomerExists:       true,
		CustomerActive:       customer.Status == domain.CustomerStatusActive,
		BillingProfileStatus: profile.ProfileStatus,
		PaymentSetupStatus:   setup.SetupStatus,
		BillingProfile:       profile,
		PaymentSetup:         setup,
	}, nil
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
