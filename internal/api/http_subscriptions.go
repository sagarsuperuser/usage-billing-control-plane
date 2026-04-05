package api

import (
	"net/http"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/service"
)

type createSubscriptionRequest struct {
	Code                string     `json:"code"`
	DisplayName         string     `json:"display_name"`
	CustomerExternalID  string     `json:"customer_external_id" validate:"required"`
	PlanID              string     `json:"plan_id" validate:"required"`
	BillingTime         string     `json:"billing_time,omitempty"`
	StartedAt           *time.Time `json:"started_at,omitempty"`
	RequestPaymentSetup bool       `json:"request_payment_setup"`
	PaymentMethodType   string     `json:"payment_method_type"`
}

type updateSubscriptionRequest struct {
	DisplayName *string    `json:"display_name,omitempty"`
	PlanID      *string    `json:"plan_id,omitempty"`
	Status      *string    `json:"status,omitempty"`
	BillingTime *string    `json:"billing_time,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
}

type subscriptionPaymentSetupRequest struct {
	PaymentMethodType string `json:"payment_method_type,omitempty"`
}

func (s *Server) listSubscriptions(w http.ResponseWriter, r *http.Request) {
	if s.subscriptionService == nil {
		writeError(w, http.StatusServiceUnavailable, "subscription service is required")
		return
	}
	tenantID := requestTenantID(r)
	items, err := s.subscriptionService.ListSubscriptions(tenantID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) createSubscription(w http.ResponseWriter, r *http.Request) {
	if s.subscriptionService == nil {
		writeError(w, http.StatusServiceUnavailable, "subscription service is required")
		return
	}
	tenantID := requestTenantID(r)
	var req createSubscriptionRequest
	if err := decodeAndValidate(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := s.subscriptionService.CreateSubscription(r.Context(), service.CreateSubscriptionRequest{
		TenantID:            tenantID,
		Code:                req.Code,
		DisplayName:         req.DisplayName,
		CustomerExternalID:  req.CustomerExternalID,
		PlanID:              req.PlanID,
		BillingTime:         req.BillingTime,
		StartedAt:           req.StartedAt,
		RequestPaymentSetup: req.RequestPaymentSetup,
		PaymentMethodType:   req.PaymentMethodType,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) getSubscription(w http.ResponseWriter, r *http.Request) {
	if s.subscriptionService == nil {
		writeError(w, http.StatusServiceUnavailable, "subscription service is required")
		return
	}
	id := urlParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	detail, err := s.subscriptionService.GetSubscription(requestTenantID(r), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) updateSubscription(w http.ResponseWriter, r *http.Request) {
	if s.subscriptionService == nil {
		writeError(w, http.StatusServiceUnavailable, "subscription service is required")
		return
	}
	id := urlParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	var req updateSubscriptionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var status *domain.SubscriptionStatus
	if req.Status != nil {
		value := domain.SubscriptionStatus(strings.TrimSpace(*req.Status))
		status = &value
	}
	var billingTime *domain.SubscriptionBillingTime
	if req.BillingTime != nil {
		value := domain.SubscriptionBillingTime(strings.TrimSpace(*req.BillingTime))
		billingTime = &value
	}
	detail, err := s.subscriptionService.UpdateSubscription(r.Context(), requestTenantID(r), id, service.UpdateSubscriptionRequest{
		DisplayName: req.DisplayName,
		PlanID:      req.PlanID,
		Status:      status,
		BillingTime: billingTime,
		StartedAt:   req.StartedAt,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) requestSubscriptionPaymentSetup(w http.ResponseWriter, r *http.Request) {
	if s.subscriptionService == nil {
		writeError(w, http.StatusServiceUnavailable, "subscription service is required")
		return
	}
	id := urlParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	var req subscriptionPaymentSetupRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := s.subscriptionService.RequestPaymentSetup(requestTenantID(r), id, req.PaymentMethodType)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) resendSubscriptionPaymentSetup(w http.ResponseWriter, r *http.Request) {
	if s.subscriptionService == nil {
		writeError(w, http.StatusServiceUnavailable, "subscription service is required")
		return
	}
	id := urlParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	var req subscriptionPaymentSetupRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := s.subscriptionService.ResendPaymentSetup(requestTenantID(r), id, req.PaymentMethodType)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
