package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"usage-billing-control-plane/internal/domain"
)

func TestNewLagoHTTPTransportRequiresConfig(t *testing.T) {
	t.Parallel()

	if _, err := NewLagoHTTPTransport(LagoClientConfig{}); err == nil {
		t.Fatalf("expected constructor error for missing config")
	}
}

func TestLagoInvoiceAdapterGetInvoice(t *testing.T) {
	t.Parallel()

	lago := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/invoices/inv_123" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"invoice":{"lago_id":"inv_123"}}`))
	}))
	defer lago.Close()

	transport, err := NewLagoHTTPTransport(LagoClientConfig{
		BaseURL: lago.URL,
		APIKey:  "test",
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}

	status, body, err := NewLagoInvoiceAdapter(transport).GetInvoice(context.Background(), "inv_123")
	if err != nil {
		t.Fatalf("proxy invoice by id: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status 200, got %d", status)
	}
	if !strings.Contains(string(body), "inv_123") {
		t.Fatalf("expected invoice body to contain invoice id, got %s", string(body))
	}
}

func TestLagoCustomerBillingAdapter(t *testing.T) {
	t.Parallel()

	lago := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/customers":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"customer":{"lago_id":"lago_cust_123","external_id":"cust_123","billing_configuration":{"payment_provider":"stripe","payment_provider_code":"stripe_test","provider_customer_id":"pcus_123","provider_payment_methods":["card"]}}}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/customers/cust_123":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"customer":{"lago_id":"lago_cust_123","external_id":"cust_123","billing_configuration":{"payment_provider":"stripe","payment_provider_code":"stripe_test","provider_customer_id":"pcus_123","provider_payment_methods":["card"]}}}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/customers/cust_123/payment_methods":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"payment_methods":[{"lago_id":"pm_lago_123","is_default":true,"provider_method_id":"pm_123"}]}`))
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/customers/cust_123/checkout_url":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"customer":{"checkout_url":"https://checkout.example.test/cust_123"}}`))
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer lago.Close()

	transport, err := NewLagoHTTPTransport(LagoClientConfig{
		BaseURL: lago.URL,
		APIKey:  "test",
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}

	adapter := NewLagoCustomerBillingAdapter(transport)
	status, body, err := adapter.UpsertCustomer(context.Background(), []byte(`{"customer":{"external_id":"cust_123"}}`))
	if err != nil {
		t.Fatalf("upsert customer: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status 200, got %d", status)
	}
	if !strings.Contains(string(body), "lago_cust_123") {
		t.Fatalf("expected lago customer id in response, got %s", string(body))
	}

	status, body, err = adapter.GetCustomer(context.Background(), "cust_123")
	if err != nil {
		t.Fatalf("get customer: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status 200, got %d", status)
	}
	if !strings.Contains(string(body), "pcus_123") {
		t.Fatalf("expected provider customer id in response, got %s", string(body))
	}

	status, body, err = adapter.ListCustomerPaymentMethods(context.Background(), "cust_123")
	if err != nil {
		t.Fatalf("list customer payment methods: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status 200, got %d", status)
	}
	if !strings.Contains(string(body), "pm_123") {
		t.Fatalf("expected provider payment method id in response, got %s", string(body))
	}

	status, body, err = adapter.GenerateCustomerCheckoutURL(context.Background(), "cust_123")
	if err != nil {
		t.Fatalf("generate customer checkout url: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status 200, got %d", status)
	}
	if !strings.Contains(string(body), "checkout.example.test/cust_123") {
		t.Fatalf("expected checkout url in response, got %s", string(body))
	}
}

func TestLagoAdaptersWithRealLago(t *testing.T) {
	baseURL := strings.TrimSpace(os.Getenv("TEST_LAGO_API_URL"))
	apiKey := strings.TrimSpace(os.Getenv("TEST_LAGO_API_KEY"))
	if baseURL == "" || apiKey == "" {
		t.Skip("TEST_LAGO_API_URL and TEST_LAGO_API_KEY are required for real Lago tests")
	}
	transport, err := NewLagoHTTPTransport(LagoClientConfig{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Timeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}

	err = NewLagoMeterSyncAdapter(transport).SyncMeter(context.Background(), domain.Meter{
		Key:                 "alpha_test_meter",
		Name:                "Alpha Test Meter",
		Aggregation:         "count",
		RatingRuleVersionID: "rrv_test",
	})
	if err != nil {
		t.Fatalf("sync meter with real lago: %v", err)
	}

	status, body, err := NewLagoInvoiceAdapter(transport).PreviewInvoice(context.Background(), []byte(`{}`))
	if err != nil {
		t.Fatalf("proxy invoice preview with real lago: %v", err)
	}
	if status == 0 {
		t.Fatalf("expected non-zero status from lago preview proxy")
	}
	if !strings.HasPrefix(strings.TrimSpace(string(body)), "{") {
		t.Fatalf("expected json response body, got %q", string(body))
	}
}
