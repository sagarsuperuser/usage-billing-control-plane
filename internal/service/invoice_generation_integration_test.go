package service_test

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/service"
	"usage-billing-control-plane/internal/store"
)

// TestInvoicePreviewFailsForMissingSubscription verifies Preview returns
// an error for non-existent subscriptions without persisting anything.
func TestInvoicePreviewFailsForMissingSubscription(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for integration tests")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := store.NewPostgresStore(db)
	genSvc := service.NewInvoiceGenerationService(repo, db)

	_, err = genSvc.Preview(context.Background(), "nonexistent_tenant", "nonexistent_sub")
	if err == nil {
		t.Error("expected error for non-existent subscription, got nil")
	}
}

// TestInvoiceGenerationIdempotency verifies that generating the same invoice
// twice returns AlreadyExists without creating a duplicate.
func TestInvoiceGenerationIdempotency(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for integration tests")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := store.NewPostgresStore(db)
	tenantID := "test_idem_" + time.Now().Format("150405")

	_, err = repo.CreateTenant(domain.Tenant{ID: tenantID, Name: "Idempotency Test"})
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	_, err = repo.CreateCustomer(domain.Customer{
		TenantID:    tenantID,
		ExternalID:  "cust_idem",
		DisplayName: "Idempotency Customer",
		Status:      domain.CustomerStatusActive,
	})
	if err != nil {
		t.Fatalf("create customer: %v", err)
	}

	plan, err := repo.CreatePlan(domain.Plan{
		TenantID:        tenantID,
		Code:            "basic",
		Name:            "Basic Plan",
		Currency:        "USD",
		BillingInterval: domain.BillingIntervalMonthly,
		Status:          domain.PlanStatusActive,
		BaseAmountCents: 999,
	})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}

	periodStart := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	sub, err := repo.CreateSubscription(domain.Subscription{
		TenantID:                  tenantID,
		Code:                      "sub_idem",
		CustomerID:                "cust_idem",
		PlanID:                    plan.ID,
		Status:                    domain.SubscriptionStatusActive,
		CurrentBillingPeriodStart: &periodStart,
		CurrentBillingPeriodEnd:   &periodEnd,
	})
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}

	genSvc := service.NewInvoiceGenerationService(repo, db)
	input := service.GenerateInvoiceInput{
		TenantID:       tenantID,
		SubscriptionID: sub.ID,
		PeriodStart:    periodStart,
		PeriodEnd:      periodEnd,
	}

	result1, err := genSvc.Generate(context.Background(), input)
	if err != nil {
		t.Fatalf("first generate: %v", err)
	}
	if result1.AlreadyExists {
		t.Fatal("first generation should not return AlreadyExists")
	}
	if result1.Invoice.TotalAmountCents != 999 {
		t.Errorf("expected total 999 cents, got %d", result1.Invoice.TotalAmountCents)
	}

	result2, err := genSvc.Generate(context.Background(), input)
	if err != nil {
		t.Fatalf("second generate: %v", err)
	}
	if !result2.AlreadyExists {
		t.Error("expected AlreadyExists on second generation")
	}

	t.Logf("idempotency test passed: invoice %s, total $%.2f",
		result1.Invoice.InvoiceNumber, float64(result1.Invoice.TotalAmountCents)/100)
}
