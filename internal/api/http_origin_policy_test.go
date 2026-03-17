package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"

	"usage-billing-control-plane/internal/api"
	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/service"
	"usage-billing-control-plane/internal/store"
)

type browserOriginAuthStore struct {
	user        domain.User
	credential  domain.UserPasswordCredential
	memberships []domain.UserTenantMembership
}

func (s browserOriginAuthStore) GetUserByEmail(email string) (domain.User, error) {
	if s.user.Email != email {
		return domain.User{}, store.ErrNotFound
	}
	return s.user, nil
}

func (s browserOriginAuthStore) GetUserPasswordCredential(userID string) (domain.UserPasswordCredential, error) {
	if s.credential.UserID != userID {
		return domain.UserPasswordCredential{}, store.ErrNotFound
	}
	return s.credential, nil
}

func (s browserOriginAuthStore) ListUserTenantMemberships(userID string) ([]domain.UserTenantMembership, error) {
	if s.user.ID != userID {
		return nil, store.ErrNotFound
	}
	return s.memberships, nil
}

func TestParseAllowedOrigins(t *testing.T) {
	got, err := api.ParseAllowedOrigins(" https://control.example.com ,https://control.example.com,https://api.example.com ")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 unique origins, got %d (%v)", len(got), got)
	}

	if _, err := api.ParseAllowedOrigins("https://control.example.com/path"); err == nil {
		t.Fatalf("expected invalid origin path to fail")
	}
}

func TestSessionOriginPolicyForUnsafeMethods(t *testing.T) {
	hash, err := service.HashPassword("reader password 123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	browserAuth, err := service.NewBrowserUserAuthService(browserOriginAuthStore{
		user: domain.User{
			ID:          "usr_reader",
			Email:       "reader@example.com",
			DisplayName: "Reader",
			Status:      domain.UserStatusActive,
		},
		credential: domain.UserPasswordCredential{
			UserID:       "usr_reader",
			PasswordHash: hash,
		},
		memberships: []domain.UserTenantMembership{
			{UserID: "usr_reader", TenantID: "default", Role: string(api.RoleReader), Status: domain.UserTenantMembershipStatusActive},
		},
	})
	if err != nil {
		t.Fatalf("new browser auth: %v", err)
	}

	sessionManager := scs.New()
	sessionManager.Lifetime = 2 * time.Hour
	sessionManager.Cookie.Name = "origin_policy_session"
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode

	handler := api.NewServer(
		nil,
		api.WithBrowserUserAuthService(browserAuth),
		api.WithSessionManager(sessionManager),
		api.WithSessionOriginPolicy(true, []string{"https://control.example.com"}),
	).Handler()
	ts := httptest.NewServer(sessionManager.LoadAndSave(handler))
	defer ts.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("new cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar, Timeout: 5 * time.Second}

	loginResp := postJSONWithHeadersAndStatus(t, client, ts.URL+"/v1/ui/sessions/login", map[string]any{
		"email":    "reader@example.com",
		"password": "reader password 123",
	}, nil, http.StatusCreated)
	csrfToken, _ := loginResp["csrf_token"].(string)
	if csrfToken == "" {
		t.Fatalf("expected csrf token from login")
	}

	_ = postJSONWithHeadersAndStatus(t, client, ts.URL+"/v1/ui/sessions/logout", map[string]any{}, map[string]string{
		"X-CSRF-Token": csrfToken,
	}, http.StatusForbidden)
	_ = postJSONWithHeadersAndStatus(t, client, ts.URL+"/v1/ui/sessions/logout", map[string]any{}, map[string]string{
		"X-CSRF-Token": csrfToken,
		"Origin":       "https://evil.example.com",
	}, http.StatusForbidden)
	_ = postJSONWithHeadersAndStatus(t, client, ts.URL+"/v1/ui/sessions/logout", map[string]any{}, map[string]string{
		"X-CSRF-Token": csrfToken,
		"Origin":       "https://control.example.com",
	}, http.StatusOK)
}

func TestAllowedOriginCORSPreflightAndRequest(t *testing.T) {
	hash, err := service.HashPassword("reader password 123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	browserAuth, err := service.NewBrowserUserAuthService(browserOriginAuthStore{
		user: domain.User{
			ID:          "usr_reader",
			Email:       "reader@example.com",
			DisplayName: "Reader",
			Status:      domain.UserStatusActive,
		},
		credential: domain.UserPasswordCredential{
			UserID:       "usr_reader",
			PasswordHash: hash,
		},
		memberships: []domain.UserTenantMembership{
			{UserID: "usr_reader", TenantID: "default", Role: string(api.RoleReader), Status: domain.UserTenantMembershipStatusActive},
		},
	})
	if err != nil {
		t.Fatalf("new browser auth: %v", err)
	}

	sessionManager := scs.New()
	sessionManager.Lifetime = 2 * time.Hour
	sessionManager.Cookie.Name = "cors_policy_session"
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode

	handler := api.NewServer(
		nil,
		api.WithBrowserUserAuthService(browserAuth),
		api.WithSessionManager(sessionManager),
		api.WithSessionOriginPolicy(true, []string{"https://control.example.com"}),
	).Handler()
	ts := httptest.NewServer(sessionManager.LoadAndSave(handler))
	defer ts.Close()

	req, err := http.NewRequest(http.MethodOptions, ts.URL+"/v1/ui/sessions/login", nil)
	if err != nil {
		t.Fatalf("new preflight request: %v", err)
	}
	req.Header.Set("Origin", "https://control.example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "content-type")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("preflight request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected preflight status %d, got %d", http.StatusNoContent, resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "https://control.example.com" {
		t.Fatalf("expected allow-origin header, got %q", got)
	}
	if got := resp.Header.Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("expected allow-credentials true, got %q", got)
	}

	loginResp := postJSONWithHeadersAndStatus(t, http.DefaultClient, ts.URL+"/v1/ui/sessions/login", map[string]any{
		"email":    "reader@example.com",
		"password": "reader password 123",
	}, map[string]string{
		"Origin": "https://control.example.com",
	}, http.StatusCreated)
	if got, _ := loginResp["role"].(string); got != string(api.RoleReader) {
		t.Fatalf("expected reader role, got %q", got)
	}
}

func TestDisallowedOriginCORSPreflightRejected(t *testing.T) {
	handler := api.NewServer(
		nil,
		api.WithSessionOriginPolicy(true, []string{"https://control.example.com"}),
	).Handler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, err := http.NewRequest(http.MethodOptions, ts.URL+"/v1/ui/sessions/login", nil)
	if err != nil {
		t.Fatalf("new preflight request: %v", err)
	}
	req.Header.Set("Origin", "https://evil.example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "content-type")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("preflight request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected preflight status %d, got %d", http.StatusForbidden, resp.StatusCode)
	}
}

func postJSONWithHeadersAndStatus(t *testing.T, client *http.Client, endpoint string, body any, headers map[string]string, expectedStatus int) map[string]any {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if resp.StatusCode != expectedStatus {
		t.Fatalf("unexpected status %d expected %d body=%s", resp.StatusCode, expectedStatus, string(bodyBytes))
	}
	if len(bytes.TrimSpace(bodyBytes)) == 0 {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(bodyBytes, &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}
