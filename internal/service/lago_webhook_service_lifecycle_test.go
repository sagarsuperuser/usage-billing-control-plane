package service

import (
	"testing"
	"time"

	"lago-usage-billing-alpha/internal/domain"
)

func TestBuildInvoicePaymentLifecycle_FailedOverdue(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 3, 13, 10, 0, 0, 0, time.UTC)
	overdueTrue := true
	view := domain.InvoicePaymentStatusView{
		TenantID:         "tenant_a",
		OrganizationID:   "org_1",
		InvoiceID:        "inv_123",
		InvoiceStatus:    "finalized",
		PaymentStatus:    "failed",
		PaymentOverdue:   &overdueTrue,
		LastEventType:    "invoice.payment_overdue",
		LastPaymentError: "card_declined",
		LastEventAt:      base.Add(4 * time.Minute),
		UpdatedAt:        base.Add(5 * time.Minute),
	}
	events := []domain.LagoWebhookEvent{
		{
			WebhookType:   "invoice.payment_failure",
			PaymentStatus: "failed",
			OccurredAt:    base.Add(1 * time.Minute),
		},
		{
			WebhookType:    "invoice.payment_overdue",
			PaymentStatus:  "failed",
			PaymentOverdue: &overdueTrue,
			OccurredAt:     base.Add(2 * time.Minute),
		},
		{
			WebhookType:   "payment_request.payment_status_updated",
			PaymentStatus: "pending",
			OccurredAt:    base.Add(3 * time.Minute),
		},
	}

	got := buildInvoicePaymentLifecycle(view, events, 2)
	if got.InvoiceID != "inv_123" {
		t.Fatalf("expected invoice id inv_123, got %q", got.InvoiceID)
	}
	if !got.RequiresAction {
		t.Fatalf("expected requires_action=true")
	}
	if !got.RetryRecommended {
		t.Fatalf("expected retry_recommended=true")
	}
	if got.RecommendedAction != "retry_payment" {
		t.Fatalf("expected recommended_action retry_payment, got %q", got.RecommendedAction)
	}
	if got.FailureEventCount != 2 {
		t.Fatalf("expected failure_event_count=2, got %d", got.FailureEventCount)
	}
	if got.PendingEventCount != 1 {
		t.Fatalf("expected pending_event_count=1, got %d", got.PendingEventCount)
	}
	if got.OverdueSignalCount != 1 {
		t.Fatalf("expected overdue_signal_count=1, got %d", got.OverdueSignalCount)
	}
	if got.LastFailureAt == nil || !got.LastFailureAt.Equal(base.Add(2*time.Minute)) {
		t.Fatalf("expected last_failure_at at +2m, got %v", got.LastFailureAt)
	}
	if got.EventWindowTruncated != true {
		t.Fatalf("expected event window truncated when events_analyzed >= event_window_limit")
	}
}

func TestBuildInvoicePaymentLifecycle_Succeeded(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 3, 13, 11, 0, 0, 0, time.UTC)
	overdueFalse := false
	view := domain.InvoicePaymentStatusView{
		TenantID:       "tenant_a",
		OrganizationID: "org_1",
		InvoiceID:      "inv_456",
		InvoiceStatus:  "finalized",
		PaymentStatus:  "succeeded",
		PaymentOverdue: &overdueFalse,
		LastEventType:  "invoice.payment_status_updated",
		LastEventAt:    base.Add(3 * time.Minute),
		UpdatedAt:      base.Add(3 * time.Minute),
	}
	events := []domain.LagoWebhookEvent{
		{
			WebhookType:   "invoice.payment_status_updated",
			PaymentStatus: "pending",
			OccurredAt:    base.Add(1 * time.Minute),
		},
		{
			WebhookType:   "invoice.payment_status_updated",
			PaymentStatus: "succeeded",
			OccurredAt:    base.Add(2 * time.Minute),
		},
	}

	got := buildInvoicePaymentLifecycle(view, events, 200)
	if got.RequiresAction {
		t.Fatalf("expected requires_action=false")
	}
	if got.RetryRecommended {
		t.Fatalf("expected retry_recommended=false")
	}
	if got.RecommendedAction != "none" {
		t.Fatalf("expected recommended_action none, got %q", got.RecommendedAction)
	}
	if got.SuccessEventCount != 1 {
		t.Fatalf("expected success_event_count=1, got %d", got.SuccessEventCount)
	}
	if got.LastSuccessAt == nil || !got.LastSuccessAt.Equal(base.Add(2*time.Minute)) {
		t.Fatalf("expected last_success_at at +2m, got %v", got.LastSuccessAt)
	}
	if got.EventWindowTruncated {
		t.Fatalf("expected event window truncated=false")
	}
}
