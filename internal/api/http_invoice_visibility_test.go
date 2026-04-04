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
	paymentStatusSvc := service.NewPaymentStatusService(repo, nil)

	ts := httptest.NewServer(api.NewServer(
		repo,
		api.WithAPIKeyAuthorizer(authorizer),
		api.WithPaymentStatusService(paymentStatusSvc),
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
	policy, err := repo.UpsertDunningPolicy(domain.DunningPolicy{
		TenantID:                       "default",
		Name:                           "Default dunning policy",
		Enabled:                        true,
		RetrySchedule:                  []string{"1d"},
		MaxRetryAttempts:               3,
		CollectPaymentReminderSchedule: []string{"0d", "2d"},
		FinalAction:                    domain.DunningFinalActionManualReview,
		CreatedAt:                      now,
		UpdatedAt:                      now,
	})
	if err != nil {
		t.Fatalf("upsert dunning policy: %v", err)
	}
	run, err := repo.CreateInvoiceDunningRun(domain.InvoiceDunningRun{
		TenantID:           "default",
		InvoiceID:          "inv_123",
		CustomerExternalID: "cust_123",
		PolicyID:           policy.ID,
		State:              domain.DunningRunStateAwaitingPaymentSetup,
		Reason:             "payment_setup_pending",
		AttemptCount:       1,
		NextActionType:     domain.DunningActionTypeCollectPaymentReminder,
		NextActionAt:       ptrTime(now.Add(2 * time.Hour)),
		CreatedAt:          now,
		UpdatedAt:          now,
	})
	if err != nil {
		t.Fatalf("create dunning run: %v", err)
	}
	if _, err := repo.CreateInvoiceDunningEvent(domain.InvoiceDunningEvent{
		RunID:              run.ID,
		TenantID:           "default",
		InvoiceID:          "inv_123",
		CustomerExternalID: "cust_123",
		EventType:          domain.DunningEventTypeNotificationSent,
		State:              run.State,
		ActionType:         domain.DunningActionTypeCollectPaymentReminder,
		AttemptCount:       run.AttemptCount,
		CreatedAt:          now.Add(time.Minute),
	}); err != nil {
		t.Fatalf("create dunning event: %v", err)
	}
	if _, err := repo.CreateDunningNotificationIntent(domain.DunningNotificationIntent{
		RunID:              run.ID,
		TenantID:           "default",
		InvoiceID:          "inv_123",
		CustomerExternalID: "cust_123",
		IntentType:         domain.DunningNotificationIntentTypePaymentMethodRequired,
		ActionType:         domain.DunningActionTypeCollectPaymentReminder,
		Status:             domain.DunningNotificationIntentStatusDispatched,
		DeliveryBackend:    "alpha_email",
		RecipientEmail:     "billing@acme.test",
		CreatedAt:          now,
		DispatchedAt:       ptrTime(now.Add(time.Minute)),
	}); err != nil {
		t.Fatalf("create dunning notification intent: %v", err)
	}

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}
	paymentStatusSvc := service.NewPaymentStatusService(repo, nil)

	ts := httptest.NewServer(api.NewServer(
		repo,
		api.WithAPIKeyAuthorizer(authorizer),
		api.WithPaymentStatusService(paymentStatusSvc),
		api.WithInvoiceBillingAdapter(service.NewStripeInvoiceBillingAdapter(repo, nil, nil)),
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
	lifecycle, ok := resp["lifecycle"].(map[string]any)
	if !ok {
		t.Fatalf("expected lifecycle object in invoice detail")
	}
	if got, _ := lifecycle["recommended_action"].(string); got != "collect_payment" {
		t.Fatalf("expected recommended_action collect_payment, got %q", got)
	}
	if got, _ := lifecycle["last_event_type"].(string); got != "invoice.payment_failure" {
		t.Fatalf("expected last_event_type invoice.payment_failure, got %q", got)
	}
	dunning, ok := resp["dunning"].(map[string]any)
	if !ok {
		t.Fatalf("expected dunning object in invoice detail")
	}
	if got, _ := dunning["state"].(string); got != "awaiting_payment_setup" {
		t.Fatalf("expected dunning state awaiting_payment_setup, got %q", got)
	}
	if got, _ := dunning["last_event_type"].(string); got != "notification_sent" {
		t.Fatalf("expected last_event_type notification_sent, got %q", got)
	}
}

func TestInvoicePaymentReceiptsEndpointReturnsLinkedReceipts(t *testing.T) {
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

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	ts := httptest.NewServer(api.NewServer(
		repo,
		api.WithAPIKeyAuthorizer(authorizer),
		api.WithInvoiceBillingAdapter(service.NewStripeInvoiceBillingAdapter(repo, nil, nil)),
	).Handler())
	defer ts.Close()

	resp := getJSON(t, ts.URL+"/v1/invoices/inv_123/payment-receipts", "tenant-a-reader", http.StatusOK)
	items, ok := resp["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected one payment receipt item, got %#v", resp["items"])
	}
	row, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("expected payment receipt row map, got %#v", items[0])
	}
	if got, _ := row["id"].(string); got != "pr_123" {
		t.Fatalf("expected payment receipt id pr_123, got %q", got)
	}
	if got, _ := row["payment_status"].(string); got != "succeeded" {
		t.Fatalf("expected payment_status succeeded, got %q", got)
	}
}

func TestInvoiceCreditNotesEndpointReturnsInvoiceScopedNotes(t *testing.T) {
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

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	ts := httptest.NewServer(api.NewServer(
		repo,
		api.WithAPIKeyAuthorizer(authorizer),
		api.WithInvoiceBillingAdapter(service.NewStripeInvoiceBillingAdapter(repo, nil, nil)),
	).Handler())
	defer ts.Close()

	resp := getJSON(t, ts.URL+"/v1/invoices/inv_123/credit-notes", "tenant-a-reader", http.StatusOK)
	items, ok := resp["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected one credit note item, got %#v", resp["items"])
	}
	row, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("expected credit note row map, got %#v", items[0])
	}
	if got, _ := row["id"].(string); got != "cn_123" {
		t.Fatalf("expected credit note id cn_123, got %q", got)
	}
	if got, _ := row["invoice_id"].(string); got != "inv_123" {
		t.Fatalf("expected invoice_id inv_123, got %q", got)
	}
}
