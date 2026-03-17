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
	"usage-billing-control-plane/internal/service"
	"usage-billing-control-plane/internal/store"
)

type stubBillingProviderAdapterAPI struct {
	result service.EnsureStripeProviderResult
	err    error
}

func (s *stubBillingProviderAdapterAPI) EnsureStripeProvider(_ context.Context, input service.EnsureStripeProviderInput) (service.EnsureStripeProviderResult, error) {
	if s.err != nil {
		return service.EnsureStripeProviderResult{}, s.err
	}
	if s.result.LagoProviderCode == "" {
		s.result.LagoProviderCode = input.LagoProviderCode
	}
	if s.result.LagoOrganizationID == "" {
		s.result.LagoOrganizationID = input.LagoOrganizationID
	}
	return s.result, nil
}

func TestInternalBillingProviderConnectionEndpoints(t *testing.T) {
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
	mustCreatePlatformAPIKey(t, repo, "platform-admin")

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	adapter := &stubBillingProviderAdapterAPI{result: service.EnsureStripeProviderResult{
		LagoOrganizationID: "org_synced",
		LagoProviderCode:   "stripe_synced",
	}}
	svc := service.NewBillingProviderConnectionService(repo, service.NewMemoryBillingSecretStore(), adapter)

	ts := httptest.NewServer(api.NewServer(repo,
		api.WithAPIKeyAuthorizer(authorizer),
		api.WithBillingProviderConnectionService(svc),
	).Handler())
	defer ts.Close()

	created := postJSON(t, ts.URL+"/internal/billing-provider-connections", map[string]any{
		"provider_type":        "stripe",
		"environment":          "test",
		"display_name":         "Stripe Platform Test",
		"scope":                "platform",
		"stripe_secret_key":    "sk_test_http",
		"lago_organization_id": "org_seed",
		"lago_provider_code":   "stripe_seed",
	}, "platform-admin", http.StatusCreated)

	connection := created["connection"].(map[string]any)
	connectionID := connection["id"].(string)
	if connection["secret_configured"] != true {
		t.Fatalf("expected secret_configured=true")
	}
	if connection["workspace_ready"] != false {
		t.Fatalf("expected workspace_ready=false before sync")
	}
	if connection["linked_workspace_count"] != float64(0) {
		t.Fatalf("expected linked workspace count 0, got %#v", connection["linked_workspace_count"])
	}
	if _, ok := connection["secret_ref"]; ok {
		t.Fatalf("expected secret_ref to be omitted from response")
	}

	list := getJSON(t, ts.URL+"/internal/billing-provider-connections?limit=10", "platform-admin", http.StatusOK)
	items, ok := list["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected 1 connection in list, got %#v", list["items"])
	}

	got := getJSON(t, ts.URL+"/internal/billing-provider-connections/"+connectionID, "platform-admin", http.StatusOK)
	gotConnection := got["connection"].(map[string]any)
	if gotConnection["display_name"] != "Stripe Platform Test" {
		t.Fatalf("expected display_name from get, got %#v", gotConnection["display_name"])
	}

	updated := patchJSON(t, ts.URL+"/internal/billing-provider-connections/"+connectionID, map[string]any{
		"display_name": "Stripe Platform Updated",
	}, "platform-admin", http.StatusOK)
	updatedConnection := updated["connection"].(map[string]any)
	if updatedConnection["display_name"] != "Stripe Platform Updated" {
		t.Fatalf("expected updated display name, got %#v", updatedConnection["display_name"])
	}

	synced := postJSON(t, ts.URL+"/internal/billing-provider-connections/"+connectionID+"/sync", map[string]any{}, "platform-admin", http.StatusOK)
	syncedConnection := synced["connection"].(map[string]any)
	if syncedConnection["status"] != "connected" {
		t.Fatalf("expected connected status after sync, got %#v", syncedConnection["status"])
	}
	if syncedConnection["workspace_ready"] != true {
		t.Fatalf("expected workspace_ready=true after sync")
	}
	if syncedConnection["lago_provider_code"] != "stripe_synced" {
		t.Fatalf("expected synced provider code, got %#v", syncedConnection["lago_provider_code"])
	}

	disabled := postJSON(t, ts.URL+"/internal/billing-provider-connections/"+connectionID+"/disable", map[string]any{}, "platform-admin", http.StatusOK)
	disabledConnection := disabled["connection"].(map[string]any)
	if disabledConnection["status"] != "disabled" {
		t.Fatalf("expected disabled status, got %#v", disabledConnection["status"])
	}
}
