package api_test

import (
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

func TestTenantOnboardingUsesBillingProviderConnection(t *testing.T) {
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

	now := time.Now().UTC()
	connectedAt := now
	lastSyncedAt := now
	connection, err := repo.CreateBillingProviderConnection(domain.BillingProviderConnection{
		ID:                 "bpc_tenant_assign_test",
		ProviderType:       domain.BillingProviderTypeStripe,
		Environment:        "test",
		DisplayName:        "Stripe Platform",
		Scope:              domain.BillingProviderConnectionScopePlatform,
		Status:             domain.BillingProviderConnectionStatusConnected,
		LagoOrganizationID: "org_platform",
		LagoProviderCode:   "stripe_platform",
		SecretRef:          "memory://billing-provider-connections/bpc_tenant_assign_test/seed",
		ConnectedAt:        &connectedAt,
		LastSyncedAt:       &lastSyncedAt,
		CreatedByType:      "platform_api_key",
		CreatedByID:        "pkey_seed",
		CreatedAt:          now,
		UpdatedAt:          now,
	})
	if err != nil {
		t.Fatalf("create billing provider connection: %v", err)
	}

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	ts := httptest.NewServer(api.NewServer(repo, api.WithAPIKeyAuthorizer(authorizer)).Handler())
	defer ts.Close()

	result := postJSON(t, ts.URL+"/internal/onboarding/tenants", map[string]any{
		"id":                             "tenant_conn_assign",
		"name":                           "Tenant Conn Assign",
		"billing_provider_connection_id": connection.ID,
		"bootstrap_admin_key":            false,
	}, "platform-admin", http.StatusCreated)

	tenant := result["tenant"].(map[string]any)
	if tenant["billing_provider_connection_id"] != connection.ID {
		t.Fatalf("expected billing_provider_connection_id %q, got %#v", connection.ID, tenant["billing_provider_connection_id"])
	}
	if tenant["lago_organization_id"] != "org_platform" {
		t.Fatalf("expected lago organization id from connection, got %#v", tenant["lago_organization_id"])
	}
	if tenant["lago_billing_provider_code"] != "stripe_platform" {
		t.Fatalf("expected lago provider code from connection, got %#v", tenant["lago_billing_provider_code"])
	}

	got := getJSON(t, ts.URL+"/internal/tenants/tenant_conn_assign", "platform-admin", http.StatusOK)
	if got["billing_provider_connection_id"] != connection.ID {
		t.Fatalf("expected persisted billing_provider_connection_id %q, got %#v", connection.ID, got["billing_provider_connection_id"])
	}
}
