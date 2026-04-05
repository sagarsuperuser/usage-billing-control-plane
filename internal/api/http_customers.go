package api

import (
	"net/http"
	"strings"

	"usage-billing-control-plane/internal/service"
)

func (s *Server) handleCustomerOnboarding(w http.ResponseWriter, r *http.Request) {
	tenantID := requestTenantID(r)
	var req service.CustomerOnboardingRequest
	if err := decodeAndValidate(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := s.customerOnboardingService.OnboardCustomer(tenantID, req)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	status := http.StatusOK
	if result.CustomerCreated {
		status = http.StatusCreated
	}
	writeJSON(w, status, result)
}

func (s *Server) listCustomers(w http.ResponseWriter, r *http.Request) {
	tenantID := requestTenantID(r)
	limit, err := parseQueryInt(r, "limit")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	offset, err := parseQueryInt(r, "offset")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	customers, err := s.customerService.ListCustomers(tenantID, service.ListCustomersRequest{
		Status:     r.URL.Query().Get("status"),
		ExternalID: r.URL.Query().Get("external_id"),
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, customers)
}

func (s *Server) createCustomer(w http.ResponseWriter, r *http.Request) {
	tenantID := requestTenantID(r)
	var req service.CreateCustomerRequest
	if err := decodeAndValidate(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	customer, err := s.customerService.CreateCustomer(tenantID, req)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, customer)
}

func (s *Server) getCustomer(w http.ResponseWriter, r *http.Request) {
	tenantID := requestTenantID(r)
	externalID := urlParam(r, "externalId")
	if externalID == "" {
		writeError(w, http.StatusBadRequest, "customer external_id is required")
		return
	}
	customer, err := s.customerService.GetCustomerByExternalID(tenantID, externalID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, customer)
}

func (s *Server) updateCustomer(w http.ResponseWriter, r *http.Request) {
	tenantID := requestTenantID(r)
	externalID := urlParam(r, "externalId")
	if externalID == "" {
		writeError(w, http.StatusBadRequest, "customer external_id is required")
		return
	}
	var req service.UpdateCustomerRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	customer, err := s.customerService.UpdateCustomer(tenantID, externalID, req)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, customer)
}

func (s *Server) getCustomerBillingProfile(w http.ResponseWriter, r *http.Request) {
	tenantID := requestTenantID(r)
	externalID := urlParam(r, "externalId")
	if externalID == "" {
		writeError(w, http.StatusBadRequest, "customer external_id is required")
		return
	}
	profile, err := s.customerService.GetCustomerBillingProfile(tenantID, externalID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (s *Server) upsertCustomerBillingProfile(w http.ResponseWriter, r *http.Request) {
	tenantID := requestTenantID(r)
	externalID := urlParam(r, "externalId")
	if externalID == "" {
		writeError(w, http.StatusBadRequest, "customer external_id is required")
		return
	}
	var req service.UpsertCustomerBillingProfileRequest
	if err := decodeAndValidate(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	profile, err := s.customerService.UpsertCustomerBillingProfile(tenantID, externalID, req)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (s *Server) retryCustomerBillingProfileSync(w http.ResponseWriter, r *http.Request) {
	tenantID := requestTenantID(r)
	externalID := urlParam(r, "externalId")
	if externalID == "" {
		writeError(w, http.StatusBadRequest, "customer external_id is required")
		return
	}
	result, err := s.customerService.RetryCustomerBillingProfileSync(tenantID, externalID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) getCustomerPaymentSetup(w http.ResponseWriter, r *http.Request) {
	tenantID := requestTenantID(r)
	externalID := urlParam(r, "externalId")
	if externalID == "" {
		writeError(w, http.StatusBadRequest, "customer external_id is required")
		return
	}
	setup, err := s.customerService.GetCustomerPaymentSetup(tenantID, externalID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, setup)
}

func (s *Server) requestCustomerPaymentSetup(w http.ResponseWriter, r *http.Request) {
	tenantID := requestTenantID(r)
	externalID := urlParam(r, "externalId")
	if externalID == "" {
		writeError(w, http.StatusBadRequest, "customer external_id is required")
		return
	}
	if s.customerPaymentSetupRequestService == nil || s.notificationService == nil || !s.notificationService.CanSendCustomerPaymentSetupRequest() {
		writeError(w, http.StatusNotImplemented, "payment setup request delivery is not configured")
		return
	}
	var req service.BeginCustomerPaymentSetupRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	principal, _ := principalFromContext(r.Context())
	actor := service.CustomerPaymentSetupRequestActor{
		SubjectType:   strings.TrimSpace(principal.SubjectType),
		SubjectID:     strings.TrimSpace(principal.SubjectID),
		UserEmail:     strings.TrimSpace(principal.UserEmail),
		ActorAPIKeyID: strings.TrimSpace(principal.APIKeyID),
	}
	result, err := s.customerPaymentSetupRequestService.Request(tenantID, externalID, actor, req.PaymentMethodType)
	attrs := []any{
		"request_id", requestIDFromContext(r.Context()),
		"tenant_id", tenantID,
		"customer_external_id", externalID,
		"request_kind", "request",
	}
	if err != nil {
		attrs = append(attrs, "error", err.Error())
		s.logger.Warn("customer payment setup request dispatch failed", attrs...)
		writeDomainError(w, err)
		return
	}
	attrs = append(attrs,
		"recipient_email", strings.TrimSpace(result.PaymentSetup.LastRequestToEmail),
		"backend", result.Dispatch.Backend,
		"action", result.Dispatch.Action,
		"domain", result.Dispatch.Domain,
	)
	s.logger.Info("customer payment setup request dispatched", attrs...)
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) resendCustomerPaymentSetup(w http.ResponseWriter, r *http.Request) {
	tenantID := requestTenantID(r)
	externalID := urlParam(r, "externalId")
	if externalID == "" {
		writeError(w, http.StatusBadRequest, "customer external_id is required")
		return
	}
	if s.customerPaymentSetupRequestService == nil || s.notificationService == nil || !s.notificationService.CanSendCustomerPaymentSetupRequest() {
		writeError(w, http.StatusNotImplemented, "payment setup request delivery is not configured")
		return
	}
	var req service.BeginCustomerPaymentSetupRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	principal, _ := principalFromContext(r.Context())
	actor := service.CustomerPaymentSetupRequestActor{
		SubjectType:   strings.TrimSpace(principal.SubjectType),
		SubjectID:     strings.TrimSpace(principal.SubjectID),
		UserEmail:     strings.TrimSpace(principal.UserEmail),
		ActorAPIKeyID: strings.TrimSpace(principal.APIKeyID),
	}
	result, err := s.customerPaymentSetupRequestService.Resend(tenantID, externalID, actor, req.PaymentMethodType)
	attrs := []any{
		"request_id", requestIDFromContext(r.Context()),
		"tenant_id", tenantID,
		"customer_external_id", externalID,
		"request_kind", "resend",
	}
	if err != nil {
		attrs = append(attrs, "error", err.Error())
		s.logger.Warn("customer payment setup request dispatch failed", attrs...)
		writeDomainError(w, err)
		return
	}
	attrs = append(attrs,
		"recipient_email", strings.TrimSpace(result.PaymentSetup.LastRequestToEmail),
		"backend", result.Dispatch.Backend,
		"action", result.Dispatch.Action,
		"domain", result.Dispatch.Domain,
	)
	s.logger.Info("customer payment setup request dispatched", attrs...)
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) getCustomerCheckoutURL(w http.ResponseWriter, r *http.Request) {
	tenantID := requestTenantID(r)
	externalID := urlParam(r, "externalId")
	if externalID == "" {
		writeError(w, http.StatusBadRequest, "customer external_id is required")
		return
	}
	var req service.BeginCustomerPaymentSetupRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := s.customerService.BeginCustomerPaymentSetup(tenantID, externalID, req)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) refreshCustomerPaymentSetup(w http.ResponseWriter, r *http.Request) {
	tenantID := requestTenantID(r)
	externalID := urlParam(r, "externalId")
	if externalID == "" {
		writeError(w, http.StatusBadRequest, "customer external_id is required")
		return
	}
	result, err := s.customerService.RefreshCustomerPaymentSetup(tenantID, externalID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if s.dunningService != nil {
		if _, err := s.dunningService.RefreshRunsForCustomer(tenantID, externalID); err != nil {
			writeDomainError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) getCustomerReadiness(w http.ResponseWriter, r *http.Request) {
	tenantID := requestTenantID(r)
	externalID := urlParam(r, "externalId")
	if externalID == "" {
		writeError(w, http.StatusBadRequest, "customer external_id is required")
		return
	}
	readiness, err := s.customerService.GetCustomerReadiness(tenantID, externalID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, readiness)
}
