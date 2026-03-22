package api

import (
	"testing"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/service"
)

func TestApplyCustomerReadinessToPaymentLifecyclePendingCollectPayment(t *testing.T) {
	lifecycle := service.InvoicePaymentLifecycle{
		PaymentStatus:     "pending",
		RecommendedAction: "monitor_processing",
	}
	readiness := service.CustomerReadiness{
		PaymentSetupStatus:           domain.PaymentSetupStatusPending,
		DefaultPaymentMethodVerified: false,
	}

	updated := applyCustomerReadinessToPaymentLifecycle(lifecycle, readiness)
	if !updated.RequiresAction {
		t.Fatalf("expected requires_action true")
	}
	if updated.RetryRecommended {
		t.Fatalf("expected retry_recommended false")
	}
	if updated.RecommendedAction != "collect_payment" {
		t.Fatalf("expected collect_payment, got %q", updated.RecommendedAction)
	}
}
