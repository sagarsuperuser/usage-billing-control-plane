package api_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alexedwards/scs/postgresstore"
	"github.com/alexedwards/scs/v2"
	_ "github.com/jackc/pgx/v5/stdlib"

	"usage-billing-control-plane/internal/api"
	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/service"
	"usage-billing-control-plane/internal/store"
)

type fakeHTTPSSOProvider struct {
	definition service.BrowserSSOProviderDefinition
	claims     service.BrowserSSOClaims
}

type fakePasswordResetEmailSender struct {
	inputs []service.PasswordResetEmail
}

func (s *fakePasswordResetEmailSender) SendPasswordReset(input service.PasswordResetEmail) error {
	s.inputs = append(s.inputs, input)
	return nil
}

func (p fakeHTTPSSOProvider) Definition() service.BrowserSSOProviderDefinition {
	return p.definition
}

func (p fakeHTTPSSOProvider) BuildAuthCodeURL(state, nonce, codeChallenge, redirectURI string) (string, error) {
	return "https://idp.example.com/auth?state=" + state, nil
}

func (p fakeHTTPSSOProvider) Exchange(ctx context.Context, redirectURI, code, codeVerifier, nonce string) (service.BrowserSSOClaims, error) {
	return p.claims, nil
}

func TestUISessionLoginMeLogoutLifecycle(t *testing.T) {
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
	if _, err := db.Exec(`TRUNCATE TABLE sessions`); err != nil {
		t.Fatalf("truncate sessions: %v", err)
	}

	mustCreateBrowserUser(t, repo, browserUserFixture{
		email:    "reader@tenant-a.test",
		password: "reader password 123",
		role:     "reader",
		tenantID: "tenant_a",
	})

	sessionManager := scs.New()
	sessionManager.Store = postgresstore.New(db)
	sessionManager.Lifetime = 12 * time.Hour
	sessionManager.Cookie.Name = "test_ui_session"
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.Secure = false
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode

	handler := api.NewServer(
		repo,
		api.WithSessionManager(sessionManager),
	).Handler()
	ts := httptest.NewServer(sessionManager.LoadAndSave(handler))
	defer ts.Close()

	client := newSessionClient(t)

	loginResp := sessionPostJSON(t, client, ts.URL+"/v1/ui/sessions/login", map[string]any{
		"email":    "reader@tenant-a.test",
		"password": "reader password 123",
	}, "", http.StatusCreated)
	csrfToken, _ := loginResp["csrf_token"].(string)
	if csrfToken == "" {
		t.Fatalf("expected csrf_token in login response")
	}
	if got, _ := loginResp["tenant_id"].(string); got != "tenant_a" {
		t.Fatalf("expected tenant_id tenant_a, got %q", got)
	}
	if got, _ := loginResp["subject_type"].(string); got != "user" {
		t.Fatalf("expected subject_type user, got %q", got)
	}

	meResp := sessionGetJSON(t, client, ts.URL+"/v1/ui/sessions/me", http.StatusOK)
	if got, _ := meResp["role"].(string); got != string(api.RoleReader) {
		t.Fatalf("expected me.role reader, got %q", got)
	}

	logoutForbidden := sessionPostJSON(t, client, ts.URL+"/v1/ui/sessions/logout", map[string]any{}, "", http.StatusForbidden)
	if got, _ := logoutForbidden["error_code"].(string); got != "forbidden" {
		t.Fatalf("expected error_code forbidden in logout response, got %q", got)
	}
	if got, _ := logoutForbidden["request_id"].(string); strings.TrimSpace(got) == "" {
		t.Fatalf("expected request_id in forbidden logout response")
	}
	_ = sessionPostJSON(t, client, ts.URL+"/v1/ui/sessions/logout", map[string]any{}, csrfToken, http.StatusOK)
	_ = sessionGetJSON(t, client, ts.URL+"/v1/ui/sessions/me", http.StatusUnauthorized)
}

func TestUISessionLoginRejectsAccessKeyPayload(t *testing.T) {
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
	if _, err := db.Exec(`TRUNCATE TABLE sessions`); err != nil {
		t.Fatalf("truncate sessions: %v", err)
	}

	sessionManager := scs.New()
	sessionManager.Store = postgresstore.New(db)
	sessionManager.Lifetime = 12 * time.Hour
	sessionManager.Cookie.Name = "test_ui_session_reject_key"
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.Secure = false
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode

	handler := api.NewServer(
		repo,
		api.WithSessionManager(sessionManager),
	).Handler()
	ts := httptest.NewServer(sessionManager.LoadAndSave(handler))
	defer ts.Close()

	client := newSessionClient(t)
	resp := sessionPostJSON(t, client, ts.URL+"/v1/ui/sessions/login", map[string]any{
		"api_key": "legacy-browser-login-key",
	}, "", http.StatusBadRequest)
	if got, _ := resp["error_code"].(string); got != "bad_request" {
		t.Fatalf("expected error_code bad_request, got %q", got)
	}
}

func TestUIPlatformSessionLoginMeLogoutLifecycle(t *testing.T) {
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
	if _, err := db.Exec(`TRUNCATE TABLE sessions`); err != nil {
		t.Fatalf("truncate sessions: %v", err)
	}

	mustCreateBrowserUser(t, repo, browserUserFixture{
		email:        "platform-admin@alpha.test",
		password:     "platform password 123",
		platformRole: "platform_admin",
	})

	sessionManager := scs.New()
	sessionManager.Store = postgresstore.New(db)
	sessionManager.Lifetime = 12 * time.Hour
	sessionManager.Cookie.Name = "test_ui_platform_session"
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.Secure = false
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode

	handler := api.NewServer(
		repo,
		api.WithSessionManager(sessionManager),
	).Handler()
	ts := httptest.NewServer(sessionManager.LoadAndSave(handler))
	defer ts.Close()

	client := newSessionClient(t)

	loginResp := sessionPostJSON(t, client, ts.URL+"/v1/ui/sessions/login", map[string]any{
		"email":    "platform-admin@alpha.test",
		"password": "platform password 123",
	}, "", http.StatusCreated)
	csrfToken, _ := loginResp["csrf_token"].(string)
	if csrfToken == "" {
		t.Fatalf("expected csrf_token in login response")
	}
	if got, _ := loginResp["scope"].(string); got != "platform" {
		t.Fatalf("expected scope platform, got %q", got)
	}
	if got, _ := loginResp["platform_role"].(string); got != string(api.PlatformRoleAdmin) {
		t.Fatalf("expected platform_role platform_admin, got %q", got)
	}
	if got, _ := loginResp["subject_type"].(string); got != "user" {
		t.Fatalf("expected subject_type user, got %q", got)
	}

	meResp := sessionGetJSON(t, client, ts.URL+"/v1/ui/sessions/me", http.StatusOK)
	if got, _ := meResp["scope"].(string); got != "platform" {
		t.Fatalf("expected me.scope platform, got %q", got)
	}
	if got, _ := meResp["platform_role"].(string); got != string(api.PlatformRoleAdmin) {
		t.Fatalf("expected me.platform_role platform_admin, got %q", got)
	}

	_ = sessionPostJSON(t, client, ts.URL+"/v1/ui/sessions/logout", map[string]any{}, "", http.StatusForbidden)
	_ = sessionPostJSON(t, client, ts.URL+"/v1/ui/sessions/logout", map[string]any{}, csrfToken, http.StatusOK)
	_ = sessionGetJSON(t, client, ts.URL+"/v1/ui/sessions/me", http.StatusUnauthorized)
}

func TestUIPlatformSessionCanOnboardTenantWithBillingConnection(t *testing.T) {
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
	if _, err := db.Exec(`TRUNCATE TABLE sessions`); err != nil {
		t.Fatalf("truncate sessions: %v", err)
	}

	mustCreateBrowserUser(t, repo, browserUserFixture{
		email:        "platform-admin@alpha.test",
		password:     "platform password 123",
		platformRole: "platform_admin",
	})

	now := time.Now().UTC()
	suffix := now.Format("20060102150405.000000000")
	connectedAt := now
	lastSyncedAt := now
	connection, err := repo.CreateBillingProviderConnection(domain.BillingProviderConnection{
		ID:                 "bpc_ui_platform_onboard_" + suffix,
		ProviderType:       domain.BillingProviderTypeStripe,
		Environment:        "test",
		DisplayName:        "Stripe Platform",
		Scope:              domain.BillingProviderConnectionScopePlatform,
		Status:             domain.BillingProviderConnectionStatusConnected,
		LagoOrganizationID: "org_platform",
		LagoProviderCode:   "stripe_platform",
		SecretRef:          "memory://billing-provider-connections/bpc_ui_platform_onboard/seed",
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

	sessionManager := scs.New()
	sessionManager.Store = postgresstore.New(db)
	sessionManager.Lifetime = 12 * time.Hour
	sessionManager.Cookie.Name = "test_ui_platform_onboard_session"
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.Secure = false
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode

	handler := api.NewServer(
		repo,
		api.WithSessionManager(sessionManager),
	).Handler()
	ts := httptest.NewServer(sessionManager.LoadAndSave(handler))
	defer ts.Close()

	client := newSessionClient(t)

	loginResp := sessionPostJSON(t, client, ts.URL+"/v1/ui/sessions/login", map[string]any{
		"email":    "platform-admin@alpha.test",
		"password": "platform password 123",
	}, "", http.StatusCreated)
	csrfToken, _ := loginResp["csrf_token"].(string)
	if csrfToken == "" {
		t.Fatalf("expected csrf_token in login response")
	}

	result := sessionPostJSON(t, client, ts.URL+"/internal/onboarding/tenants", map[string]any{
		"id":                             "tenant_ui_onboard_" + strings.NewReplacer(".", "", ":", "").Replace(suffix),
		"name":                           "Tenant UI Onboard",
		"billing_provider_connection_id": connection.ID,
		"bootstrap_admin_key":            false,
	}, csrfToken, http.StatusCreated)

	tenant := result["tenant"].(map[string]any)
	if got, _ := tenant["billing_provider_connection_id"].(string); got != connection.ID {
		t.Fatalf("expected tenant billing_provider_connection_id %q, got %q", connection.ID, got)
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
}

func TestUIPlatformSessionCanUpdateExistingTenantOnboardingWithBillingConnection(t *testing.T) {
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
	if _, err := db.Exec(`TRUNCATE TABLE sessions`); err != nil {
		t.Fatalf("truncate sessions: %v", err)
	}

	mustCreateBrowserUser(t, repo, browserUserFixture{
		email:        "platform-admin@alpha.test",
		password:     "platform password 123",
		platformRole: "platform_admin",
	})
	now := time.Now().UTC()
	suffix := now.Format("20060102150405.000000000")
	tenantID := "default_" + strings.NewReplacer(".", "", ":", "").Replace(suffix)
	if _, err := repo.CreateTenant(domain.Tenant{
		ID:        tenantID,
		Name:      "Default",
		Status:    domain.TenantStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create existing tenant: %v", err)
	}

	connectedAt := now
	lastSyncedAt := now
	connection, err := repo.CreateBillingProviderConnection(domain.BillingProviderConnection{
		ID:                 "bpc_ui_platform_update_" + suffix,
		ProviderType:       domain.BillingProviderTypeStripe,
		Environment:        "test",
		DisplayName:        "Stripe Platform",
		Scope:              domain.BillingProviderConnectionScopePlatform,
		Status:             domain.BillingProviderConnectionStatusConnected,
		LagoOrganizationID: "org_platform",
		LagoProviderCode:   "stripe_platform",
		SecretRef:          "memory://billing-provider-connections/bpc_ui_platform_update/seed",
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

	sessionManager := scs.New()
	sessionManager.Store = postgresstore.New(db)
	sessionManager.Lifetime = 12 * time.Hour
	sessionManager.Cookie.Name = "test_ui_platform_update_session"
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.Secure = false
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode

	handler := api.NewServer(
		repo,
		api.WithSessionManager(sessionManager),
	).Handler()
	ts := httptest.NewServer(sessionManager.LoadAndSave(handler))
	defer ts.Close()

	client := newSessionClient(t)

	loginResp := sessionPostJSON(t, client, ts.URL+"/v1/ui/sessions/login", map[string]any{
		"email":    "platform-admin@alpha.test",
		"password": "platform password 123",
	}, "", http.StatusCreated)
	csrfToken, _ := loginResp["csrf_token"].(string)
	if csrfToken == "" {
		t.Fatalf("expected csrf_token in login response")
	}

	result := sessionPostJSON(t, client, ts.URL+"/internal/onboarding/tenants", map[string]any{
		"id":                             tenantID,
		"name":                           "Default",
		"billing_provider_connection_id": connection.ID,
		"bootstrap_admin_key":            false,
	}, csrfToken, http.StatusOK)

	tenant := result["tenant"].(map[string]any)
	if got, _ := tenant["billing_provider_connection_id"].(string); got != connection.ID {
		t.Fatalf("expected tenant billing_provider_connection_id %q, got %q", connection.ID, got)
	}
}

func TestUISessionCSRFProtectionForUnsafeMethods(t *testing.T) {
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
	if _, err := db.Exec(`TRUNCATE TABLE sessions`); err != nil {
		t.Fatalf("truncate sessions: %v", err)
	}

	mustCreateBrowserUser(t, repo, browserUserFixture{
		email:    "writer@tenant-a.test",
		password: "writer password 123",
		role:     "writer",
		tenantID: "tenant_a",
	})

	sessionManager := scs.New()
	sessionManager.Store = postgresstore.New(db)
	sessionManager.Lifetime = 12 * time.Hour
	sessionManager.Cookie.Name = "test_ui_session_csrf"
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.Secure = false
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode

	handler := api.NewServer(
		repo,
		api.WithSessionManager(sessionManager),
	).Handler()
	ts := httptest.NewServer(sessionManager.LoadAndSave(handler))
	defer ts.Close()

	client := newSessionClient(t)

	loginResp := sessionPostJSON(t, client, ts.URL+"/v1/ui/sessions/login", map[string]any{
		"email":    "writer@tenant-a.test",
		"password": "writer password 123",
	}, "", http.StatusCreated)
	csrfToken, _ := loginResp["csrf_token"].(string)
	if csrfToken == "" {
		t.Fatalf("expected csrf_token in login response")
	}

	ruleBody := map[string]any{
		"rule_key":        "csrf_rule",
		"name":            "CSRF Test Rule",
		"version":         1,
		"lifecycle_state": "active",
		"mode":            "graduated",
		"currency":        "USD",
		"graduated_tiers": []map[string]any{{"up_to": 0, "unit_amount_cents": 1}},
	}

	_ = sessionPostJSON(t, client, ts.URL+"/v1/rating-rules", ruleBody, "", http.StatusForbidden)
	_ = sessionPostJSON(t, client, ts.URL+"/v1/rating-rules", ruleBody, "wrong-token", http.StatusForbidden)
	_ = sessionPostJSON(t, client, ts.URL+"/v1/rating-rules", ruleBody, csrfToken, http.StatusCreated)
}

func TestUISessionLoginReturnsPendingWorkspaceSelectionForMultiWorkspaceUser(t *testing.T) {
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
	if _, err := db.Exec(`TRUNCATE TABLE sessions`); err != nil {
		t.Fatalf("truncate sessions: %v", err)
	}

	now := time.Now().UTC()
	user, err := repo.CreateUser(domain.User{
		Email:       "multi@tenant.test",
		DisplayName: "Multi User",
		Status:      domain.UserStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	hash, err := service.HashPassword("multi password 123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if _, err := repo.UpsertUserPasswordCredential(domain.UserPasswordCredential{
		UserID:            user.ID,
		PasswordHash:      hash,
		PasswordUpdatedAt: now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}); err != nil {
		t.Fatalf("upsert password: %v", err)
	}
	for _, tenantID := range []string{"tenant_a", "tenant_b"} {
		if _, err := repo.CreateTenant(domain.Tenant{
			ID:        tenantID,
			Name:      strings.ToUpper(tenantID),
			Status:    domain.TenantStatusActive,
			CreatedAt: now,
			UpdatedAt: now,
		}); err != nil {
			t.Fatalf("create tenant %s: %v", tenantID, err)
		}
		if _, err := repo.UpsertUserTenantMembership(domain.UserTenantMembership{
			UserID:    user.ID,
			TenantID:  tenantID,
			Role:      "admin",
			Status:    domain.UserTenantMembershipStatusActive,
			CreatedAt: now,
			UpdatedAt: now,
		}); err != nil {
			t.Fatalf("create membership %s: %v", tenantID, err)
		}
	}

	sessionManager := scs.New()
	sessionManager.Store = postgresstore.New(db)
	sessionManager.Lifetime = 12 * time.Hour
	sessionManager.Cookie.Name = "test_ui_workspace_select"
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.Secure = false
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode

	handler := api.NewServer(
		repo,
		api.WithSessionManager(sessionManager),
		api.WithUIPublicBaseURL("https://app.example.com"),
	).Handler()
	ts := httptest.NewServer(sessionManager.LoadAndSave(handler))
	defer ts.Close()

	client := newSessionClient(t)
	loginResp := sessionPostJSON(t, client, ts.URL+"/v1/ui/sessions/login", map[string]any{
		"email":    "multi@tenant.test",
		"password": "multi password 123",
	}, "", http.StatusConflict)
	if required, _ := loginResp["required"].(bool); !required {
		t.Fatalf("expected workspace selection required response, got %#v", loginResp)
	}
	items := loginResp["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("expected 2 workspace options, got %d", len(items))
	}
	csrfToken, _ := loginResp["csrf_token"].(string)
	selectResp := sessionPostJSON(t, client, ts.URL+"/v1/ui/workspaces/select", map[string]any{
		"tenant_id": "tenant_b",
	}, csrfToken, http.StatusCreated)
	if got, _ := selectResp["tenant_id"].(string); got != "tenant_b" {
		t.Fatalf("expected tenant_b session after selection, got %q", got)
	}
}

func TestUIAuthProvidersListsConfiguredSSOProviders(t *testing.T) {
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

	authSvc, err := service.NewBrowserUserAuthService(repo)
	if err != nil {
		t.Fatalf("new browser auth service: %v", err)
	}
	ssoSvc, err := service.NewBrowserSSOService(
		repo,
		authSvc,
		[]service.BrowserSSOProvider{
			fakeHTTPSSOProvider{
				definition: service.BrowserSSOProviderDefinition{
					Key:         "google",
					DisplayName: "Google Workspace",
					Type:        domain.BrowserSSOProviderTypeOIDC,
				},
			},
		},
		service.BrowserSSOServiceConfig{},
	)
	if err != nil {
		t.Fatalf("new browser sso service: %v", err)
	}

	handler := api.NewServer(
		repo,
		api.WithBrowserUserAuthService(authSvc),
		api.WithBrowserSSOService(ssoSvc),
	).Handler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/ui/auth/providers")
	if err != nil {
		t.Fatalf("get auth providers: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	var payload struct {
		PasswordEnabled bool `json:"password_enabled"`
		SSOProviders    []struct {
			Key         string `json:"key"`
			DisplayName string `json:"display_name"`
			Type        string `json:"type"`
		} `json:"sso_providers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode auth providers: %v", err)
	}
	if !payload.PasswordEnabled {
		t.Fatalf("expected password login to stay enabled")
	}
	if len(payload.SSOProviders) != 1 {
		t.Fatalf("expected one sso provider, got %d", len(payload.SSOProviders))
	}
	if payload.SSOProviders[0].Key != "google" {
		t.Fatalf("expected google provider, got %#v", payload.SSOProviders[0])
	}
}

func TestUISSOCallbackWithoutTenantContextAllowsPlatformUser(t *testing.T) {
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
	if _, err := db.Exec(`TRUNCATE TABLE sessions`); err != nil {
		t.Fatalf("truncate sessions: %v", err)
	}

	now := time.Now().UTC()
	user, err := repo.CreateUser(domain.User{
		Email:        "platform-admin@alpha.test",
		DisplayName:  "Platform Admin",
		Status:       domain.UserStatusActive,
		PlatformRole: domain.UserPlatformRoleAdmin,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	authSvc, err := service.NewBrowserUserAuthService(repo)
	if err != nil {
		t.Fatalf("new browser auth service: %v", err)
	}
	ssoSvc, err := service.NewBrowserSSOService(
		repo,
		authSvc,
		[]service.BrowserSSOProvider{
			fakeHTTPSSOProvider{
				definition: service.BrowserSSOProviderDefinition{
					Key:         "google",
					DisplayName: "Google",
					Type:        domain.BrowserSSOProviderTypeOIDC,
				},
				claims: service.BrowserSSOClaims{
					Subject:       "google-subject-platform",
					Email:         user.Email,
					EmailVerified: true,
					DisplayName:   user.DisplayName,
				},
			},
		},
		service.BrowserSSOServiceConfig{},
	)
	if err != nil {
		t.Fatalf("new browser sso service: %v", err)
	}

	sessionManager := scs.New()
	sessionManager.Store = postgresstore.New(db)
	sessionManager.Lifetime = 12 * time.Hour
	sessionManager.Cookie.Name = "test_ui_sso_platform_session"
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.Secure = false
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode

	handler := api.NewServer(
		repo,
		api.WithSessionManager(sessionManager),
		api.WithBrowserUserAuthService(authSvc),
		api.WithBrowserSSOService(ssoSvc),
		api.WithUIPublicBaseURL("http://app.example.test"),
	).Handler()
	ts := httptest.NewServer(sessionManager.LoadAndSave(handler))
	defer ts.Close()

	client := newSessionClient(t)

	startResp, err := client.Get(ts.URL + "/v1/ui/auth/sso/google/start")
	if err != nil {
		t.Fatalf("sso start request: %v", err)
	}
	if startResp.StatusCode != http.StatusFound {
		body, _ := io.ReadAll(startResp.Body)
		startResp.Body.Close()
		t.Fatalf("expected 302 from start, got %d: %s", startResp.StatusCode, string(body))
	}
	startLocation := startResp.Header.Get("Location")
	startResp.Body.Close()
	startURL := mustParseURL(t, startLocation)
	state := startURL.Query().Get("state")
	if state == "" {
		t.Fatalf("expected state in sso start redirect")
	}

	callbackResp, err := client.Get(ts.URL + "/v1/ui/auth/sso/google/callback?code=fake-code&state=" + state)
	if err != nil {
		t.Fatalf("sso callback request: %v", err)
	}
	if callbackResp.StatusCode != http.StatusFound {
		body, _ := io.ReadAll(callbackResp.Body)
		callbackResp.Body.Close()
		t.Fatalf("expected 302 from callback, got %d: %s", callbackResp.StatusCode, string(body))
	}
	if got := callbackResp.Header.Get("Location"); got != "http://app.example.test/" {
		callbackResp.Body.Close()
		t.Fatalf("expected redirect to app root, got %q", got)
	}
	callbackResp.Body.Close()

	meResp := sessionGetJSON(t, client, ts.URL+"/v1/ui/sessions/me", http.StatusOK)
	if got, _ := meResp["scope"].(string); got != "platform" {
		t.Fatalf("expected platform scope after sso callback, got %q", got)
	}
	if got, _ := meResp["platform_role"].(string); got != string(api.PlatformRoleAdmin) {
		t.Fatalf("expected platform_role platform_admin, got %q", got)
	}
}

func TestUISSOCallbackProvisionInvitedUserAndRedirectsBackToInvitation(t *testing.T) {
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
	if _, err := db.Exec(`TRUNCATE TABLE sessions`); err != nil {
		t.Fatalf("truncate sessions: %v", err)
	}

	now := time.Now().UTC()
	if _, err := repo.CreateTenant(domain.Tenant{
		ID:        "tenant_invite_sso",
		Name:      "Tenant Invite SSO",
		Status:    domain.TenantStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	workspaceAccessSvc := service.NewWorkspaceAccessService(repo)
	issued, err := workspaceAccessSvc.IssueWorkspaceInvitation(service.CreateWorkspaceInvitationRequest{
		WorkspaceID: "tenant_invite_sso",
		Email:       "invited-sso@tenant.test",
		Role:        "writer",
	})
	if err != nil {
		t.Fatalf("issue workspace invitation: %v", err)
	}

	authSvc, err := service.NewBrowserUserAuthService(repo)
	if err != nil {
		t.Fatalf("new browser auth service: %v", err)
	}
	ssoSvc, err := service.NewBrowserSSOService(
		repo,
		authSvc,
		[]service.BrowserSSOProvider{
			fakeHTTPSSOProvider{
				definition: service.BrowserSSOProviderDefinition{
					Key:         "google",
					DisplayName: "Google",
					Type:        domain.BrowserSSOProviderTypeOIDC,
				},
				claims: service.BrowserSSOClaims{
					Subject:       "google-subject-invite",
					Email:         "invited-sso@tenant.test",
					EmailVerified: true,
					DisplayName:   "Invited SSO",
				},
			},
		},
		service.BrowserSSOServiceConfig{},
	)
	if err != nil {
		t.Fatalf("new browser sso service: %v", err)
	}

	sessionManager := scs.New()
	sessionManager.Store = postgresstore.New(db)
	sessionManager.Lifetime = 12 * time.Hour
	sessionManager.Cookie.Name = "test_ui_sso_invite_session"
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.Secure = false
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode

	handler := api.NewServer(
		repo,
		api.WithSessionManager(sessionManager),
		api.WithBrowserUserAuthService(authSvc),
		api.WithBrowserSSOService(ssoSvc),
		api.WithUIPublicBaseURL("http://app.example.test"),
	).Handler()
	ts := httptest.NewServer(sessionManager.LoadAndSave(handler))
	defer ts.Close()

	client := newSessionClient(t)
	startResp, err := client.Get(ts.URL + "/v1/ui/auth/sso/google/start?next=" + url.QueryEscape("/invite/"+issued.Token))
	if err != nil {
		t.Fatalf("sso start request: %v", err)
	}
	if startResp.StatusCode != http.StatusFound {
		body, _ := io.ReadAll(startResp.Body)
		startResp.Body.Close()
		t.Fatalf("expected 302 from start, got %d: %s", startResp.StatusCode, string(body))
	}
	startLocation := startResp.Header.Get("Location")
	startResp.Body.Close()
	startURL := mustParseURL(t, startLocation)
	state := startURL.Query().Get("state")
	if state == "" {
		t.Fatalf("expected state in sso start redirect")
	}

	callbackResp, err := client.Get(ts.URL + "/v1/ui/auth/sso/google/callback?code=fake-code&state=" + state)
	if err != nil {
		t.Fatalf("sso callback request: %v", err)
	}
	if callbackResp.StatusCode != http.StatusFound {
		body, _ := io.ReadAll(callbackResp.Body)
		callbackResp.Body.Close()
		t.Fatalf("expected 302 from callback, got %d: %s", callbackResp.StatusCode, string(body))
	}
	if got := callbackResp.Header.Get("Location"); got != "http://app.example.test/invite/"+issued.Token {
		callbackResp.Body.Close()
		t.Fatalf("expected redirect back to invitation, got %q", got)
	}
	callbackResp.Body.Close()

	user, err := repo.GetUserByEmail("invited-sso@tenant.test")
	if err != nil {
		t.Fatalf("expected invited user to be provisioned, got %v", err)
	}
	if got := user.DisplayName; got != "Invited SSO" {
		t.Fatalf("expected provisioned display name from claims, got %q", got)
	}
}

func TestUIPasswordForgotAndResetFlow(t *testing.T) {
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
	if _, err := db.Exec(`TRUNCATE TABLE sessions`); err != nil {
		t.Fatalf("truncate sessions: %v", err)
	}

	mustCreateBrowserUser(t, repo, browserUserFixture{
		email:    "reset-user@tenant.test",
		password: "old password 123",
		role:     "writer",
		tenantID: "tenant_reset",
	})

	sessionManager := scs.New()
	sessionManager.Store = postgresstore.New(db)
	sessionManager.Lifetime = 12 * time.Hour
	sessionManager.Cookie.Name = "test_ui_password_reset_session"
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.Secure = false
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode

	resetSender := &fakePasswordResetEmailSender{}
	client := newSessionClient(t)
	handler := api.NewServer(
		repo,
		api.WithSessionManager(sessionManager),
		api.WithBrowserUserAuthService(mustNewBrowserUserAuthService(t, repo)),
		api.WithPasswordResetService(service.NewPasswordResetService(repo, time.Hour)),
		api.WithPasswordResetEmailSender(resetSender),
		api.WithUIPublicBaseURL("http://app.example.test"),
	).Handler()
	ts := httptest.NewServer(sessionManager.LoadAndSave(handler))
	defer ts.Close()

	resp := sessionPostJSON(t, client, ts.URL+"/v1/ui/password/forgot", map[string]any{
		"email": "reset-user@tenant.test",
	}, "", http.StatusAccepted)
	if requested, _ := resp["requested"].(bool); !requested {
		t.Fatalf("expected password reset request to be accepted")
	}
	if len(resetSender.inputs) != 1 {
		t.Fatalf("expected one password reset email, got %d", len(resetSender.inputs))
	}
	resetURL, err := url.Parse(resetSender.inputs[0].ResetURL)
	if err != nil {
		t.Fatalf("parse reset url: %v", err)
	}
	token := strings.TrimSpace(resetURL.Query().Get("token"))
	if token == "" {
		t.Fatalf("expected reset token in password reset email")
	}

	resetResult := sessionPostJSON(t, client, ts.URL+"/v1/ui/password/reset", map[string]any{
		"token":    token,
		"password": "new password 123",
	}, "", http.StatusCreated)
	if reset, _ := resetResult["reset"].(bool); !reset {
		t.Fatalf("expected reset result true")
	}

	loginResp := sessionPostJSON(t, client, ts.URL+"/v1/ui/sessions/login", map[string]any{
		"email":    "reset-user@tenant.test",
		"password": "new password 123",
	}, "", http.StatusCreated)
	if got, _ := loginResp["tenant_id"].(string); got != "tenant_reset" {
		t.Fatalf("expected tenant_reset after login with new password, got %q", got)
	}

	sessionPostJSON(t, client, ts.URL+"/v1/ui/password/reset", map[string]any{
		"token":    token,
		"password": "another password 123",
	}, "", http.StatusGone)
}

func mustNewBrowserUserAuthService(t *testing.T, repo *store.PostgresStore) *service.BrowserUserAuthService {
	t.Helper()
	authSvc, err := service.NewBrowserUserAuthService(repo)
	if err != nil {
		t.Fatalf("new browser user auth service: %v", err)
	}
	return authSvc
}

func newSessionClient(t *testing.T) *http.Client {
	t.Helper()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("new cookie jar: %v", err)
	}
	return &http.Client{
		Jar:     jar,
		Timeout: 5 * time.Second,
	}
}

type browserUserFixture struct {
	email        string
	password     string
	role         string
	tenantID     string
	platformRole string
}

func mustCreateBrowserUser(t *testing.T, repo *store.PostgresStore, fixture browserUserFixture) {
	t.Helper()

	now := time.Now().UTC()
	user, err := repo.CreateUser(domain.User{
		Email:        fixture.email,
		DisplayName:  fixture.email,
		Status:       domain.UserStatusActive,
		PlatformRole: domain.UserPlatformRole(fixture.platformRole),
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		t.Fatalf("create user %q: %v", fixture.email, err)
	}

	hash, err := service.HashPassword(fixture.password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if _, err := repo.UpsertUserPasswordCredential(domain.UserPasswordCredential{
		UserID:            user.ID,
		PasswordHash:      hash,
		PasswordUpdatedAt: now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}); err != nil {
		t.Fatalf("upsert user password credential: %v", err)
	}

	if fixture.tenantID != "" {
		if _, err := repo.CreateTenant(domain.Tenant{
			ID:     fixture.tenantID,
			Name:   fixture.tenantID,
			Status: domain.TenantStatusActive,
		}); err != nil && err != store.ErrAlreadyExists && err != store.ErrDuplicateKey {
			t.Fatalf("create tenant %q: %v", fixture.tenantID, err)
		}
		if _, err := repo.UpsertUserTenantMembership(domain.UserTenantMembership{
			UserID:    user.ID,
			TenantID:  fixture.tenantID,
			Role:      fixture.role,
			Status:    domain.UserTenantMembershipStatusActive,
			CreatedAt: now,
			UpdatedAt: now,
		}); err != nil {
			t.Fatalf("upsert user tenant membership: %v", err)
		}
	}
}

func sessionPostJSON(t *testing.T, client *http.Client, url string, body any, csrfToken string, expectedStatus int) map[string]any {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if csrfToken != "" {
		req.Header.Set("X-CSRF-Token", csrfToken)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("post request failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if resp.StatusCode != expectedStatus {
		t.Fatalf("unexpected status %d, expected %d, body=%s", resp.StatusCode, expectedStatus, string(bodyBytes))
	}

	var out map[string]any
	if len(bytes.TrimSpace(bodyBytes)) == 0 {
		return out
	}
	if err := json.Unmarshal(bodyBytes, &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}

func sessionGetJSON(t *testing.T, client *http.Client, url string, expectedStatus int) map[string]any {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("get request failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if resp.StatusCode != expectedStatus {
		t.Fatalf("unexpected status %d, expected %d, body=%s", resp.StatusCode, expectedStatus, string(bodyBytes))
	}

	var out map[string]any
	if len(bytes.TrimSpace(bodyBytes)) == 0 {
		return out
	}
	if err := json.Unmarshal(bodyBytes, &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()

	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse url %q: %v", raw, err)
	}
	return parsed
}
