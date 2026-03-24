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

func TestInvoicePreviewEndpointIsDisabled(t *testing.T) {
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

	now := time.Now().UTC()
	if _, err := repo.CreateTenant(domain.Tenant{
		ID:        "default",
		Name:      "Default",
		Status:    domain.TenantStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	mustCreateAPIKey(t, repo, "tenant-a-reader", api.RoleReader, "default")

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	ts := httptest.NewServer(api.NewServer(
		repo,
		api.WithAPIKeyAuthorizer(authorizer),
	).Handler())
	defer ts.Close()

	resp := postJSON(t, ts.URL+"/v1/invoices/preview", map[string]any{
		"customer": map[string]any{
			"name":     "Acme",
			"currency": "INR",
		},
		"plan_code":    "plan_starter",
		"billing_time": "anniversary",
	}, "tenant-a-reader", http.StatusNotFound)

	if got, _ := resp["error"].(string); got != "invoice preview is not available in the current alpha release" {
		t.Fatalf("expected invoice preview disabled error, got %#v", resp)
	}
}
