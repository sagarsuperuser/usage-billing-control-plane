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

	plan := postJSON(t, ts.URL+"/v1/plans", map[string]any{
		"code":              "growth",
		"name":              "Growth",
		"currency":          "USD",
		"billing_interval":  "monthly",
		"status":            "active",
		"base_amount_cents": 4900,
		"meter_ids":         []string{metricID},
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
