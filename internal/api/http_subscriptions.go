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
	CustomerExternalID  string     `json:"customer_external_id"`
	PlanID              string     `json:"plan_id"`
	StartedAt           *time.Time `json:"started_at,omitempty"`
	RequestPaymentSetup bool       `json:"request_payment_setup"`
	PaymentMethodType   string     `json:"payment_method_type"`
}

type updateSubscriptionRequest struct {
	DisplayName *string    `json:"display_name,omitempty"`
	PlanID      *string    `json:"plan_id,omitempty"`
	Status      *string    `json:"status,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
}

type subscriptionPaymentSetupRequest struct {
	PaymentMethodType string `json:"payment_method_type,omitempty"`
}

func (s *Server) handleSubscriptions(w http.ResponseWriter, r *http.Request) {
	if s.subscriptionService == nil {
		writeError(w, http.StatusServiceUnavailable, "subscription service is required")
		return
	}
	tenantID := requestTenantID(r)
	switch r.Method {
	case http.MethodGet:
		items, err := s.subscriptionService.ListSubscriptions(tenantID)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, items)
	case http.MethodPost:
		var req createSubscriptionRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		result, err := s.subscriptionService.CreateSubscription(r.Context(), service.CreateSubscriptionRequest{
			TenantID:            tenantID,
			Code:                req.Code,
			DisplayName:         req.DisplayName,
			CustomerExternalID:  req.CustomerExternalID,
			PlanID:              req.PlanID,
			StartedAt:           req.StartedAt,
			RequestPaymentSetup: req.RequestPaymentSetup,
			PaymentMethodType:   req.PaymentMethodType,
		})
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, result)
	default:
		writeMethodNotAllowed(w)
	}
}

func (s *Server) handleSubscriptionByID(w http.ResponseWriter, r *http.Request) {
	if s.subscriptionService == nil {
		writeError(w, http.StatusServiceUnavailable, "subscription service is required")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1/subscriptions/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	id := strings.TrimSpace(parts[0])
	action := ""
	if len(parts) > 1 {
		action = strings.Join(parts[1:], "/")
	}

	if action == "payment-setup/request" || action == "payment-setup/resend" {
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w)
			return
		}
		var req subscriptionPaymentSetupRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		var (
			result service.SubscriptionPaymentSetupResult
			err    error
		)
		if action == "payment-setup/resend" {
			result, err = s.subscriptionService.ResendPaymentSetup(requestTenantID(r), id, req.PaymentMethodType)
		} else {
			result, err = s.subscriptionService.RequestPaymentSetup(requestTenantID(r), id, req.PaymentMethodType)
		}
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
		return
	}

	switch r.Method {
	case http.MethodGet:
		detail, err := s.subscriptionService.GetSubscription(requestTenantID(r), id)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, detail)
	case http.MethodPatch:
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
		detail, err := s.subscriptionService.UpdateSubscription(r.Context(), requestTenantID(r), id, service.UpdateSubscriptionRequest{
			DisplayName: req.DisplayName,
			PlanID:      req.PlanID,
			Status:      status,
			StartedAt:   req.StartedAt,
		})
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, detail)
	default:
		writeMethodNotAllowed(w)
	}
}
