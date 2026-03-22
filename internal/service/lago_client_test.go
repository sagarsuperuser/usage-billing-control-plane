package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func TestLagoInvoiceAdapterListInvoices(t *testing.T) {
	t.Parallel()

	lago := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/invoices" {
			http.NotFound(w, r)
			return
		}
		if got := r.URL.Query().Get("customer_external_id"); got != "cust_123" {
			t.Fatalf("expected customer_external_id filter, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"invoices":[{"lago_id":"inv_123","number":"INV-123"}],"meta":{"current_page":1}}`))
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

	query := url.Values{}
	query.Set("customer_external_id", "cust_123")
	status, body, err := NewLagoInvoiceAdapter(transport).ListInvoices(context.Background(), query)
	if err != nil {
		t.Fatalf("list invoices: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status 200, got %d", status)
	}
	if !strings.Contains(string(body), "INV-123") {
		t.Fatalf("expected invoice body to contain invoice number, got %s", string(body))
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

func TestLagoPlanSyncAdapter(t *testing.T) {
	t.Parallel()

	var sawCreate bool
	lago := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/billable_metrics/api_calls":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"billable_metric":{"lago_id":"bm_123","code":"api_calls"}}`))
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/plans":
			sawCreate = true
			w.Header().Set("Content-Type", "application/json")
			body, _ := io.ReadAll(r.Body)
			payload := string(body)
			if !strings.Contains(payload, `"code":"growth"`) {
				t.Fatalf("expected plan code in payload, got %s", payload)
			}
			if !strings.Contains(payload, `"pay_in_advance":false`) {
				t.Fatalf("expected pay_in_advance false in plan payload, got %s", payload)
			}
			if !strings.Contains(payload, `"billable_metric_id":"bm_123"`) {
				t.Fatalf("expected billable metric id in payload, got %s", payload)
			}
			if !strings.Contains(payload, `"charge_model":"standard"`) {
				t.Fatalf("expected standard charge model in payload, got %s", payload)
			}
			if !strings.Contains(payload, `"amount":"0.22"`) {
				t.Fatalf("expected decimal amount in payload, got %s", payload)
			}
			_, _ = w.Write([]byte(`{"plan":{"code":"growth"}}`))
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

	err = NewLagoPlanSyncAdapter(transport).SyncPlan(context.Background(), domain.Plan{
		Code:            "growth",
		Name:            "Growth",
		Currency:        "USD",
		BillingInterval: domain.BillingIntervalMonthly,
		BaseAmountCents: 4900,
	}, []PlanSyncComponent{{
		Meter: domain.Meter{
			Key:  "api_calls",
			Name: "API Calls",
		},
		RatingRuleVersion: domain.RatingRuleVersion{
			Mode:            domain.PricingModeFlat,
			Currency:        "USD",
			FlatAmountCents: 22,
		},
	}})
	if err != nil {
		t.Fatalf("sync plan: %v", err)
	}
	if !sawCreate {
		t.Fatalf("expected create request to lago plans endpoint")
	}
}

func TestLagoSubscriptionSyncAdapter(t *testing.T) {
	t.Parallel()

	var sawCreate bool
	startedAt := time.Date(2026, time.January, 1, 12, 30, 0, 0, time.UTC)
	lago := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/subscriptions":
			sawCreate = true
			w.Header().Set("Content-Type", "application/json")
			body, _ := io.ReadAll(r.Body)
			payload := string(body)
			if !strings.Contains(payload, `"external_customer_id":"cust_123"`) {
				t.Fatalf("expected external customer id in payload, got %s", payload)
			}
			if !strings.Contains(payload, `"plan_code":"growth"`) {
				t.Fatalf("expected plan code in payload, got %s", payload)
			}
			if !strings.Contains(payload, `"external_id":"cust_123_growth"`) {
				t.Fatalf("expected external subscription id in payload, got %s", payload)
			}
			if !strings.Contains(payload, `"subscription_at":"2026-01-01T12:30:00Z"`) {
				t.Fatalf("expected subscription_at in payload, got %s", payload)
			}
			_, _ = w.Write([]byte(`{"subscription":{"external_id":"cust_123_growth"}}`))
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

	err = NewLagoSubscriptionSyncAdapter(transport).SyncSubscription(context.Background(),
		domain.Subscription{Code: "cust_123_growth", DisplayName: "Customer Growth", StartedAt: &startedAt},
		domain.Customer{ExternalID: "cust_123"},
		domain.Plan{Code: "growth"},
	)
	if err != nil {
		t.Fatalf("sync subscription: %v", err)
	}
	if !sawCreate {
		t.Fatalf("expected create request to lago subscriptions endpoint")
	}
}

func TestLagoSubscriptionSyncAdapterFallsBackToUpdateForRename(t *testing.T) {
	t.Parallel()

	var sawUpdate bool
	startedAt := time.Date(2026, time.February, 15, 8, 0, 0, 0, time.UTC)
	lago := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/subscriptions":
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"status":422,"error":"already_exists"}`))
			return
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/subscriptions/cust_123_growth":
			sawUpdate = true
			w.Header().Set("Content-Type", "application/json")
			body, _ := io.ReadAll(r.Body)
			payload := string(body)
			if strings.Contains(payload, `"plan_code"`) {
				t.Fatalf("expected update payload to omit plan_code, got %s", payload)
			}
			if !strings.Contains(payload, `"name":"Customer Growth Renamed"`) {
				t.Fatalf("expected renamed subscription in update payload, got %s", payload)
			}
			if !strings.Contains(payload, `"subscription_at":"2026-02-15T08:00:00Z"`) {
				t.Fatalf("expected subscription_at in update payload, got %s", payload)
			}
			_, _ = w.Write([]byte(`{"subscription":{"external_id":"cust_123_growth"}}`))
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

	err = NewLagoSubscriptionSyncAdapter(transport).SyncSubscription(context.Background(),
		domain.Subscription{Code: "cust_123_growth", DisplayName: "Customer Growth Renamed", StartedAt: &startedAt},
		domain.Customer{ExternalID: "cust_123"},
		domain.Plan{Code: "growth_v2"},
	)
	if err != nil {
		t.Fatalf("sync subscription rename: %v", err)
	}
	if !sawUpdate {
		t.Fatalf("expected update request to lago subscriptions endpoint")
	}
}

func TestLagoSubscriptionSyncAdapterArchivesSubscription(t *testing.T) {
	t.Parallel()

	var sawDelete bool
	lago := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/subscriptions/cust_123_growth":
			sawDelete = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"subscription":{"external_id":"cust_123_growth","status":"terminated"}}`))
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

	err = NewLagoSubscriptionSyncAdapter(transport).SyncSubscription(context.Background(),
		domain.Subscription{Code: "cust_123_growth", DisplayName: "Customer Growth", Status: domain.SubscriptionStatusArchived},
		domain.Customer{ExternalID: "cust_123"},
		domain.Plan{Code: "growth"},
	)
	if err != nil {
		t.Fatalf("archive subscription: %v", err)
	}
	if !sawDelete {
		t.Fatalf("expected delete request to lago terminate endpoint")
	}
}

func TestLagoUsageSyncAdapter(t *testing.T) {
	t.Parallel()

	var sawCreate bool
	lago := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/events":
			sawCreate = true
			w.Header().Set("Content-Type", "application/json")
			body, _ := io.ReadAll(r.Body)
			payload := string(body)
			if !strings.Contains(payload, `"code":"api_calls"`) {
				t.Fatalf("expected billable metric code in payload, got %s", payload)
			}
			if !strings.Contains(payload, `"external_subscription_id":"cust_123_growth"`) {
				t.Fatalf("expected external subscription id in payload, got %s", payload)
			}
			if !strings.Contains(payload, `"transaction_id":"evt_sync_123"`) {
				t.Fatalf("expected transaction id in payload, got %s", payload)
			}
			if !strings.Contains(payload, `"value":"12"`) {
				t.Fatalf("expected quantity value in payload, got %s", payload)
			}
			_, _ = w.Write([]byte(`{"event":{"transaction_id":"evt_sync_123"}}`))
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

	err = NewLagoUsageSyncAdapter(transport).SyncUsageEvent(context.Background(),
		domain.UsageEvent{ID: "evt_sync_123", Quantity: 12, Timestamp: time.Unix(1_700_000_000, 0).UTC()},
		domain.Meter{Key: "api_calls", Aggregation: "sum"},
		domain.Subscription{Code: "cust_123_growth"},
	)
	if err != nil {
		t.Fatalf("sync usage event: %v", err)
	}
	if !sawCreate {
		t.Fatalf("expected create request to lago events endpoint")
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
