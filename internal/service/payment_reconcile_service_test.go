package service

import (
	"testing"
)

func TestBuildInvoiceReconcileEvent(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
		"invoice": {
			"lago_id": "inv_123",
			"organization_id": "org_abc",
			"number": "INV-123",
			"currency": "USD",
			"status": "finalized",
			"payment_status": "failed",
			"payment_overdue": true,
			"total_amount_cents": 1200,
			"total_due_amount_cents": 1200,
			"total_paid_amount_cents": 0,
			"last_payment_error": "card_declined",
			"updated_at": "2026-03-12T12:00:00Z",
			"customer": {
				"external_id": "cust_1"
			}
		}
	}`)

	event, err := BuildInvoiceReconcileEvent(payload, "tenant_a", "")
	if err != nil {
		t.Fatalf("build event: %v", err)
	}
	if event.TenantID != "tenant_a" {
		t.Fatalf("expected tenant_a, got %q", event.TenantID)
	}
	if event.OrganizationID != "org_abc" {
		t.Fatalf("expected org_abc, got %q", event.OrganizationID)
	}
	if event.InvoiceID != "inv_123" {
		t.Fatalf("expected inv_123, got %q", event.InvoiceID)
	}
	if event.WebhookType != "invoice.payment_status_reconciled" {
		t.Fatalf("unexpected webhook type: %q", event.WebhookType)
	}
	if event.ObjectType != "invoice" {
		t.Fatalf("unexpected object type: %q", event.ObjectType)
	}
	if event.PaymentStatus != "failed" {
		t.Fatalf("expected payment status failed, got %q", event.PaymentStatus)
	}
	if event.PaymentOverdue == nil || !*event.PaymentOverdue {
		t.Fatalf("expected payment_overdue=true")
	}
	if event.WebhookKey == "" {
		t.Fatalf("expected webhook key")
	}

	again, err := BuildInvoiceReconcileEvent(payload, "tenant_a", "")
	if err != nil {
		t.Fatalf("build event again: %v", err)
	}
	if event.WebhookKey != again.WebhookKey {
		t.Fatalf("expected deterministic webhook key, got %q and %q", event.WebhookKey, again.WebhookKey)
	}
}

func TestBuildInvoiceReconcileEvent_FallbackOrganization(t *testing.T) {
	t.Parallel()

	payload := []byte(`{"invoice":{"lago_id":"inv_123","payment_status":"pending"}}`)
	event, err := BuildInvoiceReconcileEvent(payload, "tenant_a", "org_fallback")
	if err != nil {
		t.Fatalf("build event with fallback org: %v", err)
	}
	if event.OrganizationID != "org_fallback" {
		t.Fatalf("expected fallback org, got %q", event.OrganizationID)
	}
}

func TestBuildInvoiceReconcileEvent_ValidatePayload(t *testing.T) {
	t.Parallel()

	if _, err := BuildInvoiceReconcileEvent([]byte(`{"invoice":{}}`), "tenant_a", "org_1"); err == nil {
		t.Fatalf("expected validation error for missing invoice id")
	}
	if _, err := BuildInvoiceReconcileEvent([]byte(`{"invoice":{"lago_id":"inv_1"}}`), "tenant_a", ""); err == nil {
		t.Fatalf("expected validation error for missing organization_id")
	}
}
