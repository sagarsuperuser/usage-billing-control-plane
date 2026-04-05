package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"usage-billing-control-plane/internal/service"
)

type billingNotificationDispatchResponse struct {
	DispatchedAt time.Time `json:"dispatched_at"`
	Dispatched   bool      `json:"dispatched"`
	Action       string    `json:"action"`
	Domain       string    `json:"domain"`
	Backend      string    `json:"backend"`
}

type resendInvoiceEmailRequest struct {
	To  []string `json:"to"`
	Cc  []string `json:"cc"`
	Bcc []string `json:"bcc"`
}

func (s *Server) handleInvoiceResendEmail(w http.ResponseWriter, r *http.Request, invoiceID string) {
	if s.notificationService == nil || !s.notificationService.CanResendInvoiceEmail() {
		writeError(w, http.StatusServiceUnavailable, "invoice notification delivery is not configured")
		return
	}

	var req resendInvoiceEmailRequest
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(strings.TrimSpace(string(rawBody))) > 0 {
		if err := json.Unmarshal(rawBody, &req); err != nil {
			writeError(w, http.StatusBadRequest, "request body must be valid json")
			return
		}
	}

	ctx := service.ContextWithBillingTenant(r.Context(), requestTenantID(r))
	dispatched, err := s.notificationService.ResendInvoiceEmail(ctx, invoiceID, service.BillingDocumentEmail{
		To:  req.To,
		Cc:  req.Cc,
		Bcc: req.Bcc,
	})
	if err != nil {
		s.logBillingNotificationDispatch(r, "invoice", invoiceID, req, service.NotificationDispatchResult{}, err)
		var dispatchErr *service.NotificationDispatchError
		if errors.As(err, &dispatchErr) {
			status := dispatchErr.StatusCode
			if status <= 0 {
				status = http.StatusBadGateway
			}
			writeError(w, status, dispatchErr.Message)
			return
		}
		writeDomainError(w, err)
		return
	}

	s.logBillingNotificationDispatch(r, "invoice", invoiceID, req, dispatched, nil)
	writeJSON(w, http.StatusAccepted, billingNotificationDispatchResponse{
		DispatchedAt: dispatched.DispatchedAt,
		Dispatched:   true,
		Action:       dispatched.Action,
		Domain:       dispatched.Domain,
		Backend:      dispatched.Backend,
	})
}

func (s *Server) resendPaymentReceiptEmail(w http.ResponseWriter, r *http.Request) {
	if s.notificationService == nil {
		writeError(w, http.StatusServiceUnavailable, "billing notification delivery is not configured")
		return
	}
	id := urlParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	s.dispatchBillingDocumentEmail(w, r, id, "payment_receipt", "payment receipt", func(docID string, req resendInvoiceEmailRequest) (service.NotificationDispatchResult, error) {
		return s.notificationService.ResendPaymentReceiptEmail(service.ContextWithBillingTenant(r.Context(), requestTenantID(r)), docID, service.BillingDocumentEmail{
			To:  req.To,
			Cc:  req.Cc,
			Bcc: req.Bcc,
		})
	})
}

func (s *Server) resendCreditNoteEmail(w http.ResponseWriter, r *http.Request) {
	if s.notificationService == nil {
		writeError(w, http.StatusServiceUnavailable, "billing notification delivery is not configured")
		return
	}
	id := urlParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	s.dispatchBillingDocumentEmail(w, r, id, "credit_note", "credit note", func(docID string, req resendInvoiceEmailRequest) (service.NotificationDispatchResult, error) {
		return s.notificationService.ResendCreditNoteEmail(service.ContextWithBillingTenant(r.Context(), requestTenantID(r)), docID, service.BillingDocumentEmail{
			To:  req.To,
			Cc:  req.Cc,
			Bcc: req.Bcc,
		})
	})
}

func (s *Server) getPaymentReceipt(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "payment receipt retrieval not yet implemented")
}

func (s *Server) getCreditNote(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "credit note retrieval not yet implemented")
}

func (s *Server) dispatchBillingDocumentEmail(
	w http.ResponseWriter,
	r *http.Request,
	resourceID string,
	resourceType string,
	resourceLabel string,
	dispatch func(id string, req resendInvoiceEmailRequest) (service.NotificationDispatchResult, error),
) {
	var req resendInvoiceEmailRequest
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(strings.TrimSpace(string(rawBody))) > 0 {
		if err := json.Unmarshal(rawBody, &req); err != nil {
			writeError(w, http.StatusBadRequest, "request body must be valid json")
			return
		}
	}

	dispatched, err := dispatch(resourceID, req)
	if err != nil {
		s.logBillingNotificationDispatch(r, resourceType, resourceID, req, service.NotificationDispatchResult{}, err)
		var dispatchErr *service.NotificationDispatchError
		if errors.As(err, &dispatchErr) {
			status := dispatchErr.StatusCode
			if status <= 0 {
				status = http.StatusBadGateway
			}
			writeError(w, status, dispatchErr.Message)
			return
		}
		writeDomainError(w, err)
		return
	}
	s.logBillingNotificationDispatch(r, resourceType, resourceID, req, dispatched, nil)

	writeJSON(w, http.StatusAccepted, billingNotificationDispatchResponse{
		DispatchedAt: dispatched.DispatchedAt,
		Dispatched:   true,
		Action:       dispatched.Action,
		Domain:       dispatched.Domain,
		Backend:      dispatched.Backend,
	})
}

func (s *Server) logBillingNotificationDispatch(
	r *http.Request,
	resourceType string,
	resourceID string,
	req resendInvoiceEmailRequest,
	dispatched service.NotificationDispatchResult,
	err error,
) {
	if s == nil {
		return
	}

	attrs := []any{
		"component", "api",
		"request_id", requestIDFromContext(r.Context()),
		"tenant_id", requestTenantID(r),
		"path", r.URL.Path,
		"method", r.Method,
		"resource_type", strings.TrimSpace(resourceType),
		"resource_id", strings.TrimSpace(resourceID),
		"to_count", len(req.To),
		"cc_count", len(req.Cc),
		"bcc_count", len(req.Bcc),
	}
	if dispatched.Action != "" {
		attrs = append(attrs,
			"action", dispatched.Action,
			"domain", dispatched.Domain,
			"backend", dispatched.Backend,
			"dispatched_at", dispatched.DispatchedAt,
		)
	}

	if err != nil {
		attrs = append(attrs, "event", "billing_notification_dispatch_failed", "error", err.Error())
		var dispatchErr *service.NotificationDispatchError
		if errors.As(err, &dispatchErr) {
			if dispatchErr.StatusCode > 0 {
				attrs = append(attrs, "backend_status", dispatchErr.StatusCode)
			}
			if strings.TrimSpace(dispatchErr.Backend) != "" {
				attrs = append(attrs, "backend", dispatchErr.Backend)
			}
		}
		s.logError("billing notification dispatch failed", attrs...)
		return
	}

	attrs = append(attrs, "event", "billing_notification_dispatched")
	s.logInfo("billing notification dispatched", attrs...)
}
