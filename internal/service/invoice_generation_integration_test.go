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

// TestBillingCycleEndToEnd verifies the full invoice generation flow:
// create customer → create plan with meter → create subscription →
// record usage → generate invoice → verify line items + totals.
//
// Requires TEST_DATABASE_URL pointing to a real Postgres instance.
func TestBillingCycleEndToEnd(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for integration tests")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := store.NewPostgresRepository(db)
	tenantID := "test_billing_cycle_" + time.Now().Format("20060102150405")

	// 1. Create tenant
	_, err = repo.UpsertTenant(domain.Tenant{
		ID:   tenantID,
		Name: "Billing Cycle Test",
	})
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	// 2. Create customer
	customer, err := repo.UpsertCustomer(domain.Customer{
		TenantID:   tenantID,
		ExternalID: "cust_e2e",
		Name:       "E2E Test Customer",
		Email:      "e2e@test.com",
		Currency:   "USD",
		Status:     domain.CustomerStatusActive,
	})
	if err != nil {
		t.Fatalf("create customer: %v", err)
	}

	// 3. Create meter
	meter, err := repo.UpsertMeter(domain.Meter{
		TenantID:    tenantID,
		Key:         "api_calls",
		Name:        "API Calls",
		Unit:        "request",
		Aggregation: domain.MeterAggregationSum,
	})
	if err != nil {
		t.Fatalf("create meter: %v", err)
	}

	// 4. Create rating rule for per-unit pricing ($0.01 per request)
	ruleVersion, err := repo.UpsertRatingRuleVersion(domain.RatingRuleVersion{
		TenantID:   tenantID,
		MeterID:    meter.ID,
		Mode:       domain.PricingModePerUnit,
		PerUnitCents: 1, // $0.01
		Currency:   "USD",
		Status:     "active",
	})
	if err != nil {
		t.Fatalf("create rating rule: %v", err)
	}

	// Update meter with rule reference
	meter.RatingRuleVersionID = ruleVersion.ID
	_, err = repo.UpsertMeter(meter)
	if err != nil {
		t.Fatalf("update meter with rule: %v", err)
	}

	// 5. Create plan with base fee + usage meter
	plan, err := repo.UpsertPlan(domain.Plan{
		TenantID:        tenantID,
		Code:            "pro",
		Name:            "Pro Plan",
		Currency:        "USD",
		BillingInterval: domain.BillingIntervalMonthly,
		Status:          domain.PlanStatusActive,
		BaseAmountCents: 2999, // $29.99
		MeterIDs:        []string{meter.ID},
	})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}

	// 6. Create subscription
	now := time.Now().UTC()
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	periodEnd := periodStart.AddDate(0, 1, 0)
	sub, err := repo.UpsertSubscription(domain.Subscription{
		TenantID:   tenantID,
		Code:       "sub_e2e",
		CustomerID: customer.ExternalID,
		PlanID:     plan.ID,
		Status:     domain.SubscriptionStatusActive,
		CurrentBillingPeriodStart: &periodStart,
		CurrentBillingPeriodEnd:   &periodEnd,
	})
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}

	// 7. Record usage events (100 API calls)
	for i := 0; i < 10; i++ {
		_, err = repo.CreateUsageEvent(domain.UsageEvent{
			TenantID:       tenantID,
			CustomerID:     customer.ExternalID,
			MeterID:        meter.ID,
			SubscriptionID: sub.ID,
			Quantity:       10,
			Timestamp:      periodStart.Add(time.Duration(i) * time.Hour),
		})
		if err != nil {
			t.Fatalf("create usage event %d: %v", i, err)
		}
	}

	// 8. Generate invoice
	genSvc := service.NewInvoiceGenerationService(repo, db)
	result, err := genSvc.Generate(context.Background(), service.GenerateInvoiceInput{
		TenantID:       tenantID,
		SubscriptionID: sub.ID,
		PeriodStart:    periodStart,
		PeriodEnd:      periodEnd,
	})
	if err != nil {
		t.Fatalf("generate invoice: %v", err)
	}

	if result.AlreadyExists {
		t.Fatal("expected new invoice, got already exists")
	}

	// 9. Verify invoice
	invoice := result.Invoice
	if invoice.Status != domain.InvoiceStatusDraft {
		t.Errorf("expected draft status, got %s", invoice.Status)
	}
	if invoice.Currency != "USD" {
		t.Errorf("expected USD currency, got %s", invoice.Currency)
	}
	if invoice.CustomerID != customer.ExternalID {
		t.Errorf("expected customer %s, got %s", customer.ExternalID, invoice.CustomerID)
	}

	// 10. Verify line items
	if len(result.LineItems) < 2 {
		t.Fatalf("expected at least 2 line items (base + usage), got %d", len(result.LineItems))
	}

	var baseFeeFound, usageFound bool
	for _, li := range result.LineItems {
		switch li.LineType {
		case domain.LineTypeBaseFee:
			baseFeeFound = true
			if li.AmountCents != 2999 {
				t.Errorf("expected base fee 2999 cents, got %d", li.AmountCents)
			}
		case domain.LineTypeUsage:
			usageFound = true
			if li.Quantity != 100 {
				t.Errorf("expected 100 usage units, got %d", li.Quantity)
			}
			// 100 requests * $0.01 = $1.00 = 100 cents
			if li.AmountCents != 100 {
				t.Errorf("expected usage amount 100 cents, got %d", li.AmountCents)
			}
		}
	}

	if !baseFeeFound {
		t.Error("base fee line item not found")
	}
	if !usageFound {
		t.Error("usage line item not found")
	}

	// 11. Verify total: $29.99 + $1.00 = $30.99 = 3099 cents
	expectedTotal := int64(3099)
	if invoice.TotalAmountCents != expectedTotal {
		t.Errorf("expected total %d cents, got %d cents", expectedTotal, invoice.TotalAmountCents)
	}

	// 12. Verify idempotency — generating again returns AlreadyExists
	result2, err := genSvc.Generate(context.Background(), service.GenerateInvoiceInput{
		TenantID:       tenantID,
		SubscriptionID: sub.ID,
		PeriodStart:    periodStart,
		PeriodEnd:      periodEnd,
	})
	if err != nil {
		t.Fatalf("second generate: %v", err)
	}
	if !result2.AlreadyExists {
		t.Error("expected AlreadyExists on second generation")
	}

	// 13. Verify preview
	preview, err := genSvc.Preview(context.Background(), tenantID, sub.ID)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.Invoice.InvoiceNumber != "(preview)" {
		t.Errorf("expected (preview) invoice number, got %s", preview.Invoice.InvoiceNumber)
	}

	t.Logf("billing cycle test passed: invoice %s, total $%.2f, %d line items",
		invoice.InvoiceNumber, float64(invoice.TotalAmountCents)/100, len(result.LineItems))
}
