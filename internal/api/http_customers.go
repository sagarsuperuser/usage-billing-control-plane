package api

import (
	"net/http"
	"strings"

	"usage-billing-control-plane/internal/service"
)

func parseCustomerPath(path string) (externalID string, action string, subaction string) {
	tail := strings.Trim(strings.TrimPrefix(path, "/v1/customers/"), "/")
	if tail == "" {
		return "", "", ""
	}
	parts := strings.Split(tail, "/")
	externalID = strings.TrimSpace(parts[0])
	if len(parts) > 1 {
		action = strings.TrimSpace(parts[1])
	}
	if len(parts) > 2 {
		subaction = strings.TrimSpace(parts[2])
	}
	return externalID, action, subaction
}

func (s *Server) handleCustomerOnboarding(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	tenantID := requestTenantID(r)
	var req service.CustomerOnboardingRequest
	if err := decodeJSON(r, &req); err != nil {
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

func (s *Server) handleCustomers(w http.ResponseWriter, r *http.Request) {
	tenantID := requestTenantID(r)
	switch r.Method {
	case http.MethodPost:
		var req service.CreateCustomerRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		customer, err := s.customerService.CreateCustomer(tenantID, req)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, customer)
	case http.MethodGet:
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
	default:
		writeMethodNotAllowed(w)
	}
}

func (s *Server) handleCustomerByExternalID(w http.ResponseWriter, r *http.Request) {
	tenantID := requestTenantID(r)
	externalID, action, subaction := parseCustomerPath(r.URL.Path)
	if externalID == "" {
		writeError(w, http.StatusBadRequest, "customer external_id is required")
		return
	}

	switch action {
	case "":
		switch r.Method {
		case http.MethodGet:
			customer, err := s.customerService.GetCustomerByExternalID(tenantID, externalID)
			if err != nil {
				writeDomainError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, customer)
		case http.MethodPatch:
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
		default:
			writeMethodNotAllowed(w)
		}
	case "billing-profile":
		if subaction == "retry-sync" {
			if r.Method != http.MethodPost {
				writeMethodNotAllowed(w)
				return
			}
			result, err := s.customerService.RetryCustomerBillingProfileSync(tenantID, externalID)
			if err != nil {
				writeDomainError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
			return
		}
		switch r.Method {
		case http.MethodGet:
			profile, err := s.customerService.GetCustomerBillingProfile(tenantID, externalID)
			if err != nil {
				writeDomainError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, profile)
		case http.MethodPut:
			var req service.UpsertCustomerBillingProfileRequest
			if err := decodeJSON(r, &req); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			profile, err := s.customerService.UpsertCustomerBillingProfile(tenantID, externalID, req)
			if err != nil {
				writeDomainError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, profile)
		default:
			writeMethodNotAllowed(w)
		}
	case "payment-setup":
		if subaction == "request" || subaction == "resend" {
			if r.Method != http.MethodPost {
				writeMethodNotAllowed(w)
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
			var (
				result service.CustomerPaymentSetupRequestResult
				err    error
			)
			if subaction == "resend" {
				result, err = s.customerPaymentSetupRequestService.Resend(tenantID, externalID, actor, req.PaymentMethodType)
			} else {
				result, err = s.customerPaymentSetupRequestService.Request(tenantID, externalID, actor, req.PaymentMethodType)
			}
			attrs := []any{
				"request_id", requestIDFromContext(r.Context()),
				"tenant_id", tenantID,
				"customer_external_id", externalID,
				"request_kind", subaction,
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
			return
		}
		if subaction == "checkout-url" {
			if r.Method != http.MethodPost {
				writeMethodNotAllowed(w)
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
			return
		}
		if subaction == "refresh" {
			if r.Method != http.MethodPost {
				writeMethodNotAllowed(w)
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
			return
		}
		switch r.Method {
		case http.MethodGet:
			setup, err := s.customerService.GetCustomerPaymentSetup(tenantID, externalID)
			if err != nil {
				writeDomainError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, setup)
		default:
			writeMethodNotAllowed(w)
		}
	case "readiness":
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w)
			return
		}
		readiness, err := s.customerService.GetCustomerReadiness(tenantID, externalID)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, readiness)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}
