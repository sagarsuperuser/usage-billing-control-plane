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
	workspaceBilling, ok := tenant["workspace_billing"].(map[string]any)
	if !ok {
		t.Fatalf("expected workspace_billing object in tenant response")
	}
	if got, _ := workspaceBilling["active_billing_connection_id"].(string); got != connection.ID {
		t.Fatalf("expected workspace_billing.active_billing_connection_id %q, got %q", connection.ID, got)
	}
	if connected, _ := workspaceBilling["connected"].(bool); !connected {
		t.Fatalf("expected workspace_billing.connected=true")
	}

	got := getJSON(t, ts.URL+"/internal/tenants/tenant_conn_assign", "platform-admin", http.StatusOK)
	if got["billing_provider_connection_id"] != connection.ID {
		t.Fatalf("expected persisted billing_provider_connection_id %q, got %#v", connection.ID, got["billing_provider_connection_id"])
	}
	binding, err := repo.GetWorkspaceBillingBinding("tenant_conn_assign")
	if err != nil {
		t.Fatalf("get workspace billing binding: %v", err)
	}
	if binding.BillingProviderConnectionID != connection.ID {
		t.Fatalf("expected binding connection id %q, got %q", connection.ID, binding.BillingProviderConnectionID)
	}
}

func TestTenantResponseUsesEffectiveWorkspaceBillingWhenTenantFieldIsEmpty(t *testing.T) {
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
		ID:                 "bpc_workspace_effective_only",
		ProviderType:       domain.BillingProviderTypeStripe,
		Environment:        "test",
		DisplayName:        "Stripe Effective",
		Scope:              domain.BillingProviderConnectionScopePlatform,
		Status:             domain.BillingProviderConnectionStatusConnected,
		LagoOrganizationID: "org_effective",
		LagoProviderCode:   "stripe_effective",
		SecretRef:          "memory://billing-provider-connections/bpc_workspace_effective_only/seed",
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

	_, err = repo.CreateTenant(domain.Tenant{
		ID:        "tenant_workspace_effective_only",
		Name:      "Tenant Workspace Effective Only",
		Status:    domain.TenantStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	_, err = repo.CreateWorkspaceBillingBinding(domain.WorkspaceBillingBinding{
		ID:                          "wbb_workspace_effective_only",
		WorkspaceID:                 "tenant_workspace_effective_only",
		BillingProviderConnectionID: connection.ID,
		Backend:                     domain.WorkspaceBillingBackendLago,
		BackendOrganizationID:       connection.LagoOrganizationID,
		BackendProviderCode:         connection.LagoProviderCode,
		IsolationMode:               domain.WorkspaceBillingIsolationModeShared,
		Status:                      domain.WorkspaceBillingBindingStatusConnected,
		ConnectedAt:                 &connectedAt,
		CreatedByType:               "platform_api_key",
		CreatedByID:                 "pkey_seed",
		CreatedAt:                   now,
		UpdatedAt:                   now,
	})
	if err != nil {
		t.Fatalf("create workspace billing binding: %v", err)
	}

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	ts := httptest.NewServer(api.NewServer(repo, api.WithAPIKeyAuthorizer(authorizer)).Handler())
	defer ts.Close()

	got := getJSON(t, ts.URL+"/internal/tenants/tenant_workspace_effective_only", "platform-admin", http.StatusOK)
	workspaceBilling, ok := got["workspace_billing"].(map[string]any)
	if !ok {
		t.Fatalf("expected workspace_billing object in tenant response")
	}
	if configured, _ := workspaceBilling["configured"].(bool); !configured {
		t.Fatalf("expected workspace billing configured=true when effective binding exists")
	}
	if gotID, _ := workspaceBilling["active_billing_connection_id"].(string); gotID != connection.ID {
		t.Fatalf("expected workspace billing active connection %q, got %q", connection.ID, gotID)
	}
	if connected, _ := workspaceBilling["connected"].(bool); !connected {
		t.Fatalf("expected workspace billing connected=true when effective binding exists")
	}
	if status, _ := workspaceBilling["status"].(string); status != string(domain.WorkspaceBillingBindingStatusConnected) {
		t.Fatalf("expected workspace billing status %q, got %q", domain.WorkspaceBillingBindingStatusConnected, status)
	}
}

func TestTenantResponseDoesNotMarkVerificationFailedWorkspaceBillingConnected(t *testing.T) {
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
		ID:                 "bpc_workspace_verification_failed",
		ProviderType:       domain.BillingProviderTypeStripe,
		Environment:        "test",
		DisplayName:        "Stripe Verification Failed",
		Scope:              domain.BillingProviderConnectionScopePlatform,
		Status:             domain.BillingProviderConnectionStatusConnected,
		LagoOrganizationID: "org_failed",
		LagoProviderCode:   "stripe_failed",
		SecretRef:          "memory://billing-provider-connections/bpc_workspace_verification_failed/seed",
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

	_, err = repo.CreateTenant(domain.Tenant{
		ID:        "tenant_workspace_verification_failed",
		Name:      "Tenant Workspace Verification Failed",
		Status:    domain.TenantStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	_, err = repo.CreateWorkspaceBillingBinding(domain.WorkspaceBillingBinding{
		ID:                          "wbb_workspace_verification_failed",
		WorkspaceID:                 "tenant_workspace_verification_failed",
		BillingProviderConnectionID: connection.ID,
		Backend:                     domain.WorkspaceBillingBackendLago,
		BackendOrganizationID:       connection.LagoOrganizationID,
		BackendProviderCode:         connection.LagoProviderCode,
		IsolationMode:               domain.WorkspaceBillingIsolationModeShared,
		Status:                      domain.WorkspaceBillingBindingStatusVerificationFailed,
		ProvisioningError:           "provider verification failed",
		CreatedByType:               "platform_api_key",
		CreatedByID:                 "pkey_seed",
		CreatedAt:                   now,
		UpdatedAt:                   now,
	})
	if err != nil {
		t.Fatalf("create workspace billing binding: %v", err)
	}

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	ts := httptest.NewServer(api.NewServer(repo, api.WithAPIKeyAuthorizer(authorizer)).Handler())
	defer ts.Close()

	got := getJSON(t, ts.URL+"/internal/tenants/tenant_workspace_verification_failed", "platform-admin", http.StatusOK)
	workspaceBilling, ok := got["workspace_billing"].(map[string]any)
	if !ok {
		t.Fatalf("expected workspace_billing object in tenant response")
	}
	if configured, _ := workspaceBilling["configured"].(bool); !configured {
		t.Fatalf("expected workspace billing configured=true when binding exists")
	}
	if connected, _ := workspaceBilling["connected"].(bool); connected {
		t.Fatalf("expected workspace billing connected=false when binding verification failed")
	}
	if status, _ := workspaceBilling["status"].(string); status != string(domain.WorkspaceBillingBindingStatusVerificationFailed) {
		t.Fatalf("expected workspace billing status %q, got %q", domain.WorkspaceBillingBindingStatusVerificationFailed, status)
	}
	if code, _ := workspaceBilling["diagnosis_code"].(string); code != "verification_failed" {
		t.Fatalf("expected diagnosis_code verification_failed, got %q", code)
	}
	if summary, _ := workspaceBilling["diagnosis_summary"].(string); summary == "" {
		t.Fatalf("expected diagnosis_summary in workspace billing response")
	}
}

func TestTenantOnboardingStatusMarksVerificationFailedBillingPending(t *testing.T) {
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
		ID:                 "bpc_onboarding_verification_failed",
		ProviderType:       domain.BillingProviderTypeStripe,
		Environment:        "test",
		DisplayName:        "Stripe Verification Failed",
		Scope:              domain.BillingProviderConnectionScopePlatform,
		Status:             domain.BillingProviderConnectionStatusConnected,
		LagoOrganizationID: "org_failed",
		LagoProviderCode:   "stripe_failed",
		SecretRef:          "memory://billing-provider-connections/bpc_onboarding_verification_failed/seed",
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

	_, err = repo.CreateTenant(domain.Tenant{
		ID:        "tenant_onboarding_verification_failed",
		Name:      "Tenant Onboarding Verification Failed",
		Status:    domain.TenantStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	_, err = repo.CreateWorkspaceBillingBinding(domain.WorkspaceBillingBinding{
		ID:                          "wbb_onboarding_verification_failed",
		WorkspaceID:                 "tenant_onboarding_verification_failed",
		BillingProviderConnectionID: connection.ID,
		Backend:                     domain.WorkspaceBillingBackendLago,
		BackendOrganizationID:       connection.LagoOrganizationID,
		BackendProviderCode:         connection.LagoProviderCode,
		IsolationMode:               domain.WorkspaceBillingIsolationModeShared,
		Status:                      domain.WorkspaceBillingBindingStatusVerificationFailed,
		ProvisioningError:           "provider verification failed",
		CreatedByType:               "platform_api_key",
		CreatedByID:                 "pkey_seed",
		CreatedAt:                   now,
		UpdatedAt:                   now,
	})
	if err != nil {
		t.Fatalf("create workspace billing binding: %v", err)
	}

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	ts := httptest.NewServer(api.NewServer(repo, api.WithAPIKeyAuthorizer(authorizer)).Handler())
	defer ts.Close()

	got := getJSON(t, ts.URL+"/internal/onboarding/tenants/tenant_onboarding_verification_failed", "platform-admin", http.StatusOK)
	readiness, ok := got["readiness"].(map[string]any)
	if !ok {
		t.Fatalf("expected readiness object")
	}
	billingReadiness, ok := readiness["billing_integration"].(map[string]any)
	if !ok {
		t.Fatalf("expected billing_integration readiness object")
	}
	if connected, _ := billingReadiness["billing_connected"].(bool); connected {
		t.Fatalf("expected billing_connected=false when workspace billing verification failed")
	}
	if status, _ := billingReadiness["status"].(string); status != "pending" {
		t.Fatalf("expected billing integration status pending when verification failed, got %q", status)
	}
	if workspaceStatus, _ := billingReadiness["workspace_billing_status"].(string); workspaceStatus != string(domain.WorkspaceBillingBindingStatusVerificationFailed) {
		t.Fatalf("expected workspace_billing_status %q, got %q", domain.WorkspaceBillingBindingStatusVerificationFailed, workspaceStatus)
	}
	if code, _ := billingReadiness["diagnosis_code"].(string); code != "verification_failed" {
		t.Fatalf("expected diagnosis_code verification_failed, got %q", code)
	}
	if nextAction, _ := billingReadiness["next_action"].(string); nextAction == "" {
		t.Fatalf("expected next_action in billing readiness")
	}
	missingSteps, ok := billingReadiness["missing_steps"].([]any)
	if !ok {
		t.Fatalf("expected billing missing_steps array")
	}
	found := false
	for _, raw := range missingSteps {
		if step, _ := raw.(string); step == "billing_verification" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected billing_verification missing step, got %v", missingSteps)
	}
}

func TestTenantWorkspaceBillingSubresourceUpdatesActiveConnection(t *testing.T) {
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
	connectionA, err := repo.CreateBillingProviderConnection(domain.BillingProviderConnection{
		ID:                 "bpc_workspace_billing_a",
		ProviderType:       domain.BillingProviderTypeStripe,
		Environment:        "test",
		DisplayName:        "Stripe A",
		Scope:              domain.BillingProviderConnectionScopePlatform,
		Status:             domain.BillingProviderConnectionStatusConnected,
		LagoOrganizationID: "org_a",
		LagoProviderCode:   "stripe_a",
		SecretRef:          "memory://billing-provider-connections/bpc_workspace_billing_a/seed",
		ConnectedAt:        &connectedAt,
		LastSyncedAt:       &lastSyncedAt,
		CreatedByType:      "platform_api_key",
		CreatedByID:        "pkey_seed",
		CreatedAt:          now,
		UpdatedAt:          now,
	})
	if err != nil {
		t.Fatalf("create billing provider connection A: %v", err)
	}
	connectionB, err := repo.CreateBillingProviderConnection(domain.BillingProviderConnection{
		ID:                 "bpc_workspace_billing_b",
		ProviderType:       domain.BillingProviderTypeStripe,
		Environment:        "test",
		DisplayName:        "Stripe B",
		Scope:              domain.BillingProviderConnectionScopePlatform,
		Status:             domain.BillingProviderConnectionStatusConnected,
		LagoOrganizationID: "org_b",
		LagoProviderCode:   "stripe_b",
		SecretRef:          "memory://billing-provider-connections/bpc_workspace_billing_b/seed",
		ConnectedAt:        &connectedAt,
		LastSyncedAt:       &lastSyncedAt,
		CreatedByType:      "platform_api_key",
		CreatedByID:        "pkey_seed",
		CreatedAt:          now,
		UpdatedAt:          now,
	})
	if err != nil {
		t.Fatalf("create billing provider connection B: %v", err)
	}

	_, err = repo.CreateTenant(domain.Tenant{
		ID:                          "tenant_workspace_billing",
		Name:                        "Tenant Workspace Billing",
		Status:                      domain.TenantStatusActive,
		BillingProviderConnectionID: connectionA.ID,
		LagoOrganizationID:          connectionA.LagoOrganizationID,
		LagoBillingProviderCode:     connectionA.LagoProviderCode,
		CreatedAt:                   now,
		UpdatedAt:                   now,
	})
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	_, err = repo.CreateWorkspaceBillingBinding(domain.WorkspaceBillingBinding{
		ID:                          "wbb_workspace_billing",
		WorkspaceID:                 "tenant_workspace_billing",
		BillingProviderConnectionID: connectionA.ID,
		Backend:                     domain.WorkspaceBillingBackendLago,
		BackendOrganizationID:       connectionA.LagoOrganizationID,
		BackendProviderCode:         connectionA.LagoProviderCode,
		IsolationMode:               domain.WorkspaceBillingIsolationModeShared,
		Status:                      domain.WorkspaceBillingBindingStatusConnected,
		ConnectedAt:                 &connectedAt,
		CreatedByType:               "platform_api_key",
		CreatedByID:                 "pkey_seed",
		CreatedAt:                   now,
		UpdatedAt:                   now,
	})
	if err != nil {
		t.Fatalf("create workspace billing binding: %v", err)
	}

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	ts := httptest.NewServer(api.NewServer(repo, api.WithAPIKeyAuthorizer(authorizer)).Handler())
	defer ts.Close()

	result := patchJSON(t, ts.URL+"/internal/tenants/tenant_workspace_billing/workspace-billing", map[string]any{
		"billing_provider_connection_id": connectionB.ID,
	}, "platform-admin", http.StatusOK)
	tenant := result["tenant"].(map[string]any)
	workspaceBilling := tenant["workspace_billing"].(map[string]any)
	if got, _ := workspaceBilling["active_billing_connection_id"].(string); got != connectionB.ID {
		t.Fatalf("expected workspace billing connection to switch to %q, got %q", connectionB.ID, got)
	}
	if connected, _ := workspaceBilling["connected"].(bool); !connected {
		t.Fatalf("expected workspace billing to remain connected")
	}

	binding, err := repo.GetWorkspaceBillingBinding("tenant_workspace_billing")
	if err != nil {
		t.Fatalf("get workspace billing binding: %v", err)
	}
	if binding.BillingProviderConnectionID != connectionB.ID {
		t.Fatalf("expected binding connection id %q, got %q", connectionB.ID, binding.BillingProviderConnectionID)
	}
}
