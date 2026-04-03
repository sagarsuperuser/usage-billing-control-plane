package service

import "testing"

func TestBuildInvoiceExplainability(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
		"invoice": {
			"lago_id": "inv_123",
			"number": "INV-123",
			"status": "finalized",
			"currency": "USD",
			"total_amount_cents": 170,
			"fees": [
				{
					"lago_id": "fee_1",
					"lago_charge_id": "charge_1",
					"amount_cents": 100,
					"taxes_amount_cents": 20,
					"total_amount_cents": 120,
					"units": 10,
					"events_count": 10,
					"created_at": "2026-03-10T10:00:00Z",
					"charges_from_datetime": "2026-03-01T00:00:00Z",
					"charges_to_datetime": "2026-03-10T00:00:00Z",
					"amount_details": {"charge_model":"graduated","billable_metric_code":"token"},
					"item": {
						"type": "charge",
						"code": "api_calls",
						"name": "API Calls",
						"invoice_display_name": "API Calls (tiered)"
					}
				},
				{
					"lago_id": "fee_2",
					"lago_subscription_id": "sub_1",
					"amount_cents": 50,
					"taxes_amount_cents": 0,
					"total_amount_cents": 50,
					"created_at": "2026-03-10T11:00:00Z",
					"amount_details": {},
					"item": {
						"type": "subscription",
						"code": "startup",
						"name": "Startup Plan"
					}
				}
			]
		}
	}`)

	opts, err := NewInvoiceExplainabilityOptions(nil, "", 0, 0)
	if err != nil {
		t.Fatalf("new explainability options: %v", err)
	}

	out, err := BuildInvoiceExplainability(payload, opts)
	if err != nil {
		t.Fatalf("build explainability: %v", err)
	}

	if out.InvoiceID != "inv_123" {
		t.Fatalf("expected invoice_id inv_123, got %q", out.InvoiceID)
	}
	if out.LineItemsCount != 2 {
		t.Fatalf("expected line_items_count 2, got %d", out.LineItemsCount)
	}
	if len(out.LineItems) != 2 {
		t.Fatalf("expected 2 line items, got %d", len(out.LineItems))
	}
	if out.ExplainabilityDigest == "" {
		t.Fatalf("expected explainability digest")
	}
	if out.LineItems[0].FeeID != "fee_1" {
		t.Fatalf("expected created_at asc ordering with fee_1 first, got %q", out.LineItems[0].FeeID)
	}
	if out.LineItems[0].ComputationMode != "charge:graduated" {
		t.Fatalf("expected charge computation_mode, got %q", out.LineItems[0].ComputationMode)
	}
	if out.LineItems[0].RuleReference != "charge:api_calls" {
		t.Fatalf("expected charge rule reference, got %q", out.LineItems[0].RuleReference)
	}
	if out.LineItems[0].BillableMetricCode != "token" {
		t.Fatalf("expected billable metric code token, got %q", out.LineItems[0].BillableMetricCode)
	}
}

func TestBuildInvoiceExplainability_FilterSortPaginate(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
		"invoice": {
			"lago_id": "inv_123",
			"number": "INV-123",
			"status": "finalized",
			"currency": "USD",
			"total_amount_cents": 400,
			"fees": [
				{"lago_id":"fee_1","amount_cents":100,"taxes_amount_cents":0,"total_amount_cents":100,"created_at":"2026-03-10T10:00:00Z","item":{"type":"charge","code":"a","name":"A"}},
				{"lago_id":"fee_2","amount_cents":220,"taxes_amount_cents":0,"total_amount_cents":220,"created_at":"2026-03-10T10:10:00Z","item":{"type":"charge","code":"b","name":"B"}},
				{"lago_id":"fee_3","amount_cents":80,"taxes_amount_cents":0,"total_amount_cents":80,"created_at":"2026-03-10T10:20:00Z","item":{"type":"subscription","code":"s","name":"S"}}
			]
		}
	}`)

	opts, err := NewInvoiceExplainabilityOptions([]string{"charge"}, "amount_cents_desc", 1, 1)
	if err != nil {
		t.Fatalf("new explainability options: %v", err)
	}

	out, err := BuildInvoiceExplainability(payload, opts)
	if err != nil {
		t.Fatalf("build explainability: %v", err)
	}

	if out.LineItemsCount != 2 {
		t.Fatalf("expected filtered line_items_count 2, got %d", out.LineItemsCount)
	}
	if len(out.LineItems) != 1 {
		t.Fatalf("expected paged line items length 1, got %d", len(out.LineItems))
	}
	if out.LineItems[0].FeeID != "fee_2" {
		t.Fatalf("expected highest amount charge fee_2 first, got %q", out.LineItems[0].FeeID)
	}
}

func TestBuildInvoiceExplainability_AcceptsStringUnitsAndEventsCount(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
		"invoice": {
			"lago_id": "inv_live_1",
			"number": "INV-LIVE-1",
			"status": "finalized",
			"currency": "USD",
			"total_amount_cents": 100,
			"fees": [
				{
					"lago_id": "fee_live_1",
					"amount_cents": 100,
					"taxes_amount_cents": 0,
					"total_amount_cents": 100,
					"units": "1.0",
					"events_count": "1",
					"created_at": "2026-03-10T10:00:00Z",
					"item": {
						"type": "charge",
						"code": "api_calls",
						"name": "API Calls"
					}
				}
			]
		}
	}`)

	opts, err := NewInvoiceExplainabilityOptions(nil, "", 0, 0)
	if err != nil {
		t.Fatalf("new explainability options: %v", err)
	}

	out, err := BuildInvoiceExplainability(payload, opts)
	if err != nil {
		t.Fatalf("build explainability: %v", err)
	}

	if len(out.LineItems) != 1 {
		t.Fatalf("expected one line item, got %d", len(out.LineItems))
	}
	if out.LineItems[0].Units == nil || *out.LineItems[0].Units != 1 {
		t.Fatalf("expected units=1, got %#v", out.LineItems[0].Units)
	}
	if out.LineItems[0].EventsCount == nil || *out.LineItems[0].EventsCount != 1 {
		t.Fatalf("expected events_count=1, got %#v", out.LineItems[0].EventsCount)
	}
}

func TestNewInvoiceExplainabilityOptions_ValidateSort(t *testing.T) {
	t.Parallel()

	if _, err := NewInvoiceExplainabilityOptions(nil, "bad_sort", 0, 0); err == nil {
		t.Fatalf("expected validation error for invalid sort")
	}
}
