package api_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"usage-billing-control-plane/internal/api"
	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/service"
	"usage-billing-control-plane/internal/store"
)

type stubMeterSyncAdapter struct{}

func (stubMeterSyncAdapter) SyncMeter(_ context.Context, _ domain.Meter) error {
	return nil
}

type stubPlanSyncAdapter struct{}

func (stubPlanSyncAdapter) SyncPlan(_ context.Context, _ domain.Plan, _ []service.PlanSyncComponent) error {
	return nil
}

type stubTaxSyncAdapter struct{}

func (stubTaxSyncAdapter) SyncTax(_ context.Context, _ domain.Tax) error {
	return nil
}

func TestPricingMetricAndPlanEndpoints(t *testing.T) {
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
	mustCreateAPIKey(t, repo, "pricing-reader-key", api.RoleReader, "default")
	mustCreateAPIKey(t, repo, "pricing-writer-key", api.RoleWriter, "default")

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	ts := httptest.NewServer(api.NewServer(repo,
		api.WithAPIKeyAuthorizer(authorizer),
		api.WithMeterSyncAdapter(stubMeterSyncAdapter{}),
		api.WithTaxSyncAdapter(stubTaxSyncAdapter{}),
		api.WithPlanSyncAdapter(stubPlanSyncAdapter{}),
	).Handler())
	defer ts.Close()

	metric := postJSON(t, ts.URL+"/v1/pricing/metrics", map[string]any{
		"key":         "api_calls",
		"name":        "API Calls",
		"unit":        "request",
		"aggregation": "sum",
		"currency":    "USD",
	}, "pricing-writer-key", http.StatusCreated)
	metricID := metric["id"].(string)

	metrics := getJSONArray(t, ts.URL+"/v1/pricing/metrics", "pricing-reader-key", http.StatusOK)
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}

	gotMetric := getJSON(t, ts.URL+"/v1/pricing/metrics/"+metricID, "pricing-reader-key", http.StatusOK)
	if gotMetric["key"] != "api_calls" {
		t.Fatalf("expected api_calls key, got %#v", gotMetric["key"])
	}

	addOn := postJSON(t, ts.URL+"/v1/add-ons", map[string]any{
		"code":             "priority_support",
		"name":             "Priority support",
		"currency":         "USD",
		"billing_interval": "monthly",
		"status":           "active",
		"amount_cents":     1500,
	}, "pricing-writer-key", http.StatusCreated)
	addOnID := addOn["id"].(string)

	addOns := getJSONArray(t, ts.URL+"/v1/add-ons", "pricing-reader-key", http.StatusOK)
	if len(addOns) != 1 {
		t.Fatalf("expected 1 add-on, got %d", len(addOns))
	}

	coupon := postJSON(t, ts.URL+"/v1/coupons", map[string]any{
		"code":             "launch_20",
		"name":             "Launch 20",
		"status":           "active",
		"discount_type":    "percent_off",
		"percent_off":      20,
		"amount_off_cents": 0,
	}, "pricing-writer-key", http.StatusCreated)
	couponID := coupon["id"].(string)

	coupons := getJSONArray(t, ts.URL+"/v1/coupons", "pricing-reader-key", http.StatusOK)
	if len(coupons) != 1 {
		t.Fatalf("expected 1 coupon, got %d", len(coupons))
	}

	tax := postJSON(t, ts.URL+"/v1/taxes", map[string]any{
		"code":        "gst_in",
		"name":        "GST India",
		"status":      "active",
		"rate":        18,
		"description": "India GST",
	}, "pricing-writer-key", http.StatusCreated)
	taxID := tax["id"].(string)

	taxes := getJSONArray(t, ts.URL+"/v1/taxes", "pricing-reader-key", http.StatusOK)
	if len(taxes) != 1 {
		t.Fatalf("expected 1 tax, got %d", len(taxes))
	}
	gotTax := getJSON(t, ts.URL+"/v1/taxes/"+taxID, "pricing-reader-key", http.StatusOK)
	if gotTax["code"] != "gst_in" {
		t.Fatalf("expected gst_in code, got %#v", gotTax["code"])
	}

	plan := postJSON(t, ts.URL+"/v1/plans", map[string]any{
		"code":              "growth",
		"name":              "Growth",
		"currency":          "USD",
		"billing_interval":  "monthly",
		"status":            "active",
		"base_amount_cents": 4900,
		"meter_ids":         []string{metricID},
		"add_on_ids":        []string{addOnID},
		"coupon_ids":        []string{couponID},
	}, "pricing-writer-key", http.StatusCreated)
	planID := plan["id"].(string)

	plans := getJSONArray(t, ts.URL+"/v1/plans", "pricing-reader-key", http.StatusOK)
	if len(plans) != 1 {
		t.Fatalf("expected 1 plan, got %d", len(plans))
	}
	gotPlan := getJSON(t, ts.URL+"/v1/plans/"+planID, "pricing-reader-key", http.StatusOK)
	if gotPlan["code"] != "growth" {
		t.Fatalf("expected growth code, got %#v", gotPlan["code"])
	}
	meterIDs, ok := gotPlan["meter_ids"].([]any)
	if !ok || len(meterIDs) != 1 {
		t.Fatalf("expected 1 linked meter, got %#v", gotPlan["meter_ids"])
	}
	addOnIDs, ok := gotPlan["add_on_ids"].([]any)
	if !ok || len(addOnIDs) != 1 {
		t.Fatalf("expected 1 linked add-on, got %#v", gotPlan["add_on_ids"])
	}
	couponIDs, ok := gotPlan["coupon_ids"].([]any)
	if !ok || len(couponIDs) != 1 {
		t.Fatalf("expected 1 linked coupon, got %#v", gotPlan["coupon_ids"])
	}

	_ = postJSON(t, ts.URL+"/v1/plans", map[string]any{
		"code":              "reader-blocked",
		"name":              "Reader Blocked",
		"currency":          "USD",
		"billing_interval":  "monthly",
		"status":            "draft",
		"base_amount_cents": 0,
		"meter_ids":         []string{metricID},
	}, "pricing-reader-key", http.StatusForbidden)
}
