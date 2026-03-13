package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lago-usage-billing-alpha/internal/api"
)

func TestRequestIDMiddlewareGeneratesAndPropagates(t *testing.T) {
	ts := httptest.NewServer(api.NewServer(nil).Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("health request: %v", err)
	}
	defer resp.Body.Close()

	generated := strings.TrimSpace(resp.Header.Get("X-Request-ID"))
	if generated == "" {
		t.Fatalf("expected generated X-Request-ID header")
	}

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/health", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("X-Request-ID", "req-123.alpha")

	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("health request with request id: %v", err)
	}
	defer resp2.Body.Close()

	if got := strings.TrimSpace(resp2.Header.Get("X-Request-ID")); got != "req-123.alpha" {
		t.Fatalf("expected echoed request id req-123.alpha, got %q", got)
	}
}

func TestErrorResponseExcludesRequestIDBodyField(t *testing.T) {
	authorizer, err := api.NewStaticAPIKeyAuthorizer(map[string]api.Role{
		"reader": api.RoleReader,
	})
	if err != nil {
		t.Fatalf("new static authorizer: %v", err)
	}

	ts := httptest.NewServer(api.NewServer(nil, api.WithAPIKeyAuthorizer(authorizer)).Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/rating-rules")
	if err != nil {
		t.Fatalf("protected request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 unauthorized, got %d", resp.StatusCode)
	}

	headerRequestID := strings.TrimSpace(resp.Header.Get("X-Request-ID"))
	if headerRequestID == "" {
		t.Fatalf("expected X-Request-ID header")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode response json: %v", err)
	}
	if got, _ := payload["error"].(string); got != "unauthorized" {
		t.Fatalf("expected unauthorized error, got %q", got)
	}
	if _, exists := payload["request_id"]; exists {
		t.Fatalf("expected no request_id in error response body")
	}
}
