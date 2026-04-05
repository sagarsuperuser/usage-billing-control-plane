package service

import (
	"errors"
	"fmt"
	"strings"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

type CustomerOnboardingService struct {
	customers *CustomerService
}

type CustomerOnboardingRequest struct {
	ExternalID        string                               `json:"external_id" validate:"required"`
	DisplayName       string                               `json:"display_name,omitempty"`
	Email             string                               `json:"email,omitempty" validate:"omitempty,email"`
	BillingProfile    *UpsertCustomerBillingProfileRequest `json:"billing_profile,omitempty"`
	StartPaymentSetup bool                                 `json:"start_payment_setup,omitempty"`
	PaymentMethodType string                               `json:"payment_method_type,omitempty"`
}

type CustomerOnboardingResult struct {
	Customer              domain.Customer               `json:"customer"`
	CustomerCreated       bool                          `json:"customer_created"`
	BillingProfileApplied bool                          `json:"billing_profile_applied"`
	PaymentSetupStarted   bool                          `json:"payment_setup_started"`
	CheckoutURL           string                        `json:"checkout_url,omitempty"`
	BillingProfile        domain.CustomerBillingProfile `json:"billing_profile"`
	PaymentSetup          domain.CustomerPaymentSetup   `json:"payment_setup"`
	Readiness             CustomerReadiness             `json:"readiness"`
}

func NewCustomerOnboardingService(customers *CustomerService) *CustomerOnboardingService {
	return &CustomerOnboardingService{customers: customers}
}

func (s *CustomerOnboardingService) OnboardCustomer(tenantID string, req CustomerOnboardingRequest) (CustomerOnboardingResult, error) {
	if s == nil || s.customers == nil {
		return CustomerOnboardingResult{}, fmt.Errorf("%w: customer onboarding service is required", ErrValidation)
	}

	externalID := strings.TrimSpace(req.ExternalID)
	if externalID == "" {
		return CustomerOnboardingResult{}, fmt.Errorf("%w: external_id is required", ErrValidation)
	}

	customer, created, err := s.ensureCustomer(tenantID, externalID, req)
	if err != nil {
		return CustomerOnboardingResult{}, err
	}

	billingProfileApplied := false
	if req.BillingProfile != nil {
		billingProfileApplied = true
		if _, err := s.customers.UpsertCustomerBillingProfile(tenantID, externalID, *req.BillingProfile); err != nil {
			return CustomerOnboardingResult{}, err
		}
	}

	checkoutURL := ""
	paymentSetupStarted := false
	if req.StartPaymentSetup {
		setupResult, err := s.customers.BeginCustomerPaymentSetup(tenantID, externalID, BeginCustomerPaymentSetupRequest{
			PaymentMethodType: strings.TrimSpace(req.PaymentMethodType),
		})
		if err != nil {
			return CustomerOnboardingResult{}, err
		}
		checkoutURL = setupResult.CheckoutURL
		paymentSetupStarted = true
	}

	customer, err = s.customers.GetCustomerByExternalID(tenantID, externalID)
	if err != nil {
		return CustomerOnboardingResult{}, err
	}
	readiness, err := s.customers.GetCustomerReadiness(tenantID, externalID)
	if err != nil {
		return CustomerOnboardingResult{}, err
	}

	return CustomerOnboardingResult{
		Customer:              customer,
		CustomerCreated:       created,
		BillingProfileApplied: billingProfileApplied,
		PaymentSetupStarted:   paymentSetupStarted,
		CheckoutURL:           checkoutURL,
		BillingProfile:        readiness.BillingProfile,
		PaymentSetup:          readiness.PaymentSetup,
		Readiness:             readiness,
	}, nil
}

func (s *CustomerOnboardingService) ensureCustomer(tenantID, externalID string, req CustomerOnboardingRequest) (domain.Customer, bool, error) {
	customer, err := s.customers.GetCustomerByExternalID(tenantID, externalID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			return domain.Customer{}, false, err
		}
		created, createErr := s.customers.CreateCustomer(tenantID, CreateCustomerRequest{
			ExternalID:  externalID,
			DisplayName: strings.TrimSpace(req.DisplayName),
			Email:       strings.TrimSpace(req.Email),
		})
		return created, true, createErr
	}

	updateReq := UpdateCustomerRequest{}
	needsUpdate := false
	if displayName := strings.TrimSpace(req.DisplayName); displayName != "" && displayName != customer.DisplayName {
		updateReq.DisplayName = &displayName
		needsUpdate = true
	}
	if email := strings.TrimSpace(req.Email); email != "" && email != customer.Email {
		updateReq.Email = &email
		needsUpdate = true
	}
	if !needsUpdate {
		return customer, false, nil
	}
	updated, err := s.customers.UpdateCustomer(tenantID, externalID, updateReq)
	return updated, false, err
}
