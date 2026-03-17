package api_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
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

	_ = sessionGetJSON(t, client, ts.URL+"/v1/rating-rules", http.StatusOK)
	meResp := sessionGetJSON(t, client, ts.URL+"/v1/ui/sessions/me", http.StatusOK)
	if got, _ := meResp["role"].(string); got != string(api.RoleReader) {
		t.Fatalf("expected me.role reader, got %q", got)
	}

	_ = sessionPostJSON(t, client, ts.URL+"/v1/ui/sessions/logout", map[string]any{}, "", http.StatusForbidden)
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
	_ = sessionPostJSON(t, client, ts.URL+"/v1/ui/sessions/login", map[string]any{
		"api_key": "legacy-browser-login-key",
	}, "", http.StatusBadRequest)
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

	_ = sessionGetJSON(t, client, ts.URL+"/internal/metrics", http.StatusOK)
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
