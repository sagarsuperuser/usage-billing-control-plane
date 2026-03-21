package api_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"usage-billing-control-plane/internal/api"
	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

type stubUsageSyncAdapter struct {
	called       bool
	event        domain.UsageEvent
	meter        domain.Meter
	subscription domain.Subscription
}

func (s *stubUsageSyncAdapter) SyncUsageEvent(_ context.Context, event domain.UsageEvent, meter domain.Meter, subscription domain.Subscription) error {
	s.called = true
	s.event = event
	s.meter = meter
	s.subscription = subscription
	return nil
}

func TestUsageEventsSyncWithSubscriptionTarget(t *testing.T) {
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
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	resetTables(t, db)
	mustCreateAPIKey(t, repo, "usage-writer-key", api.RoleWriter, "default")
	mustCreateAPIKey(t, repo, "usage-reader-key", api.RoleReader, "default")

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	customer, err := repo.CreateCustomer(domain.Customer{
		TenantID:    "default",
		ExternalID:  "cust_usage_sub",
		DisplayName: "Usage Subscriber",
		Status:      domain.CustomerStatusActive,
	})
	if err != nil {
		t.Fatalf("create customer: %v", err)
	}

	rule, err := repo.CreateRatingRuleVersion(domain.RatingRuleVersion{
		TenantID:        "default",
		RuleKey:         "usage_sync_rule",
		Name:            "Usage Sync Rule",
		Version:         1,
		LifecycleState:  domain.RatingRuleLifecycleActive,
		Mode:            domain.PricingModeFlat,
		Currency:        "USD",
		FlatAmountCents: 100,
	})
	if err != nil {
		t.Fatalf("create rule version: %v", err)
	}

	meter, err := repo.CreateMeter(domain.Meter{
		TenantID:            "default",
		Key:                 "usage_sync_meter",
		Name:                "Usage Sync Meter",
		Unit:                "event",
		Aggregation:         "sum",
		RatingRuleVersionID: rule.ID,
	})
	if err != nil {
		t.Fatalf("create meter: %v", err)
	}

	plan, err := repo.CreatePlan(domain.Plan{
		TenantID:        "default",
		Code:            "usage_sync_plan",
		Name:            "Usage Sync Plan",
		Currency:        "USD",
		BillingInterval: domain.BillingIntervalMonthly,
		Status:          domain.PlanStatusActive,
		BaseAmountCents: 4900,
		MeterIDs:        []string{meter.ID},
	})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}

	subscription, err := repo.CreateSubscription(domain.Subscription{
		TenantID:    "default",
		Code:        "usage_sync_subscription",
		DisplayName: "Usage Sync Subscription",
		CustomerID:  customer.ID,
		PlanID:      plan.ID,
		Status:      domain.SubscriptionStatusActive,
	})
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}

	usageSync := &stubUsageSyncAdapter{}
	ts := httptest.NewServer(api.NewServer(
		repo,
		api.WithAPIKeyAuthorizer(authorizer),
		api.WithUsageSyncAdapter(usageSync),
	).Handler())
	defer ts.Close()

	occurredAt := time.Now().UTC().Truncate(time.Second)
	created := postJSON(t, ts.URL+"/v1/usage-events", map[string]any{
		"customer_id":     customer.ID,
		"meter_id":        meter.ID,
		"subscription_id": subscription.ID,
		"quantity":        12,
		"idempotency_key": "usage-sync-idem-1",
		"timestamp":       occurredAt.Format(time.RFC3339),
	}, "usage-writer-key", http.StatusCreated)

	if got, _ := created["subscription_id"].(string); got != subscription.ID {
		t.Fatalf("expected subscription_id %q, got %q", subscription.ID, got)
	}
	if !usageSync.called {
		t.Fatalf("expected usage sync adapter to be called")
	}
	if usageSync.event.SubscriptionID != subscription.ID {
		t.Fatalf("expected synced event subscription_id %q, got %q", subscription.ID, usageSync.event.SubscriptionID)
	}
	if usageSync.meter.ID != meter.ID {
		t.Fatalf("expected synced meter id %q, got %q", meter.ID, usageSync.meter.ID)
	}
	if usageSync.subscription.ID != subscription.ID {
		t.Fatalf("expected synced subscription id %q, got %q", subscription.ID, usageSync.subscription.ID)
	}

	list := getJSON(t, ts.URL+"/v1/usage-events?meter_id="+meter.ID, "usage-reader-key", http.StatusOK)
	items := listItemsFromResponse(t, list)
	if len(items) != 1 {
		t.Fatalf("expected 1 usage event, got %d", len(items))
	}
	row, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("expected usage event row map, got %#v", items[0])
	}
	if got, _ := row["subscription_id"].(string); got != subscription.ID {
		t.Fatalf("expected listed subscription_id %q, got %q", subscription.ID, got)
	}
}
