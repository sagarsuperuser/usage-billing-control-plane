package api

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/service"
	"usage-billing-control-plane/internal/store"
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
	Dunning   *domain.DunningSummary          `json:"dunning,omitempty"`
}

func (s *Server) listPayments(w http.ResponseWriter, r *http.Request) {
	if s.paymentStatusSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "payment status service is required")
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

	items, err := s.paymentStatusSvc.ListInvoicePaymentStatusViews(
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

func (s *Server) getPayment(w http.ResponseWriter, r *http.Request) {
	if s.paymentStatusSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "payment status service is required")
		return
	}

	invoiceID := urlParam(r, "id")
	if invoiceID == "" {
		writeError(w, http.StatusBadRequest, "payment id is required")
		return
	}

	view, err := s.paymentStatusSvc.GetInvoicePaymentStatusView(requestTenantID(r), invoiceID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	lifecycle, err := s.paymentStatusSvc.GetInvoicePaymentLifecycle(requestTenantID(r), invoiceID, 50)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	lifecycle, err = s.enrichPaymentLifecycleWithCustomerReadiness(requestTenantID(r), view.CustomerExternalID, lifecycle)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	customer, err := s.lookupInvoiceCustomer(requestTenantID(r), view.CustomerExternalID, map[string]*domain.Customer{})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	dunning, err := s.lookupInvoiceDunningSummary(requestTenantID(r), invoiceID)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, paymentDetailFromStatusView(view, customer, lifecycle, dunning))
}

func (s *Server) retryPayment(w http.ResponseWriter, r *http.Request) {
	if s.invoiceBillingAdapter == nil {
		writeError(w, http.StatusServiceUnavailable, "invoice billing adapter is required")
		return
	}

	invoiceID := urlParam(r, "id")
	if invoiceID == "" {
		writeError(w, http.StatusBadRequest, "payment id is required")
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
	ctx := service.ContextWithBillingTenant(r.Context(), requestTenantID(r))
	statusCode, body, err := s.invoiceBillingAdapter.RetryInvoicePayment(ctx, invoiceID, rawBody)
	if err != nil {
		s.writeInternalError(w, r, http.StatusBadGateway, "payment retry failed", err)
		return
	}
	if statusCode >= 200 && statusCode < 300 {
		if syncErr := s.materializeRetryPaymentProjection(r.Context(), requestTenantID(r), invoiceID); syncErr != nil {
			s.logWarn("materialize retry payment projection failed", "invoice_id", invoiceID, "tenant_id", requestTenantID(r), "error", syncErr)
		}
	}
	if statusCode < 200 || statusCode >= 300 {
		writeTranslatedUpstreamError(w, statusCode, "Payment retry could not be started right now.", body)
		return
	}
	writeJSONRaw(w, statusCode, body)
}

func (s *Server) listPaymentEvents(w http.ResponseWriter, r *http.Request) {
	if s.paymentStatusSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "payment status service is required")
		return
	}

	invoiceID := urlParam(r, "id")
	if invoiceID == "" {
		writeError(w, http.StatusBadRequest, "payment id is required")
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
	events, err := s.paymentStatusSvc.ListBillingEvents(
		requestTenantID(r),
		service.ListBillingEventsRequest{
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

func paymentDetailFromStatusView(view domain.InvoicePaymentStatusView, customer *domain.Customer, lifecycle service.InvoicePaymentLifecycle, dunning *domain.DunningSummary) paymentDetailResponse {
	return paymentDetailResponse{
		paymentSummaryResponse: paymentSummaryFromStatusView(view, customer),
		Lifecycle:              lifecycle,
		Dunning:                dunning,
	}
}

func (s *Server) materializeRetryPaymentProjection(ctx context.Context, tenantID, invoiceID string) error {
	if s == nil || s.repo == nil || s.invoiceBillingAdapter == nil {
		return nil
	}
	ctx = service.ContextWithBillingTenant(ctx, tenantID)
	statusCode, body, err := s.invoiceBillingAdapter.GetInvoice(ctx, strings.TrimSpace(invoiceID))
	if err != nil {
		return err
	}
	if statusCode < 200 || statusCode >= 300 {
		return fmt.Errorf("get invoice after retry returned status %d", statusCode)
	}
	invoicePayload, err := extractInvoicePayload(body)
	if err != nil {
		return err
	}
	view := buildInvoicePaymentStatusViewFromInvoicePayload(tenantID, invoicePayload, "invoice.payment_status_observed", time.Now().UTC())
	if strings.TrimSpace(view.InvoiceID) == "" {
		return fmt.Errorf("invoice payload missing invoice id")
	}
	if _, err := s.repo.UpsertInvoicePaymentStatusView(view); err != nil {
		return err
	}
	if s.dunningService != nil {
		if _, err := s.dunningService.EnsureRunForInvoice(strings.TrimSpace(tenantID), view.InvoiceID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) lookupInvoiceDunningSummary(tenantID, invoiceID string) (*domain.DunningSummary, error) {
	if s == nil || s.dunningService == nil {
		return nil, nil
	}
	return s.dunningService.GetInvoiceSummary(tenantID, invoiceID)
}

func (s *Server) enrichPaymentLifecycleWithCustomerReadiness(tenantID, customerExternalID string, lifecycle service.InvoicePaymentLifecycle) (service.InvoicePaymentLifecycle, error) {
	customerExternalID = strings.TrimSpace(customerExternalID)
	if s == nil || s.customerService == nil || customerExternalID == "" {
		return lifecycle, nil
	}
	readiness, err := s.customerService.GetCustomerReadiness(tenantID, customerExternalID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return lifecycle, nil
		}
		return lifecycle, err
	}
	return applyCustomerReadinessToPaymentLifecycle(lifecycle, readiness), nil
}

func applyCustomerReadinessToPaymentLifecycle(lifecycle service.InvoicePaymentLifecycle, readiness service.CustomerReadiness) service.InvoicePaymentLifecycle {
	if readiness.PaymentSetupStatus == domain.PaymentSetupStatusReady && readiness.DefaultPaymentMethodVerified {
		return lifecycle
	}

	shouldCollect := false
	switch strings.ToLower(strings.TrimSpace(lifecycle.PaymentStatus)) {
	case "failed", "pending":
		shouldCollect = true
	default:
		shouldCollect = lifecycle.PaymentOverdue != nil && *lifecycle.PaymentOverdue
	}
	if !shouldCollect {
		return lifecycle
	}

	lifecycle.RequiresAction = true
	lifecycle.RetryRecommended = false
	lifecycle.RecommendedAction = "collect_payment"
	lifecycle.RecommendedActionNote = "Customer payment setup is not ready. Send or refresh payment setup before retrying collection."
	return lifecycle
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
