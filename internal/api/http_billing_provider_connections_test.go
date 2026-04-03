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
	if s.result.StripeProviderCode == "" {
		s.result.StripeProviderCode = input.StripeProviderCode
	}
	if s.result.StripeAccountID == "" {
		s.result.StripeAccountID = input.StripeAccountID
	}
	return s.result, nil
}

type stubStripeConnectionVerifierAPI struct {
	result service.StripeConnectionVerificationResult
	err    error
}

func (s *stubStripeConnectionVerifierAPI) VerifyStripeSecret(_ context.Context, _ string) (service.StripeConnectionVerificationResult, error) {
	if s.err != nil {
		return service.StripeConnectionVerificationResult{}, s.err
	}
	return s.result, nil
}

func TestInternalBillingProviderConnectionEndpointsSyncOnlyRequiresStripeSecret(t *testing.T) {
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

	svc := service.NewBillingProviderConnectionService(repo, service.NewMemoryBillingSecretStore(), &stubBillingProviderAdapterAPI{}).
		WithStripeConnectionVerifier(&stubStripeConnectionVerifierAPI{result: service.StripeConnectionVerificationResult{AccountID: "acct_ok"}})

	ts := httptest.NewServer(api.NewServer(repo,
		api.WithAPIKeyAuthorizer(authorizer),
		api.WithBillingProviderConnectionService(svc),
	).Handler())
	defer ts.Close()

	created := postJSON(t, ts.URL+"/internal/billing-provider-connections", map[string]any{
		"provider_type":     "stripe",
		"environment":       "test",
		"display_name":      "Stripe Missing Org",
		"scope":             "platform",
		"stripe_secret_key": "sk_test_http",
	}, "platform-admin", http.StatusCreated)

	connection := created["connection"].(map[string]any)
	connectionID := connection["id"].(string)

	synced := postJSON(t, ts.URL+"/internal/billing-provider-connections/"+connectionID+"/sync", map[string]any{}, "platform-admin", http.StatusOK)
	syncedConnection := synced["connection"].(map[string]any)
	if syncedConnection["status"] != "connected" {
		t.Fatalf("expected connected status after check, got %#v", syncedConnection["status"])
	}

	got := getJSON(t, ts.URL+"/internal/billing-provider-connections/"+connectionID, "platform-admin", http.StatusOK)
	connectionAfterCheck := got["connection"].(map[string]any)
	if connectionAfterCheck["check_ready"] != true {
		t.Fatalf("expected check_ready=true when secret is present")
	}
	if connectionAfterCheck["workspace_ready"] != true {
		t.Fatalf("expected workspace_ready=true after Stripe check")
	}
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
		StripeAccountID: "org_synced",
		StripeProviderCode:   "stripe_synced",
	}}
	svc := service.NewBillingProviderConnectionService(repo, service.NewMemoryBillingSecretStore(), adapter).
		WithStripeConnectionVerifier(&stubStripeConnectionVerifierAPI{result: service.StripeConnectionVerificationResult{AccountID: "acct_ok"}})

	ts := httptest.NewServer(api.NewServer(repo,
		api.WithAPIKeyAuthorizer(authorizer),
		api.WithBillingProviderConnectionService(svc),
	).Handler())
	defer ts.Close()

	created := postJSON(t, ts.URL+"/internal/billing-provider-connections", map[string]any{
		"provider_type":     "stripe",
		"environment":       "test",
		"display_name":      "Stripe Platform Test",
		"scope":             "platform",
		"stripe_secret_key": "sk_test_http",
	}, "platform-admin", http.StatusCreated)

	connection := created["connection"].(map[string]any)
	connectionID := connection["id"].(string)
	if connection["secret_configured"] != true {
		t.Fatalf("expected secret_configured=true")
	}
	if connection["workspace_ready"] != false {
		t.Fatalf("expected workspace_ready=false before sync")
	}
	if connection["check_ready"] != true {
		t.Fatalf("expected check_ready=true when connection has a secret")
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
	if syncedConnection["check_ready"] != true {
		t.Fatalf("expected check_ready=true after sync")
	}
	if syncedConnection["sync_summary"] != "Stripe credentials are verified and ready for workspace assignment." {
		t.Fatalf("expected provider verification summary, got %#v", syncedConnection["sync_summary"])
	}

	disabled := postJSON(t, ts.URL+"/internal/billing-provider-connections/"+connectionID+"/disable", map[string]any{}, "platform-admin", http.StatusOK)
	disabledConnection := disabled["connection"].(map[string]any)
	if disabledConnection["status"] != "disabled" {
		t.Fatalf("expected disabled status, got %#v", disabledConnection["status"])
	}
}

func TestInternalBillingProviderConnectionRotateSecretEndpoint(t *testing.T) {
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
		StripeAccountID: "org_synced",
		StripeProviderCode:   "stripe_synced",
	}}
	secretStore := service.NewMemoryBillingSecretStore()
	svc := service.NewBillingProviderConnectionService(repo, secretStore, adapter).
		WithStripeConnectionVerifier(&stubStripeConnectionVerifierAPI{result: service.StripeConnectionVerificationResult{AccountID: "acct_ok"}})

	ts := httptest.NewServer(api.NewServer(repo,
		api.WithAPIKeyAuthorizer(authorizer),
		api.WithBillingProviderConnectionService(svc),
	).Handler())
	defer ts.Close()

	created := postJSON(t, ts.URL+"/internal/billing-provider-connections", map[string]any{
		"provider_type":     "stripe",
		"environment":       "test",
		"display_name":      "Stripe Rotate Test",
		"scope":             "platform",
		"stripe_secret_key": "sk_test_http",
	}, "platform-admin", http.StatusCreated)

	connection := created["connection"].(map[string]any)
	connectionID := connection["id"].(string)
	synced := postJSON(t, ts.URL+"/internal/billing-provider-connections/"+connectionID+"/sync", map[string]any{}, "platform-admin", http.StatusOK)
	syncedConnection := synced["connection"].(map[string]any)
	if syncedConnection["status"] != "connected" {
		t.Fatalf("expected connected status after sync, got %#v", syncedConnection["status"])
	}

	rotated := postJSON(t, ts.URL+"/internal/billing-provider-connections/"+connectionID+"/rotate-secret", map[string]any{
		"stripe_secret_key": "sk_test_rotated_http",
	}, "platform-admin", http.StatusOK)
	rotatedConnection := rotated["connection"].(map[string]any)
	if rotatedConnection["status"] != "pending" {
		t.Fatalf("expected pending status after rotate, got %#v", rotatedConnection["status"])
	}
	if rotatedConnection["sync_state"] != "pending" {
		t.Fatalf("expected pending sync_state after rotate, got %#v", rotatedConnection["sync_state"])
	}
	if rotatedConnection["sync_summary"] != "Run another connection check before using this connection." {
		t.Fatalf("expected pending sync_summary after rotate, got %#v", rotatedConnection["sync_summary"])
	}
	if rotatedConnection["workspace_ready"] != false {
		t.Fatalf("expected workspace_ready=false after rotate")
	}
	if rotatedConnection["check_ready"] != true {
		t.Fatalf("expected check_ready=true after rotate")
	}
}
