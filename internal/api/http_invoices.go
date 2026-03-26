package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/service"
	"usage-billing-control-plane/internal/store"
)

type invoiceDetailResponse struct {
	domain.InvoiceDetail
	Lifecycle *service.InvoicePaymentLifecycle `json:"lifecycle,omitempty"`
}

func (s *Server) handleInvoices(w http.ResponseWriter, r *http.Request) {
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

	items, err := s.lagoWebhookSvc.ListInvoicePaymentStatusViews(
		requestTenantID(r),
		service.ListInvoicePaymentStatusViewsRequest{
			OrganizationID:     r.URL.Query().Get("organization_id"),
			CustomerExternalID: customerExternalID,
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

	summaries, err := s.buildInvoiceSummaries(requestTenantID(r), items)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":  summaries,
		"limit":  limit,
		"offset": offset,
		"filters": map[string]any{
			"organization_id":      r.URL.Query().Get("organization_id"),
			"customer_external_id": customerExternalID,
			"payment_status":       r.URL.Query().Get("payment_status"),
			"invoice_status":       r.URL.Query().Get("invoice_status"),
			"payment_overdue":      paymentOverdue,
			"sort_by":              r.URL.Query().Get("sort_by"),
			"order":                r.URL.Query().Get("order"),
		},
	})
}

func (s *Server) buildInvoiceSummaries(tenantID string, items []domain.InvoicePaymentStatusView) ([]domain.InvoiceSummary, error) {
	cache := make(map[string]*domain.Customer)
	out := make([]domain.InvoiceSummary, 0, len(items))

	for _, item := range items {
		customer, err := s.lookupInvoiceCustomer(tenantID, item.CustomerExternalID, cache)
		if err != nil {
			return nil, err
		}
		out = append(out, invoiceSummaryFromStatusView(item, customer))
	}

	return out, nil
}

func (s *Server) loadInvoiceDetail(ctx context.Context, tenantID, invoiceID string) (int, []byte, invoiceDetailResponse, error) {
	if s.invoiceBillingAdapter == nil {
		return 0, nil, invoiceDetailResponse{}, fmt.Errorf("%w: invoice billing adapter is required", service.ErrValidation)
	}

	ctx = service.ContextWithLagoTenant(ctx, tenantID)
	statusCode, body, err := s.invoiceBillingAdapter.GetInvoice(ctx, invoiceID)
	if err != nil {
		return 0, nil, invoiceDetailResponse{}, err
	}
	if statusCode < 200 || statusCode >= 300 {
		return statusCode, body, invoiceDetailResponse{}, nil
	}

	invoicePayload, err := extractInvoicePayload(body)
	if err != nil {
		return 0, nil, invoiceDetailResponse{}, err
	}

	var (
		view      *domain.InvoicePaymentStatusView
		customer  *domain.Customer
		lifecycle *service.InvoicePaymentLifecycle
	)
	if s.lagoWebhookSvc != nil {
		item, viewErr := s.lagoWebhookSvc.GetInvoicePaymentStatusView(tenantID, invoiceID)
		if viewErr != nil && !errors.Is(viewErr, store.ErrNotFound) {
			return 0, nil, invoiceDetailResponse{}, viewErr
		}
		if viewErr == nil {
			view = &item
		} else if errors.Is(viewErr, store.ErrNotFound) {
			synthetic := buildInvoicePaymentStatusViewFromInvoicePayload(tenantID, invoicePayload, "invoice.payment_status_observed", time.Now().UTC())
			view = &synthetic
		}
		if view != nil {
			if viewErr == nil {
				itemLifecycle, lifecycleErr := s.lagoWebhookSvc.GetInvoicePaymentLifecycle(tenantID, invoiceID, 50)
				if lifecycleErr != nil {
					return 0, nil, invoiceDetailResponse{}, lifecycleErr
				}
				lifecycle = &itemLifecycle
			} else {
				fallback := service.BuildInvoicePaymentLifecycleFromView(*view, nil, 0)
				lifecycle = &fallback
			}
		}
	}

	customerExternalID := invoiceCustomerExternalID(invoicePayload)
	if customerExternalID == "" && view != nil {
		customerExternalID = strings.TrimSpace(view.CustomerExternalID)
	}
	if customerExternalID != "" {
		customer, err = s.lookupInvoiceCustomer(tenantID, customerExternalID, map[string]*domain.Customer{})
		if err != nil {
			return 0, nil, invoiceDetailResponse{}, err
		}
	}
	if lifecycle != nil {
		enriched, lifecycleErr := s.enrichPaymentLifecycleWithCustomerReadiness(tenantID, customerExternalID, *lifecycle)
		if lifecycleErr != nil {
			return 0, nil, invoiceDetailResponse{}, lifecycleErr
		}
		lifecycle = &enriched
	}

	detail := buildInvoiceDetail(invoicePayload, view, customer)
	if s.dunningService != nil {
		dunning, err := s.dunningService.GetInvoiceSummary(tenantID, invoiceID)
		if err != nil {
			return 0, nil, invoiceDetailResponse{}, err
		}
		detail.Dunning = dunning
	}
	return statusCode, body, invoiceDetailResponse{
		InvoiceDetail: detail,
		Lifecycle:     lifecycle,
	}, nil
}

func (s *Server) handleInvoicePaymentReceipts(w http.ResponseWriter, r *http.Request, invoiceID string) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if s.invoiceBillingAdapter == nil {
		writeError(w, http.StatusServiceUnavailable, "invoice billing adapter is required")
		return
	}

	ctx := service.ContextWithLagoTenant(r.Context(), requestTenantID(r))
	statusCode, body, err := s.invoiceBillingAdapter.ListPaymentReceipts(ctx, url.Values{
		"invoice_id": []string{invoiceID},
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to load payment receipts from lago: "+err.Error())
		return
	}
	if statusCode < 200 || statusCode >= 300 {
		writeTranslatedUpstreamError(w, statusCode, "Payment receipts could not be loaded right now.", body)
		return
	}

	items, err := extractCollectionPayload(body, "payment_receipts")
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": buildPaymentReceiptSummaries(items),
	})
}

func (s *Server) handleInvoiceCreditNotes(w http.ResponseWriter, r *http.Request, invoiceID string) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if s.invoiceBillingAdapter == nil {
		writeError(w, http.StatusServiceUnavailable, "invoice billing adapter is required")
		return
	}

	tenantID := requestTenantID(r)
	ctx := service.ContextWithLagoTenant(r.Context(), tenantID)
	customerExternalID, statusCode, body, err := s.loadInvoiceCustomerExternalID(ctx, invoiceID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if statusCode < 200 || statusCode >= 300 {
		writeTranslatedUpstreamError(w, statusCode, "Invoice details could not be loaded right now.", body)
		return
	}
	if customerExternalID == "" {
		writeJSON(w, http.StatusOK, map[string]any{"items": []domain.CreditNoteSummary{}})
		return
	}

	statusCode, body, err = s.invoiceBillingAdapter.ListCreditNotes(ctx, url.Values{
		"external_customer_id": []string{customerExternalID},
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to load credit notes from lago: "+err.Error())
		return
	}
	if statusCode < 200 || statusCode >= 300 {
		writeTranslatedUpstreamError(w, statusCode, "Credit notes could not be loaded right now.", body)
		return
	}

	items, err := extractCollectionPayload(body, "credit_notes")
	if err != nil {
		writeDomainError(w, err)
		return
	}

	summaries := buildCreditNoteSummaries(items)
	filtered := make([]domain.CreditNoteSummary, 0, len(summaries))
	for _, item := range summaries {
		if strings.TrimSpace(item.InvoiceID) == invoiceID {
			filtered = append(filtered, item)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": filtered,
	})
}

func (s *Server) loadInvoiceCustomerExternalID(ctx context.Context, invoiceID string) (string, int, []byte, error) {
	statusCode, body, err := s.invoiceBillingAdapter.GetInvoice(ctx, invoiceID)
	if err != nil {
		return "", 0, nil, err
	}
	if statusCode < 200 || statusCode >= 300 {
		return "", statusCode, body, nil
	}

	invoicePayload, err := extractInvoicePayload(body)
	if err != nil {
		return "", 0, nil, err
	}
	return invoiceCustomerExternalID(invoicePayload), statusCode, body, nil
}

func (s *Server) lookupInvoiceCustomer(tenantID, externalID string, cache map[string]*domain.Customer) (*domain.Customer, error) {
	externalID = strings.TrimSpace(externalID)
	if externalID == "" {
		return nil, nil
	}
	if cached, ok := cache[externalID]; ok {
		return cached, nil
	}
	customer, err := s.repo.GetCustomerByExternalID(tenantID, externalID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			cache[externalID] = nil
			return nil, nil
		}
		return nil, err
	}
	copied := customer
	cache[externalID] = &copied
	return &copied, nil
}

func extractInvoicePayload(body []byte) (map[string]any, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("%w: invalid invoice payload", service.ErrValidation)
	}
	invoice, ok := payload["invoice"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: invoice payload missing invoice object", service.ErrValidation)
	}
	return invoice, nil
}

func extractCollectionPayload(body []byte, key string) ([]map[string]any, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("%w: invalid collection payload", service.ErrValidation)
	}
	rawItems, ok := payload[key].([]any)
	if !ok {
		return nil, fmt.Errorf("%w: collection payload missing %s", service.ErrValidation, key)
	}
	items := make([]map[string]any, 0, len(rawItems))
	for _, raw := range rawItems {
		row, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%w: collection payload contains invalid %s item", service.ErrValidation, key)
		}
		items = append(items, row)
	}
	return items, nil
}

func buildInvoicePaymentStatusViewFromInvoicePayload(tenantID string, invoice map[string]any, lastEventType string, observedAt time.Time) domain.InvoicePaymentStatusView {
	eventAt := observedAt.UTC()
	if eventAt.IsZero() {
		eventAt = time.Now().UTC()
	}
	if updatedAt := firstTimePtr(timeValue(invoice["updated_at"]), timeValue(invoice["created_at"])); updatedAt != nil && updatedAt.After(eventAt) {
		eventAt = updatedAt.UTC()
	}
	return domain.InvoicePaymentStatusView{
		TenantID:             strings.TrimSpace(tenantID),
		OrganizationID:       firstNonEmpty(stringValue(invoice["billing_entity_code"]), stringValue(invoice["organization_id"])),
		InvoiceID:            firstNonEmpty(stringValue(invoice["lago_id"]), stringValue(invoice["id"])),
		CustomerExternalID:   invoiceCustomerExternalID(invoice),
		InvoiceNumber:        stringValue(invoice["number"]),
		Currency:             stringValue(invoice["currency"]),
		InvoiceStatus:        stringValue(invoice["status"]),
		PaymentStatus:        stringValue(invoice["payment_status"]),
		PaymentOverdue:       boolPtr(invoice["payment_overdue"]),
		TotalAmountCents:     int64Ptr(invoice["total_amount_cents"]),
		TotalDueAmountCents:  int64Ptr(invoice["total_due_amount_cents"]),
		TotalPaidAmountCents: int64Ptr(invoice["total_paid_amount_cents"]),
		LastPaymentError:     firstNonEmpty(stringValue(invoice["payment_error"]), stringValue(invoice["last_payment_error"])),
		LastEventType:        strings.TrimSpace(lastEventType),
		LastWebhookKey:       fmt.Sprintf("synthetic:%s:%d", firstNonEmpty(stringValue(invoice["lago_id"]), stringValue(invoice["id"])), eventAt.UnixNano()),
		LastEventAt:          eventAt,
		UpdatedAt:            eventAt,
	}
}

func invoiceSummaryFromStatusView(view domain.InvoicePaymentStatusView, customer *domain.Customer) domain.InvoiceSummary {
	summary := domain.InvoiceSummary{
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
		UpdatedAt:            timePtr(view.UpdatedAt),
		LastEventAt:          timePtr(view.LastEventAt),
	}
	if customer != nil {
		summary.CustomerDisplayName = strings.TrimSpace(customer.DisplayName)
	}
	return summary
}

func buildInvoiceDetail(invoice map[string]any, view *domain.InvoicePaymentStatusView, customer *domain.Customer) domain.InvoiceDetail {
	customerPayload := objectValue(invoice["customer"])
	customerExternalID := strings.TrimSpace(stringValue(customerPayload["external_id"]))
	if customerExternalID == "" && view != nil {
		customerExternalID = strings.TrimSpace(view.CustomerExternalID)
	}

	out := domain.InvoiceDetail{
		InvoiceSummary: domain.InvoiceSummary{
			InvoiceID:            firstNonEmpty(stringValue(invoice["lago_id"]), viewString(view, func(v *domain.InvoicePaymentStatusView) string { return v.InvoiceID })),
			InvoiceNumber:        firstNonEmpty(stringValue(invoice["number"]), viewString(view, func(v *domain.InvoicePaymentStatusView) string { return v.InvoiceNumber })),
			CustomerExternalID:   customerExternalID,
			OrganizationID:       firstNonEmpty(stringValue(invoice["billing_entity_code"]), viewString(view, func(v *domain.InvoicePaymentStatusView) string { return v.OrganizationID })),
			Currency:             firstNonEmpty(stringValue(invoice["currency"]), viewString(view, func(v *domain.InvoicePaymentStatusView) string { return v.Currency })),
			InvoiceStatus:        firstNonEmpty(stringValue(invoice["status"]), viewString(view, func(v *domain.InvoicePaymentStatusView) string { return v.InvoiceStatus })),
			PaymentStatus:        firstNonEmpty(stringValue(invoice["payment_status"]), viewString(view, func(v *domain.InvoicePaymentStatusView) string { return v.PaymentStatus })),
			PaymentOverdue:       firstBoolPtr(boolPtr(invoice["payment_overdue"]), viewBoolPtr(view, func(v *domain.InvoicePaymentStatusView) *bool { return v.PaymentOverdue })),
			TotalAmountCents:     firstInt64Ptr(int64Ptr(invoice["total_amount_cents"]), viewInt64Ptr(view, func(v *domain.InvoicePaymentStatusView) *int64 { return v.TotalAmountCents })),
			TotalDueAmountCents:  firstInt64Ptr(int64Ptr(invoice["total_due_amount_cents"]), viewInt64Ptr(view, func(v *domain.InvoicePaymentStatusView) *int64 { return v.TotalDueAmountCents })),
			TotalPaidAmountCents: firstInt64Ptr(int64Ptr(invoice["total_paid_amount_cents"]), viewInt64Ptr(view, func(v *domain.InvoicePaymentStatusView) *int64 { return v.TotalPaidAmountCents })),
			LastPaymentError:     firstNonEmpty(stringValue(invoice["payment_error"]), stringValue(invoice["last_payment_error"]), viewString(view, func(v *domain.InvoicePaymentStatusView) string { return v.LastPaymentError })),
			LastEventType:        viewString(view, func(v *domain.InvoicePaymentStatusView) string { return v.LastEventType }),
			IssuingDate:          timeValue(invoice["issuing_date"]),
			PaymentDueDate:       timeValue(invoice["payment_due_date"]),
			CreatedAt:            timeValue(invoice["created_at"]),
			UpdatedAt:            firstTimePtr(timeValue(invoice["updated_at"]), viewTimePtr(view, func(v *domain.InvoicePaymentStatusView) time.Time { return v.UpdatedAt })),
			LastEventAt:          viewTimePtr(view, func(v *domain.InvoicePaymentStatusView) time.Time { return v.LastEventAt }),
		},
		LagoID:            stringValue(invoice["lago_id"]),
		BillingEntityCode: stringValue(invoice["billing_entity_code"]),
		SequentialID:      invoice["sequential_id"],
		InvoiceType:       stringValue(invoice["invoice_type"]),
		NetPaymentTerm:    invoice["net_payment_term"],
		FileURL:           stringValue(invoice["file_url"]),
		XMLURL:            stringValue(invoice["xml_url"]),
		VersionNumber:     invoice["version_number"],
		SelfBilled:        boolPtr(invoice["self_billed"]),
		VoidedAt:          timeValue(invoice["voided_at"]),
		Customer:          customerPayload,
		Subscriptions:     sliceValue(invoice["subscriptions"]),
		Fees:              sliceValue(invoice["fees"]),
		Metadata:          sliceValue(invoice["metadata"]),
		AppliedTaxes:      sliceValue(invoice["applied_taxes"]),
	}

	if customer != nil {
		out.CustomerDisplayName = strings.TrimSpace(customer.DisplayName)
	}
	if out.CustomerDisplayName == "" {
		out.CustomerDisplayName = firstNonEmpty(stringValue(customerPayload["name"]), stringValue(customerPayload["display_name"]))
	}

	return out
}

func buildPaymentReceiptSummaries(items []map[string]any) []domain.PaymentReceiptSummary {
	out := make([]domain.PaymentReceiptSummary, 0, len(items))
	for _, item := range items {
		payment := objectValue(item["payment"])
		invoiceIDs := stringSliceValue(payment["invoice_ids"])
		out = append(out, domain.PaymentReceiptSummary{
			ID:            stringValue(item["lago_id"]),
			Number:        stringValue(item["number"]),
			InvoiceID:     firstString(invoiceIDs...),
			PaymentID:     stringValue(payment["lago_id"]),
			PaymentStatus: stringValue(payment["payment_status"]),
			AmountCents:   int64Ptr(payment["amount_cents"]),
			Currency:      firstNonEmpty(stringValue(payment["amount_currency"]), stringValue(payment["currency"])),
			FileURL:       stringValue(item["file_url"]),
			XMLURL:        stringValue(item["xml_url"]),
			CreatedAt:     timeValue(item["created_at"]),
		})
	}
	return out
}

func buildCreditNoteSummaries(items []map[string]any) []domain.CreditNoteSummary {
	out := make([]domain.CreditNoteSummary, 0, len(items))
	for _, item := range items {
		out = append(out, domain.CreditNoteSummary{
			ID:               stringValue(item["lago_id"]),
			Number:           stringValue(item["number"]),
			InvoiceID:        stringValue(item["lago_invoice_id"]),
			InvoiceNumber:    stringValue(item["invoice_number"]),
			CreditStatus:     stringValue(item["credit_status"]),
			RefundStatus:     stringValue(item["refund_status"]),
			Currency:         stringValue(item["currency"]),
			TotalAmountCents: int64Ptr(item["total_amount_cents"]),
			FileURL:          stringValue(item["file_url"]),
			XMLURL:           stringValue(item["xml_url"]),
			IssuingDate:      timeValue(item["issuing_date"]),
			CreatedAt:        timeValue(item["created_at"]),
		})
	}
	return out
}

func objectValue(v any) map[string]any {
	if value, ok := v.(map[string]any); ok {
		return value
	}
	return nil
}

func sliceValue(v any) []any {
	if items, ok := v.([]any); ok {
		return items
	}
	return nil
}

func stringValue(v any) string {
	switch value := v.(type) {
	case string:
		return strings.TrimSpace(value)
	default:
		return ""
	}
}

func stringSliceValue(v any) []string {
	items, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if value := stringValue(item); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func int64Ptr(v any) *int64 {
	switch value := v.(type) {
	case int64:
		return &value
	case int32:
		out := int64(value)
		return &out
	case int:
		out := int64(value)
		return &out
	case float64:
		out := int64(value)
		return &out
	case json.Number:
		if parsed, err := value.Int64(); err == nil {
			return &parsed
		}
	}
	return nil
}

func boolPtr(v any) *bool {
	if value, ok := v.(bool); ok {
		return &value
	}
	return nil
}

func timeValue(v any) *time.Time {
	raw, ok := v.(string)
	if !ok || strings.TrimSpace(raw) == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil
	}
	return &parsed
}

func timePtr(v time.Time) *time.Time {
	if v.IsZero() {
		return nil
	}
	value := v
	return &value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstString(values ...string) string {
	return firstNonEmpty(values...)
}

func firstInt64Ptr(values ...*int64) *int64 {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func firstBoolPtr(values ...*bool) *bool {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func firstTimePtr(values ...*time.Time) *time.Time {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func viewString(view *domain.InvoicePaymentStatusView, getter func(*domain.InvoicePaymentStatusView) string) string {
	if view == nil {
		return ""
	}
	return getter(view)
}

func viewBoolPtr(view *domain.InvoicePaymentStatusView, getter func(*domain.InvoicePaymentStatusView) *bool) *bool {
	if view == nil {
		return nil
	}
	return getter(view)
}

func viewInt64Ptr(view *domain.InvoicePaymentStatusView, getter func(*domain.InvoicePaymentStatusView) *int64) *int64 {
	if view == nil {
		return nil
	}
	return getter(view)
}

func viewTimePtr(view *domain.InvoicePaymentStatusView, getter func(*domain.InvoicePaymentStatusView) time.Time) *time.Time {
	if view == nil {
		return nil
	}
	return timePtr(getter(view))
}

func invoiceCustomerExternalID(invoice map[string]any) string {
	customer := objectValue(invoice["customer"])
	return strings.TrimSpace(stringValue(customer["external_id"]))
}
