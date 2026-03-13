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

	"lago-usage-billing-alpha/internal/api"
)

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
	authorizer, err := api.NewStaticAPIKeyAuthorizer(map[string]api.Role{
		"reader-key": api.RoleReader,
	})
	if err != nil {
		t.Fatalf("new static authorizer: %v", err)
	}

	sessionManager := scs.New()
	sessionManager.Lifetime = 2 * time.Hour
	sessionManager.Cookie.Name = "origin_policy_session"
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode

	handler := api.NewServer(
		nil,
		api.WithAPIKeyAuthorizer(authorizer),
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
		"api_key": "reader-key",
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
