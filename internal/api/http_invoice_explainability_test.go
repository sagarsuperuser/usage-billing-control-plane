package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"usage-billing-control-plane/internal/api"
	"usage-billing-control-plane/internal/service"
)

func TestInvoiceExplainabilityEndpoint(t *testing.T) {
	t.Skip("requires Lago mock server; will be rewritten for Stripe-direct adapter")
	t.Parallel()

	lago := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/invoices/inv_123" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"invoice": {
				"lago_id": "inv_123",
				"number": "INV-123",
				"status": "finalized",
				"currency": "USD",
				"total_amount_cents": 300,
				"fees": [
					{
						"lago_id":"fee_1",
						"amount_cents":100,
						"taxes_amount_cents":20,
						"total_amount_cents":120,
						"created_at":"2026-03-10T10:00:00Z",
						"amount_details":{"charge_model":"graduated"},
						"item":{"type":"charge","code":"api_calls","name":"API Calls"}
					},
					{
						"lago_id":"fee_2",
						"amount_cents":180,
						"taxes_amount_cents":0,
						"total_amount_cents":180,
						"created_at":"2026-03-10T10:01:00Z",
						"item":{"type":"subscription","code":"startup","name":"Startup"}
					}
				]
			}
		}`))
	}))
	defer lago.Close()

	lagoTransport, err := service.NewLagoHTTPTransport(service.LagoClientConfig{
		BaseURL: lago.URL,
		APIKey:  "test",
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}

	ts := httptest.NewServer(api.NewServer(nil, api.WithMeterSyncAdapter(service.NewLagoMeterSyncAdapter(lagoTransport)), api.WithInvoiceBillingAdapter(service.NewLagoInvoiceAdapter(lagoTransport))).Handler())
	defer ts.Close()

	resp := getJSON(t, ts.URL+"/v1/invoices/inv_123/explainability?fee_types=charge&line_item_sort=amount_cents_desc&limit=1&page=1", "", http.StatusOK)
	if got, _ := resp["invoice_id"].(string); got != "inv_123" {
		t.Fatalf("expected invoice_id inv_123, got %q", got)
	}
	if got, ok := resp["line_items_count"].(float64); !ok || int(got) != 1 {
		t.Fatalf("expected line_items_count=1, got %v", resp["line_items_count"])
	}
	if _, ok := resp["explainability_digest"].(string); !ok {
		t.Fatalf("expected explainability_digest in response")
	}
	items := listItemsFromResponse(t, map[string]any{"items": resp["line_items"]})
	if len(items) != 1 {
		t.Fatalf("expected 1 line item after pagination, got %d", len(items))
	}
	row, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("expected explainability line item object")
	}
	if got, _ := row["fee_id"].(string); got != "fee_1" {
		t.Fatalf("expected filtered charge fee_1, got %q", got)
	}
}

func TestInvoiceExplainabilityEndpoint_InvalidSort(t *testing.T) {
	t.Parallel()

	lago := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"invoice":{"lago_id":"inv_123","number":"INV-123","status":"finalized","currency":"USD","total_amount_cents":0,"fees":[]}}`))
	}))
	defer lago.Close()

	lagoTransport, err := service.NewLagoHTTPTransport(service.LagoClientConfig{
		BaseURL: lago.URL,
		APIKey:  "test",
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}

	ts := httptest.NewServer(api.NewServer(nil, api.WithMeterSyncAdapter(service.NewLagoMeterSyncAdapter(lagoTransport)), api.WithInvoiceBillingAdapter(service.NewLagoInvoiceAdapter(lagoTransport))).Handler())
	defer ts.Close()

	resp := getJSON(t, ts.URL+"/v1/invoices/inv_123/explainability?line_item_sort=bad_sort", "", http.StatusBadRequest)
	if got := resp["error"]; got == nil {
		t.Fatalf("expected validation error for invalid sort")
	}
}
