package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"lago-usage-billing-alpha/internal/api"
)

func TestEndToEndPreviewReplayReconciliation(t *testing.T) {
	ts := httptest.NewServer(api.NewServer().Handler())
	defer ts.Close()

	rule := postJSON(t, ts.URL+"/v1/rating-rules", map[string]any{
		"name":            "API Calls v1",
		"version":         1,
		"mode":            "graduated",
		"currency":        "USD",
		"graduated_tiers": []map[string]any{{"up_to": 100, "unit_amount_cents": 2}, {"up_to": 0, "unit_amount_cents": 1}},
	}, http.StatusCreated)

	ruleID := rule["id"].(string)
	meter := postJSON(t, ts.URL+"/v1/meters", map[string]any{
		"key":                    "api_calls",
		"name":                   "API Calls",
		"unit":                   "call",
		"aggregation":            "sum",
		"rating_rule_version_id": ruleID,
	}, http.StatusCreated)
	meterID := meter["id"].(string)

	preview := postJSON(t, ts.URL+"/v1/invoices/preview", map[string]any{
		"customer_id": "cust_1",
		"currency":    "USD",
		"items": []map[string]any{{
			"meter_id": meterID,
			"quantity": 120,
		}},
	}, http.StatusOK)

	if int(preview["total_cents"].(float64)) != 220 {
		t.Fatalf("expected preview total 220, got %v", preview["total_cents"])
	}

	_ = postJSON(t, ts.URL+"/v1/usage-events", map[string]any{
		"customer_id": "cust_1",
		"meter_id":    meterID,
		"quantity":    120,
	}, http.StatusCreated)

	_ = postJSON(t, ts.URL+"/v1/billed-entries", map[string]any{
		"customer_id":  "cust_1",
		"meter_id":     meterID,
		"amount_cents": 200,
	}, http.StatusCreated)

	replayResp := postJSON(t, ts.URL+"/v1/replay-jobs", map[string]any{
		"idempotency_key": "idem_1",
		"customer_id":     "cust_1",
	}, http.StatusCreated)
	job := replayResp["job"].(map[string]any)
	jobID := job["id"].(string)

	for i := 0; i < 5; i++ {
		statusResp := getJSON(t, ts.URL+"/v1/replay-jobs/"+jobID, http.StatusOK)
		if statusResp["status"] == "done" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	recon := getJSON(t, ts.URL+"/v1/reconciliation-report?customer_id=cust_1", http.StatusOK)
	if int(recon["mismatch_row_count"].(float64)) != 1 {
		t.Fatalf("expected mismatch row count 1, got %v", recon["mismatch_row_count"])
	}
}

func postJSON(t *testing.T, url string, body any, expectedStatus int) map[string]any {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("post request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != expectedStatus {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected status %d, expected %d, body=%s", resp.StatusCode, expectedStatus, string(b))
	}

	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}

func getJSON(t *testing.T, url string, expectedStatus int) map[string]any {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("get request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != expectedStatus {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected status %d, expected %d, body=%s", resp.StatusCode, expectedStatus, string(b))
	}

	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}
