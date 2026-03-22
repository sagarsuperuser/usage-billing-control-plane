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
	"usage-billing-control-plane/internal/service"
	"usage-billing-control-plane/internal/store"
)

type stubSubscriptionPlanSyncAdapter struct{}

func (stubSubscriptionPlanSyncAdapter) SyncPlan(_ context.Context, _ domain.Plan, _ []service.PlanSyncComponent) error {
	return nil
}

type stubSubscriptionSyncAdapter struct{}

func (stubSubscriptionSyncAdapter) SyncSubscription(_ context.Context, _ domain.Subscription, _ domain.Customer, _ domain.Plan) error {
	return nil
}

func TestSubscriptionEndpoints(t *testing.T) {
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
	mustCreateAPIKey(t, repo, "subscription-reader-key", api.RoleReader, "default")
	mustCreateAPIKey(t, repo, "subscription-writer-key", api.RoleWriter, "default")

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	customerPaymentMethodReady := false
	lagoMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/customers":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"customer":{"lago_id":"lago_cust_sub","external_id":"cust_sub","billing_configuration":{"payment_provider":"stripe","payment_provider_code":"stripe_test","provider_customer_id":"pcus_sub","provider_payment_methods":["card"]}}}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/customers/cust_sub/payment_methods":
			w.Header().Set("Content-Type", "application/json")
			if customerPaymentMethodReady {
				_, _ = w.Write([]byte(`{"payment_methods":[{"lago_id":"pm_sub","is_default":true,"provider_method_id":"pm_sub_default"}]}`))
				return
			}
			_, _ = w.Write([]byte(`{"payment_methods":[]}`))
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/customers/cust_sub/checkout_url":
			customerPaymentMethodReady = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"customer":{"external_customer_id":"cust_sub","checkout_url":"https://checkout.example.test/cust_sub"}}`))
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer lagoMock.Close()

	lagoTransport, err := service.NewLagoHTTPTransport(service.LagoClientConfig{
		BaseURL: lagoMock.URL,
		APIKey:  "test-api-key",
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}

	mustSetTenantMappings(t, repo, "default", "org_default", "stripe_test")
	ts := httptest.NewServer(api.NewServer(
		repo,
		api.WithAPIKeyAuthorizer(authorizer),
		api.WithCustomerBillingAdapter(service.NewLagoCustomerBillingAdapter(lagoTransport)),
		api.WithSubscriptionSyncAdapter(stubSubscriptionSyncAdapter{}),
	).Handler())
	defer ts.Close()

	ratingService := service.NewRatingService(repo)
	meterService := service.NewMeterService(repo)
	planService := service.NewPlanService(repo).WithPlanSyncAdapter(stubSubscriptionPlanSyncAdapter{})

	rule, err := ratingService.CreateRuleVersion(domain.RatingRuleVersion{
		TenantID:       "default",
		RuleKey:        "subscription_test_rule",
		Name:           "Subscription Test Rule",
		Version:        1,
		LifecycleState: domain.RatingRuleLifecycleActive,
		Mode:           domain.PricingModeGraduated,
		Currency:       "USD",
		GraduatedTiers: []domain.RatingTier{{UpTo: 0, UnitAmountCents: 1}},
	})
	if err != nil {
		t.Fatalf("create rule version: %v", err)
	}

	meter, err := meterService.CreateMeter(domain.Meter{
		TenantID:            "default",
		Key:                 "subscription_test_meter",
		Name:                "Subscription Test Meter",
		Unit:                "request",
		Aggregation:         "sum",
		RatingRuleVersionID: rule.ID,
	})
	if err != nil {
		t.Fatalf("create meter: %v", err)
	}

	plan, err := planService.CreatePlan(context.Background(), domain.Plan{
		TenantID:        "default",
		Code:            "growth",
		Name:            "Growth",
		Currency:        "USD",
		BillingInterval: domain.BillingIntervalMonthly,
		Status:          domain.PlanStatusActive,
		BaseAmountCents: 4900,
		MeterIDs:        []string{meter.ID},
	})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}

	revisedPlan, err := planService.CreatePlan(context.Background(), domain.Plan{
		TenantID:        "default",
		Code:            "scale",
		Name:            "Scale",
		Currency:        "USD",
		BillingInterval: domain.BillingIntervalMonthly,
		Status:          domain.PlanStatusActive,
		BaseAmountCents: 9900,
		MeterIDs:        []string{meter.ID},
	})
	if err != nil {
		t.Fatalf("create revised plan: %v", err)
	}

	_ = postJSON(t, ts.URL+"/v1/customers", map[string]any{
		"external_id":  "cust_sub",
		"display_name": "Subscriber Co",
		"email":        "billing@subscriber.test",
	}, "subscription-writer-key", http.StatusCreated)

	_ = putJSON(t, ts.URL+"/v1/customers/cust_sub/billing-profile", map[string]any{
		"legal_name":            "Subscriber Co Pvt Ltd",
		"email":                 "billing@subscriber.test",
		"billing_address_line1": "1 Billing St",
		"billing_city":          "Bengaluru",
		"billing_postal_code":   "560001",
		"billing_country":       "IN",
		"currency":              "USD",
		"provider_code":         "stripe_default",
	}, "subscription-writer-key", http.StatusOK)

	created := postJSON(t, ts.URL+"/v1/subscriptions", map[string]any{
		"display_name":          "Subscriber Growth",
		"code":                  "subscriber_growth",
		"customer_external_id":  "cust_sub",
		"plan_id":               plan.ID,
		"request_payment_setup": false,
	}, "subscription-writer-key", http.StatusCreated)

	subscription, ok := created["subscription"].(map[string]any)
	if !ok {
		t.Fatalf("expected subscription payload, got %#v", created)
	}
	subscriptionID, _ := subscription["id"].(string)
	if subscriptionID == "" {
		t.Fatalf("expected subscription id, got %#v", subscription["id"])
	}
	if got, _ := subscription["status"].(string); got != "draft" {
		t.Fatalf("expected draft subscription, got %q", got)
	}
	if got, _ := subscription["payment_setup_status"].(string); got != "missing" {
		t.Fatalf("expected payment_setup_status missing, got %q", got)
	}

	items := getJSONArray(t, ts.URL+"/v1/subscriptions", "subscription-reader-key", http.StatusOK)
	if len(items) != 1 {
		t.Fatalf("expected 1 subscription, got %d", len(items))
	}

	detail := getJSON(t, ts.URL+"/v1/subscriptions/"+subscriptionID, "subscription-reader-key", http.StatusOK)
	if got, _ := detail["plan_name"].(string); got != "Growth" {
		t.Fatalf("expected plan_name Growth, got %q", got)
	}
	if got, _ := detail["customer_display_name"].(string); got != "Subscriber Co" {
		t.Fatalf("expected customer_display_name Subscriber Co, got %q", got)
	}

	updatedPlan := patchJSON(t, ts.URL+"/v1/subscriptions/"+subscriptionID, map[string]any{
		"plan_id": revisedPlan.ID,
	}, "subscription-writer-key", http.StatusOK)
	if got, _ := updatedPlan["plan_id"].(string); got != revisedPlan.ID {
		t.Fatalf("expected updated plan_id %q, got %q", revisedPlan.ID, got)
	}
	if got, _ := updatedPlan["plan_name"].(string); got != "Scale" {
		t.Fatalf("expected updated plan_name Scale, got %q", got)
	}
	if got, _ := updatedPlan["plan_code"].(string); got != "scale" {
		t.Fatalf("expected updated plan_code scale, got %q", got)
	}

	archived := patchJSON(t, ts.URL+"/v1/subscriptions/"+subscriptionID, map[string]any{
		"status": "archived",
	}, "subscription-writer-key", http.StatusOK)
	if got, _ := archived["status"].(string); got != "archived" {
		t.Fatalf("expected archived subscription status, got %q", got)
	}

	requested := postJSON(t, ts.URL+"/v1/subscriptions/"+subscriptionID+"/payment-setup/request", map[string]any{
		"payment_method_type": "card",
	}, "subscription-writer-key", http.StatusOK)
	if got, _ := requested["checkout_url"].(string); got != "https://checkout.example.test/cust_sub" {
		t.Fatalf("expected checkout_url, got %q", got)
	}
	requestedSubscription, ok := requested["subscription"].(map[string]any)
	if !ok {
		t.Fatalf("expected subscription detail in payment setup response")
	}
	if got, _ := requestedSubscription["status"].(string); got != "pending_payment_setup" {
		t.Fatalf("expected pending_payment_setup status, got %q", got)
	}
	if got, _ := requestedSubscription["payment_setup_status"].(string); got != "pending" {
		t.Fatalf("expected payment_setup_status pending, got %q", got)
	}

	_ = postJSON(t, ts.URL+"/v1/subscriptions/"+subscriptionID+"/payment-setup/request", map[string]any{
		"payment_method_type": "card",
	}, "subscription-reader-key", http.StatusForbidden)
}
