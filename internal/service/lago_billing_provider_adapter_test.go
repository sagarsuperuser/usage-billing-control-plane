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
		calls++
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/graphql" && r.Method == http.MethodPost:
			_, _ = w.Write([]byte(`{"data":{"organization":{"id":"org_test","hmacKey":"hmac_test_key"}}}`))
		case r.URL.Path == "/api/v1/payment_providers/stripe/alpha_stripe_test_bpc_test" && r.Method == http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"status":404,"error":"Not Found","code":"payment_provider_not_found"}`))
		case r.URL.Path == "/api/v1/payment_providers/stripe" && r.Method == http.MethodPost:
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			provider, _ := body["payment_provider"].(map[string]any)
			if got := provider["secret_key"]; got != "sk_test_123" {
				t.Fatalf("expected secret_key to be forwarded, got %#v", got)
			}
			_, _ = w.Write([]byte(`{"payment_provider":{"lago_id":"pp_123","lago_organization_id":"org_test","code":"alpha_stripe_test_bpc_test","name":"Stripe Test","provider_type":"stripe"}}`))
		default:
			http.NotFound(w, r)
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
	if result.LagoWebhookHMACKey != "hmac_test_key" {
		t.Fatalf("expected hmac key to be returned, got %q", result.LagoWebhookHMACKey)
	}
	if calls != 3 {
		t.Fatalf("expected 3 rest calls, got %d", calls)
	}
}

func TestLagoBillingProviderAdapterUpdatesExistingStripeProvider(t *testing.T) {
	t.Parallel()

	lago := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/graphql" && r.Method == http.MethodPost:
			_, _ = w.Write([]byte(`{"data":{"organization":{"id":"org_test","hmacKey":"hmac_test_key"}}}`))
		case r.URL.Path == "/api/v1/payment_providers/stripe/stripe_existing" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"payment_provider":{"lago_id":"pp_existing","lago_organization_id":"org_test","code":"stripe_existing","name":"Old","provider_type":"stripe"}}`))
		case r.URL.Path == "/api/v1/payment_providers/stripe" && r.Method == http.MethodPost:
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			provider, _ := body["payment_provider"].(map[string]any)
			if got := provider["code"]; got != "stripe_existing" {
				t.Fatalf("expected existing provider code, got %#v", got)
			}
			_, _ = w.Write([]byte(`{"payment_provider":{"lago_id":"pp_existing","lago_organization_id":"org_test","code":"stripe_existing","name":"Updated","provider_type":"stripe"}}`))
		default:
			http.NotFound(w, r)
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
	if result.LagoWebhookHMACKey != "hmac_test_key" {
		t.Fatalf("expected hmac key to be returned, got %q", result.LagoWebhookHMACKey)
	}
}

func TestLagoBillingProviderAdapterRejectsUnexpectedProviderType(t *testing.T) {
	t.Parallel()

	lago := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/graphql" && r.Method == http.MethodPost:
			_, _ = w.Write([]byte(`{"data":{"organization":{"id":"org_test","hmacKey":"hmac_test_key"}}}`))
		case r.URL.Path == "/api/v1/payment_providers/stripe/code_taken" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"payment_provider":{"lago_id":"pp_existing","lago_organization_id":"org_test","code":"code_taken","name":"Other","provider_type":"gocardless"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer lago.Close()

	transport, err := NewLagoHTTPTransport(LagoClientConfig{BaseURL: lago.URL, APIKey: "test", Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}

	adapter := NewLagoBillingProviderAdapter(transport, "https://alpha.example.test/return")
	_, err = adapter.EnsureStripeProvider(context.Background(), EnsureStripeProviderInput{
		ConnectionID:       "bpc_existing",
		DisplayName:        "Updated",
		Environment:        "test",
		SecretKey:          "sk_test_123",
		LagoOrganizationID: "org_test",
		LagoProviderCode:   "code_taken",
	})
	if err == nil || !strings.Contains(err.Error(), `already exists as gocardless`) {
		t.Fatalf("expected provider type conflict error, got %v", err)
	}
}

func TestLagoBillingProviderAdapterFallsBackWhenGraphQLOrganizationContextIsMissing(t *testing.T) {
	t.Parallel()

	lago := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/graphql" && r.Method == http.MethodPost:
			_, _ = w.Write([]byte(`{"errors":[{"message":"Missing organization id"}]}`))
		case r.URL.Path == "/api/v1/payment_providers/stripe/alpha_stripe_test_bpc_test" && r.Method == http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"status":404,"error":"Not Found","code":"payment_provider_not_found"}`))
		case r.URL.Path == "/api/v1/payment_providers/stripe" && r.Method == http.MethodPost:
			_, _ = w.Write([]byte(`{"payment_provider":{"lago_id":"pp_123","lago_organization_id":"org_test","code":"alpha_stripe_test_bpc_test","name":"Stripe Test","provider_type":"stripe"}}`))
		default:
			http.NotFound(w, r)
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
		t.Fatalf("ensure stripe provider with graphql org fallback: %v", err)
	}
	if result.LagoOrganizationID != "org_test" {
		t.Fatalf("expected requested organization id to be preserved, got %q", result.LagoOrganizationID)
	}
	if result.LagoWebhookHMACKey != "" {
		t.Fatalf("expected empty hmac key when graphql organization context is missing, got %q", result.LagoWebhookHMACKey)
	}
	if result.LagoProviderCode != "alpha_stripe_test_bpc_test" {
		t.Fatalf("expected provider code to be returned, got %q", result.LagoProviderCode)
	}
}
