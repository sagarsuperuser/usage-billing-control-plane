package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"usage-billing-control-plane/internal/service"
)

func (s *Server) handlePaymentReceiptByID(w http.ResponseWriter, r *http.Request) {
	s.handleBillingDocumentNotificationByID(w, r, "/v1/payment-receipts/", "payment receipt", func(id string, req resendInvoiceEmailRequest) (service.NotificationDispatchResult, error) {
		return s.notificationService.ResendPaymentReceiptEmail(r.Context(), id, service.BillingDocumentEmail{
			To:  req.To,
			Cc:  req.Cc,
			Bcc: req.Bcc,
		})
	})
}

func (s *Server) handleCreditNoteByID(w http.ResponseWriter, r *http.Request) {
	s.handleBillingDocumentNotificationByID(w, r, "/v1/credit-notes/", "credit note", func(id string, req resendInvoiceEmailRequest) (service.NotificationDispatchResult, error) {
		return s.notificationService.ResendCreditNoteEmail(r.Context(), id, service.BillingDocumentEmail{
			To:  req.To,
			Cc:  req.Cc,
			Bcc: req.Bcc,
		})
	})
}

func (s *Server) handleBillingDocumentNotificationByID(
	w http.ResponseWriter,
	r *http.Request,
	prefix string,
	resourceLabel string,
	dispatch func(id string, req resendInvoiceEmailRequest) (service.NotificationDispatchResult, error),
) {
	tail := strings.TrimPrefix(r.URL.Path, prefix)
	parts := strings.Split(strings.Trim(tail, "/"), "/")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || !strings.EqualFold(strings.TrimSpace(parts[1]), "resend-email") {
		writeError(w, http.StatusBadRequest, "unsupported "+resourceLabel+" subresource")
		return
	}
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if s.notificationService == nil {
		writeError(w, http.StatusServiceUnavailable, "billing notification delivery is not configured")
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

	dispatched, err := dispatch(strings.TrimSpace(parts[0]), req)
	if err != nil {
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

	writeJSON(w, http.StatusAccepted, billingNotificationDispatchResponse{
		DispatchedAt: dispatched.DispatchedAt,
		Dispatched:   true,
		Action:       dispatched.Action,
		Domain:       dispatched.Domain,
		Backend:      dispatched.Backend,
	})
}
