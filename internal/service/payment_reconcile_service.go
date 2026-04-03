package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

const (
	defaultPaymentReconcileBatchLimit = 100
	maxPaymentReconcileBatchLimit     = 1000
	defaultPaymentReconcileStaleAfter = 5 * time.Minute
)

type PaymentReconcileService struct {
	repo           store.Repository
	invoiceAdapter InvoiceBillingAdapter
}

type PaymentReconcileBatchRequest struct {
	Limit      int
	StaleAfter time.Duration
}

type PaymentReconcileBatchResult struct {
	Scanned          int `json:"scanned"`
	Reconciled       int `json:"reconciled"`
	CreatedEvents    int `json:"created_events"`
	IdempotentEvents int `json:"idempotent_events"`
	Failures         int `json:"failures"`
}

func NewPaymentReconcileService(repo store.Repository, invoiceAdapter InvoiceBillingAdapter) (*PaymentReconcileService, error) {
	if repo == nil {
		return nil, fmt.Errorf("%w: repository is required", ErrValidation)
	}
	if invoiceAdapter == nil {
		return nil, fmt.Errorf("%w: invoice billing adapter is required", ErrValidation)
	}
	return &PaymentReconcileService{
		repo:           repo,
		invoiceAdapter: invoiceAdapter,
	}, nil
}

func (s *PaymentReconcileService) ReconcileBatch(ctx context.Context, req PaymentReconcileBatchRequest) (PaymentReconcileBatchResult, error) {
	if s == nil || s.repo == nil || s.invoiceAdapter == nil {
		return PaymentReconcileBatchResult{}, fmt.Errorf("%w: payment reconcile service is not configured", ErrValidation)
	}

	limit := req.Limit
	if limit <= 0 {
		limit = defaultPaymentReconcileBatchLimit
	}
	if limit > maxPaymentReconcileBatchLimit {
		limit = maxPaymentReconcileBatchLimit
	}

	staleAfter := req.StaleAfter
	if staleAfter <= 0 {
		staleAfter = defaultPaymentReconcileStaleAfter
	}
	if staleAfter < 0 {
		return PaymentReconcileBatchResult{}, fmt.Errorf("%w: stale_after must be >= 0", ErrValidation)
	}

	staleBefore := time.Now().UTC().Add(-staleAfter)
	candidates, err := s.repo.ListInvoicePaymentSyncCandidates(store.InvoicePaymentSyncCandidateFilter{
		StaleBefore: staleBefore,
		Limit:       limit,
	})
	if err != nil {
		return PaymentReconcileBatchResult{}, err
	}

	result := PaymentReconcileBatchResult{}
	for _, candidate := range candidates {
		result.Scanned++

		billingCtx := ContextWithBillingScope(ctx, candidate.TenantID, candidate.OrganizationID)
		statusCode, body, err := s.invoiceAdapter.GetInvoice(billingCtx, candidate.InvoiceID)
		if err != nil {
			result.Failures++
			continue
		}
		if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
			result.Failures++
			continue
		}

		event, err := BuildInvoiceReconcileEvent(body, candidate.TenantID, candidate.OrganizationID)
		if err != nil {
			result.Failures++
			continue
		}
		stored, created, err := s.repo.IngestBillingEvent(event)
		if err != nil {
			result.Failures++
			continue
		}

		result.Reconciled++
		if created {
			result.CreatedEvents++
		} else if strings.TrimSpace(stored.ID) != "" {
			result.IdempotentEvents++
		}
	}

	return result, nil
}

func BuildInvoiceReconcileEvent(payload []byte, tenantID, fallbackOrganizationID string) (domain.BillingEvent, error) {
	if !json.Valid(payload) {
		return domain.BillingEvent{}, fmt.Errorf("%w: invoice payload must be valid json", ErrValidation)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return domain.BillingEvent{}, fmt.Errorf("%w: decode invoice payload", ErrValidation)
	}

	invoicePayload, ok := decoded["invoice"].(map[string]any)
	if !ok || invoicePayload == nil {
		return domain.BillingEvent{}, fmt.Errorf("%w: invoice payload missing invoice", ErrValidation)
	}

	invoiceID := strings.TrimSpace(stringValue(invoicePayload["lago_id"]))
	if invoiceID == "" {
		return domain.BillingEvent{}, fmt.Errorf("%w: invoice id is required", ErrValidation)
	}

	organizationID := strings.TrimSpace(stringValue(invoicePayload["organization_id"]))
	if organizationID == "" {
		organizationID = strings.TrimSpace(fallbackOrganizationID)
	}
	if organizationID == "" {
		return domain.BillingEvent{}, fmt.Errorf("%w: organization_id is required", ErrValidation)
	}

	receivedAt := time.Now().UTC()
	occurredAt := firstTimestamp(invoicePayload["updated_at"], invoicePayload["created_at"], receivedAt)
	customerExternalID := ""
	if customer, ok := invoicePayload["customer"].(map[string]any); ok {
		customerExternalID = stringValue(customer["external_id"])
	}
	paymentOverdue := boolPtr(invoicePayload["payment_overdue"])
	totalAmountCents := int64Ptr(invoicePayload["total_amount_cents"])
	totalDueAmountCents := int64Ptr(invoicePayload["total_due_amount_cents"])
	totalPaidAmountCents := int64Ptr(invoicePayload["total_paid_amount_cents"])
	lastPaymentError := firstNonEmptyString(
		stringValue(invoicePayload["payment_error"]),
		stringValue(invoicePayload["last_payment_error"]),
	)

	event := domain.BillingEvent{
		TenantID:             normalizeTenantID(tenantID),
		OrganizationID:       organizationID,
		WebhookType:          "invoice.payment_status_reconciled",
		ObjectType:           "invoice",
		InvoiceID:            invoiceID,
		CustomerExternalID:   customerExternalID,
		InvoiceNumber:        stringValue(invoicePayload["number"]),
		Currency:             stringValue(invoicePayload["currency"]),
		InvoiceStatus:        stringValue(invoicePayload["status"]),
		PaymentStatus:        stringValue(invoicePayload["payment_status"]),
		PaymentOverdue:       paymentOverdue,
		TotalAmountCents:     totalAmountCents,
		TotalDueAmountCents:  totalDueAmountCents,
		TotalPaidAmountCents: totalPaidAmountCents,
		LastPaymentError:     lastPaymentError,
		Payload:              decoded,
		ReceivedAt:           receivedAt,
		OccurredAt:           occurredAt,
	}
	event.WebhookKey = buildInvoiceReconcileWebhookKey(event)
	return event, nil
}

func buildInvoiceReconcileWebhookKey(event domain.BillingEvent) string {
	due := int64ValueOrZero(event.TotalDueAmountCents)
	paid := int64ValueOrZero(event.TotalPaidAmountCents)
	total := int64ValueOrZero(event.TotalAmountCents)
	overdue := boolValueOrFalse(event.PaymentOverdue)

	fingerprint := strings.Join([]string{
		strings.TrimSpace(event.InvoiceID),
		strings.TrimSpace(event.InvoiceStatus),
		strings.TrimSpace(event.PaymentStatus),
		fmt.Sprintf("%t", overdue),
		fmt.Sprintf("%d", total),
		fmt.Sprintf("%d", due),
		fmt.Sprintf("%d", paid),
		event.OccurredAt.UTC().Format(time.RFC3339Nano),
	}, "|")
	sum := sha256.Sum256([]byte(fingerprint))
	return "reconcile:" + strings.TrimSpace(event.InvoiceID) + ":" + hex.EncodeToString(sum[:8])
}

func int64ValueOrZero(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

func boolValueOrFalse(v *bool) bool {
	return v != nil && *v
}
