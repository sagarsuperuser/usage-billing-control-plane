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

func TestTenantWorkspaceBillingSettingsSubresourcePersistsAndReturnsSettings(t *testing.T) {
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
	if _, err := repo.CreateTenant(domain.Tenant{
		ID:        "tenant_workspace_settings",
		Name:      "Tenant Workspace Settings",
		Status:    domain.TenantStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	ts := httptest.NewServer(api.NewServer(repo, api.WithAPIKeyAuthorizer(authorizer)).Handler())
	defer ts.Close()

	initial := getJSON(t, ts.URL+"/internal/tenants/tenant_workspace_settings", "platform-admin", http.StatusOK)
	initialSettings, ok := initial["workspace_billing_settings"].(map[string]any)
	if !ok {
		t.Fatalf("expected workspace_billing_settings in tenant response")
	}
	if hasOverrides, _ := initialSettings["has_overrides"].(bool); hasOverrides {
		t.Fatalf("expected workspace billing settings to start without overrides")
	}

	result := patchJSON(t, ts.URL+"/internal/tenants/tenant_workspace_settings/workspace-billing-settings", map[string]any{
		"billing_entity_code":   "be_us_primary",
		"net_payment_term_days": 14,
		"tax_codes":             []string{"GST_IN", "VAT_DE"},
		"invoice_memo":          "Thank you for your business.",
		"invoice_footer":        "Wire details available on request.",
	}, "platform-admin", http.StatusOK)

	settings, ok := result["workspace_billing_settings"].(map[string]any)
	if !ok {
		t.Fatalf("expected workspace_billing_settings object")
	}
	if got, _ := settings["billing_entity_code"].(string); got != "be_us_primary" {
		t.Fatalf("expected billing_entity_code be_us_primary, got %q", got)
	}
	if got, _ := settings["invoice_memo"].(string); got != "Thank you for your business." {
		t.Fatalf("expected invoice_memo to round-trip, got %q", got)
	}
	if got, _ := settings["invoice_footer"].(string); got != "Wire details available on request." {
		t.Fatalf("expected invoice_footer to round-trip, got %q", got)
	}
	if got, ok := settings["net_payment_term_days"].(float64); !ok || int(got) != 14 {
		t.Fatalf("expected net_payment_term_days 14, got %#v", settings["net_payment_term_days"])
	}
	if got, ok := settings["tax_codes"].([]any); !ok || len(got) != 2 {
		t.Fatalf("expected tax_codes to round-trip, got %#v", settings["tax_codes"])
	}
	if hasOverrides, _ := settings["has_overrides"].(bool); !hasOverrides {
		t.Fatalf("expected has_overrides=true after patch")
	}

	tenantResp := getJSON(t, ts.URL+"/internal/tenants/tenant_workspace_settings", "platform-admin", http.StatusOK)
	tenantSettings := tenantResp["workspace_billing_settings"].(map[string]any)
	if got, _ := tenantSettings["billing_entity_code"].(string); got != "be_us_primary" {
		t.Fatalf("expected tenant response billing_entity_code be_us_primary, got %q", got)
	}
}
