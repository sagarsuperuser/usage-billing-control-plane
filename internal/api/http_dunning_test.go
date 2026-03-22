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

func TestDunningPolicyAndRunEndpoints(t *testing.T) {
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
	mustCreateAPIKey(t, repo, "tenant-a-writer", api.RoleWriter, "default")
	now := time.Now().UTC().Truncate(time.Second)

	policy, err := repo.UpsertDunningPolicy(domain.DunningPolicy{
		TenantID:                       "default",
		Name:                           "Default dunning policy",
		Enabled:                        true,
		RetrySchedule:                  []string{"1d", "3d"},
		MaxRetryAttempts:               2,
		CollectPaymentReminderSchedule: []string{"0d", "2d"},
		FinalAction:                    domain.DunningFinalActionManualReview,
		GracePeriodDays:                1,
		CreatedAt:                      now,
		UpdatedAt:                      now,
	})
	if err != nil {
		t.Fatalf("upsert dunning policy: %v", err)
	}

	run, err := repo.CreateInvoiceDunningRun(domain.InvoiceDunningRun{
		TenantID:           "default",
		InvoiceID:          "inv_dunning_123",
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
		InvoiceID:          run.InvoiceID,
		CustomerExternalID: run.CustomerExternalID,
		EventType:          domain.DunningEventTypeStarted,
		State:              run.State,
		ActionType:         run.NextActionType,
		AttemptCount:       run.AttemptCount,
		CreatedAt:          now,
	}); err != nil {
		t.Fatalf("create dunning event: %v", err)
	}
	if _, err := repo.CreateDunningNotificationIntent(domain.DunningNotificationIntent{
		RunID:              run.ID,
		TenantID:           "default",
		InvoiceID:          run.InvoiceID,
		CustomerExternalID: run.CustomerExternalID,
		IntentType:         domain.DunningNotificationIntentTypePaymentMethodRequired,
		ActionType:         domain.DunningActionTypeCollectPaymentReminder,
		Status:             domain.DunningNotificationIntentStatusQueued,
		CreatedAt:          now,
	}); err != nil {
		t.Fatalf("create dunning notification intent: %v", err)
	}

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	ts := httptest.NewServer(api.NewServer(
		repo,
		api.WithAPIKeyAuthorizer(authorizer),
	).Handler())
	defer ts.Close()

	getResp := getJSON(t, ts.URL+"/v1/dunning/policy", "tenant-a-reader", http.StatusOK)
	gotPolicy, ok := getResp["policy"].(map[string]any)
	if !ok {
		t.Fatalf("expected policy object, got %#v", getResp["policy"])
	}
	if got, _ := gotPolicy["name"].(string); got != "Default dunning policy" {
		t.Fatalf("expected policy name, got %q", got)
	}

	putResp := putJSON(t, ts.URL+"/v1/dunning/policy", map[string]any{
		"name":                              "Collections policy",
		"enabled":                           true,
		"retry_schedule":                    []string{"2d", "4d"},
		"max_retry_attempts":                4,
		"collect_payment_reminder_schedule": []string{"0d", "1d", "3d"},
		"final_action":                      "pause",
		"grace_period_days":                 2,
	}, "tenant-a-writer", http.StatusOK)
	updatedPolicy, ok := putResp["policy"].(map[string]any)
	if !ok {
		t.Fatalf("expected updated policy object")
	}
	if got, _ := updatedPolicy["name"].(string); got != "Collections policy" {
		t.Fatalf("expected updated policy name, got %q", got)
	}
	if got, _ := updatedPolicy["final_action"].(string); got != "pause" {
		t.Fatalf("expected final_action pause, got %q", got)
	}

	listResp := getJSON(t, ts.URL+"/v1/dunning/runs?customer_external_id=cust_123&active_only=true", "tenant-a-reader", http.StatusOK)
	items, ok := listResp["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected one dunning run, got %#v", listResp["items"])
	}
	listedRun, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("expected run object")
	}
	if got, _ := listedRun["id"].(string); got != run.ID {
		t.Fatalf("expected run id %q, got %q", run.ID, got)
	}

	detailResp := getJSON(t, ts.URL+"/v1/dunning/runs/"+run.ID, "tenant-a-reader", http.StatusOK)
	detailRun, ok := detailResp["run"].(map[string]any)
	if !ok {
		t.Fatalf("expected run detail object")
	}
	if got, _ := detailRun["invoice_id"].(string); got != run.InvoiceID {
		t.Fatalf("expected invoice_id %q, got %q", run.InvoiceID, got)
	}
	events, ok := detailResp["events"].([]any)
	if !ok || len(events) != 1 {
		t.Fatalf("expected one dunning event, got %#v", detailResp["events"])
	}
	intents, ok := detailResp["notification_intents"].([]any)
	if !ok || len(intents) != 1 {
		t.Fatalf("expected one notification intent, got %#v", detailResp["notification_intents"])
	}
}

func TestDunningCollectPaymentReminderDispatchEndpoint(t *testing.T) {
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

	mustCreateAPIKey(t, repo, "customer-reader-key", api.RoleReader, "default")
	mustCreateAPIKey(t, repo, "customer-writer-key", api.RoleWriter, "default")

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	customerPaymentMethodReady := false
	lagoMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/customers":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"customer":{"lago_id":"lago_cust_alpha","external_id":"cust_alpha","billing_configuration":{"payment_provider":"stripe","payment_provider_code":"stripe_test","provider_customer_id":"pcus_123","provider_payment_methods":["card"]}}}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/customers/cust_alpha/payment_methods":
			w.Header().Set("Content-Type", "application/json")
			if customerPaymentMethodReady {
				_, _ = w.Write([]byte(`{"payment_methods":[{"lago_id":"pm_lago_alpha","is_default":true,"provider_method_id":"pm_123"}]}`))
				return
			}
			_, _ = w.Write([]byte(`{"payment_methods":[]}`))
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/customers/cust_alpha/checkout_url":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"customer":{"external_customer_id":"cust_alpha","checkout_url":"https://checkout.example.test/cust_alpha"}}`))
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer lagoMock.Close()

	lagoTransport, err := service.NewLagoHTTPTransport(service.LagoClientConfig{
		BaseURL: lagoMock.URL,
		APIKey:  "test-api-key",
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}
	mustSetTenantMappings(t, repo, "default", "org_default", "stripe_test")

	emailSender := &fakeCustomerPaymentSetupRequestEmailSender{}
	ts := httptest.NewServer(api.NewServer(
		repo,
		api.WithAPIKeyAuthorizer(authorizer),
		api.WithCustomerBillingAdapter(service.NewLagoCustomerBillingAdapter(lagoTransport)),
		api.WithNotificationService(service.NewNotificationService(nil, nil, emailSender, nil)),
	).Handler())
	defer ts.Close()

	_ = postJSON(t, ts.URL+"/v1/customers", map[string]any{
		"external_id":  "cust_alpha",
		"display_name": "Alpha Co",
		"email":        "billing@alpha.test",
	}, "customer-writer-key", http.StatusCreated)
	_ = putJSON(t, ts.URL+"/v1/customers/cust_alpha/billing-profile", map[string]any{
		"legal_name":            "Alpha Company Pvt Ltd",
		"email":                 "billing@alpha.test",
		"billing_address_line1": "1 Billing St",
		"billing_city":          "Bengaluru",
		"billing_postal_code":   "560001",
		"billing_country":       "IN",
		"currency":              "USD",
		"provider_code":         "stripe_test",
	}, "customer-writer-key", http.StatusOK)

	now := time.Now().UTC().Truncate(time.Second)
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
		InvoiceID:          "inv_dispatch_123",
		CustomerExternalID: "cust_alpha",
		PolicyID:           policy.ID,
		State:              domain.DunningRunStateAwaitingPaymentSetup,
		Reason:             "payment_setup_pending",
		NextActionType:     domain.DunningActionTypeCollectPaymentReminder,
		NextActionAt:       ptrTime(now.Add(6 * time.Hour)),
		CreatedAt:          now,
		UpdatedAt:          now,
	})
	if err != nil {
		t.Fatalf("create dunning run: %v", err)
	}

	resp := postJSON(t, ts.URL+"/v1/dunning/runs/"+run.ID+"/collect-payment-reminder", map[string]any{}, "customer-writer-key", http.StatusOK)
	intent, ok := resp["notification_intent"].(map[string]any)
	if !ok {
		t.Fatalf("expected notification_intent object")
	}
	if got, _ := intent["status"].(string); got != "dispatched" {
		t.Fatalf("expected dispatched notification intent, got %q", got)
	}
	dispatchEvent, ok := resp["dispatch_event"].(map[string]any)
	if !ok {
		t.Fatalf("expected dispatch_event object")
	}
	if got, _ := dispatchEvent["event_type"].(string); got != "notification_sent" {
		t.Fatalf("expected notification_sent event, got %q", got)
	}
	if len(emailSender.inputs) != 1 {
		t.Fatalf("expected one payment setup email, got %d", len(emailSender.inputs))
	}
	if got := emailSender.inputs[0].RequestKind; got != "resent" {
		t.Fatalf("expected resend request kind, got %q", got)
	}

	detailResp := getJSON(t, ts.URL+"/v1/dunning/runs/"+run.ID, "customer-reader-key", http.StatusOK)
	intents, ok := detailResp["notification_intents"].([]any)
	if !ok || len(intents) != 1 {
		t.Fatalf("expected one notification intent after dispatch, got %#v", detailResp["notification_intents"])
	}
	latestIntent, ok := intents[0].(map[string]any)
	if !ok {
		t.Fatalf("expected intent detail object")
	}
	if got, _ := latestIntent["status"].(string); got != "dispatched" {
		t.Fatalf("expected persisted dispatched intent, got %q", got)
	}
}

func TestDunningRetryNowEndpoint(t *testing.T) {
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

	mustCreateAPIKey(t, repo, "dunning-retry-reader", api.RoleReader, "default")
	mustCreateAPIKey(t, repo, "dunning-retry-writer", api.RoleWriter, "default")

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	lagoMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/invoices/inv_retry_123/retry_payment" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"invoice":{"lago_id":"inv_retry_123"},"status":"queued"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer lagoMock.Close()

	lagoTransport, err := service.NewLagoHTTPTransport(service.LagoClientConfig{
		BaseURL: lagoMock.URL,
		APIKey:  "test-api-key",
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	policy, err := repo.UpsertDunningPolicy(domain.DunningPolicy{
		TenantID:         "default",
		Name:             "Default dunning policy",
		Enabled:          true,
		RetrySchedule:    []string{"1d", "3d"},
		MaxRetryAttempts: 3,
		FinalAction:      domain.DunningFinalActionManualReview,
		CreatedAt:        now,
		UpdatedAt:        now,
	})
	if err != nil {
		t.Fatalf("upsert dunning policy: %v", err)
	}
	run, err := repo.CreateInvoiceDunningRun(domain.InvoiceDunningRun{
		TenantID:           "default",
		InvoiceID:          "inv_retry_123",
		CustomerExternalID: "cust_retry_123",
		PolicyID:           policy.ID,
		State:              domain.DunningRunStateRetryDue,
		Reason:             "payment_setup_ready",
		NextActionType:     domain.DunningActionTypeRetryPayment,
		NextActionAt:       ptrTime(now.Add(6 * time.Hour)),
		CreatedAt:          now,
		UpdatedAt:          now,
	})
	if err != nil {
		t.Fatalf("create dunning run: %v", err)
	}

	ts := httptest.NewServer(api.NewServer(
		repo,
		api.WithAPIKeyAuthorizer(authorizer),
		api.WithInvoiceBillingAdapter(service.NewLagoInvoiceAdapter(lagoTransport)),
	).Handler())
	defer ts.Close()

	resp := postJSON(t, ts.URL+"/v1/dunning/runs/"+run.ID+"/retry-now", map[string]any{}, "dunning-retry-writer", http.StatusOK)
	event, ok := resp["event"].(map[string]any)
	if !ok {
		t.Fatalf("expected event object")
	}
	if got, _ := event["event_type"].(string); got != "retry_attempted" {
		t.Fatalf("expected retry_attempted event, got %q", got)
	}
	runResp, ok := resp["run"].(map[string]any)
	if !ok {
		t.Fatalf("expected run object")
	}
	if got, _ := runResp["state"].(string); got != "awaiting_retry_result" {
		t.Fatalf("expected awaiting_retry_result state, got %q", got)
	}
}

func TestDunningPauseResumeResolveEndpoints(t *testing.T) {
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

	mustCreateAPIKey(t, repo, "dunning-control-reader", api.RoleReader, "default")
	mustCreateAPIKey(t, repo, "dunning-control-writer", api.RoleWriter, "default")

	now := time.Now().UTC().Truncate(time.Second)
	policy, err := repo.UpsertDunningPolicy(domain.DunningPolicy{
		TenantID:         "default",
		Name:             "Default dunning policy",
		Enabled:          true,
		RetrySchedule:    []string{"1d", "3d"},
		MaxRetryAttempts: 3,
		FinalAction:      domain.DunningFinalActionManualReview,
		CreatedAt:        now,
		UpdatedAt:        now,
	})
	if err != nil {
		t.Fatalf("upsert dunning policy: %v", err)
	}
	customer, err := repo.CreateCustomer(domain.Customer{
		TenantID:    "default",
		ExternalID:  "cust_control",
		DisplayName: "Control Customer",
		Email:       "billing@control.test",
		Status:      domain.CustomerStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		t.Fatalf("create customer: %v", err)
	}
	if _, err := repo.UpsertCustomerPaymentSetup(domain.CustomerPaymentSetup{
		CustomerID:                  customer.ID,
		TenantID:                    "default",
		SetupStatus:                 domain.PaymentSetupStatusReady,
		DefaultPaymentMethodPresent: true,
		LastVerifiedAt:              ptrTime(now),
		CreatedAt:                   now,
		UpdatedAt:                   now,
	}); err != nil {
		t.Fatalf("upsert customer payment setup: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO invoice_payment_status_views (
			tenant_id, organization_id, invoice_id, customer_external_id,
			invoice_number, currency, invoice_status, payment_status, payment_overdue,
			total_amount_cents, total_due_amount_cents, total_paid_amount_cents,
			last_payment_error, last_event_type, last_event_at, last_webhook_key, updated_at
		) VALUES (
			'default', 'org_default', 'inv_control_123', 'cust_control',
			'INV-CONTROL-123', 'USD', 'finalized', 'failed', false,
			1000, 1000, 0,
			'', 'invoice.payment_failure', $1, 'webhook_control', $1
		)
	`, now); err != nil {
		t.Fatalf("insert invoice payment status view: %v", err)
	}

	run, err := repo.CreateInvoiceDunningRun(domain.InvoiceDunningRun{
		TenantID:           "default",
		InvoiceID:          "inv_control_123",
		CustomerExternalID: "cust_control",
		PolicyID:           policy.ID,
		State:              domain.DunningRunStateRetryDue,
		Reason:             "payment_setup_ready",
		NextActionType:     domain.DunningActionTypeRetryPayment,
		NextActionAt:       ptrTime(now.Add(time.Hour)),
		CreatedAt:          now,
		UpdatedAt:          now,
	})
	if err != nil {
		t.Fatalf("create dunning run: %v", err)
	}

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	ts := httptest.NewServer(api.NewServer(
		repo,
		api.WithAPIKeyAuthorizer(authorizer),
	).Handler())
	defer ts.Close()

	pauseResp := postJSON(t, ts.URL+"/v1/dunning/runs/"+run.ID+"/pause", map[string]any{}, "dunning-control-writer", http.StatusOK)
	pauseRun, ok := pauseResp["run"].(map[string]any)
	if !ok {
		t.Fatalf("expected run object from pause")
	}
	if got, _ := pauseRun["state"].(string); got != "paused" {
		t.Fatalf("expected paused state, got %q", got)
	}

	resumeResp := postJSON(t, ts.URL+"/v1/dunning/runs/"+run.ID+"/resume", map[string]any{}, "dunning-control-writer", http.StatusOK)
	resumeRun, ok := resumeResp["run"].(map[string]any)
	if !ok {
		t.Fatalf("expected run object from resume")
	}
	if got, _ := resumeRun["state"].(string); got != "retry_due" {
		t.Fatalf("expected retry_due state, got %q", got)
	}

	resolveResp := postJSON(t, ts.URL+"/v1/dunning/runs/"+run.ID+"/resolve", map[string]any{}, "dunning-control-writer", http.StatusOK)
	resolveRun, ok := resolveResp["run"].(map[string]any)
	if !ok {
		t.Fatalf("expected run object from resolve")
	}
	if got, _ := resolveRun["state"].(string); got != "resolved" {
		t.Fatalf("expected resolved state, got %q", got)
	}
	if got, _ := resolveRun["resolution"].(string); got != "operator_resolved" {
		t.Fatalf("expected operator_resolved resolution, got %q", got)
	}
}
