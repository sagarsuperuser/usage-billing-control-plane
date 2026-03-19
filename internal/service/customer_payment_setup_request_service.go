package service

import (
	"fmt"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

type CustomerPaymentSetupRequestActor struct {
	SubjectType   string
	SubjectID     string
	UserEmail     string
	ActorAPIKeyID string
}

type CustomerPaymentSetupRequestResult struct {
	Action       string                      `json:"action"`
	ExternalID   string                      `json:"external_id"`
	CheckoutURL  string                      `json:"checkout_url"`
	PaymentSetup domain.CustomerPaymentSetup `json:"payment_setup"`
	Readiness    CustomerReadiness           `json:"readiness"`
	Dispatch     NotificationDispatchResult  `json:"dispatch"`
}

type CustomerPaymentSetupRequestService struct {
	store         store.Repository
	customers     *CustomerService
	notifications *NotificationService
}

func NewCustomerPaymentSetupRequestService(store store.Repository, customers *CustomerService, notifications *NotificationService) *CustomerPaymentSetupRequestService {
	return &CustomerPaymentSetupRequestService{
		store:         store,
		customers:     customers,
		notifications: notifications,
	}
}

func (s *CustomerPaymentSetupRequestService) Request(tenantID, externalID string, actor CustomerPaymentSetupRequestActor, paymentMethodType string) (CustomerPaymentSetupRequestResult, error) {
	return s.send(tenantID, externalID, actor, paymentMethodType, "requested")
}

func (s *CustomerPaymentSetupRequestService) Resend(tenantID, externalID string, actor CustomerPaymentSetupRequestActor, paymentMethodType string) (CustomerPaymentSetupRequestResult, error) {
	return s.send(tenantID, externalID, actor, paymentMethodType, "resent")
}

func (s *CustomerPaymentSetupRequestService) send(tenantID, externalID string, actor CustomerPaymentSetupRequestActor, paymentMethodType, action string) (CustomerPaymentSetupRequestResult, error) {
	if s == nil || s.store == nil || s.customers == nil {
		return CustomerPaymentSetupRequestResult{}, fmt.Errorf("%w: payment setup request services are required", ErrValidation)
	}
	if s.notifications == nil || !s.notifications.CanSendCustomerPaymentSetupRequest() {
		return CustomerPaymentSetupRequestResult{}, fmt.Errorf("%w: payment setup request notification backend is required", ErrValidation)
	}
	customer, err := s.customers.GetCustomerByExternalID(tenantID, externalID)
	if err != nil {
		return CustomerPaymentSetupRequestResult{}, err
	}
	email := strings.ToLower(strings.TrimSpace(customer.Email))
	if email == "" {
		return CustomerPaymentSetupRequestResult{}, fmt.Errorf("%w: customer email is required before sending payment setup request", ErrValidation)
	}
	tenant, err := s.store.GetTenant(normalizeTenantID(tenantID))
	if err != nil {
		return CustomerPaymentSetupRequestResult{}, err
	}

	beginResult, err := s.customers.BeginCustomerPaymentSetup(tenantID, externalID, BeginCustomerPaymentSetupRequest{
		PaymentMethodType: strings.TrimSpace(paymentMethodType),
	})
	if err != nil {
		return CustomerPaymentSetupRequestResult{}, err
	}

	requestKind := strings.ToLower(strings.TrimSpace(action))
	if requestKind == "" {
		requestKind = "requested"
	}
	setup := beginResult.PaymentSetup
	now := time.Now().UTC()
	setup.LastRequestStatus = domain.PaymentSetupRequestStatusSent
	setup.LastRequestKind = requestKind
	setup.LastRequestToEmail = email
	setup.LastRequestSentAt = &now
	setup.LastRequestError = ""
	setup.UpdatedAt = now

	dispatch, sendErr := s.notifications.SendCustomerPaymentSetupRequest(CustomerPaymentSetupRequestEmail{
		ToEmail:          email,
		CustomerName:     customer.DisplayName,
		WorkspaceName:    tenant.Name,
		CheckoutURL:      beginResult.CheckoutURL,
		RequestedByEmail: strings.TrimSpace(actor.UserEmail),
		RequestKind:      requestKind,
	})
	if sendErr != nil {
		setup.LastRequestStatus = domain.PaymentSetupRequestStatusFailed
		setup.LastRequestError = sendErr.Error()
		setup.UpdatedAt = time.Now().UTC()
		if _, persistErr := s.store.UpsertCustomerPaymentSetup(setup); persistErr != nil {
			return CustomerPaymentSetupRequestResult{}, persistErr
		}
		return CustomerPaymentSetupRequestResult{}, sendErr
	}

	updatedSetup, err := s.store.UpsertCustomerPaymentSetup(setup)
	if err != nil {
		return CustomerPaymentSetupRequestResult{}, err
	}
	readiness, err := s.customers.GetCustomerReadiness(tenantID, externalID)
	if err != nil {
		return CustomerPaymentSetupRequestResult{}, err
	}
	readiness.PaymentSetup = updatedSetup
	readiness.PaymentSetupStatus = updatedSetup.SetupStatus

	if _, auditErr := s.store.CreateTenantAuditEvent(domain.TenantAuditEvent{
		TenantID:      normalizeTenantID(tenantID),
		ActorAPIKeyID: strings.TrimSpace(actor.ActorAPIKeyID),
		Action:        "payment_setup_" + requestKind,
		Metadata: map[string]any{
			"customer_external_id": customer.ExternalID,
			"customer_id":          customer.ID,
			"recipient_email":      email,
			"request_kind":         requestKind,
			"subject_type":         strings.TrimSpace(actor.SubjectType),
			"subject_id":           strings.TrimSpace(actor.SubjectID),
			"user_email":           strings.TrimSpace(actor.UserEmail),
		},
		CreatedAt: dispatch.DispatchedAt,
	}); auditErr != nil {
		// Best-effort audit: notification has already been sent successfully.
	}

	return CustomerPaymentSetupRequestResult{
		Action:       requestKind,
		ExternalID:   customer.ExternalID,
		CheckoutURL:  beginResult.CheckoutURL,
		PaymentSetup: updatedSetup,
		Readiness:    readiness,
		Dispatch:     dispatch,
	}, nil
}
