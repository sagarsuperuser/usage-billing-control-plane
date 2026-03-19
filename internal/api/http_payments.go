package api

import (
	"encoding/csv"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/service"
)

type paymentSummaryResponse struct {
	InvoiceID            string     `json:"invoice_id"`
	InvoiceNumber        string     `json:"invoice_number,omitempty"`
	CustomerExternalID   string     `json:"customer_external_id,omitempty"`
	CustomerDisplayName  string     `json:"customer_display_name,omitempty"`
	OrganizationID       string     `json:"organization_id,omitempty"`
	Currency             string     `json:"currency,omitempty"`
	InvoiceStatus        string     `json:"invoice_status,omitempty"`
	PaymentStatus        string     `json:"payment_status,omitempty"`
	PaymentOverdue       *bool      `json:"payment_overdue,omitempty"`
	TotalAmountCents     *int64     `json:"total_amount_cents,omitempty"`
	TotalDueAmountCents  *int64     `json:"total_due_amount_cents,omitempty"`
	TotalPaidAmountCents *int64     `json:"total_paid_amount_cents,omitempty"`
	LastPaymentError     string     `json:"last_payment_error,omitempty"`
	LastEventType        string     `json:"last_event_type,omitempty"`
	LastEventAt          *time.Time `json:"last_event_at,omitempty"`
	UpdatedAt            *time.Time `json:"updated_at,omitempty"`
}

type paymentDetailResponse struct {
	paymentSummaryResponse
	Lifecycle service.InvoicePaymentLifecycle `json:"lifecycle"`
}

func (s *Server) handlePayments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if s.lagoWebhookSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "lago webhook service is required")
		return
	}

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
	paymentOverdue, err := parseOptionalQueryBool(r, "payment_overdue")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	customerExternalID := strings.TrimSpace(r.URL.Query().Get("customer_external_id"))
	if customerExternalID == "" {
		customerExternalID = strings.TrimSpace(r.URL.Query().Get("customer_id"))
	}
	invoiceID := strings.TrimSpace(r.URL.Query().Get("invoice_id"))
	invoiceNumber := strings.TrimSpace(r.URL.Query().Get("invoice_number"))
	lastEventType := strings.TrimSpace(r.URL.Query().Get("last_event_type"))

	items, err := s.lagoWebhookSvc.ListInvoicePaymentStatusViews(
		requestTenantID(r),
		service.ListInvoicePaymentStatusViewsRequest{
			OrganizationID:     r.URL.Query().Get("organization_id"),
			CustomerExternalID: customerExternalID,
			InvoiceID:          invoiceID,
			InvoiceNumber:      invoiceNumber,
			LastEventType:      lastEventType,
			PaymentStatus:      r.URL.Query().Get("payment_status"),
			InvoiceStatus:      r.URL.Query().Get("invoice_status"),
			PaymentOverdue:     paymentOverdue,
			SortBy:             r.URL.Query().Get("sort_by"),
			Order:              r.URL.Query().Get("order"),
			Limit:              limit,
			Offset:             offset,
		},
	)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	payments, err := s.buildPaymentSummaries(requestTenantID(r), items)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	if strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("format")), "csv") {
		csvData, err := generatePaymentsCSV(payments)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to generate payments csv")
			return
		}
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename=payments.csv")
		_, _ = w.Write([]byte(csvData))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":  payments,
		"limit":  limit,
		"offset": offset,
		"filters": map[string]any{
			"organization_id":      r.URL.Query().Get("organization_id"),
			"customer_external_id": customerExternalID,
			"invoice_id":           invoiceID,
			"invoice_number":       invoiceNumber,
			"last_event_type":      lastEventType,
			"payment_status":       r.URL.Query().Get("payment_status"),
			"invoice_status":       r.URL.Query().Get("invoice_status"),
			"payment_overdue":      paymentOverdue,
			"sort_by":              r.URL.Query().Get("sort_by"),
			"order":                r.URL.Query().Get("order"),
		},
	})
}

func (s *Server) handlePaymentByID(w http.ResponseWriter, r *http.Request) {
	tail := strings.TrimPrefix(r.URL.Path, "/v1/payments/")
	parts := strings.Split(strings.Trim(tail, "/"), "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		writeError(w, http.StatusBadRequest, "payment id is required")
		return
	}

	invoiceID := strings.TrimSpace(parts[0])
	if len(parts) == 2 && strings.EqualFold(strings.TrimSpace(parts[1]), "retry") {
		if s.invoiceBillingAdapter == nil {
			writeError(w, http.StatusServiceUnavailable, "invoice billing adapter is required")
			return
		}
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		rawBody, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if len(strings.TrimSpace(string(rawBody))) == 0 {
			rawBody = []byte("{}")
		}
		statusCode, body, err := s.invoiceBillingAdapter.RetryInvoicePayment(r.Context(), invoiceID, rawBody)
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to proxy payment retry to lago: "+err.Error())
			return
		}
		writeJSONRaw(w, statusCode, body)
		return
	}

	if s.lagoWebhookSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "lago webhook service is required")
		return
	}

	if len(parts) == 2 && strings.EqualFold(strings.TrimSpace(parts[1]), "events") {
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w)
			return
		}
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
		events, err := s.lagoWebhookSvc.ListLagoWebhookEvents(
			requestTenantID(r),
			service.ListLagoWebhookEventsRequest{
				OrganizationID: r.URL.Query().Get("organization_id"),
				InvoiceID:      invoiceID,
				WebhookType:    r.URL.Query().Get("webhook_type"),
				SortBy:         r.URL.Query().Get("sort_by"),
				Order:          r.URL.Query().Get("order"),
				Limit:          limit,
				Offset:         offset,
			},
		)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"items":      events,
			"limit":      limit,
			"offset":     offset,
			"invoice_id": invoiceID,
		})
		return
	}

	if len(parts) != 1 || r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	view, err := s.lagoWebhookSvc.GetInvoicePaymentStatusView(requestTenantID(r), invoiceID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	lifecycle, err := s.lagoWebhookSvc.GetInvoicePaymentLifecycle(requestTenantID(r), invoiceID, 50)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	customer, err := s.lookupInvoiceCustomer(requestTenantID(r), view.CustomerExternalID, map[string]*domain.Customer{})
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, paymentDetailFromStatusView(view, customer, lifecycle))
}

func (s *Server) buildPaymentSummaries(tenantID string, items []domain.InvoicePaymentStatusView) ([]paymentSummaryResponse, error) {
	cache := make(map[string]*domain.Customer)
	out := make([]paymentSummaryResponse, 0, len(items))
	for _, item := range items {
		customer, err := s.lookupInvoiceCustomer(tenantID, item.CustomerExternalID, cache)
		if err != nil {
			return nil, err
		}
		out = append(out, paymentSummaryFromStatusView(item, customer))
	}
	return out, nil
}

func paymentSummaryFromStatusView(view domain.InvoicePaymentStatusView, customer *domain.Customer) paymentSummaryResponse {
	out := paymentSummaryResponse{
		InvoiceID:            strings.TrimSpace(view.InvoiceID),
		InvoiceNumber:        strings.TrimSpace(view.InvoiceNumber),
		CustomerExternalID:   strings.TrimSpace(view.CustomerExternalID),
		OrganizationID:       strings.TrimSpace(view.OrganizationID),
		Currency:             strings.TrimSpace(view.Currency),
		InvoiceStatus:        strings.TrimSpace(view.InvoiceStatus),
		PaymentStatus:        strings.TrimSpace(view.PaymentStatus),
		PaymentOverdue:       view.PaymentOverdue,
		TotalAmountCents:     view.TotalAmountCents,
		TotalDueAmountCents:  view.TotalDueAmountCents,
		TotalPaidAmountCents: view.TotalPaidAmountCents,
		LastPaymentError:     strings.TrimSpace(view.LastPaymentError),
		LastEventType:        strings.TrimSpace(view.LastEventType),
		LastEventAt:          timePtr(view.LastEventAt),
		UpdatedAt:            timePtr(view.UpdatedAt),
	}
	if customer != nil {
		out.CustomerDisplayName = strings.TrimSpace(customer.DisplayName)
	}
	return out
}

func paymentDetailFromStatusView(view domain.InvoicePaymentStatusView, customer *domain.Customer, lifecycle service.InvoicePaymentLifecycle) paymentDetailResponse {
	return paymentDetailResponse{
		paymentSummaryResponse: paymentSummaryFromStatusView(view, customer),
		Lifecycle:              lifecycle,
	}
}

func generatePaymentsCSV(items []paymentSummaryResponse) (string, error) {
	var b strings.Builder
	writer := csv.NewWriter(&b)
	if err := writer.Write([]string{
		"invoice_id",
		"invoice_number",
		"customer_external_id",
		"customer_display_name",
		"organization_id",
		"currency",
		"invoice_status",
		"payment_status",
		"payment_overdue",
		"total_amount_cents",
		"total_due_amount_cents",
		"total_paid_amount_cents",
		"last_payment_error",
		"last_event_type",
		"last_event_at",
		"updated_at",
	}); err != nil {
		return "", err
	}

	for _, item := range items {
		if err := writer.Write([]string{
			item.InvoiceID,
			item.InvoiceNumber,
			item.CustomerExternalID,
			item.CustomerDisplayName,
			item.OrganizationID,
			item.Currency,
			item.InvoiceStatus,
			item.PaymentStatus,
			formatCSVBool(item.PaymentOverdue),
			formatCSVInt64(item.TotalAmountCents),
			formatCSVInt64(item.TotalDueAmountCents),
			formatCSVInt64(item.TotalPaidAmountCents),
			item.LastPaymentError,
			item.LastEventType,
			formatCSVTime(item.LastEventAt),
			formatCSVTime(item.UpdatedAt),
		}); err != nil {
			return "", err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}
	return b.String(), nil
}

func formatCSVBool(v *bool) string {
	if v == nil {
		return ""
	}
	return strconv.FormatBool(*v)
}

func formatCSVInt64(v *int64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatInt(*v, 10)
}

func formatCSVTime(v *time.Time) string {
	if v == nil || v.IsZero() {
		return ""
	}
	return v.UTC().Format(time.RFC3339)
}
