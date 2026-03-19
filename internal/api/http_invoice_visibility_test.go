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
	"usage-billing-control-plane/internal/service"
	"usage-billing-control-plane/internal/store"
)

func TestInvoiceListEndpointReturnsNormalizedSummaries(t *testing.T) {
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

	mustCreateAPIKey(t, repo, "tenant-a-reader", api.RoleReader, "default")
	now := time.Now().UTC().Truncate(time.Second)
	if _, err := repo.CreateCustomer(domain.Customer{
		TenantID:    "default",
		ExternalID:  "cust_123",
		DisplayName: "Acme Corp",
		Email:       "billing@acme.test",
		Status:      domain.CustomerStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}); err != nil {
		t.Fatalf("create customer: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO invoice_payment_status_views (
			tenant_id, organization_id, invoice_id, customer_external_id, invoice_number, currency,
			invoice_status, payment_status, payment_overdue, total_amount_cents, total_due_amount_cents,
			total_paid_amount_cents, last_payment_error, last_event_type, last_event_at, last_webhook_key, updated_at
		) VALUES (
			'default', 'org_test', 'inv_123', 'cust_123', 'INV-123', 'USD',
			'finalized', 'failed', true, 12500, 12500,
			0, 'card_declined', 'invoice.payment_failure', $1, 'whk_inv_123', $2
		)
	`, now, now); err != nil {
		t.Fatalf("insert invoice projection: %v", err)
	}

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}
	lagoWebhookSvc := service.NewLagoWebhookService(repo, nil, nil, nil)

	ts := httptest.NewServer(api.NewServer(
		repo,
		api.WithAPIKeyAuthorizer(authorizer),
		api.WithLagoWebhookService(lagoWebhookSvc),
	).Handler())
	defer ts.Close()

	resp := getJSON(t, ts.URL+"/v1/invoices?customer_external_id=cust_123&payment_status=failed", "tenant-a-reader", http.StatusOK)
	items, ok := resp["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected one invoice item, got %#v", resp["items"])
	}
	row, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("expected invoice row map, got %#v", items[0])
	}
	if got, _ := row["invoice_id"].(string); got != "inv_123" {
		t.Fatalf("expected invoice_id inv_123, got %q", got)
	}
	if got, _ := row["customer_display_name"].(string); got != "Acme Corp" {
		t.Fatalf("expected customer_display_name Acme Corp, got %q", got)
	}
	if got, _ := row["invoice_number"].(string); got != "INV-123" {
		t.Fatalf("expected invoice_number INV-123, got %q", got)
	}
	filters, ok := resp["filters"].(map[string]any)
	if !ok {
		t.Fatalf("expected filters object in response")
	}
	if got, _ := filters["customer_external_id"].(string); got != "cust_123" {
		t.Fatalf("expected customer_external_id filter cust_123, got %q", got)
	}
}

func TestInvoiceDetailEndpointReturnsNormalizedDetail(t *testing.T) {
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

	mustCreateAPIKey(t, repo, "tenant-a-reader", api.RoleReader, "default")
	now := time.Now().UTC().Truncate(time.Second)
	if _, err := repo.CreateCustomer(domain.Customer{
		TenantID:    "default",
		ExternalID:  "cust_123",
		DisplayName: "Acme Corp",
		Email:       "billing@acme.test",
		Status:      domain.CustomerStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}); err != nil {
		t.Fatalf("create customer: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO invoice_payment_status_views (
			tenant_id, organization_id, invoice_id, customer_external_id, invoice_number, currency,
			invoice_status, payment_status, payment_overdue, total_amount_cents, total_due_amount_cents,
			total_paid_amount_cents, last_payment_error, last_event_type, last_event_at, last_webhook_key, updated_at
		) VALUES (
			'default', 'org_test', 'inv_123', 'cust_123', 'INV-123', 'USD',
			'finalized', 'failed', true, 12500, 12500,
			0, 'card_declined', 'invoice.payment_failure', $1, 'whk_inv_123', $2
		)
	`, now, now); err != nil {
		t.Fatalf("insert invoice projection: %v", err)
	}

	lago := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/invoices/inv_123" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"invoice": {
				"lago_id": "inv_123",
				"billing_entity_code": "be_default",
				"number": "INV-123",
				"status": "finalized",
				"payment_status": "failed",
				"payment_overdue": true,
				"currency": "USD",
				"issuing_date": "2026-03-01T00:00:00Z",
				"payment_due_date": "2026-03-15T00:00:00Z",
				"total_amount_cents": 12500,
				"total_due_amount_cents": 12500,
				"total_paid_amount_cents": 0,
				"payment_error": "card_declined",
				"created_at": "2026-03-01T00:00:00Z",
				"updated_at": "2026-03-10T00:00:00Z",
				"customer": {
					"external_id": "cust_123"
				},
				"fees": [],
				"metadata": [],
				"applied_taxes": []
			}
		}`))
	}))
	defer lago.Close()

	transport, err := service.NewLagoHTTPTransport(service.LagoClientConfig{
		BaseURL: lago.URL,
		APIKey:  "test",
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}
	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}
	lagoWebhookSvc := service.NewLagoWebhookService(repo, nil, nil, nil)

	ts := httptest.NewServer(api.NewServer(
		repo,
		api.WithAPIKeyAuthorizer(authorizer),
		api.WithLagoWebhookService(lagoWebhookSvc),
		api.WithInvoiceBillingAdapter(service.NewLagoInvoiceAdapter(transport)),
	).Handler())
	defer ts.Close()

	resp := getJSON(t, ts.URL+"/v1/invoices/inv_123", "tenant-a-reader", http.StatusOK)
	if got, _ := resp["invoice_id"].(string); got != "inv_123" {
		t.Fatalf("expected invoice_id inv_123, got %q", got)
	}
	if got, _ := resp["customer_display_name"].(string); got != "Acme Corp" {
		t.Fatalf("expected customer_display_name Acme Corp, got %q", got)
	}
	if got, _ := resp["invoice_number"].(string); got != "INV-123" {
		t.Fatalf("expected invoice_number INV-123, got %q", got)
	}
	if got, _ := resp["billing_entity_code"].(string); got != "be_default" {
		t.Fatalf("expected billing_entity_code be_default, got %q", got)
	}
	if got, _ := resp["payment_status"].(string); got != "failed" {
		t.Fatalf("expected payment_status failed, got %q", got)
	}
}
