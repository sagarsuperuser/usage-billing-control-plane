package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

type LagoWebhookService struct {
	repo         store.Repository
	verifier     LagoWebhookVerifier
	tenantMapper LagoOrganizationTenantMapper
	customerSvc  *CustomerService
	dunningSvc   *DunningService
}

func NewLagoWebhookService(repo store.Repository, verifier LagoWebhookVerifier, tenantMapper LagoOrganizationTenantMapper, customerSvc *CustomerService) *LagoWebhookService {
	if verifier == nil {
		verifier = NoopLagoWebhookVerifier{}
	}
	if tenantMapper == nil {
		tenantMapper = NewTenantBackedLagoOrganizationTenantMapper(repo)
	}
	return &LagoWebhookService{
		repo:         repo,
		verifier:     verifier,
		tenantMapper: tenantMapper,
		customerSvc:  customerSvc,
	}
}

func (s *LagoWebhookService) WithDunningService(dunningSvc *DunningService) *LagoWebhookService {
	if s == nil {
		return nil
	}
	s.dunningSvc = dunningSvc
	return s
}

type IngestLagoWebhookResult struct {
	Event      domain.LagoWebhookEvent `json:"event"`
	Idempotent bool                    `json:"idempotent"`
}

func (s *LagoWebhookService) Ingest(ctx context.Context, headers http.Header, body []byte) (IngestLagoWebhookResult, error) {
	if s == nil || s.repo == nil {
		return IngestLagoWebhookResult{}, fmt.Errorf("%w: lago webhook service is not configured", ErrValidation)
	}
	if err := s.verifier.Verify(ctx, headers, body); err != nil {
		return IngestLagoWebhookResult{}, err
	}

	event, err := parseLagoWebhookEnvelope(body)
	if err != nil {
		return IngestLagoWebhookResult{}, err
	}
	event.WebhookKey = strings.TrimSpace(headers.Get("X-Lago-Unique-Key"))
	event.TenantID = s.tenantMapper.TenantIDForOrganization(event.OrganizationID)
	if strings.TrimSpace(event.TenantID) == "" {
		return IngestLagoWebhookResult{}, fmt.Errorf("%w: organization_id %q is not mapped to a tenant", ErrValidation, strings.TrimSpace(event.OrganizationID))
	}
	if event.WebhookKey == "" {
		event.WebhookKey = buildWebhookKey(event)
	}

	stored, created, err := s.repo.IngestLagoWebhookEvent(event)
	if err != nil {
		return IngestLagoWebhookResult{}, err
	}
	if err := s.applyCustomerWebhookEffects(stored); err != nil {
		return IngestLagoWebhookResult{}, err
	}
	if err := s.applyDunningWebhookEffects(stored); err != nil {
		return IngestLagoWebhookResult{}, err
	}
	return IngestLagoWebhookResult{
		Event:      stored,
		Idempotent: !created,
	}, nil
}

func (s *LagoWebhookService) applyCustomerWebhookEffects(event domain.LagoWebhookEvent) error {
	if s == nil || s.customerSvc == nil {
		return nil
	}
	tenantID := normalizeTenantID(event.TenantID)
	customerExternalID := strings.TrimSpace(event.CustomerExternalID)
	if tenantID == "" || customerExternalID == "" {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(event.WebhookType)) {
	case "customer.payment_provider_created":
		_, err := s.customerSvc.RefreshCustomerPaymentSetup(tenantID, customerExternalID)
		return err
	case "customer.payment_provider_error":
		errMessage := strings.TrimSpace(event.LastPaymentError)
		if errMessage == "" {
			errMessage = "payment provider error"
		}
		_, err := s.customerSvc.RecordCustomerPaymentProviderError(tenantID, customerExternalID, errMessage)
		return err
	default:
		return nil
	}
}

func (s *LagoWebhookService) applyDunningWebhookEffects(event domain.LagoWebhookEvent) error {
	if s == nil || s.dunningSvc == nil {
		return nil
	}
	tenantID := normalizeTenantID(event.TenantID)
	invoiceID := strings.TrimSpace(event.InvoiceID)
	if tenantID == "" || invoiceID == "" {
		return nil
	}
	_, err := s.dunningSvc.EnsureRunForInvoice(tenantID, invoiceID)
	return err
}

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

func (s *LagoWebhookService) ListInvoicePaymentStatusViews(tenantID string, req ListInvoicePaymentStatusViewsRequest) ([]domain.InvoicePaymentStatusView, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("%w: lago webhook service is not configured", ErrValidation)
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

func (s *LagoWebhookService) GetInvoicePaymentStatusSummary(tenantID string, req GetInvoicePaymentStatusSummaryRequest) (InvoicePaymentStatusSummary, error) {
	if s == nil || s.repo == nil {
		return InvoicePaymentStatusSummary{}, fmt.Errorf("%w: lago webhook service is not configured", ErrValidation)
	}
	if req.StaleAfterSeconds < 0 {
		return InvoicePaymentStatusSummary{}, fmt.Errorf("%w: stale_after_sec must be >= 0", ErrValidation)
	}
	if req.StaleAfterSeconds > maxSummaryStaleAfterSeconds {
		return InvoicePaymentStatusSummary{}, fmt.Errorf("%w: stale_after_sec must be <= %d", ErrValidation, maxSummaryStaleAfterSeconds)
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

func (s *LagoWebhookService) GetInvoicePaymentStatusView(tenantID, invoiceID string) (domain.InvoicePaymentStatusView, error) {
	if s == nil || s.repo == nil {
		return domain.InvoicePaymentStatusView{}, fmt.Errorf("%w: lago webhook service is not configured", ErrValidation)
	}
	return s.repo.GetInvoicePaymentStatusView(normalizeTenantID(tenantID), strings.TrimSpace(invoiceID))
}

func (s *LagoWebhookService) GetInvoicePaymentLifecycle(tenantID, invoiceID string, eventLimit int) (InvoicePaymentLifecycle, error) {
	if s == nil || s.repo == nil {
		return InvoicePaymentLifecycle{}, fmt.Errorf("%w: lago webhook service is not configured", ErrValidation)
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

	events, err := s.repo.ListLagoWebhookEvents(store.LagoWebhookEventListFilter{
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

func BuildInvoicePaymentLifecycleFromView(view domain.InvoicePaymentStatusView, events []domain.LagoWebhookEvent, eventLimit int) InvoicePaymentLifecycle {
	if eventLimit <= 0 {
		eventLimit = defaultLifecycleEventLimit
	}
	if eventLimit > maxWebhookListLimit {
		eventLimit = maxWebhookListLimit
	}
	return buildInvoicePaymentLifecycle(view, events, eventLimit)
}

type ListLagoWebhookEventsRequest struct {
	OrganizationID string
	InvoiceID      string
	WebhookType    string
	SortBy         string
	Order          string
	Limit          int
	Offset         int
}

func (s *LagoWebhookService) ListLagoWebhookEvents(tenantID string, req ListLagoWebhookEventsRequest) ([]domain.LagoWebhookEvent, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("%w: lago webhook service is not configured", ErrValidation)
	}
	limit, offset, err := validateWebhookListWindow(req.Limit, req.Offset)
	if err != nil {
		return nil, err
	}
	sortBy, err := normalizeLagoWebhookEventSortBy(req.SortBy)
	if err != nil {
		return nil, err
	}
	sortDesc, err := normalizeWebhookListOrder(req.Order)
	if err != nil {
		return nil, err
	}
	return s.repo.ListLagoWebhookEvents(store.LagoWebhookEventListFilter{
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
