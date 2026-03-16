package api_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"usage-billing-control-plane/internal/api"
)

type stubRateLimiter struct {
	mu       sync.Mutex
	calls    []api.RateLimitRequest
	decision func(req api.RateLimitRequest) (api.RateLimitDecision, error)
}

func (s *stubRateLimiter) Allow(_ context.Context, req api.RateLimitRequest) (api.RateLimitDecision, error) {
	s.mu.Lock()
	s.calls = append(s.calls, req)
	s.mu.Unlock()
	if s.decision != nil {
		return s.decision(req)
	}
	return api.RateLimitDecision{
		Allowed:   true,
		Limit:     100,
		Remaining: 99,
		ResetAt:   time.Now().UTC().Add(10 * time.Second),
	}, nil
}

func TestRateLimitBlocksLoginWhenExceeded(t *testing.T) {
	authorizer, err := api.NewStaticAPIKeyAuthorizer(map[string]api.Role{
		"reader-key": api.RoleReader,
	})
	if err != nil {
		t.Fatalf("new static authorizer: %v", err)
	}

	limiter := &stubRateLimiter{
		decision: func(req api.RateLimitRequest) (api.RateLimitDecision, error) {
			if req.Policy != api.RateLimitPolicyPreAuthLogin {
				t.Fatalf("expected policy %s, got %s", api.RateLimitPolicyPreAuthLogin, req.Policy)
			}
			if !strings.Contains(req.Identifier, ":route:/v1/ui/sessions/login") {
				t.Fatalf("expected login rate limit identifier to include login route, got %q", req.Identifier)
			}
			return api.RateLimitDecision{
				Allowed:   false,
				Limit:     20,
				Remaining: 0,
				ResetAt:   time.Now().UTC().Add(3 * time.Second),
			}, nil
		},
	}

	ts := httptest.NewServer(api.NewServer(nil, api.WithAPIKeyAuthorizer(authorizer), api.WithRateLimiter(limiter, true, false)).Handler())
	defer ts.Close()

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/ui/sessions/login", bytes.NewBufferString(`{"api_key":"x"}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", resp.StatusCode)
	}
	if got := strings.TrimSpace(resp.Header.Get("Retry-After")); got == "" {
		t.Fatalf("expected Retry-After header")
	}
	if got := strings.TrimSpace(resp.Header.Get("X-RateLimit-Limit")); got != "20" {
		t.Fatalf("expected X-RateLimit-Limit=20, got %q", got)
	}
}

func TestRateLimitBlocksProtectedRouteAfterAuth(t *testing.T) {
	authorizer, err := api.NewStaticAPIKeyAuthorizer(map[string]api.Role{
		"reader-key": api.RoleReader,
	})
	if err != nil {
		t.Fatalf("new static authorizer: %v", err)
	}

	limiter := &stubRateLimiter{
		decision: func(req api.RateLimitRequest) (api.RateLimitDecision, error) {
			if req.Policy == api.RateLimitPolicyAuthRead {
				return api.RateLimitDecision{
					Allowed:   false,
					Limit:     1200,
					Remaining: 0,
					ResetAt:   time.Now().UTC().Add(3 * time.Second),
				}, nil
			}
			return api.RateLimitDecision{
				Allowed:   true,
				Limit:     100,
				Remaining: 99,
				ResetAt:   time.Now().UTC().Add(10 * time.Second),
			}, nil
		},
	}

	ts := httptest.NewServer(api.NewServer(nil, api.WithAPIKeyAuthorizer(authorizer), api.WithRateLimiter(limiter, true, false)).Handler())
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/v1/unknown", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("X-API-Key", "reader-key")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", resp.StatusCode)
	}
	if got := strings.TrimSpace(resp.Header.Get("X-RateLimit-Limit")); got != "1200" {
		t.Fatalf("expected X-RateLimit-Limit=1200, got %q", got)
	}
	if len(limiter.calls) == 0 || !strings.Contains(limiter.calls[len(limiter.calls)-1].Identifier, "tenant:default:") {
		t.Fatalf("expected tenant-scoped rate limit identifier, got %#v", limiter.calls)
	}
}

func TestRateLimitFailOpenOnProtectedRoutes(t *testing.T) {
	authorizer, err := api.NewStaticAPIKeyAuthorizer(map[string]api.Role{
		"reader-key": api.RoleReader,
	})
	if err != nil {
		t.Fatalf("new static authorizer: %v", err)
	}

	limiter := &stubRateLimiter{
		decision: func(req api.RateLimitRequest) (api.RateLimitDecision, error) {
			if req.Policy == api.RateLimitPolicyAuthRead {
				return api.RateLimitDecision{}, errors.New("redis unavailable")
			}
			return api.RateLimitDecision{
				Allowed:   true,
				Limit:     100,
				Remaining: 99,
				ResetAt:   time.Now().UTC().Add(10 * time.Second),
			}, nil
		},
	}

	ts := httptest.NewServer(api.NewServer(nil, api.WithAPIKeyAuthorizer(authorizer), api.WithRateLimiter(limiter, true, false)).Handler())
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/v1/unknown", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("X-API-Key", "reader-key")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	// Route is protected but not registered, so we should fall through to 404 when limiter fails open.
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 with fail-open behavior, got %d", resp.StatusCode)
	}
}

func TestRateLimitProbeDoesNotShareLoginBucket(t *testing.T) {
	authorizer, err := api.NewStaticAPIKeyAuthorizer(map[string]api.Role{
		"reader-key": api.RoleReader,
	})
	if err != nil {
		t.Fatalf("new static authorizer: %v", err)
	}

	limiter := &stubRateLimiter{
		decision: func(req api.RateLimitRequest) (api.RateLimitDecision, error) {
			if req.Policy != api.RateLimitPolicyPreAuthLogin {
				return api.RateLimitDecision{Allowed: true, Limit: 100, Remaining: 99, ResetAt: time.Now().UTC().Add(10 * time.Second)}, nil
			}
			if strings.Contains(req.Identifier, ":route:/v1/ui/sessions/rate-limit-probe") {
				return api.RateLimitDecision{Allowed: false, Limit: 20, Remaining: 0, ResetAt: time.Now().UTC().Add(3 * time.Second)}, nil
			}
			return api.RateLimitDecision{Allowed: true, Limit: 20, Remaining: 19, ResetAt: time.Now().UTC().Add(3 * time.Second)}, nil
		},
	}

	ts := httptest.NewServer(api.NewServer(nil, api.WithAPIKeyAuthorizer(authorizer), api.WithRateLimiter(limiter, true, false)).Handler())
	defer ts.Close()

	probeReq, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/ui/sessions/rate-limit-probe", bytes.NewBufferString(`{}`))
	if err != nil {
		t.Fatalf("new probe request: %v", err)
	}
	probeReq.Header.Set("Content-Type", "application/json")
	probeResp, err := http.DefaultClient.Do(probeReq)
	if err != nil {
		t.Fatalf("do probe request: %v", err)
	}
	defer probeResp.Body.Close()
	if probeResp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected probe 429, got %d", probeResp.StatusCode)
	}

	loginReq, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/ui/sessions/login", bytes.NewBufferString(`{"api_key":"missing"}`))
	if err != nil {
		t.Fatalf("new login request: %v", err)
	}
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := http.DefaultClient.Do(loginReq)
	if err != nil {
		t.Fatalf("do login request: %v", err)
	}
	defer loginResp.Body.Close()
	if loginResp.StatusCode == http.StatusTooManyRequests {
		t.Fatalf("expected login not to share probe bucket")
	}
}
