package api_test

import (
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"usage-billing-control-plane/internal/api"
	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/service"
	"usage-billing-control-plane/internal/store"
)

func TestPaymentsListEndpointReturnsNormalizedSummaries(t *testing.T) {
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
			'default', 'org_test', 'inv_pay_123', 'cust_123', 'INV-123', 'USD',
			'finalized', 'failed', true, 12500, 12500,
			0, 'card_declined', 'invoice.payment_failure', $1, 'whk_inv_123', $2
		)
	`, now, now); err != nil {
		t.Fatalf("insert payment projection: %v", err)
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

	resp := getJSON(t, ts.URL+"/v1/payments?customer_external_id=cust_123&payment_status=failed", "tenant-a-reader", http.StatusOK)
	items, ok := resp["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected one payment item, got %#v", resp["items"])
	}
	row, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("expected payment row map, got %#v", items[0])
	}
	if got, _ := row["invoice_id"].(string); got != "inv_pay_123" {
		t.Fatalf("expected invoice_id inv_pay_123, got %q", got)
	}
	if got, _ := row["customer_display_name"].(string); got != "Acme Corp" {
		t.Fatalf("expected customer_display_name Acme Corp, got %q", got)
	}
	if got, _ := row["payment_status"].(string); got != "failed" {
		t.Fatalf("expected payment_status failed, got %q", got)
	}
}

func TestPaymentsListEndpointSupportsExtendedFiltersAndCSV(t *testing.T) {
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
	if _, err := db.Exec(`
		INSERT INTO invoice_payment_status_views (
			tenant_id, organization_id, invoice_id, customer_external_id, invoice_number, currency,
			invoice_status, payment_status, payment_overdue, total_amount_cents, total_due_amount_cents,
			total_paid_amount_cents, last_payment_error, last_event_type, last_event_at, last_webhook_key, updated_at
		) VALUES
		(
			'default', 'org_test', 'inv_pay_123', 'cust_123', 'INV-123', 'USD',
			'finalized', 'failed', true, 12500, 12500,
			0, 'card_declined', 'invoice.payment_failure', $1, 'whk_inv_123', $2
		),
		(
			'default', 'org_test', 'inv_pay_999', 'cust_999', 'INV-999', 'USD',
			'finalized', 'succeeded', false, 6400, 0,
			6400, '', 'invoice.payment_succeeded', $1, 'whk_inv_999', $2
		)
	`, now, now); err != nil {
		t.Fatalf("insert payment projections: %v", err)
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

	resp := getJSON(t, ts.URL+"/v1/payments?invoice_number=INV-123&last_event_type=invoice.payment_failure", "tenant-a-reader", http.StatusOK)
	items, ok := resp["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected one filtered payment item, got %#v", resp["items"])
	}
	row, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("expected payment row map, got %#v", items[0])
	}
	if got, _ := row["invoice_id"].(string); got != "inv_pay_123" {
		t.Fatalf("expected filtered invoice_id inv_pay_123, got %q", got)
	}

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/v1/payments?invoice_number=INV-123&format=csv", nil)
	if err != nil {
		t.Fatalf("new csv request: %v", err)
	}
	req.Header.Set("X-API-Key", "tenant-a-reader")
	csvResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do csv request: %v", err)
	}
	defer csvResp.Body.Close()
	if csvResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(csvResp.Body)
		t.Fatalf("expected csv status 200, got %d body=%s", csvResp.StatusCode, string(body))
	}
	if !strings.Contains(strings.ToLower(csvResp.Header.Get("Content-Type")), "text/csv") {
		t.Fatalf("expected text/csv content type, got %q", csvResp.Header.Get("Content-Type"))
	}
	body, err := io.ReadAll(csvResp.Body)
	if err != nil {
		t.Fatalf("read csv body: %v", err)
	}
	csvBody := string(body)
	if !strings.Contains(csvBody, "invoice_id,invoice_number") {
		t.Fatalf("expected csv header row, got %q", csvBody)
	}
	if !strings.Contains(csvBody, "inv_pay_123,INV-123") {
		t.Fatalf("expected filtered csv row for inv_pay_123, got %q", csvBody)
	}
	if strings.Contains(csvBody, "inv_pay_999") {
		t.Fatalf("expected csv to exclude inv_pay_999, got %q", csvBody)
	}
}

func TestPaymentDetailEndpointReturnsLifecycleAndEvents(t *testing.T) {
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
			'default', 'org_test', 'inv_pay_123', 'cust_123', 'INV-123', 'USD',
			'finalized', 'failed', true, 12500, 12500,
			0, 'card_declined', 'invoice.payment_failure', $1, 'whk_inv_123', $2
		)
	`, now, now); err != nil {
		t.Fatalf("insert payment projection: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO lago_webhook_events (
			id, tenant_id, organization_id, webhook_key, webhook_type, object_type, invoice_id,
			customer_external_id, invoice_number, currency, invoice_status, payment_status,
			payment_overdue, total_amount_cents, total_due_amount_cents, total_paid_amount_cents,
			last_payment_error, payload, received_at, occurred_at
		) VALUES (
			'evt_1', 'default', 'org_test', 'whk_evt_1', 'invoice.payment_failure', 'invoice', 'inv_pay_123',
			'cust_123', 'INV-123', 'USD', 'finalized', 'failed',
			true, 12500, 12500, 0,
			'card_declined', '{}'::jsonb, $1, $2
		)
	`, now, now); err != nil {
		t.Fatalf("insert webhook event: %v", err)
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

	detail := getJSON(t, ts.URL+"/v1/payments/inv_pay_123", "tenant-a-reader", http.StatusOK)
	if got, _ := detail["invoice_id"].(string); got != "inv_pay_123" {
		t.Fatalf("expected invoice_id inv_pay_123, got %q", got)
	}
	if got, _ := detail["customer_display_name"].(string); got != "Acme Corp" {
		t.Fatalf("expected customer_display_name Acme Corp, got %q", got)
	}
	lifecycle, ok := detail["lifecycle"].(map[string]any)
	if !ok {
		t.Fatalf("expected lifecycle object in payment detail")
	}
	if got, _ := lifecycle["recommended_action"].(string); got != "retry_payment" {
		t.Fatalf("expected recommended_action retry_payment, got %q", got)
	}
	if got, _ := lifecycle["requires_action"].(bool); !got {
		t.Fatalf("expected requires_action true")
	}

	events := getJSON(t, ts.URL+"/v1/payments/inv_pay_123/events", "tenant-a-reader", http.StatusOK)
	items, ok := events["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected one payment event, got %#v", events["items"])
	}
}
