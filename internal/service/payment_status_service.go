package service

// PaymentStatusService provides read access to invoice payment status views
// and webhook event timelines. This replaces the PaymentStatusService's
// query methods while the webhook ingestion is now handled by StripeWebhookService.
//
// The type is aliased as PaymentStatusService for backward compatibility with
// the API handler layer (http.go) which references s.lagoWebhookSvc.
// This alias will be cleaned up when the API handlers are refactored.

import (
	"fmt"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

// PaymentStatusService is the backward-compatible type name used by the API
// handler layer. It provides invoice payment status reads and lifecycle analysis.
type PaymentStatusService struct {
	repo        store.Repository
	customerSvc *CustomerService
	dunningSvc  *DunningService
}

func NewPaymentStatusService(repo store.Repository, customerSvc *CustomerService) *PaymentStatusService {
	return &PaymentStatusService{
		repo:        repo,
		customerSvc: customerSvc,
	}
}

func (s *PaymentStatusService) WithDunningService(dunningSvc *DunningService) *PaymentStatusService {
	if s == nil {
		return nil
	}
	s.dunningSvc = dunningSvc
	return s
}

// Ingest is a no-op stub for backward compatibility. Actual webhook ingestion
// is now handled by StripeWebhookService.
func (s *PaymentStatusService) Ingest(_ interface{}, _ interface{}, _ []byte) (IngestWebhookResult, error) {
	return IngestWebhookResult{}, fmt.Errorf("lago webhook ingestion is deprecated; use stripe webhook service")
}

type IngestWebhookResult struct {
	Event      domain.BillingEvent `json:"event"`
	Idempotent bool                    `json:"idempotent"`
}

// ---------------------------------------------------------------------------
// Request/Response types
// ---------------------------------------------------------------------------

type ListInvoicePaymentStatusViewsRequest struct {
	OrganizationID     string
	CustomerExternalID string
	InvoiceID          string
	InvoiceNumber      string
	LastEventType      string
	PaymentStatus      string
	InvoiceStatus      string
	PaymentOverdue     *bool
	SortBy             string
	Order              string
	Limit              int
	Offset             int
}

type GetInvoicePaymentStatusSummaryRequest struct {
	OrganizationID    string
	StaleAfterSeconds int
}

type InvoicePaymentStatusSummary struct {
	TotalInvoices          int64            `json:"total_invoices"`
	OverdueCount           int64            `json:"overdue_count"`
	AttentionRequiredCount int64            `json:"attention_required_count"`
	StaleAttentionRequired int64            `json:"stale_attention_required"`
	StaleAfterSeconds      int              `json:"stale_after_seconds,omitempty"`
	StaleBefore            *time.Time       `json:"stale_before,omitempty"`
	LatestEventAt          *time.Time       `json:"latest_event_at,omitempty"`
	PaymentStatusCounts    map[string]int64 `json:"payment_status_counts"`
	InvoiceStatusCounts    map[string]int64 `json:"invoice_status_counts"`
}

type InvoicePaymentLifecycle struct {
	TenantID              string     `json:"tenant_id"`
	OrganizationID        string     `json:"organization_id"`
	InvoiceID             string     `json:"invoice_id"`
	InvoiceStatus         string     `json:"invoice_status,omitempty"`
	PaymentStatus         string     `json:"payment_status,omitempty"`
	PaymentOverdue        *bool      `json:"payment_overdue,omitempty"`
	LastPaymentError      string     `json:"last_payment_error,omitempty"`
	LastEventType         string     `json:"last_event_type,omitempty"`
	LastEventAt           *time.Time `json:"last_event_at,omitempty"`
	UpdatedAt             *time.Time `json:"updated_at,omitempty"`
	EventsAnalyzed        int        `json:"events_analyzed"`
	EventWindowLimit      int        `json:"event_window_limit"`
	EventWindowTruncated  bool       `json:"event_window_truncated"`
	DistinctWebhookTypes  []string   `json:"distinct_webhook_types,omitempty"`
	FailureEventCount     int        `json:"failure_event_count"`
	SuccessEventCount     int        `json:"success_event_count"`
	PendingEventCount     int        `json:"pending_event_count"`
	OverdueSignalCount    int        `json:"overdue_signal_count"`
	LastFailureAt         *time.Time `json:"last_failure_at,omitempty"`
	LastSuccessAt         *time.Time `json:"last_success_at,omitempty"`
	LastPendingAt         *time.Time `json:"last_pending_at,omitempty"`
	LastOverdueAt         *time.Time `json:"last_overdue_at,omitempty"`
	RequiresAction        bool       `json:"requires_action"`
	RetryRecommended      bool       `json:"retry_recommended"`
	RecommendedAction     string     `json:"recommended_action"`
	RecommendedActionNote string     `json:"recommended_action_note"`
}

type ListBillingEventsRequest struct {
	OrganizationID string
	InvoiceID      string
	WebhookType    string
	SortBy         string
	Order          string
	Limit          int
	Offset         int
}

const (
	maxWebhookListLimit          = 200
	maxSummaryStaleAfterSeconds  = 86400
	defaultLifecycleEventLimit   = 50
)

// ---------------------------------------------------------------------------
// Query methods
// ---------------------------------------------------------------------------

func (s *PaymentStatusService) ListInvoicePaymentStatusViews(tenantID string, req ListInvoicePaymentStatusViewsRequest) ([]domain.InvoicePaymentStatusView, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("%w: payment status service is not configured", ErrValidation)
	}
	limit, offset, err := validateWebhookListWindow(req.Limit, req.Offset)
	if err != nil {
		return nil, err
	}
	sortBy, err := normalizeInvoicePaymentStatusSortBy(req.SortBy)
	if err != nil {
		return nil, err
	}
	sortDesc, err := normalizeWebhookListOrder(req.Order)
	if err != nil {
		return nil, err
	}
	return s.repo.ListInvoicePaymentStatusViews(store.InvoicePaymentStatusListFilter{
		TenantID:           normalizeTenantID(tenantID),
		OrganizationID:     strings.TrimSpace(req.OrganizationID),
		CustomerExternalID: strings.TrimSpace(req.CustomerExternalID),
		InvoiceID:          strings.TrimSpace(req.InvoiceID),
		InvoiceNumber:      strings.TrimSpace(req.InvoiceNumber),
		LastEventType:      strings.TrimSpace(req.LastEventType),
		PaymentStatus:      strings.TrimSpace(req.PaymentStatus),
		InvoiceStatus:      strings.TrimSpace(req.InvoiceStatus),
		PaymentOverdue:     req.PaymentOverdue,
		SortBy:             sortBy,
		SortDesc:           sortDesc,
		Limit:              limit,
		Offset:             offset,
	})
}

func (s *PaymentStatusService) GetInvoicePaymentStatusSummary(tenantID string, req GetInvoicePaymentStatusSummaryRequest) (InvoicePaymentStatusSummary, error) {
	if s == nil || s.repo == nil {
		return InvoicePaymentStatusSummary{}, fmt.Errorf("%w: payment status service is not configured", ErrValidation)
	}
	if req.StaleAfterSeconds < 0 || req.StaleAfterSeconds > maxSummaryStaleAfterSeconds {
		return InvoicePaymentStatusSummary{}, fmt.Errorf("%w: stale_after_sec must be between 0 and %d", ErrValidation, maxSummaryStaleAfterSeconds)
	}

	filter := store.InvoicePaymentStatusSummaryFilter{
		TenantID:       normalizeTenantID(tenantID),
		OrganizationID: strings.TrimSpace(req.OrganizationID),
	}
	var staleBefore *time.Time
	if req.StaleAfterSeconds > 0 {
		v := time.Now().UTC().Add(-time.Duration(req.StaleAfterSeconds) * time.Second)
		staleBefore = &v
		filter.StaleBefore = staleBefore
	}

	summary, err := s.repo.GetInvoicePaymentStatusSummary(filter)
	if err != nil {
		return InvoicePaymentStatusSummary{}, err
	}
	return InvoicePaymentStatusSummary{
		TotalInvoices:          summary.TotalInvoices,
		OverdueCount:           summary.OverdueCount,
		AttentionRequiredCount: summary.AttentionRequiredCount,
		StaleAttentionRequired: summary.StaleAttentionRequired,
		StaleAfterSeconds:      req.StaleAfterSeconds,
		StaleBefore:            staleBefore,
		LatestEventAt:          summary.LatestEventAt,
		PaymentStatusCounts:    summary.PaymentStatusCounts,
		InvoiceStatusCounts:    summary.InvoiceStatusCounts,
	}, nil
}

func (s *PaymentStatusService) GetInvoicePaymentStatusView(tenantID, invoiceID string) (domain.InvoicePaymentStatusView, error) {
	if s == nil || s.repo == nil {
		return domain.InvoicePaymentStatusView{}, fmt.Errorf("%w: payment status service is not configured", ErrValidation)
	}
	return s.repo.GetInvoicePaymentStatusView(normalizeTenantID(tenantID), strings.TrimSpace(invoiceID))
}

func (s *PaymentStatusService) GetInvoicePaymentLifecycle(tenantID, invoiceID string, eventLimit int) (InvoicePaymentLifecycle, error) {
	if s == nil || s.repo == nil {
		return InvoicePaymentLifecycle{}, fmt.Errorf("%w: payment status service is not configured", ErrValidation)
	}
	invoiceID = strings.TrimSpace(invoiceID)
	if invoiceID == "" {
		return InvoicePaymentLifecycle{}, fmt.Errorf("%w: invoice id is required", ErrValidation)
	}
	if eventLimit <= 0 {
		eventLimit = defaultLifecycleEventLimit
	}
	if eventLimit > maxWebhookListLimit {
		return InvoicePaymentLifecycle{}, fmt.Errorf("%w: event_limit must be between 1 and %d", ErrValidation, maxWebhookListLimit)
	}

	tenantID = normalizeTenantID(tenantID)
	view, err := s.repo.GetInvoicePaymentStatusView(tenantID, invoiceID)
	if err != nil {
		return InvoicePaymentLifecycle{}, err
	}

	events, err := s.repo.ListBillingEvents(store.BillingEventListFilter{
		TenantID:  tenantID,
		InvoiceID: invoiceID,
		SortBy:    "occurred_at",
		SortDesc:  false,
		Limit:     eventLimit,
		Offset:    0,
	})
	if err != nil {
		return InvoicePaymentLifecycle{}, err
	}

	return buildInvoicePaymentLifecycle(view, events, eventLimit), nil
}

func BuildInvoicePaymentLifecycleFromView(view domain.InvoicePaymentStatusView, events []domain.BillingEvent, eventLimit int) InvoicePaymentLifecycle {
	if eventLimit <= 0 {
		eventLimit = defaultLifecycleEventLimit
	}
	if eventLimit > maxWebhookListLimit {
		eventLimit = maxWebhookListLimit
	}
	return buildInvoicePaymentLifecycle(view, events, eventLimit)
}

func (s *PaymentStatusService) ListBillingEvents(tenantID string, req ListBillingEventsRequest) ([]domain.BillingEvent, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("%w: payment status service is not configured", ErrValidation)
	}
	limit, offset, err := validateWebhookListWindow(req.Limit, req.Offset)
	if err != nil {
		return nil, err
	}
	sortBy, err := normalizeBillingEventSortBy(req.SortBy)
	if err != nil {
		return nil, err
	}
	sortDesc, err := normalizeWebhookListOrder(req.Order)
	if err != nil {
		return nil, err
	}
	return s.repo.ListBillingEvents(store.BillingEventListFilter{
		TenantID:       normalizeTenantID(tenantID),
		OrganizationID: strings.TrimSpace(req.OrganizationID),
		InvoiceID:      strings.TrimSpace(req.InvoiceID),
		WebhookType:    strings.TrimSpace(req.WebhookType),
		SortBy:         sortBy,
		SortDesc:       sortDesc,
		Limit:          limit,
		Offset:         offset,
	})
}

// ---------------------------------------------------------------------------
// Validation / normalization helpers
// ---------------------------------------------------------------------------

func validateWebhookListWindow(limit, offset int) (int, int, error) {
	if limit < 0 || limit > maxWebhookListLimit {
		return 0, 0, fmt.Errorf("%w: limit must be between 0 and %d", ErrValidation, maxWebhookListLimit)
	}
	if offset < 0 {
		return 0, 0, fmt.Errorf("%w: offset must be >= 0", ErrValidation)
	}
	if limit == 0 {
		limit = 50
	}
	return limit, offset, nil
}

func normalizeWebhookListOrder(raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "desc":
		return true, nil
	case "asc":
		return false, nil
	default:
		return false, fmt.Errorf("%w: order must be 'asc' or 'desc'", ErrValidation)
	}
}

func normalizeInvoicePaymentStatusSortBy(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "last_event_at":
		return "last_event_at", nil
	case "updated_at":
		return "updated_at", nil
	case "total_due_amount_cents":
		return "total_due_amount_cents", nil
	case "total_amount_cents":
		return "total_amount_cents", nil
	default:
		return "", fmt.Errorf("%w: invalid sort_by for payment status views", ErrValidation)
	}
}

func normalizeBillingEventSortBy(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "occurred_at":
		return "occurred_at", nil
	case "received_at":
		return "received_at", nil
	default:
		return "", fmt.Errorf("%w: invalid sort_by for webhook events", ErrValidation)
	}
}

// ---------------------------------------------------------------------------
// Lifecycle analysis (builds recommended actions from payment event patterns).
// ---------------------------------------------------------------------------

func buildInvoicePaymentLifecycle(view domain.InvoicePaymentStatusView, events []domain.BillingEvent, eventLimit int) InvoicePaymentLifecycle {
	lifecycle := InvoicePaymentLifecycle{
		TenantID:         view.TenantID,
		OrganizationID:   view.OrganizationID,
		InvoiceID:        view.InvoiceID,
		InvoiceStatus:    view.InvoiceStatus,
		PaymentStatus:    view.PaymentStatus,
		PaymentOverdue:   view.PaymentOverdue,
		LastPaymentError: view.LastPaymentError,
		LastEventType:    view.LastEventType,
		EventWindowLimit: eventLimit,
	}
	if !view.LastEventAt.IsZero() {
		lifecycle.LastEventAt = &view.LastEventAt
	}

	typeSet := make(map[string]struct{})
	for _, event := range events {
		lifecycle.EventsAnalyzed++
		typeSet[event.WebhookType] = struct{}{}

		switch {
		case strings.Contains(event.WebhookType, "failure") || strings.Contains(event.WebhookType, "failed"):
			lifecycle.FailureEventCount++
			setLatestLifecycleTime(&lifecycle.LastFailureAt, event.OccurredAt)
		case strings.Contains(event.WebhookType, "succeeded") || event.PaymentStatus == "succeeded":
			lifecycle.SuccessEventCount++
			setLatestLifecycleTime(&lifecycle.LastSuccessAt, event.OccurredAt)
		case strings.Contains(event.WebhookType, "pending"):
			lifecycle.PendingEventCount++
			setLatestLifecycleTime(&lifecycle.LastPendingAt, event.OccurredAt)
		case event.PaymentOverdue != nil && *event.PaymentOverdue:
			lifecycle.OverdueSignalCount++
			setLatestLifecycleTime(&lifecycle.LastOverdueAt, event.OccurredAt)
		}
	}
	lifecycle.EventWindowTruncated = len(events) >= eventLimit

	for t := range typeSet {
		lifecycle.DistinctWebhookTypes = append(lifecycle.DistinctWebhookTypes, t)
	}

	// Determine recommended action.
	switch {
	case view.PaymentStatus == "succeeded":
		lifecycle.RecommendedAction = "none"
		lifecycle.RecommendedActionNote = "Payment succeeded."
	case lifecycle.FailureEventCount > 0 && lifecycle.SuccessEventCount == 0:
		lifecycle.RequiresAction = true
		lifecycle.RetryRecommended = true
		lifecycle.RecommendedAction = "retry_payment"
		lifecycle.RecommendedActionNote = fmt.Sprintf("%d failure(s) with no success. Retry or collect payment.", lifecycle.FailureEventCount)
	case lifecycle.OverdueSignalCount > 0:
		lifecycle.RequiresAction = true
		lifecycle.RecommendedAction = "collect_payment"
		lifecycle.RecommendedActionNote = "Invoice is overdue. Collect payment from the customer."
	case view.PaymentStatus == "pending" || view.PaymentStatus == "processing":
		lifecycle.RecommendedAction = "monitor_processing"
		lifecycle.RecommendedActionNote = "Payment is processing."
	default:
		lifecycle.RecommendedAction = "investigate"
		lifecycle.RecommendedActionNote = "Payment status requires investigation."
		lifecycle.RequiresAction = true
	}

	return lifecycle
}

func setLatestLifecycleTime(dst **time.Time, candidate time.Time) {
	if candidate.IsZero() {
		return
	}
	if *dst == nil || candidate.After(**dst) {
		t := candidate
		*dst = &t
	}
}
