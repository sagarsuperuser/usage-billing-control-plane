package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLagoBillingProviderAdapterCreatesStripeProvider(t *testing.T) {
	t.Parallel()

	calls := 0
	lago := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/graphql" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("x-lago-organization"); got != "org_test" {
			t.Fatalf("expected x-lago-organization header, got %q", got)
		}
		calls++
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode graphql request: %v", err)
		}
		query, _ := body["query"].(string)
		variables, _ := body["variables"].(map[string]any)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(query, "paymentProvider(code:"):
			_, _ = w.Write([]byte(`{"data":{"paymentProvider":null}}`))
		case strings.Contains(query, "addStripePaymentProvider"):
			input, _ := variables["input"].(map[string]any)
			if got := input["secretKey"]; got != "sk_test_123" {
				t.Fatalf("expected secretKey to be forwarded, got %#v", got)
			}
			_, _ = w.Write([]byte(`{"data":{"addStripePaymentProvider":{"id":"pp_123","code":"alpha_stripe_test_bpc_test","name":"Stripe Test"}}}`))
		default:
			t.Fatalf("unexpected graphql query: %s", query)
		}
	}))
	defer lago.Close()

	transport, err := NewLagoHTTPTransport(LagoClientConfig{BaseURL: lago.URL, APIKey: "test", Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}

	adapter := NewLagoBillingProviderAdapter(transport, "https://alpha.example.test/return")
	result, err := adapter.EnsureStripeProvider(context.Background(), EnsureStripeProviderInput{
		ConnectionID:       "bpc_test",
		DisplayName:        "Stripe Test",
		Environment:        "test",
		SecretKey:          "sk_test_123",
		LagoOrganizationID: "org_test",
	})
	if err != nil {
		t.Fatalf("ensure stripe provider: %v", err)
	}
	if result.LagoProviderCode != "alpha_stripe_test_bpc_test" {
		t.Fatalf("expected provider code to be returned, got %q", result.LagoProviderCode)
	}
	if calls != 2 {
		t.Fatalf("expected 2 graphql calls, got %d", calls)
	}
}

func TestLagoBillingProviderAdapterUpdatesExistingStripeProvider(t *testing.T) {
	t.Parallel()

	lago := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/graphql" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("x-lago-organization"); got != "org_test" {
			t.Fatalf("expected x-lago-organization header, got %q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode graphql request: %v", err)
		}
		query, _ := body["query"].(string)
		variables, _ := body["variables"].(map[string]any)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(query, "paymentProvider(code:"):
			_, _ = w.Write([]byte(`{"data":{"paymentProvider":{"__typename":"StripeProvider","id":"pp_existing","code":"stripe_existing","name":"Old"}}}`))
		case strings.Contains(query, "updateStripePaymentProvider"):
			input, _ := variables["input"].(map[string]any)
			if got := input["id"]; got != "pp_existing" {
				t.Fatalf("expected existing provider id, got %#v", got)
			}
			_, _ = w.Write([]byte(`{"data":{"updateStripePaymentProvider":{"id":"pp_existing","code":"stripe_existing","name":"Updated"}}}`))
		default:
			t.Fatalf("unexpected graphql query: %s", query)
		}
	}))
	defer lago.Close()

	transport, err := NewLagoHTTPTransport(LagoClientConfig{BaseURL: lago.URL, APIKey: "test", Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}

	adapter := NewLagoBillingProviderAdapter(transport, "https://alpha.example.test/return")
	result, err := adapter.EnsureStripeProvider(context.Background(), EnsureStripeProviderInput{
		ConnectionID:       "bpc_existing",
		DisplayName:        "Updated",
		Environment:        "test",
		SecretKey:          "sk_test_123",
		LagoOrganizationID: "org_test",
		LagoProviderCode:   "stripe_existing",
	})
	if err != nil {
		t.Fatalf("ensure existing stripe provider: %v", err)
	}
	if result.LagoProviderCode != "stripe_existing" {
		t.Fatalf("expected existing provider code, got %q", result.LagoProviderCode)
	}
}
