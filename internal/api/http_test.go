package api_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	temporalclient "go.temporal.io/sdk/client"
	temporalsdkworker "go.temporal.io/sdk/worker"

	"usage-billing-control-plane/internal/api"
	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/reconcile"
	"usage-billing-control-plane/internal/replay"
	"usage-billing-control-plane/internal/service"
	"usage-billing-control-plane/internal/store"
	"usage-billing-control-plane/internal/temporalutil"
)

type fakeCustomerPaymentSetupRequestEmailSender struct {
	inputs []service.CustomerPaymentSetupRequestEmail
	err    error
}

func (s *fakeCustomerPaymentSetupRequestEmailSender) SendCustomerPaymentSetupRequest(input service.CustomerPaymentSetupRequestEmail) error {
	s.inputs = append(s.inputs, input)
	return s.err
}

func TestEndToEndPreviewReplayReconciliation(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for integration tests")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := store.NewPostgresStore(db)
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	resetTables(t, db)

	mustCreateAPIKey(t, repo, "test-reader-key", api.RoleReader, "default")
	mustCreateAPIKey(t, repo, "test-writer-key", api.RoleWriter, "default")
	mustCreateAPIKey(t, repo, "test-admin-key", api.RoleAdmin, "default")
	mustCreatePlatformAPIKey(t, repo, "test-platform-admin")

	replayMetricsProvider, replayRuntimeCleanup := startTemporalReplayRuntime(t, repo)
	defer replayRuntimeCleanup()

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}
	lagoTransport, lagoCleanup := newTestLagoTransport(t)
	defer lagoCleanup()

	ts := httptest.NewServer(api.NewServer(repo, api.WithMetricsProvider(replayMetricsProvider), api.WithAPIKeyAuthorizer(authorizer), api.WithMeterSyncAdapter(service.NewLagoMeterSyncAdapter(lagoTransport)), api.WithInvoiceBillingAdapter(service.NewLagoInvoiceAdapter(lagoTransport))).Handler())
	defer ts.Close()

	getJSON(t, ts.URL+"/v1/rating-rules", "", http.StatusUnauthorized)
	_ = postJSON(t, ts.URL+"/v1/rating-rules", map[string]any{
		"name":            "Forbidden write",
		"version":         1,
		"mode":            "graduated",
		"currency":        "USD",
		"graduated_tiers": []map[string]any{{"up_to": 10, "unit_amount_cents": 2}, {"up_to": 0, "unit_amount_cents": 1}},
	}, "test-reader-key", http.StatusForbidden)

	rule := postJSON(t, ts.URL+"/v1/rating-rules", map[string]any{
		"rule_key":        "api_calls",
		"name":            "API Calls v1",
		"version":         1,
		"lifecycle_state": "active",
		"mode":            "graduated",
		"currency":        "USD",
		"graduated_tiers": []map[string]any{{"up_to": 100, "unit_amount_cents": 2}, {"up_to": 0, "unit_amount_cents": 1}},
	}, "test-writer-key", http.StatusCreated)

	ruleID := rule["id"].(string)
	meter := postJSON(t, ts.URL+"/v1/meters", map[string]any{
		"key":                    "api_calls",
		"name":                   "API Calls",
		"unit":                   "call",
		"aggregation":            "sum",
		"rating_rule_version_id": ruleID,
	}, "test-writer-key", http.StatusCreated)
	meterID := meter["id"].(string)

	usageCreated := postJSON(t, ts.URL+"/v1/usage-events", map[string]any{
		"customer_id":     "cust_1",
		"meter_id":        meterID,
		"quantity":        120,
		"idempotency_key": "evt-idem-1",
	}, "test-writer-key", http.StatusCreated)
	usageCreatedID, _ := usageCreated["id"].(string)
	if strings.TrimSpace(usageCreatedID) == "" {
		t.Fatalf("expected created usage event id")
	}
	usageIdempotent := postJSON(t, ts.URL+"/v1/usage-events", map[string]any{
		"customer_id":     "cust_1",
		"meter_id":        meterID,
		"quantity":        120,
		"idempotency_key": "evt-idem-1",
	}, "test-writer-key", http.StatusOK)
	if gotID, _ := usageIdempotent["id"].(string); gotID != usageCreatedID {
		t.Fatalf("expected idempotent usage event to return existing id %q, got %q", usageCreatedID, gotID)
	}
	usageIdempotencyConflict := postJSON(t, ts.URL+"/v1/usage-events", map[string]any{
		"customer_id":     "cust_1",
		"meter_id":        meterID,
		"quantity":        121,
		"idempotency_key": "evt-idem-1",
	}, "test-writer-key", http.StatusConflict)
	if got, _ := usageIdempotencyConflict["error"].(string); !strings.Contains(strings.ToLower(got), "different payload") {
		t.Fatalf("expected usage idempotency conflict error, got %q", got)
	}

	_ = postJSON(t, ts.URL+"/v1/usage-events", map[string]any{
		"customer_id": "cust_2",
		"meter_id":    meterID,
		"quantity":    5,
	}, "test-writer-key", http.StatusCreated)

	usagePageOne := getJSON(
		t,
		ts.URL+"/v1/usage-events?meter_id="+url.QueryEscape(meterID)+"&limit=1",
		"test-reader-key",
		http.StatusOK,
	)
	usageItemsPageOne := listItemsFromResponse(t, usagePageOne)
	if len(usageItemsPageOne) != 1 {
		t.Fatalf("expected first usage events page size 1, got %d", len(usageItemsPageOne))
	}
	usageNextCursor, _ := usagePageOne["next_cursor"].(string)
	if strings.TrimSpace(usageNextCursor) == "" {
		t.Fatalf("expected usage events next_cursor on first page")
	}
	usagePageTwo := getJSON(
		t,
		ts.URL+"/v1/usage-events?meter_id="+url.QueryEscape(meterID)+"&limit=1&cursor="+url.QueryEscape(usageNextCursor),
		"test-reader-key",
		http.StatusOK,
	)
	usageItemsPageTwo := listItemsFromResponse(t, usagePageTwo)
	if len(usageItemsPageTwo) != 1 {
		t.Fatalf("expected second usage events page size 1, got %d", len(usageItemsPageTwo))
	}
	usageFull := getJSON(
		t,
		ts.URL+"/v1/usage-events?meter_id="+url.QueryEscape(meterID)+"&limit=10",
		"test-reader-key",
		http.StatusOK,
	)
	usageFullItems := listItemsFromResponse(t, usageFull)
	if len(usageFullItems) != 2 {
		t.Fatalf("expected usage list to contain 2 unique events after idempotent retry, got %d", len(usageFullItems))
	}
	usageCursorOffsetErr := getJSON(
		t,
		ts.URL+"/v1/usage-events?meter_id="+url.QueryEscape(meterID)+"&limit=1&offset=1&cursor="+url.QueryEscape(usageNextCursor),
		"test-reader-key",
		http.StatusBadRequest,
	)
	if got, _ := usageCursorOffsetErr["error"].(string); !strings.Contains(got, "offset cannot be used with cursor") {
		t.Fatalf("expected usage offset/cursor validation error, got %q", got)
	}
	usageBadCursor := getJSON(t, ts.URL+"/v1/usage-events?cursor=invalid", "test-reader-key", http.StatusBadRequest)
	if got, _ := usageBadCursor["error"].(string); !strings.Contains(got, "invalid cursor") {
		t.Fatalf("expected usage invalid cursor error, got %q", got)
	}
	usageDesc := getJSON(
		t,
		ts.URL+"/v1/usage-events?meter_id="+url.QueryEscape(meterID)+"&limit=1&order=desc",
		"test-reader-key",
		http.StatusOK,
	)
	usageDescItems := listItemsFromResponse(t, usageDesc)
	if len(usageDescItems) != 1 {
		t.Fatalf("expected usage desc page size 1, got %d", len(usageDescItems))
	}
	usageDescRow, ok := usageDescItems[0].(map[string]any)
	if !ok {
		t.Fatalf("expected usage desc row object")
	}
	if customerID, _ := usageDescRow["customer_id"].(string); customerID != "cust_2" {
		t.Fatalf("expected usage desc first row customer_id=cust_2, got %q", customerID)
	}
	usageBadOrder := getJSON(t, ts.URL+"/v1/usage-events?order=invalid", "test-reader-key", http.StatusBadRequest)
	if got, _ := usageBadOrder["error"].(string); !strings.Contains(got, "order must be asc or desc") {
		t.Fatalf("expected usage invalid order error, got %q", got)
	}

	billedCreated := postJSON(t, ts.URL+"/v1/billed-entries", map[string]any{
		"customer_id":     "cust_1",
		"meter_id":        meterID,
		"amount_cents":    200,
		"idempotency_key": "bil-idem-1",
	}, "test-writer-key", http.StatusCreated)
	billedCreatedID, _ := billedCreated["id"].(string)
	if strings.TrimSpace(billedCreatedID) == "" {
		t.Fatalf("expected created billed entry id")
	}
	billedIdempotent := postJSON(t, ts.URL+"/v1/billed-entries", map[string]any{
		"customer_id":     "cust_1",
		"meter_id":        meterID,
		"amount_cents":    200,
		"idempotency_key": "bil-idem-1",
	}, "test-writer-key", http.StatusOK)
	if gotID, _ := billedIdempotent["id"].(string); gotID != billedCreatedID {
		t.Fatalf("expected idempotent billed entry to return existing id %q, got %q", billedCreatedID, gotID)
	}
	billedIdempotencyConflict := postJSON(t, ts.URL+"/v1/billed-entries", map[string]any{
		"customer_id":     "cust_1",
		"meter_id":        meterID,
		"amount_cents":    201,
		"idempotency_key": "bil-idem-1",
	}, "test-writer-key", http.StatusConflict)
	if got, _ := billedIdempotencyConflict["error"].(string); !strings.Contains(strings.ToLower(got), "different payload") {
		t.Fatalf("expected billed idempotency conflict error, got %q", got)
	}

	replayResp := postJSON(t, ts.URL+"/v1/replay-jobs", map[string]any{
		"idempotency_key": "idem_1",
		"customer_id":     "cust_1",
	}, "test-writer-key", http.StatusCreated)
	job := replayResp["job"].(map[string]any)
	jobID := job["id"].(string)
	createdLinks, ok := job["artifact_links"].(map[string]any)
	if !ok {
		t.Fatalf("expected replay create response to include artifact_links")
	}
	if strings.TrimSpace(createdLinks["report_json"].(string)) == "" {
		t.Fatalf("expected report_json artifact link in replay create response")
	}
	if _, ok := job["workflow_telemetry"].(map[string]any); !ok {
		t.Fatalf("expected replay create response to include workflow_telemetry")
	}
	replayRespIdempotent := postJSON(t, ts.URL+"/v1/replay-jobs", map[string]any{
		"idempotency_key": "idem_1",
		"customer_id":     "cust_1",
	}, "test-writer-key", http.StatusOK)
	if idem, _ := replayRespIdempotent["idempotent_replay"].(bool); !idem {
		t.Fatalf("expected idempotent_replay=true on replay retry")
	}
	replayJobIdempotent, ok := replayRespIdempotent["job"].(map[string]any)
	if !ok {
		t.Fatalf("expected replay idempotent response job object")
	}
	if gotID, _ := replayJobIdempotent["id"].(string); gotID != jobID {
		t.Fatalf("expected idempotent replay retry to return existing job id %q, got %q", jobID, gotID)
	}
	replayConflict := postJSON(t, ts.URL+"/v1/replay-jobs", map[string]any{
		"idempotency_key": "idem_1",
		"customer_id":     "cust_conflict",
	}, "test-writer-key", http.StatusConflict)
	if got, _ := replayConflict["error"].(string); !strings.Contains(strings.ToLower(got), "different payload") {
		t.Fatalf("expected replay idempotency conflict error, got %q", got)
	}

	var replayStatusResp map[string]any
	for i := 0; i < 200; i++ {
		replayStatusResp = getJSON(t, ts.URL+"/v1/replay-jobs/"+jobID, "test-reader-key", http.StatusOK)
		if replayStatusResp["status"] == "done" {
			break
		}
		time.Sleep(25 * time.Millisecond)
		if i == 199 {
			t.Fatalf("replay job did not finish")
		}
	}
	statusTelemetry, ok := replayStatusResp["workflow_telemetry"].(map[string]any)
	if !ok {
		t.Fatalf("expected replay status response to include workflow_telemetry")
	}
	if got, _ := statusTelemetry["current_step"].(string); got != "completed" {
		t.Fatalf("expected replay workflow_telemetry.current_step=completed, got %q", got)
	}
	if got, _ := statusTelemetry["progress_percent"].(float64); int(got) != 100 {
		t.Fatalf("expected replay workflow_telemetry.progress_percent=100, got %v", statusTelemetry["progress_percent"])
	}

	statusLinks, ok := replayStatusResp["artifact_links"].(map[string]any)
	if !ok {
		t.Fatalf("expected replay status response to include artifact_links")
	}
	reportJSONURL, _ := statusLinks["report_json"].(string)
	reportCSVURL, _ := statusLinks["report_csv"].(string)
	digestURL, _ := statusLinks["dataset_digest"].(string)
	if strings.TrimSpace(reportJSONURL) == "" || strings.TrimSpace(reportCSVURL) == "" || strings.TrimSpace(digestURL) == "" {
		t.Fatalf("expected non-empty replay artifact links")
	}
	reportJSONBody, reportJSONHeaders := getRaw(t, reportJSONURL, "test-reader-key", http.StatusOK)
	if !strings.Contains(strings.ToLower(reportJSONHeaders.Get("Content-Type")), "application/json") {
		t.Fatalf("expected report.json content type json, got %q", reportJSONHeaders.Get("Content-Type"))
	}
	if !strings.Contains(reportJSONBody, "\"job_id\"") {
		t.Fatalf("expected report.json payload to include job_id")
	}
	reportCSVBody, reportCSVHeaders := getRaw(t, reportCSVURL, "test-reader-key", http.StatusOK)
	if !strings.Contains(strings.ToLower(reportCSVHeaders.Get("Content-Type")), "text/csv") {
		t.Fatalf("expected report.csv content type text/csv, got %q", reportCSVHeaders.Get("Content-Type"))
	}
	if !strings.Contains(reportCSVBody, "job_id,status,customer_id,meter_id") {
		t.Fatalf("expected report.csv header")
	}
	digestBody, digestHeaders := getRaw(t, digestURL, "test-reader-key", http.StatusOK)
	if !strings.Contains(strings.ToLower(digestHeaders.Get("Content-Type")), "text/plain") {
		t.Fatalf("expected dataset_digest content type text/plain, got %q", digestHeaders.Get("Content-Type"))
	}
	if len(strings.TrimSpace(digestBody)) != 64 {
		t.Fatalf("expected dataset digest length 64, got %d", len(strings.TrimSpace(digestBody)))
	}

	replayResp2 := postJSON(t, ts.URL+"/v1/replay-jobs", map[string]any{
		"idempotency_key": "idem_2",
		"customer_id":     "cust_ghost",
	}, "test-writer-key", http.StatusCreated)
	job2 := replayResp2["job"].(map[string]any)
	jobID2 := job2["id"].(string)
	if jobID2 == "" {
		t.Fatalf("expected second replay job id")
	}

	replayPageOne := getJSON(t, ts.URL+"/v1/replay-jobs?limit=1", "test-reader-key", http.StatusOK)
	replayPageOneItems := listItemsFromResponse(t, replayPageOne)
	if len(replayPageOneItems) != 1 {
		t.Fatalf("expected replay jobs first page size 1, got %d", len(replayPageOneItems))
	}
	replayPageOneRow, ok := replayPageOneItems[0].(map[string]any)
	if !ok {
		t.Fatalf("expected replay jobs first page row object")
	}
	if _, ok := replayPageOneRow["workflow_telemetry"].(map[string]any); !ok {
		t.Fatalf("expected replay jobs list row to include workflow_telemetry")
	}
	if _, ok := replayPageOneRow["artifact_links"].(map[string]any); !ok {
		t.Fatalf("expected replay jobs list row to include artifact_links")
	}
	replayNextCursor, _ := replayPageOne["next_cursor"].(string)
	if strings.TrimSpace(replayNextCursor) == "" {
		t.Fatalf("expected replay jobs next_cursor on first page")
	}
	replayPageTwo := getJSON(t, ts.URL+"/v1/replay-jobs?limit=1&cursor="+url.QueryEscape(replayNextCursor), "test-reader-key", http.StatusOK)
	replayPageTwoItems := listItemsFromResponse(t, replayPageTwo)
	if len(replayPageTwoItems) != 1 {
		t.Fatalf("expected replay jobs second page size 1, got %d", len(replayPageTwoItems))
	}
	replayStatusFiltered := getJSON(t, ts.URL+"/v1/replay-jobs?status=done&limit=10", "test-reader-key", http.StatusOK)
	replayDoneItems := listItemsFromResponse(t, replayStatusFiltered)
	if len(replayDoneItems) == 0 {
		t.Fatalf("expected at least one done replay job")
	}
	replayCursorOffsetErr := getJSON(t, ts.URL+"/v1/replay-jobs?limit=1&offset=1&cursor="+url.QueryEscape(replayNextCursor), "test-reader-key", http.StatusBadRequest)
	if got, _ := replayCursorOffsetErr["error"].(string); !strings.Contains(got, "offset cannot be used with cursor") {
		t.Fatalf("expected replay offset/cursor validation error, got %q", got)
	}
	replayBadCursor := getJSON(t, ts.URL+"/v1/replay-jobs?cursor=invalid", "test-reader-key", http.StatusBadRequest)
	if got, _ := replayBadCursor["error"].(string); !strings.Contains(got, "invalid cursor") {
		t.Fatalf("expected replay invalid cursor error, got %q", got)
	}
	replayBadStatus := getJSON(t, ts.URL+"/v1/replay-jobs?status=unknown", "test-reader-key", http.StatusBadRequest)
	if got, _ := replayBadStatus["error"].(string); !strings.Contains(got, "invalid status") {
		t.Fatalf("expected replay invalid status error, got %q", got)
	}
	replayDiag := getJSON(t, ts.URL+"/v1/replay-jobs/"+jobID+"/events", "test-reader-key", http.StatusOK)
	if int(replayDiag["usage_events_count"].(float64)) != 1 {
		t.Fatalf("expected replay diagnostics usage_events_count=1, got %v", replayDiag["usage_events_count"])
	}
	if int(replayDiag["usage_quantity"].(float64)) != 120 {
		t.Fatalf("expected replay diagnostics usage_quantity=120, got %v", replayDiag["usage_quantity"])
	}
	if int(replayDiag["billed_entries_count"].(float64)) != 2 {
		t.Fatalf("expected replay diagnostics billed_entries_count=2, got %v", replayDiag["billed_entries_count"])
	}
	if int(replayDiag["billed_amount_cents"].(float64)) != 220 {
		t.Fatalf("expected replay diagnostics billed_amount_cents=220, got %v", replayDiag["billed_amount_cents"])
	}
	replayRetryDone := postJSON(t, ts.URL+"/v1/replay-jobs/"+jobID+"/retry", map[string]any{}, "test-writer-key", http.StatusBadRequest)
	if got, _ := replayRetryDone["error"].(string); !strings.Contains(strings.ToLower(got), "status=failed") {
		t.Fatalf("expected replay retry invalid-state error, got %q", got)
	}
	replayRetryForbidden := postJSON(t, ts.URL+"/v1/replay-jobs/"+jobID+"/retry", map[string]any{}, "test-reader-key", http.StatusForbidden)
	if got, _ := replayRetryForbidden["error"].(string); !strings.Contains(strings.ToLower(got), "forbidden") {
		t.Fatalf("expected replay retry forbidden error, got %q", got)
	}

	recon := getJSON(t, ts.URL+"/v1/reconciliation-report?customer_id=cust_1", "test-reader-key", http.StatusOK)
	if int(recon["mismatch_row_count"].(float64)) != 0 {
		t.Fatalf("expected mismatch row count 0 after replay adjustments, got %v", recon["mismatch_row_count"])
	}
	if int(recon["total_delta_cents"].(float64)) != 0 {
		t.Fatalf("expected total_delta_cents 0 after replay adjustments, got %v", recon["total_delta_cents"])
	}
	reconAPIOnly := getJSON(t, ts.URL+"/v1/reconciliation-report?customer_id=cust_1&billed_source=api", "test-reader-key", http.StatusOK)
	if int(reconAPIOnly["mismatch_row_count"].(float64)) != 1 {
		t.Fatalf("expected mismatch row count 1 for billed_source=api, got %v", reconAPIOnly["mismatch_row_count"])
	}
	if int(reconAPIOnly["total_billed_cents"].(float64)) != 200 {
		t.Fatalf("expected total_billed_cents 200 for billed_source=api, got %v", reconAPIOnly["total_billed_cents"])
	}
	reconReplayOnly := getJSON(t, ts.URL+"/v1/reconciliation-report?customer_id=cust_1&billed_source=replay_adjustment&billed_replay_job_id="+url.QueryEscape(jobID), "test-reader-key", http.StatusOK)
	if int(reconReplayOnly["mismatch_row_count"].(float64)) != 1 {
		t.Fatalf("expected mismatch row count 1 for replay adjustment-only view, got %v", reconReplayOnly["mismatch_row_count"])
	}
	if int(reconReplayOnly["total_billed_cents"].(float64)) != 20 {
		t.Fatalf("expected total_billed_cents 20 for replay adjustment-only view, got %v", reconReplayOnly["total_billed_cents"])
	}
	reconMismatchOnly := getJSON(t, ts.URL+"/v1/reconciliation-report?customer_id=cust_1&billed_source=api&mismatch_only=true", "test-reader-key", http.StatusOK)
	if int(reconMismatchOnly["mismatch_row_count"].(float64)) != 1 {
		t.Fatalf("expected mismatch_only report mismatch_row_count=1, got %v", reconMismatchOnly["mismatch_row_count"])
	}
	reconAbsDeltaTooHigh := getJSON(t, ts.URL+"/v1/reconciliation-report?customer_id=cust_1&billed_source=api&abs_delta_gte=30", "test-reader-key", http.StatusOK)
	if int(reconAbsDeltaTooHigh["mismatch_row_count"].(float64)) != 0 {
		t.Fatalf("expected abs_delta_gte=30 report mismatch_row_count=0, got %v", reconAbsDeltaTooHigh["mismatch_row_count"])
	}
	reconAbsDeltaMatch := getJSON(t, ts.URL+"/v1/reconciliation-report?customer_id=cust_1&billed_source=api&abs_delta_gte=20", "test-reader-key", http.StatusOK)
	if int(reconAbsDeltaMatch["mismatch_row_count"].(float64)) != 1 {
		t.Fatalf("expected abs_delta_gte=20 report mismatch_row_count=1, got %v", reconAbsDeltaMatch["mismatch_row_count"])
	}
	reconBadMismatchOnly := getJSON(t, ts.URL+"/v1/reconciliation-report?customer_id=cust_1&mismatch_only=abc", "test-reader-key", http.StatusBadRequest)
	if got, _ := reconBadMismatchOnly["error"].(string); !strings.Contains(strings.ToLower(got), "mismatch_only") {
		t.Fatalf("expected mismatch_only validation error, got %q", got)
	}
	reconBadAbsDelta := getJSON(t, ts.URL+"/v1/reconciliation-report?customer_id=cust_1&abs_delta_gte=-1", "test-reader-key", http.StatusBadRequest)
	if got, _ := reconBadAbsDelta["error"].(string); !strings.Contains(strings.ToLower(got), "abs_delta_gte") {
		t.Fatalf("expected abs_delta_gte validation error, got %q", got)
	}
	invalidFilter := getJSON(t, ts.URL+"/v1/reconciliation-report?customer_id=cust_1&billed_source=invalid", "test-reader-key", http.StatusBadRequest)
	if got, _ := invalidFilter["error"].(string); !strings.Contains(got, "invalid billed_source") {
		t.Fatalf("expected invalid billed_source error, got %q", got)
	}

	billedEntries, err := repo.ListBilledEntries(store.Filter{
		TenantID:   "default",
		CustomerID: "cust_1",
		MeterID:    meterID,
	})
	if err != nil {
		t.Fatalf("list billed entries: %v", err)
	}
	if len(billedEntries) != 2 {
		t.Fatalf("expected 2 billed entries (api + replay adjustment), got %d", len(billedEntries))
	}

	var foundAPI bool
	var foundReplayAdjustment bool
	for _, entry := range billedEntries {
		switch entry.Source {
		case domain.BilledEntrySourceAPI:
			foundAPI = true
			if entry.ReplayJobID != "" {
				t.Fatalf("expected api billed entry replay_job_id to be empty, got %q", entry.ReplayJobID)
			}
		case domain.BilledEntrySourceReplayAdjustment:
			foundReplayAdjustment = true
			if entry.ReplayJobID != jobID {
				t.Fatalf("expected replay adjustment replay_job_id=%s, got %q", jobID, entry.ReplayJobID)
			}
		default:
			t.Fatalf("unexpected billed entry source: %q", entry.Source)
		}
	}
	if !foundAPI || !foundReplayAdjustment {
		t.Fatalf("expected both api and replay adjustment billed entry sources, got api=%t replay_adjustment=%t", foundAPI, foundReplayAdjustment)
	}

	billedList := getJSON(
		t,
		ts.URL+"/v1/billed-entries?customer_id=cust_1&meter_id="+url.QueryEscape(meterID)+"&billed_source=replay_adjustment&billed_replay_job_id="+url.QueryEscape(jobID)+"&limit=10",
		"test-reader-key",
		http.StatusOK,
	)
	listItems := listItemsFromResponse(t, billedList)
	if len(listItems) != 1 {
		t.Fatalf("expected one replay_adjustment billed entry from GET /v1/billed-entries, got %d", len(listItems))
	}
	row, ok := listItems[0].(map[string]any)
	if !ok {
		t.Fatalf("expected billed entry row object")
	}
	if source, _ := row["source"].(string); source != "replay_adjustment" {
		t.Fatalf("expected billed entry source replay_adjustment, got %q", source)
	}
	if replayJobID, _ := row["replay_job_id"].(string); replayJobID != jobID {
		t.Fatalf("expected billed entry replay_job_id=%s, got %q", jobID, replayJobID)
	}
	billedInvalid := getJSON(t, ts.URL+"/v1/billed-entries?billed_source=invalid", "test-reader-key", http.StatusBadRequest)
	if got, _ := billedInvalid["error"].(string); !strings.Contains(got, "billed_source") {
		t.Fatalf("expected billed_source validation error, got %q", got)
	}

	billedPageOne := getJSON(
		t,
		ts.URL+"/v1/billed-entries?customer_id=cust_1&meter_id="+url.QueryEscape(meterID)+"&limit=1",
		"test-reader-key",
		http.StatusOK,
	)
	billedPageOneItems := listItemsFromResponse(t, billedPageOne)
	if len(billedPageOneItems) != 1 {
		t.Fatalf("expected first billed entries page size 1, got %d", len(billedPageOneItems))
	}
	billedNextCursor, _ := billedPageOne["next_cursor"].(string)
	if strings.TrimSpace(billedNextCursor) == "" {
		t.Fatalf("expected next_cursor for billed entries first page")
	}

	billedPageTwo := getJSON(
		t,
		ts.URL+"/v1/billed-entries?customer_id=cust_1&meter_id="+url.QueryEscape(meterID)+"&limit=1&cursor="+url.QueryEscape(billedNextCursor),
		"test-reader-key",
		http.StatusOK,
	)
	billedPageTwoItems := listItemsFromResponse(t, billedPageTwo)
	if len(billedPageTwoItems) != 1 {
		t.Fatalf("expected second billed entries page size 1, got %d", len(billedPageTwoItems))
	}
	if next, _ := billedPageTwo["next_cursor"].(string); strings.TrimSpace(next) != "" {
		t.Fatalf("expected no next_cursor on final billed entries page, got %q", next)
	}

	billedCursorOffsetErr := getJSON(
		t,
		ts.URL+"/v1/billed-entries?customer_id=cust_1&meter_id="+url.QueryEscape(meterID)+"&limit=1&offset=1&cursor="+url.QueryEscape(billedNextCursor),
		"test-reader-key",
		http.StatusBadRequest,
	)
	if got, _ := billedCursorOffsetErr["error"].(string); !strings.Contains(got, "offset cannot be used with cursor") {
		t.Fatalf("expected offset/cursor validation error, got %q", got)
	}

	billedBadCursor := getJSON(t, ts.URL+"/v1/billed-entries?cursor=invalid", "test-reader-key", http.StatusBadRequest)
	if got, _ := billedBadCursor["error"].(string); !strings.Contains(got, "invalid cursor") {
		t.Fatalf("expected invalid cursor error, got %q", got)
	}
	billedDesc := getJSON(
		t,
		ts.URL+"/v1/billed-entries?customer_id=cust_1&meter_id="+url.QueryEscape(meterID)+"&limit=1&order=desc",
		"test-reader-key",
		http.StatusOK,
	)
	billedDescItems := listItemsFromResponse(t, billedDesc)
	if len(billedDescItems) != 1 {
		t.Fatalf("expected billed desc page size 1, got %d", len(billedDescItems))
	}
	billedDescRow, ok := billedDescItems[0].(map[string]any)
	if !ok {
		t.Fatalf("expected billed desc row object")
	}
	if source, _ := billedDescRow["source"].(string); source != "replay_adjustment" {
		t.Fatalf("expected billed desc first row source replay_adjustment, got %q", source)
	}
	billedBadOrder := getJSON(t, ts.URL+"/v1/billed-entries?order=invalid", "test-reader-key", http.StatusBadRequest)
	if got, _ := billedBadOrder["error"].(string); !strings.Contains(got, "order must be asc or desc") {
		t.Fatalf("expected billed invalid order error, got %q", got)
	}

	getJSON(t, ts.URL+"/internal/metrics", "test-writer-key", http.StatusForbidden)
	getJSON(t, ts.URL+"/internal/metrics", "test-admin-key", http.StatusForbidden)
	metrics := getJSON(t, ts.URL+"/internal/metrics", "test-platform-admin", http.StatusOK)
	if _, ok := metrics["metrics"]; !ok {
		t.Fatalf("expected metrics payload")
	}
	metricsMap, ok := metrics["metrics"].(map[string]any)
	if !ok {
		t.Fatalf("expected metrics object")
	}
	if _, ok := metricsMap["http_requests_total"]; !ok {
		t.Fatalf("expected http_requests_total in metrics payload")
	}
	if _, ok := metricsMap["tenant_http_requests_total"]; !ok {
		t.Fatalf("expected tenant_http_requests_total in metrics payload")
	}
	if _, ok := metricsMap["tenant_http_auth_denied_total"]; !ok {
		t.Fatalf("expected tenant_http_auth_denied_total in metrics payload")
	}
	if _, ok := metricsMap["tenant_http_rate_limited_total"]; !ok {
		t.Fatalf("expected tenant_http_rate_limited_total in metrics payload")
	}
	getJSON(t, ts.URL+"/internal/ready", "test-writer-key", http.StatusForbidden)
	getJSON(t, ts.URL+"/internal/ready", "test-admin-key", http.StatusForbidden)
	ready := getJSON(t, ts.URL+"/internal/ready", "test-platform-admin", http.StatusOK)
	if ready["status"] != "ready" {
		t.Fatalf("expected internal ready status to be ready, got %v", ready["status"])
	}

	writerPrefix := api.KeyPrefixFromHash(api.HashAPIKey("test-writer-key"))
	writerKey, err := repo.GetAPIKeyByPrefix(writerPrefix)
	if err != nil {
		t.Fatalf("get writer api key: %v", err)
	}
	if writerKey.LastUsedAt == nil {
		t.Fatalf("expected writer api key last_used_at to be updated")
	}
}

func resetTables(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`TRUNCATE TABLE password_reset_tokens, user_tenant_memberships, user_password_credentials, workspace_invitations, service_accounts, users, workspace_billing_settings, workspace_billing_bindings, billing_provider_connections, platform_api_keys, tenant_audit_events, lago_webhook_events, invoice_payment_status_views, api_key_audit_export_jobs, api_key_audit_events, subscriptions, plan_coupons, coupons, plan_add_ons, add_ons, plan_metrics, plans, api_keys, replay_jobs, billed_entries, usage_events, meters, rating_rule_versions RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
	_, err = db.Exec(`DELETE FROM tenants WHERE id <> 'default'`)
	if err != nil {
		t.Fatalf("delete non-default tenants: %v", err)
	}
	_, err = db.Exec(`INSERT INTO tenants (
			id,
			name,
			status,
			billing_provider_connection_id,
			lago_organization_id,
			lago_billing_provider_code,
			lago_api_key,
			created_at,
			updated_at
		) VALUES (
			'default',
			'Default Tenant',
			'active',
			NULL,
			NULL,
			NULL,
			NULL,
			NOW(),
			NOW()
		)
		ON CONFLICT (id) DO UPDATE
		SET name = EXCLUDED.name,
		    status = EXCLUDED.status,
		    billing_provider_connection_id = EXCLUDED.billing_provider_connection_id,
		    lago_organization_id = EXCLUDED.lago_organization_id,
		    lago_billing_provider_code = EXCLUDED.lago_billing_provider_code,
		    lago_api_key = EXCLUDED.lago_api_key,
		    updated_at = NOW()`)
	if err != nil {
		t.Fatalf("upsert default tenant: %v", err)
	}
}

func TestLargeReplayDatasetDriftThresholds(t *testing.T) {
	if strings.TrimSpace(os.Getenv("RUN_LARGE_REPLAY_DATASET")) != "1" {
		t.Skip("RUN_LARGE_REPLAY_DATASET=1 is required for large replay dataset integration test")
	}

	databaseURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for integration tests")
	}

	eventsCount := envIntOrDefault(t, "LARGE_REPLAY_EVENT_COUNT", 2000)
	maxMismatchRows := envIntOrDefault(t, "LARGE_REPLAY_MAX_MISMATCH_ROWS", 0)
	maxTotalAbsDelta := envInt64OrDefault(t, "LARGE_REPLAY_MAX_TOTAL_ABS_DELTA_CENTS", 0)
	if eventsCount < 1 {
		t.Fatalf("LARGE_REPLAY_EVENT_COUNT must be >= 1")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := store.NewPostgresStore(db)
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	resetTables(t, db)

	_, replayRuntimeCleanup := startTemporalReplayRuntime(t, repo)
	defer replayRuntimeCleanup()

	ratingService := service.NewRatingService(repo)
	meterService := service.NewMeterService(repo)
	usageService := service.NewUsageService(repo)
	replayService := replay.NewService(repo)
	reconcileService := reconcile.NewService(repo)

	rule, err := ratingService.CreateRuleVersion(domain.RatingRuleVersion{
		TenantID:       "default",
		RuleKey:        "nightly_large_replay_dataset",
		Name:           "Nightly Large Replay Dataset",
		Version:        1,
		LifecycleState: domain.RatingRuleLifecycleActive,
		Mode:           domain.PricingModeGraduated,
		Currency:       "USD",
		GraduatedTiers: []domain.RatingTier{
			{UpTo: 0, UnitAmountCents: 1},
		},
	})
	if err != nil {
		t.Fatalf("create rule version: %v", err)
	}

	meter, err := meterService.CreateMeter(domain.Meter{
		TenantID:            "default",
		Key:                 "nightly_large_replay_meter",
		Name:                "Nightly Large Replay Meter",
		Unit:                "request",
		Aggregation:         "sum",
		RatingRuleVersionID: rule.ID,
	})
	if err != nil {
		t.Fatalf("create meter: %v", err)
	}

	customerID := "nightly_large_cust"
	baseTime := time.Now().UTC().Add(-2 * time.Minute)
	for i := 0; i < eventsCount; i++ {
		_, _, err := usageService.CreateUsageEventWithIdempotency(domain.UsageEvent{
			TenantID:       "default",
			CustomerID:     customerID,
			MeterID:        meter.ID,
			Quantity:       1,
			IdempotencyKey: fmt.Sprintf("nightly-large-evt-%d", i),
			Timestamp:      baseTime.Add(time.Duration(i) * time.Millisecond),
		})
		if err != nil {
			t.Fatalf("create usage event %d/%d: %v", i+1, eventsCount, err)
		}
	}

	from := baseTime.Add(-1 * time.Minute)
	to := baseTime.Add(time.Duration(eventsCount)*time.Millisecond + 5*time.Minute)
	job, idempotent, err := replayService.CreateJob(replay.CreateReplayJobRequest{
		TenantID:       "default",
		CustomerID:     customerID,
		MeterID:        meter.ID,
		From:           &from,
		To:             &to,
		IdempotencyKey: "nightly-large-replay-job",
	})
	if err != nil {
		t.Fatalf("create replay job: %v", err)
	}
	if idempotent {
		t.Fatalf("expected first large replay job to be non-idempotent")
	}

	deadline := time.Now().Add(3 * time.Minute)
	for {
		statusJob, statusErr := replayService.GetJob("default", job.ID)
		if statusErr != nil {
			t.Fatalf("get replay job status: %v", statusErr)
		}
		switch statusJob.Status {
		case domain.ReplayDone:
			if statusJob.ProcessedRecords <= 0 {
				t.Fatalf("expected processed_records > 0 for large replay dataset")
			}
			goto report
		case domain.ReplayFailed:
			t.Fatalf("large replay job failed: %s", statusJob.Error)
		}
		if time.Now().After(deadline) {
			t.Fatalf("large replay job did not finish before deadline")
		}
		time.Sleep(200 * time.Millisecond)
	}

report:
	reportData, err := reconcileService.GenerateReport(reconcile.Filter{
		TenantID:   "default",
		CustomerID: customerID,
		From:       &from,
		To:         &to,
	})
	if err != nil {
		t.Fatalf("generate reconciliation report: %v", err)
	}

	totalAbsDelta := reportData.TotalDeltaCents
	if totalAbsDelta < 0 {
		totalAbsDelta = -totalAbsDelta
	}
	if reportData.MismatchRowCount > maxMismatchRows {
		t.Fatalf("mismatch_row_count=%d exceeds threshold=%d", reportData.MismatchRowCount, maxMismatchRows)
	}
	if totalAbsDelta > maxTotalAbsDelta {
		t.Fatalf("total_abs_delta_cents=%d exceeds threshold=%d", totalAbsDelta, maxTotalAbsDelta)
	}
}

func TestLagoWebhookVisibilityFlow(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for integration tests")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := store.NewPostgresStore(db)
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	resetTables(t, db)

	mustCreateAPIKey(t, repo, "tenant-a-reader", api.RoleReader, "tenant_a")
	mustCreateAPIKey(t, repo, "tenant-b-reader", api.RoleReader, "tenant_b")

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}
	mustSetTenantMappings(t, repo, "tenant_a", "org_test_1", "stripe_test")
	mustSetTenantMappings(t, repo, "tenant_b", "org_test_2", "stripe_test")
	lagoWebhookSvc := service.NewLagoWebhookService(
		repo,
		service.NoopLagoWebhookVerifier{},
		nil,
		nil,
	)

	ts := httptest.NewServer(api.NewServer(
		repo,
		api.WithAPIKeyAuthorizer(authorizer),
		api.WithLagoWebhookService(lagoWebhookSvc),
	).Handler())
	defer ts.Close()

	webhookPayload := map[string]any{
		"webhook_type":    "invoice.payment_failure",
		"object_type":     "payment_provider_invoice_payment_error",
		"organization_id": "org_test_1",
		"payment_provider_invoice_payment_error": map[string]any{
			"lago_invoice_id":      "inv_123",
			"external_customer_id": "cust_ext_1",
			"provider_error":       "card_declined",
		},
	}

	first := postJSONWithHeaders(t, ts.URL+"/internal/lago/webhooks", webhookPayload, map[string]string{
		"X-Lago-Signature-Algorithm": "hmac",
		"X-Lago-Signature":           "test-signature",
		"X-Lago-Unique-Key":          "whk_123",
	}, "", http.StatusAccepted)
	if idem, _ := first["idempotent"].(bool); idem {
		t.Fatalf("expected first webhook delivery to be non-idempotent")
	}

	second := postJSONWithHeaders(t, ts.URL+"/internal/lago/webhooks", webhookPayload, map[string]string{
		"X-Lago-Signature-Algorithm": "hmac",
		"X-Lago-Signature":           "test-signature",
		"X-Lago-Unique-Key":          "whk_123",
	}, "", http.StatusOK)
	if idem, _ := second["idempotent"].(bool); !idem {
		t.Fatalf("expected duplicate webhook delivery to be idempotent")
	}

	status := getJSON(t, ts.URL+"/v1/invoice-payment-statuses/inv_123", "tenant-a-reader", http.StatusOK)
	if got, _ := status["payment_status"].(string); got != "failed" {
		t.Fatalf("expected invoice payment_status failed, got %q", got)
	}
	if got, _ := status["customer_external_id"].(string); got != "cust_ext_1" {
		t.Fatalf("expected customer_external_id cust_ext_1, got %q", got)
	}

	listResp := getJSON(t, ts.URL+"/v1/invoice-payment-statuses?payment_status=failed", "tenant-a-reader", http.StatusOK)
	listItems := listItemsFromResponse(t, listResp)
	if len(listItems) != 1 {
		t.Fatalf("expected one payment status row, got %d", len(listItems))
	}

	eventsResp := getJSON(t, ts.URL+"/v1/invoice-payment-statuses/inv_123/events", "tenant-a-reader", http.StatusOK)
	eventItems := listItemsFromResponse(t, eventsResp)
	if len(eventItems) != 1 {
		t.Fatalf("expected one webhook event after idempotent duplicate, got %d", len(eventItems))
	}

	_ = getJSON(t, ts.URL+"/v1/invoice-payment-statuses/inv_123", "tenant-b-reader", http.StatusNotFound)
}

func TestCustomerPaymentSetupAutoRefreshFromWebhook(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for integration tests")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := store.NewPostgresStore(db)
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	resetTables(t, db)

	mustCreateAPIKey(t, repo, "tenant-a-reader", api.RoleReader, "tenant_a")
	mustCreateAPIKey(t, repo, "tenant-a-writer", api.RoleWriter, "tenant_a")

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	paymentMethodReady := false
	lagoMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/customers":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"customer":{"lago_id":"lago_cust_alpha","external_id":"cust_alpha","billing_configuration":{"payment_provider":"stripe","payment_provider_code":"stripe_test","provider_customer_id":"pcus_123","provider_payment_methods":["card"]}}}`))
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/customers/cust_alpha/checkout_url":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"customer":{"external_customer_id":"cust_alpha","checkout_url":"https://checkout.example.test/cust_alpha"}}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/customers/cust_alpha/payment_methods":
			w.Header().Set("Content-Type", "application/json")
			if paymentMethodReady {
				_, _ = w.Write([]byte(`{"payment_methods":[{"lago_id":"pm_lago_alpha","is_default":true,"provider_method_id":"pm_123"}]}`))
				return
			}
			_, _ = w.Write([]byte(`{"payment_methods":[]}`))
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer lagoMock.Close()

	lagoTransport, err := service.NewLagoHTTPTransport(service.LagoClientConfig{
		BaseURL: lagoMock.URL,
		APIKey:  "test-api-key",
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}
	customerBillingAdapter := service.NewLagoCustomerBillingAdapter(lagoTransport)
	mustSetTenantMappings(t, repo, "tenant_a", "org_test_1", "stripe_test")

	lagoWebhookSvc := service.NewLagoWebhookService(
		repo,
		service.NoopLagoWebhookVerifier{},
		nil,
		service.NewCustomerService(repo, customerBillingAdapter),
	)

	ts := httptest.NewServer(api.NewServer(
		repo,
		api.WithAPIKeyAuthorizer(authorizer),
		api.WithCustomerBillingAdapter(customerBillingAdapter),
		api.WithLagoWebhookService(lagoWebhookSvc),
	).Handler())
	defer ts.Close()

	_ = postJSON(t, ts.URL+"/v1/customers", map[string]any{
		"external_id":  "cust_alpha",
		"display_name": "Alpha Co",
		"email":        "billing@alpha.test",
	}, "tenant-a-writer", http.StatusCreated)
	_ = putJSON(t, ts.URL+"/v1/customers/cust_alpha/billing-profile", map[string]any{
		"legal_name":            "Alpha Company Pvt Ltd",
		"email":                 "billing@alpha.test",
		"billing_address_line1": "1 Billing St",
		"billing_city":          "Bengaluru",
		"billing_postal_code":   "560001",
		"billing_country":       "IN",
		"currency":              "USD",
		"provider_code":         "stripe_test",
	}, "tenant-a-writer", http.StatusOK)
	_ = postJSON(t, ts.URL+"/v1/customers/cust_alpha/payment-setup/checkout-url", map[string]any{
		"payment_method_type": "card",
	}, "tenant-a-writer", http.StatusOK)

	pendingReadiness := getJSON(t, ts.URL+"/v1/customers/cust_alpha/readiness", "tenant-a-reader", http.StatusOK)
	if got, _ := pendingReadiness["status"].(string); got != "pending" {
		t.Fatalf("expected readiness pending before webhook refresh, got %q", got)
	}

	paymentMethodReady = true
	webhookPayload := map[string]any{
		"webhook_type":    "customer.payment_provider_created",
		"object_type":     "customer",
		"organization_id": "org_test_1",
		"customer": map[string]any{
			"external_id": "cust_alpha",
			"updated_at":  time.Now().UTC().Format(time.RFC3339),
		},
	}
	result := postJSONWithHeaders(t, ts.URL+"/internal/lago/webhooks", webhookPayload, map[string]string{
		"X-Lago-Signature-Algorithm": "hmac",
		"X-Lago-Signature":           "test-signature",
		"X-Lago-Unique-Key":          "whk_customer_created_1",
	}, "", http.StatusAccepted)
	if idem, _ := result["idempotent"].(bool); idem {
		t.Fatalf("expected first webhook delivery to be non-idempotent")
	}

	setup := getJSON(t, ts.URL+"/v1/customers/cust_alpha/payment-setup", "tenant-a-reader", http.StatusOK)
	if got, _ := setup["setup_status"].(string); got != "ready" {
		t.Fatalf("expected payment setup ready after webhook refresh, got %q", got)
	}
	readiness := getJSON(t, ts.URL+"/v1/customers/cust_alpha/readiness", "tenant-a-reader", http.StatusOK)
	if got, _ := readiness["status"].(string); got != "ready" {
		t.Fatalf("expected readiness ready after webhook refresh, got %q", got)
	}
	if got, _ := readiness["default_payment_method_verified"].(bool); !got {
		t.Fatalf("expected default_payment_method_verified=true after webhook refresh")
	}
}

func TestPaymentFailureLifecycleRetryAndOutOfOrderWebhooks(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for integration tests")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := store.NewPostgresStore(db)
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	resetTables(t, db)

	mustCreateAPIKey(t, repo, "tenant-a-reader", api.RoleReader, "tenant_a")
	mustCreateAPIKey(t, repo, "tenant-a-writer", api.RoleWriter, "tenant_a")
	mustCreateAPIKey(t, repo, "tenant-b-reader", api.RoleReader, "tenant_b")

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	retryCalls := 0
	lagoMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/invoices/inv_123/retry_payment" {
			retryCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"invoice":{"lago_id":"inv_123","payment_status":"pending"}}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer lagoMock.Close()

	lagoTransport, err := service.NewLagoHTTPTransport(service.LagoClientConfig{
		BaseURL: lagoMock.URL,
		APIKey:  "test-api-key",
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}
	mustSetTenantMappings(t, repo, "tenant_a", "org_test_1", "stripe_test")
	mustSetTenantMappings(t, repo, "tenant_b", "org_test_2", "stripe_test")

	lagoWebhookSvc := service.NewLagoWebhookService(
		repo,
		service.NoopLagoWebhookVerifier{},
		nil,
		nil,
	)

	ts := httptest.NewServer(api.NewServer(
		repo,
		api.WithAPIKeyAuthorizer(authorizer),
		api.WithMeterSyncAdapter(service.NewLagoMeterSyncAdapter(lagoTransport)), api.WithInvoiceBillingAdapter(service.NewLagoInvoiceAdapter(lagoTransport)),
		api.WithLagoWebhookService(lagoWebhookSvc),
	).Handler())
	defer ts.Close()

	baseTS := time.Now().UTC()
	invoiceCreatedTS := baseTS.Add(-1 * time.Hour).Format(time.RFC3339)
	overdueUpdatedTS := baseTS.Add(10 * time.Second).Format(time.RFC3339)
	successUpdatedTS := baseTS.Add(20 * time.Second).Format(time.RFC3339)
	staleUpdatedTS := baseTS.Add(5 * time.Second).Format(time.RFC3339)

	failurePayload := map[string]any{
		"webhook_type":    "invoice.payment_failure",
		"object_type":     "payment_provider_invoice_payment_error",
		"organization_id": "org_test_1",
		"payment_provider_invoice_payment_error": map[string]any{
			"lago_invoice_id":      "inv_123",
			"external_customer_id": "cust_ext_1",
			"provider_error":       "card_declined",
		},
	}
	firstFailure := postJSONWithHeaders(t, ts.URL+"/internal/lago/webhooks", failurePayload, map[string]string{
		"X-Lago-Signature-Algorithm": "hmac",
		"X-Lago-Signature":           "test-signature",
		"X-Lago-Unique-Key":          "whk_fail_1",
	}, "", http.StatusAccepted)
	if idem, _ := firstFailure["idempotent"].(bool); idem {
		t.Fatalf("expected first failure delivery to be non-idempotent")
	}
	duplicateFailure := postJSONWithHeaders(t, ts.URL+"/internal/lago/webhooks", failurePayload, map[string]string{
		"X-Lago-Signature-Algorithm": "hmac",
		"X-Lago-Signature":           "test-signature",
		"X-Lago-Unique-Key":          "whk_fail_1",
	}, "", http.StatusOK)
	if idem, _ := duplicateFailure["idempotent"].(bool); !idem {
		t.Fatalf("expected duplicate failure delivery to be idempotent")
	}

	overduePayload := map[string]any{
		"webhook_type":    "invoice.payment_overdue",
		"object_type":     "invoice",
		"organization_id": "org_test_1",
		"invoice": map[string]any{
			"lago_id":                 "inv_123",
			"status":                  "finalized",
			"payment_status":          "failed",
			"payment_overdue":         true,
			"number":                  "INV-123",
			"currency":                "USD",
			"total_amount_cents":      1200,
			"total_due_amount_cents":  1200,
			"total_paid_amount_cents": 0,
			"updated_at":              overdueUpdatedTS,
			"created_at":              invoiceCreatedTS,
			"customer": map[string]any{
				"external_id": "cust_ext_1",
			},
		},
	}
	_ = postJSONWithHeaders(t, ts.URL+"/internal/lago/webhooks", overduePayload, map[string]string{
		"X-Lago-Signature-Algorithm": "hmac",
		"X-Lago-Signature":           "test-signature",
		"X-Lago-Unique-Key":          "whk_overdue_1",
	}, "", http.StatusAccepted)

	retryResp := postJSON(
		t,
		ts.URL+"/v1/invoices/inv_123/retry-payment",
		map[string]any{},
		"tenant-a-writer",
		http.StatusOK,
	)
	retryInvoice, ok := retryResp["invoice"].(map[string]any)
	if !ok {
		t.Fatalf("expected retry response to include invoice object")
	}
	if got, _ := retryInvoice["payment_status"].(string); got != "pending" {
		t.Fatalf("expected retry payment_status pending, got %q", got)
	}
	if retryCalls != 1 {
		t.Fatalf("expected exactly one retry call to lago, got %d", retryCalls)
	}

	successPayload := map[string]any{
		"webhook_type":    "invoice.payment_status_updated",
		"object_type":     "invoice",
		"organization_id": "org_test_1",
		"invoice": map[string]any{
			"lago_id":                 "inv_123",
			"status":                  "finalized",
			"payment_status":          "succeeded",
			"payment_overdue":         false,
			"number":                  "INV-123",
			"currency":                "USD",
			"total_amount_cents":      1200,
			"total_due_amount_cents":  0,
			"total_paid_amount_cents": 1200,
			"updated_at":              successUpdatedTS,
			"created_at":              invoiceCreatedTS,
			"customer": map[string]any{
				"external_id": "cust_ext_1",
			},
		},
	}
	_ = postJSONWithHeaders(t, ts.URL+"/internal/lago/webhooks", successPayload, map[string]string{
		"X-Lago-Signature-Algorithm": "hmac",
		"X-Lago-Signature":           "test-signature",
		"X-Lago-Unique-Key":          "whk_success_1",
	}, "", http.StatusAccepted)

	staleFailedPayload := map[string]any{
		"webhook_type":    "invoice.payment_status_updated",
		"object_type":     "invoice",
		"organization_id": "org_test_1",
		"invoice": map[string]any{
			"lago_id":                 "inv_123",
			"status":                  "finalized",
			"payment_status":          "failed",
			"payment_overdue":         true,
			"number":                  "INV-123",
			"currency":                "USD",
			"total_amount_cents":      1200,
			"total_due_amount_cents":  1200,
			"total_paid_amount_cents": 0,
			"updated_at":              staleUpdatedTS,
			"created_at":              invoiceCreatedTS,
			"customer": map[string]any{
				"external_id": "cust_ext_1",
			},
		},
	}
	_ = postJSONWithHeaders(t, ts.URL+"/internal/lago/webhooks", staleFailedPayload, map[string]string{
		"X-Lago-Signature-Algorithm": "hmac",
		"X-Lago-Signature":           "test-signature",
		"X-Lago-Unique-Key":          "whk_stale_1",
	}, "", http.StatusAccepted)

	otherOrgPayload := map[string]any{
		"webhook_type":    "invoice.payment_status_updated",
		"object_type":     "invoice",
		"organization_id": "org_test_2",
		"invoice": map[string]any{
			"lago_id":                 "inv_999",
			"status":                  "finalized",
			"payment_status":          "failed",
			"payment_overdue":         true,
			"number":                  "INV-999",
			"currency":                "USD",
			"total_amount_cents":      900,
			"total_due_amount_cents":  900,
			"total_paid_amount_cents": 0,
			"updated_at":              baseTS.Add(-30 * time.Second).Format(time.RFC3339),
			"created_at":              invoiceCreatedTS,
			"customer": map[string]any{
				"external_id": "cust_ext_2",
			},
		},
	}
	_ = postJSONWithHeaders(t, ts.URL+"/internal/lago/webhooks", otherOrgPayload, map[string]string{
		"X-Lago-Signature-Algorithm": "hmac",
		"X-Lago-Signature":           "test-signature",
		"X-Lago-Unique-Key":          "whk_org2_1",
	}, "", http.StatusAccepted)

	status := getJSON(t, ts.URL+"/v1/invoice-payment-statuses/inv_123", "tenant-a-reader", http.StatusOK)
	if got, _ := status["payment_status"].(string); got != "succeeded" {
		t.Fatalf("expected final payment_status succeeded, got %q", got)
	}
	if got, _ := status["last_webhook_key"].(string); got != "whk_success_1" {
		t.Fatalf("expected last_webhook_key whk_success_1, got %q", got)
	}
	overdueValue, ok := status["payment_overdue"].(bool)
	if !ok || overdueValue {
		t.Fatalf("expected final payment_overdue=false, got %v", status["payment_overdue"])
	}
	lifecycle := getJSON(t, ts.URL+"/v1/invoice-payment-statuses/inv_123/lifecycle", "tenant-a-reader", http.StatusOK)
	if got, _ := lifecycle["payment_status"].(string); got != "succeeded" {
		t.Fatalf("expected lifecycle payment_status succeeded, got %q", got)
	}
	if got, _ := lifecycle["recommended_action"].(string); got != "none" {
		t.Fatalf("expected lifecycle recommended_action none, got %q", got)
	}
	if got, _ := lifecycle["requires_action"].(bool); got {
		t.Fatalf("expected lifecycle requires_action=false, got true")
	}
	if got, _ := lifecycle["retry_recommended"].(bool); got {
		t.Fatalf("expected lifecycle retry_recommended=false, got true")
	}
	if got, _ := lifecycle["events_analyzed"].(float64); int(got) != 4 {
		t.Fatalf("expected lifecycle events_analyzed=4, got %v", lifecycle["events_analyzed"])
	}
	if got, _ := lifecycle["failure_event_count"].(float64); int(got) != 3 {
		t.Fatalf("expected lifecycle failure_event_count=3, got %v", lifecycle["failure_event_count"])
	}
	if got, _ := lifecycle["overdue_signal_count"].(float64); int(got) != 2 {
		t.Fatalf("expected lifecycle overdue_signal_count=2, got %v", lifecycle["overdue_signal_count"])
	}
	if got, _ := lifecycle["event_window_limit"].(float64); int(got) != 200 {
		t.Fatalf("expected lifecycle event_window_limit default 200, got %v", lifecycle["event_window_limit"])
	}
	webhookTypesRaw, ok := lifecycle["distinct_webhook_types"].([]any)
	if !ok || len(webhookTypesRaw) != 3 {
		t.Fatalf("expected lifecycle distinct_webhook_types size 3, got %v", lifecycle["distinct_webhook_types"])
	}
	lastFailureAt, ok := lifecycle["last_failure_at"].(string)
	if !ok || strings.TrimSpace(lastFailureAt) == "" {
		t.Fatalf("expected lifecycle last_failure_at timestamp")
	}
	badLifecycleLimit := getJSON(t, ts.URL+"/v1/invoice-payment-statuses/inv_123/lifecycle?event_limit=501", "tenant-a-reader", http.StatusBadRequest)
	if got, _ := badLifecycleLimit["error"].(string); !strings.Contains(got, "event_limit must be between 1 and 500") {
		t.Fatalf("expected lifecycle event_limit validation error, got %q", got)
	}

	succeededList := getJSON(t, ts.URL+"/v1/invoice-payment-statuses?payment_status=succeeded", "tenant-a-reader", http.StatusOK)
	succeededItems := listItemsFromResponse(t, succeededList)
	if len(succeededItems) != 1 {
		t.Fatalf("expected one succeeded payment row, got %d", len(succeededItems))
	}
	orgFiltered := getJSON(t, ts.URL+"/v1/invoice-payment-statuses?organization_id=org_test_2", "tenant-a-reader", http.StatusOK)
	orgFilteredItems := listItemsFromResponse(t, orgFiltered)
	if len(orgFilteredItems) != 0 {
		t.Fatalf("expected tenant_a to see no rows for organization filter org_test_2, got %d", len(orgFilteredItems))
	}
	tenantBOrgFiltered := getJSON(t, ts.URL+"/v1/invoice-payment-statuses?organization_id=org_test_2", "tenant-b-reader", http.StatusOK)
	tenantBOrgFilteredItems := listItemsFromResponse(t, tenantBOrgFiltered)
	if len(tenantBOrgFilteredItems) != 1 {
		t.Fatalf("expected tenant_b to see one row for organization filter org_test_2, got %d", len(tenantBOrgFilteredItems))
	}
	orgFilteredRow, ok := tenantBOrgFilteredItems[0].(map[string]any)
	if !ok {
		t.Fatalf("expected tenant_b organization filtered row to be object")
	}
	if got, _ := orgFilteredRow["invoice_id"].(string); got != "inv_999" {
		t.Fatalf("expected tenant_b org_test_2 row invoice_id inv_999, got %q", got)
	}
	ascendingList := getJSON(t, ts.URL+"/v1/invoice-payment-statuses?sort_by=last_event_at&order=asc&limit=1", "tenant-a-reader", http.StatusOK)
	ascendingItems := listItemsFromResponse(t, ascendingList)
	if len(ascendingItems) != 1 {
		t.Fatalf("expected one row in ascending list, got %d", len(ascendingItems))
	}
	ascendingRow, ok := ascendingItems[0].(map[string]any)
	if !ok {
		t.Fatalf("expected ascending row to be object")
	}
	if got, _ := ascendingRow["invoice_id"].(string); got != "inv_123" {
		t.Fatalf("expected ascending first row invoice_id inv_123, got %q", got)
	}
	descendingList := getJSON(t, ts.URL+"/v1/invoice-payment-statuses?sort_by=last_event_at&order=desc&limit=1", "tenant-a-reader", http.StatusOK)
	descendingItems := listItemsFromResponse(t, descendingList)
	if len(descendingItems) != 1 {
		t.Fatalf("expected one row in descending list, got %d", len(descendingItems))
	}
	descendingRow, ok := descendingItems[0].(map[string]any)
	if !ok {
		t.Fatalf("expected descending row to be object")
	}
	if got, _ := descendingRow["invoice_id"].(string); got != "inv_123" {
		t.Fatalf("expected descending first row invoice_id inv_123, got %q", got)
	}
	badStatusOrder := getJSON(t, ts.URL+"/v1/invoice-payment-statuses?order=invalid", "tenant-a-reader", http.StatusBadRequest)
	if got, _ := badStatusOrder["error"].(string); !strings.Contains(got, "order must be asc or desc") {
		t.Fatalf("expected status invalid order validation error, got %q", got)
	}
	badStatusSort := getJSON(t, ts.URL+"/v1/invoice-payment-statuses?sort_by=bad_sort", "tenant-a-reader", http.StatusBadRequest)
	if got, _ := badStatusSort["error"].(string); !strings.Contains(got, "sort_by must be one of") {
		t.Fatalf("expected status invalid sort validation error, got %q", got)
	}
	overdueList := getJSON(t, ts.URL+"/v1/invoice-payment-statuses?payment_overdue=true", "tenant-a-reader", http.StatusOK)
	overdueItems := listItemsFromResponse(t, overdueList)
	if len(overdueItems) != 0 {
		t.Fatalf("expected tenant_a to see no currently overdue payment rows, got %d", len(overdueItems))
	}
	summary := getJSON(t, ts.URL+"/v1/invoice-payment-statuses/summary", "tenant-a-reader", http.StatusOK)
	if got, _ := summary["total_invoices"].(float64); int(got) != 1 {
		t.Fatalf("expected summary total_invoices=1, got %v", summary["total_invoices"])
	}
	if got, _ := summary["attention_required_count"].(float64); int(got) != 0 {
		t.Fatalf("expected summary attention_required_count=0, got %v", summary["attention_required_count"])
	}
	paymentStatusCounts, ok := summary["payment_status_counts"].(map[string]any)
	if !ok {
		t.Fatalf("expected payment_status_counts object")
	}
	if got, _ := paymentStatusCounts["succeeded"].(float64); int(got) != 1 {
		t.Fatalf("expected summary payment_status_counts.succeeded=1, got %v", paymentStatusCounts["succeeded"])
	}
	if got, exists := paymentStatusCounts["failed"]; exists && int(got.(float64)) != 0 {
		t.Fatalf("expected summary payment_status_counts.failed to be absent or 0, got %v", got)
	}

	var staleSummary map[string]any
	staleMatched := false
	for attempt := 0; attempt < 30; attempt++ {
		staleSummary = getJSON(t, ts.URL+"/v1/invoice-payment-statuses/summary?stale_after_sec=1", "tenant-a-reader", http.StatusOK)
		if got, _ := staleSummary["stale_attention_required"].(float64); int(got) == 0 {
			staleMatched = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !staleMatched {
		t.Fatalf("expected stale summary stale_attention_required=0, got %v", staleSummary["stale_attention_required"])
	}
	badSummary := getJSON(t, ts.URL+"/v1/invoice-payment-statuses/summary?stale_after_sec=-1", "tenant-a-reader", http.StatusBadRequest)
	if got, _ := badSummary["error"].(string); !strings.Contains(got, "stale_after_sec must be >= 0") {
		t.Fatalf("expected stale_after_sec validation error, got %q", got)
	}

	eventsResp := getJSON(t, ts.URL+"/v1/invoice-payment-statuses/inv_123/events", "tenant-a-reader", http.StatusOK)
	eventItems := listItemsFromResponse(t, eventsResp)
	if len(eventItems) != 4 {
		t.Fatalf("expected 4 unique webhook events in timeline, got %d", len(eventItems))
	}
	eventsSortedAsc := getJSON(t, ts.URL+"/v1/invoice-payment-statuses/inv_123/events?sort_by=occurred_at&order=asc&limit=10", "tenant-a-reader", http.StatusOK)
	eventsSortedAscItems := listItemsFromResponse(t, eventsSortedAsc)
	if len(eventsSortedAscItems) != 4 {
		t.Fatalf("expected 4 events in occurred_at asc list, got %d", len(eventsSortedAscItems))
	}
	firstEventRow, ok := eventsSortedAscItems[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first events row to be object")
	}
	if got, _ := firstEventRow["webhook_type"].(string); got != "invoice.payment_failure" {
		t.Fatalf("expected first asc event webhook_type invoice.payment_failure, got %q", got)
	}
	filteredEvents := getJSON(t, ts.URL+"/v1/invoice-payment-statuses/inv_123/events?webhook_type=invoice.payment_overdue", "tenant-a-reader", http.StatusOK)
	filteredEventItems := listItemsFromResponse(t, filteredEvents)
	if len(filteredEventItems) != 1 {
		t.Fatalf("expected one filtered overdue event, got %d", len(filteredEventItems))
	}
	badEventSort := getJSON(t, ts.URL+"/v1/invoice-payment-statuses/inv_123/events?sort_by=bad_sort", "tenant-a-reader", http.StatusBadRequest)
	if got, _ := badEventSort["error"].(string); !strings.Contains(got, "sort_by must be one of") {
		t.Fatalf("expected event invalid sort validation error, got %q", got)
	}

	_ = getJSON(t, ts.URL+"/v1/invoice-payment-statuses/inv_123", "tenant-b-reader", http.StatusNotFound)
}

func TestTenantIsolationAcrossAPIKeys(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for integration tests")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := store.NewPostgresStore(db)
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	resetTables(t, db)

	mustCreateAPIKey(t, repo, "tenant-a-writer", api.RoleWriter, "tenant_a")
	mustCreateAPIKey(t, repo, "tenant-b-writer", api.RoleWriter, "tenant_b")

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}
	paymentMethodReady := false
	lagoMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/billable_metrics":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"billable_metric":{"code":"tenant_onboard_api_calls"}}`))
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/customers":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"customer":{"lago_id":"lago_cust_onboard","external_id":"cust_onboard","billing_configuration":{"payment_provider":"stripe","payment_provider_code":"stripe_onboard","provider_customer_id":"pcus_onboard","provider_payment_methods":["card"]}}}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/customers/cust_onboard/payment_methods":
			w.Header().Set("Content-Type", "application/json")
			if paymentMethodReady {
				_, _ = w.Write([]byte(`{"payment_methods":[{"lago_id":"pm_lago_onboard","is_default":true,"provider_method_id":"pm_onboard"}]}`))
				return
			}
			_, _ = w.Write([]byte(`{"payment_methods":[]}`))
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/customers/cust_onboard/checkout_url":
			paymentMethodReady = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"customer":{"external_customer_id":"cust_onboard","checkout_url":"https://checkout.example.test/cust_onboard"}}`))
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer lagoMock.Close()
	lagoTransport, err := service.NewLagoHTTPTransport(service.LagoClientConfig{
		BaseURL: lagoMock.URL,
		APIKey:  "test-api-key",
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}
	lagoCleanup := func() {}
	defer lagoCleanup()

	ts := httptest.NewServer(api.NewServer(
		repo,
		api.WithAPIKeyAuthorizer(authorizer),
		api.WithMeterSyncAdapter(service.NewLagoMeterSyncAdapter(lagoTransport)),
		api.WithInvoiceBillingAdapter(service.NewLagoInvoiceAdapter(lagoTransport)),
		api.WithCustomerBillingAdapter(service.NewLagoCustomerBillingAdapter(lagoTransport)),
	).Handler())
	defer ts.Close()

	rule := postJSON(t, ts.URL+"/v1/rating-rules", map[string]any{
		"rule_key":        "tenant_a_rule",
		"name":            "Tenant A Rule",
		"version":         1,
		"lifecycle_state": "active",
		"mode":            "graduated",
		"currency":        "USD",
		"graduated_tiers": []map[string]any{{"up_to": 10, "unit_amount_cents": 2}, {"up_to": 0, "unit_amount_cents": 1}},
	}, "tenant-a-writer", http.StatusCreated)
	ruleID := rule["id"].(string)

	meter := postJSON(t, ts.URL+"/v1/meters", map[string]any{
		"key":                    "tenant_a_meter",
		"name":                   "Tenant A Meter",
		"unit":                   "call",
		"aggregation":            "sum",
		"rating_rule_version_id": ruleID,
	}, "tenant-a-writer", http.StatusCreated)
	meterID := meter["id"].(string)

	_ = getJSON(t, ts.URL+"/v1/meters/"+meterID, "tenant-a-writer", http.StatusOK)
	_ = getJSON(t, ts.URL+"/v1/meters/"+meterID, "tenant-b-writer", http.StatusNotFound)

	usageErr := postJSON(t, ts.URL+"/v1/usage-events", map[string]any{
		"customer_id": "cust_tenant_b",
		"meter_id":    meterID,
		"quantity":    1,
	}, "tenant-b-writer", http.StatusBadRequest)
	if got, _ := usageErr["error"].(string); !strings.Contains(got, "meter not found") {
		t.Fatalf("expected tenant isolation meter not found validation error, got %q", got)
	}
}

func TestRatingRuleGovernanceLifecycle(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for integration tests")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := store.NewPostgresStore(db)
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	resetTables(t, db)

	mustCreateAPIKey(t, repo, "governance-writer", api.RoleWriter, "default")

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}
	paymentMethodReady := false
	lagoMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/billable_metrics":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"billable_metric":{"code":"tenant_onboard_api_calls"}}`))
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/customers":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"customer":{"lago_id":"lago_cust_onboard","external_id":"cust_onboard","billing_configuration":{"payment_provider":"stripe","payment_provider_code":"stripe_onboard","provider_customer_id":"pcus_onboard","provider_payment_methods":["card"]}}}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/customers/cust_onboard/payment_methods":
			w.Header().Set("Content-Type", "application/json")
			if paymentMethodReady {
				_, _ = w.Write([]byte(`{"payment_methods":[{"lago_id":"pm_lago_onboard","is_default":true,"provider_method_id":"pm_onboard"}]}`))
				return
			}
			_, _ = w.Write([]byte(`{"payment_methods":[]}`))
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/customers/cust_onboard/checkout_url":
			paymentMethodReady = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"customer":{"external_customer_id":"cust_onboard","checkout_url":"https://checkout.example.test/cust_onboard"}}`))
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer lagoMock.Close()
	lagoTransport, err := service.NewLagoHTTPTransport(service.LagoClientConfig{
		BaseURL: lagoMock.URL,
		APIKey:  "test-api-key",
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}

	ts := httptest.NewServer(api.NewServer(
		repo,
		api.WithAPIKeyAuthorizer(authorizer),
		api.WithMeterSyncAdapter(service.NewLagoMeterSyncAdapter(lagoTransport)),
		api.WithInvoiceBillingAdapter(service.NewLagoInvoiceAdapter(lagoTransport)),
		api.WithCustomerBillingAdapter(service.NewLagoCustomerBillingAdapter(lagoTransport)),
	).Handler())
	defer ts.Close()

	v1 := postJSON(t, ts.URL+"/v1/rating-rules", map[string]any{
		"rule_key":        "api_calls",
		"name":            "API Calls v1",
		"version":         1,
		"lifecycle_state": "active",
		"mode":            "graduated",
		"currency":        "USD",
		"graduated_tiers": []map[string]any{{"up_to": 100, "unit_amount_cents": 2}, {"up_to": 0, "unit_amount_cents": 1}},
	}, "governance-writer", http.StatusCreated)
	v1ID, _ := v1["id"].(string)
	if strings.TrimSpace(v1ID) == "" {
		t.Fatalf("expected rating rule v1 id")
	}
	if got, _ := v1["rule_key"].(string); got != "api_calls" {
		t.Fatalf("expected v1 rule_key=api_calls, got %q", got)
	}
	if got, _ := v1["lifecycle_state"].(string); got != "active" {
		t.Fatalf("expected v1 lifecycle_state=active, got %q", got)
	}

	v2 := postJSON(t, ts.URL+"/v1/rating-rules", map[string]any{
		"rule_key":        "api_calls",
		"name":            "API Calls v2",
		"version":         2,
		"lifecycle_state": "active",
		"mode":            "graduated",
		"currency":        "USD",
		"graduated_tiers": []map[string]any{{"up_to": 100, "unit_amount_cents": 3}, {"up_to": 0, "unit_amount_cents": 2}},
	}, "governance-writer", http.StatusCreated)
	if got, _ := v2["lifecycle_state"].(string); got != "active" {
		t.Fatalf("expected v2 lifecycle_state=active, got %q", got)
	}

	v1Reload := getJSON(t, ts.URL+"/v1/rating-rules/"+v1ID, "governance-writer", http.StatusOK)
	if got, _ := v1Reload["lifecycle_state"].(string); got != "archived" {
		t.Fatalf("expected v1 lifecycle_state=archived after v2 activation, got %q", got)
	}

	dupVersion := postJSON(t, ts.URL+"/v1/rating-rules", map[string]any{
		"rule_key":        "api_calls",
		"name":            "API Calls v2 duplicate",
		"version":         2,
		"lifecycle_state": "draft",
		"mode":            "graduated",
		"currency":        "USD",
		"graduated_tiers": []map[string]any{{"up_to": 10, "unit_amount_cents": 1}, {"up_to": 0, "unit_amount_cents": 1}},
	}, "governance-writer", http.StatusConflict)
	if got, _ := dupVersion["error"].(string); !strings.Contains(strings.ToLower(got), "duplicate") {
		t.Fatalf("expected duplicate version conflict, got %q", got)
	}

	lowerVersion := postJSON(t, ts.URL+"/v1/rating-rules", map[string]any{
		"rule_key":        "api_calls",
		"name":            "API Calls old",
		"version":         1,
		"lifecycle_state": "draft",
		"mode":            "graduated",
		"currency":        "USD",
		"graduated_tiers": []map[string]any{{"up_to": 10, "unit_amount_cents": 1}, {"up_to": 0, "unit_amount_cents": 1}},
	}, "governance-writer", http.StatusConflict)
	if got, _ := lowerVersion["error"].(string); !strings.Contains(strings.ToLower(got), "duplicate") {
		t.Fatalf("expected lower version conflict, got %q", got)
	}

	v3Draft := postJSON(t, ts.URL+"/v1/rating-rules", map[string]any{
		"rule_key":        "api_calls",
		"name":            "API Calls v3 Draft",
		"version":         3,
		"lifecycle_state": "draft",
		"mode":            "graduated",
		"currency":        "USD",
		"graduated_tiers": []map[string]any{{"up_to": 100, "unit_amount_cents": 4}, {"up_to": 0, "unit_amount_cents": 2}},
	}, "governance-writer", http.StatusCreated)
	if got, _ := v3Draft["lifecycle_state"].(string); got != "draft" {
		t.Fatalf("expected v3 lifecycle_state=draft, got %q", got)
	}

	latestAny := getJSONArray(t, ts.URL+"/v1/rating-rules?rule_key=api_calls&latest_only=true", "governance-writer", http.StatusOK)
	if len(latestAny) != 1 {
		t.Fatalf("expected one latest api_calls rule, got %d", len(latestAny))
	}
	latestAnyRow, ok := latestAny[0].(map[string]any)
	if !ok {
		t.Fatalf("expected latest api_calls row object")
	}
	if got := int(latestAnyRow["version"].(float64)); got != 3 {
		t.Fatalf("expected latest api_calls version=3, got %d", got)
	}
	if got, _ := latestAnyRow["lifecycle_state"].(string); got != "draft" {
		t.Fatalf("expected latest api_calls lifecycle_state=draft, got %q", got)
	}

	activeOnly := getJSONArray(t, ts.URL+"/v1/rating-rules?rule_key=api_calls&lifecycle_state=active", "governance-writer", http.StatusOK)
	if len(activeOnly) != 1 {
		t.Fatalf("expected one active api_calls rule, got %d", len(activeOnly))
	}
	activeOnlyRow, ok := activeOnly[0].(map[string]any)
	if !ok {
		t.Fatalf("expected active api_calls row object")
	}
	if got := int(activeOnlyRow["version"].(float64)); got != 2 {
		t.Fatalf("expected active api_calls version=2, got %d", got)
	}

	activeLatestOnly := getJSONArray(t, ts.URL+"/v1/rating-rules?rule_key=api_calls&lifecycle_state=active&latest_only=true", "governance-writer", http.StatusOK)
	if len(activeLatestOnly) != 1 {
		t.Fatalf("expected one active latest api_calls rule, got %d", len(activeLatestOnly))
	}
	activeLatestRow, ok := activeLatestOnly[0].(map[string]any)
	if !ok {
		t.Fatalf("expected active latest api_calls row object")
	}
	if got := int(activeLatestRow["version"].(float64)); got != 2 {
		t.Fatalf("expected active latest api_calls version=2, got %d", got)
	}

	badLifecycle := getJSON(t, ts.URL+"/v1/rating-rules?lifecycle_state=invalid", "governance-writer", http.StatusBadRequest)
	if got, _ := badLifecycle["error"].(string); !strings.Contains(strings.ToLower(got), "lifecycle_state") {
		t.Fatalf("expected invalid lifecycle_state error, got %q", got)
	}
	badLatestOnly := getJSON(t, ts.URL+"/v1/rating-rules?latest_only=maybe", "governance-writer", http.StatusBadRequest)
	if got, _ := badLatestOnly["error"].(string); !strings.Contains(strings.ToLower(got), "latest_only") {
		t.Fatalf("expected invalid latest_only error, got %q", got)
	}

	allRules := getJSONArray(t, ts.URL+"/v1/rating-rules", "governance-writer", http.StatusOK)
	activeCount := 0
	activeVersion := 0
	for _, item := range allRules {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		ruleKey, _ := row["rule_key"].(string)
		state, _ := row["lifecycle_state"].(string)
		if ruleKey == "api_calls" && state == "active" {
			activeCount++
			activeVersion = int(row["version"].(float64))
		}
	}
	if activeCount != 1 {
		t.Fatalf("expected exactly one active api_calls rule version, got %d", activeCount)
	}
	if activeVersion != 2 {
		t.Fatalf("expected active api_calls version=2, got %d", activeVersion)
	}
}

func TestAPIKeyLifecycleEndpoints(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for integration tests")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := store.NewPostgresStore(db)
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	resetTables(t, db)

	mustCreateAPIKey(t, repo, "tenant-a-admin", api.RoleAdmin, "tenant_a")
	mustCreateAPIKey(t, repo, "tenant-a-writer", api.RoleWriter, "tenant_a")
	mustCreateAPIKey(t, repo, "tenant-b-admin", api.RoleAdmin, "tenant_b")

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}
	paymentMethodReady := false
	lagoMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/billable_metrics":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"billable_metric":{"code":"tenant_onboard_api_calls"}}`))
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/customers":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"customer":{"lago_id":"lago_cust_onboard","external_id":"cust_onboard","billing_configuration":{"payment_provider":"stripe","payment_provider_code":"stripe_onboard","provider_customer_id":"pcus_onboard","provider_payment_methods":["card"]}}}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/customers/cust_onboard/payment_methods":
			w.Header().Set("Content-Type", "application/json")
			if paymentMethodReady {
				_, _ = w.Write([]byte(`{"payment_methods":[{"lago_id":"pm_lago_onboard","is_default":true,"provider_method_id":"pm_onboard"}]}`))
				return
			}
			_, _ = w.Write([]byte(`{"payment_methods":[]}`))
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/customers/cust_onboard/checkout_url":
			paymentMethodReady = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"customer":{"external_customer_id":"cust_onboard","checkout_url":"https://checkout.example.test/cust_onboard"}}`))
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer lagoMock.Close()
	lagoTransport, err := service.NewLagoHTTPTransport(service.LagoClientConfig{
		BaseURL: lagoMock.URL,
		APIKey:  "test-api-key",
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}

	ts := httptest.NewServer(api.NewServer(
		repo,
		api.WithAPIKeyAuthorizer(authorizer),
		api.WithMeterSyncAdapter(service.NewLagoMeterSyncAdapter(lagoTransport)),
		api.WithInvoiceBillingAdapter(service.NewLagoInvoiceAdapter(lagoTransport)),
		api.WithCustomerBillingAdapter(service.NewLagoCustomerBillingAdapter(lagoTransport)),
	).Handler())
	defer ts.Close()

	_ = postJSON(t, ts.URL+"/v1/api-keys", map[string]any{
		"name": "forbidden-create",
		"role": "reader",
	}, "tenant-a-writer", http.StatusForbidden)

	created := postJSON(t, ts.URL+"/v1/api-keys", map[string]any{
		"name": "tenant-a-runtime-writer",
		"role": "writer",
	}, "tenant-a-admin", http.StatusCreated)
	extraWriterA := postJSON(t, ts.URL+"/v1/api-keys", map[string]any{
		"name": "tenant-a-runtime-writer-2",
		"role": "writer",
	}, "tenant-a-admin", http.StatusCreated)
	extraWriterB := postJSON(t, ts.URL+"/v1/api-keys", map[string]any{
		"name": "tenant-a-runtime-writer-3",
		"role": "writer",
	}, "tenant-a-admin", http.StatusCreated)

	createdSecret, _ := created["secret"].(string)
	if createdSecret == "" {
		t.Fatalf("expected create api key response to include one-time secret")
	}
	createdAPIKey, ok := created["api_key"].(map[string]any)
	if !ok {
		t.Fatalf("expected create api key response to include api_key object")
	}
	createdID, _ := createdAPIKey["id"].(string)
	if createdID == "" {
		t.Fatalf("expected created api key id")
	}
	if createdAPIKey["tenant_id"] != "tenant_a" {
		t.Fatalf("expected created key tenant tenant_a, got %v", createdAPIKey["tenant_id"])
	}
	if createdAPIKey["owner_type"] != "workspace_credential" {
		t.Fatalf("expected created key owner_type workspace_credential, got %v", createdAPIKey["owner_type"])
	}

	tenantAList := getJSON(t, ts.URL+"/v1/api-keys", "tenant-a-admin", http.StatusOK)
	tenantAKeys := listItemsFromResponse(t, tenantAList)
	if !containsID(tenantAKeys, createdID) {
		t.Fatalf("expected tenant-a list to include created key id=%s", createdID)
	}
	_, deprecatedHeaders := getRaw(t, ts.URL+"/v1/api-keys", "tenant-a-admin", http.StatusOK)
	if got := deprecatedHeaders.Get("Deprecation"); got != "true" {
		t.Fatalf("expected deprecation header on raw api-keys endpoint, got %q", got)
	}
	if got := deprecatedHeaders.Get("Link"); !strings.Contains(got, "/v1/workspace/service-accounts") {
		t.Fatalf("expected successor Link header on raw api-keys endpoint, got %q", got)
	}

	pagedWriters := getJSON(t, ts.URL+"/v1/api-keys?role=writer&limit=2", "tenant-a-admin", http.StatusOK)
	pageOneItems := listItemsFromResponse(t, pagedWriters)
	if len(pageOneItems) != 2 {
		t.Fatalf("expected first writer page to contain 2 keys, got %d", len(pageOneItems))
	}
	nextCursor, _ := pagedWriters["next_cursor"].(string)
	if strings.TrimSpace(nextCursor) == "" {
		t.Fatalf("expected next_cursor on first writer page")
	}
	nextPage := getJSON(t, ts.URL+"/v1/api-keys?role=writer&limit=2&cursor="+url.QueryEscape(nextCursor), "tenant-a-admin", http.StatusOK)
	pageTwoItems := listItemsFromResponse(t, nextPage)
	if len(pageTwoItems) == 0 {
		t.Fatalf("expected second writer page to contain at least one key")
	}
	extraWriterAID := nestedID(t, extraWriterA, "api_key")
	extraWriterBID := nestedID(t, extraWriterB, "api_key")
	if !(containsID(pageOneItems, extraWriterAID) || containsID(pageOneItems, extraWriterBID) || containsID(pageTwoItems, extraWriterAID) || containsID(pageTwoItems, extraWriterBID)) {
		t.Fatalf("expected paginated writer listing to include newly created writer keys")
	}

	tenantBList := getJSON(t, ts.URL+"/v1/api-keys", "tenant-b-admin", http.StatusOK)
	tenantBKeys := listItemsFromResponse(t, tenantBList)
	if containsID(tenantBKeys, createdID) {
		t.Fatalf("tenant-b should not see tenant-a key id=%s", createdID)
	}

	_ = getJSONArray(t, ts.URL+"/v1/meters", createdSecret, http.StatusOK)

	revoked := postJSON(t, ts.URL+"/v1/api-keys/"+createdID+"/revoke", map[string]any{}, "tenant-a-admin", http.StatusOK)
	if revoked["revoked_at"] == nil {
		t.Fatalf("expected revoked_at to be set after revoke")
	}
	revokedList := getJSON(t, ts.URL+"/v1/api-keys?state=revoked&limit=10", "tenant-a-admin", http.StatusOK)
	revokedItems := listItemsFromResponse(t, revokedList)
	if !containsID(revokedItems, createdID) {
		t.Fatalf("expected revoked key list to include id=%s", createdID)
	}

	_ = getJSON(t, ts.URL+"/v1/reconciliation-report", createdSecret, http.StatusUnauthorized)

	adminPrefix := api.KeyPrefixFromHash(api.HashAPIKey("tenant-a-admin"))
	adminKey, err := repo.GetAPIKeyByPrefix(adminPrefix)
	if err != nil {
		t.Fatalf("get admin key by prefix: %v", err)
	}

	rotated := postJSON(t, ts.URL+"/v1/api-keys/"+adminKey.ID+"/rotate", map[string]any{}, "tenant-a-admin", http.StatusOK)
	newAdminSecret, _ := rotated["secret"].(string)
	if newAdminSecret == "" {
		t.Fatalf("expected rotate api key response to include one-time secret")
	}
	rotatedAPIKey, ok := rotated["api_key"].(map[string]any)
	if !ok {
		t.Fatalf("expected rotate api key response to include api_key object")
	}
	if rotatedAPIKey["role"] != "admin" {
		t.Fatalf("expected rotated api key role admin, got %v", rotatedAPIKey["role"])
	}

	_ = getJSON(t, ts.URL+"/v1/reconciliation-report", "tenant-a-admin", http.StatusUnauthorized)
	_ = getJSON(t, ts.URL+"/v1/api-keys", newAdminSecret, http.StatusOK)

	auditResp := getJSON(t, ts.URL+"/v1/api-keys/audit?limit=100", newAdminSecret, http.StatusOK)
	auditItems := listItemsFromResponse(t, auditResp)
	if !containsActionForKey(auditItems, createdID, "created") {
		t.Fatalf("expected audit stream to include created action for key id=%s", createdID)
	}
	if !containsActionForKey(auditItems, createdID, "revoked") {
		t.Fatalf("expected audit stream to include revoked action for key id=%s", createdID)
	}
	if !containsActionForKey(auditItems, adminKey.ID, "rotated") {
		t.Fatalf("expected audit stream to include rotated action for key id=%s", adminKey.ID)
	}

	auditPage := getJSON(t, ts.URL+"/v1/api-keys/audit?limit=1", newAdminSecret, http.StatusOK)
	auditCursor, _ := auditPage["next_cursor"].(string)
	if strings.TrimSpace(auditCursor) == "" {
		t.Fatalf("expected next_cursor for paginated audit list")
	}
	_ = getJSON(t, ts.URL+"/v1/api-keys/audit?limit=1&cursor="+url.QueryEscape(auditCursor), newAdminSecret, http.StatusOK)

	csvBody, csvHeaders := getRaw(t, ts.URL+"/v1/api-keys/audit?format=csv&action=created", newAdminSecret, http.StatusOK)
	if !strings.Contains(strings.ToLower(csvHeaders.Get("Content-Type")), "text/csv") {
		t.Fatalf("expected csv content type, got %q", csvHeaders.Get("Content-Type"))
	}
	if !strings.Contains(csvBody, "id,tenant_id,api_key_id,actor_api_key_id,action,metadata,created_at") {
		t.Fatalf("expected csv header in audit export")
	}
	if !strings.Contains(csvBody, createdID) {
		t.Fatalf("expected csv export to include created api key id=%s", createdID)
	}

	tenantBAuditResp := getJSON(t, ts.URL+"/v1/api-keys/audit?limit=100", "tenant-b-admin", http.StatusOK)
	tenantBAuditItems := listItemsFromResponse(t, tenantBAuditResp)
	if containsActionForKey(tenantBAuditItems, createdID, "created") {
		t.Fatalf("tenant-b should not see tenant-a audit events")
	}
}

func TestBlockedTenantCannotUseAPIKey(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for integration tests")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := store.NewPostgresStore(db)
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	resetTables(t, db)

	for _, status := range []string{"suspended", "deleted"} {
		resetTables(t, db)
		mustCreateAPIKey(t, repo, "blocked-"+status+"-admin", api.RoleAdmin, "tenant_blocked_"+status)
		if _, err := db.Exec(`UPDATE tenants SET status = $1 WHERE id = $2`, status, "tenant_blocked_"+status); err != nil {
			t.Fatalf("update tenant status to %s: %v", status, err)
		}

		authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
		if err != nil {
			t.Fatalf("new authorizer: %v", err)
		}
		lagoTransport, lagoCleanup := newTestLagoTransport(t)
		defer lagoCleanup()

		ts := httptest.NewServer(api.NewServer(repo, api.WithAPIKeyAuthorizer(authorizer), api.WithMeterSyncAdapter(service.NewLagoMeterSyncAdapter(lagoTransport)), api.WithInvoiceBillingAdapter(service.NewLagoInvoiceAdapter(lagoTransport))).Handler())
		resp := getJSON(t, ts.URL+"/v1/api-keys", "blocked-"+status+"-admin", http.StatusForbidden)
		if got, _ := resp["error"].(string); got != "forbidden" {
			t.Fatalf("expected forbidden error for status=%s, got %v", status, resp["error"])
		}
		ts.Close()
	}
}

func TestInternalTenantOperatorEndpoints(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for integration tests")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := store.NewPostgresStore(db)
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	resetTables(t, db)

	mustCreatePlatformAPIKey(t, repo, "platform-admin")
	mustCreateAPIKey(t, repo, "tenant-a-admin", api.RoleAdmin, "tenant_a")

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}
	lagoTransport, lagoCleanup := newTestLagoTransport(t)
	defer lagoCleanup()

	ts := httptest.NewServer(api.NewServer(repo, api.WithAPIKeyAuthorizer(authorizer), api.WithMeterSyncAdapter(service.NewLagoMeterSyncAdapter(lagoTransport)), api.WithInvoiceBillingAdapter(service.NewLagoInvoiceAdapter(lagoTransport))).Handler())
	defer ts.Close()

	created := postJSON(t, ts.URL+"/internal/tenants", map[string]any{
		"id":                         "tenant_ops",
		"name":                       "Tenant Ops",
		"lago_organization_id":       "org_ops",
		"lago_billing_provider_code": "stripe_ops",
	}, "platform-admin", http.StatusCreated)
	createdTenant, ok := created["tenant"].(map[string]any)
	if !ok {
		t.Fatalf("expected tenant object in create response")
	}
	if createdTenant["id"] != "tenant_ops" {
		t.Fatalf("expected tenant_ops id, got %v", createdTenant["id"])
	}

	duplicateCreate := postJSON(t, ts.URL+"/internal/tenants", map[string]any{
		"id":   "tenant_ops",
		"name": "Tenant Ops Again",
	}, "platform-admin", http.StatusConflict)
	if got, _ := duplicateCreate["error"].(string); !strings.Contains(strings.ToLower(got), "already exists") {
		t.Fatalf("expected duplicate tenant create conflict, got %v", duplicateCreate["error"])
	}

	_ = postJSON(t, ts.URL+"/internal/tenants", map[string]any{
		"id":   "tenant_denied",
		"name": "Denied",
	}, "tenant-a-admin", http.StatusForbidden)

	list := getJSONArray(t, ts.URL+"/internal/tenants?status=active", "platform-admin", http.StatusOK)
	if len(list) == 0 {
		t.Fatalf("expected active tenant list to include tenants")
	}

	got := getJSON(t, ts.URL+"/internal/tenants/tenant_ops", "platform-admin", http.StatusOK)
	if got["id"] != "tenant_ops" {
		t.Fatalf("expected tenant_ops get response, got %v", got["id"])
	}

	updated2 := patchJSON(t, ts.URL+"/internal/tenants/tenant_ops", map[string]any{
		"lago_billing_provider_code": "stripe_v2",
	}, "platform-admin", http.StatusOK)
	if updated2["id"] != "tenant_ops" {
		t.Fatalf("expected updated tenant response to remain sanitized, got %v", updated2["id"])
	}
	tenantOps, err := repo.GetTenant("tenant_ops")
	if err != nil {
		t.Fatalf("get tenant_ops from repo: %v", err)
	}
	if tenantOps.LagoBillingProviderCode != "stripe_v2" {
		t.Fatalf("expected persisted provider code stripe_v2, got %q", tenantOps.LagoBillingProviderCode)
	}

	auditPage := getJSON(t, ts.URL+"/internal/tenants/audit?tenant_id=tenant_ops&limit=10", "platform-admin", http.StatusOK)
	if got, _ := auditPage["total"].(float64); got < 3 {
		t.Fatalf("expected at least 3 tenant audit events, got %v", got)
	}
	auditItems := listItemsFromResponse(t, auditPage)
	if !containsTenantAuditAction(auditItems, "tenant_ops", "workspace.created") {
		t.Fatalf("expected workspace.created tenant audit event")
	}
	if !containsTenantAuditAction(auditItems, "tenant_ops", "workspace.status_changed") {
		t.Fatalf("expected workspace.status_changed tenant audit event")
	}
	if !containsTenantAuditAction(auditItems, "tenant_ops", "workspace.billing_connection_changed") {
		t.Fatalf("expected workspace.billing_connection_changed tenant audit event")
	}

	_ = patchJSON(t, ts.URL+"/internal/tenants/default", map[string]any{
		"status": "suspended",
	}, "platform-admin", http.StatusBadRequest)

	bootstrapped := postJSON(t, ts.URL+"/internal/tenants/tenant_ops/bootstrap-admin-key", map[string]any{
		"name": "tenant-ops-bootstrap",
	}, "platform-admin", http.StatusCreated)
	serviceAccount, ok := bootstrapped["service_account"].(map[string]any)
	if !ok {
		t.Fatalf("expected bootstrap response service_account object")
	}
	if gotName, _ := serviceAccount["name"].(string); gotName != "tenant-ops-bootstrap" {
		t.Fatalf("expected bootstrap service account name tenant-ops-bootstrap, got %q", gotName)
	}
	credential, ok := bootstrapped["credential"].(map[string]any)
	if !ok {
		t.Fatalf("expected bootstrap response credential object")
	}
	if gotRole, _ := credential["role"].(string); gotRole != "admin" {
		t.Fatalf("expected bootstrapped tenant admin role admin, got %q", gotRole)
	}
	if gotTenant, _ := credential["tenant_id"].(string); gotTenant != "tenant_ops" {
		t.Fatalf("expected bootstrapped tenant admin tenant_id tenant_ops, got %q", gotTenant)
	}
	if gotOwnerType, _ := credential["owner_type"].(string); gotOwnerType != "bootstrap" {
		t.Fatalf("expected bootstrapped tenant admin owner_type bootstrap, got %q", gotOwnerType)
	}
	if secret, _ := bootstrapped["secret"].(string); strings.TrimSpace(secret) == "" {
		t.Fatalf("expected bootstrap response secret")
	}

	updated := patchJSON(t, ts.URL+"/internal/tenants/tenant_ops", map[string]any{
		"status": "suspended",
	}, "platform-admin", http.StatusOK)
	if updated["status"] != "suspended" {
		t.Fatalf("expected suspended status, got %v", updated["status"])
	}

	_ = postJSON(t, ts.URL+"/internal/tenants/tenant_ops/bootstrap-admin-key", map[string]any{
		"name": "tenant-ops-bootstrap-2",
	}, "platform-admin", http.StatusBadRequest)
}

func TestInternalTenantOnboardingFlow(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for integration tests")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := store.NewPostgresStore(db)
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	resetTables(t, db)

	mustCreatePlatformAPIKey(t, repo, "platform-admin")

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}
	lagoTransport, lagoCleanup := newTestLagoTransport(t)
	defer lagoCleanup()

	ts := httptest.NewServer(api.NewServer(repo, api.WithAPIKeyAuthorizer(authorizer), api.WithMeterSyncAdapter(service.NewLagoMeterSyncAdapter(lagoTransport)), api.WithInvoiceBillingAdapter(service.NewLagoInvoiceAdapter(lagoTransport))).Handler())
	defer ts.Close()

	onboarded := postJSON(t, ts.URL+"/internal/onboarding/tenants", map[string]any{
		"id":                         "tenant_onboard",
		"name":                       "Tenant Onboard",
		"lago_organization_id":       "org_onboard",
		"lago_billing_provider_code": "stripe_onboard",
		"admin_key_name":             "tenant-onboard-admin",
	}, "platform-admin", http.StatusCreated)

	tenant, ok := onboarded["tenant"].(map[string]any)
	if !ok {
		t.Fatalf("expected tenant object in onboarding response")
	}
	if tenant["id"] != "tenant_onboard" {
		t.Fatalf("expected tenant_onboard id, got %v", tenant["id"])
	}
	bootstrap, ok := onboarded["tenant_admin_bootstrap"].(map[string]any)
	if !ok {
		t.Fatalf("expected tenant_admin_bootstrap object")
	}
	if created, _ := bootstrap["created"].(bool); !created {
		t.Fatalf("expected tenant admin bootstrap to create a credential")
	}
	if _, ok := bootstrap["service_account"].(map[string]any); !ok {
		t.Fatalf("expected tenant_admin_bootstrap service_account object")
	}
	if credential, ok := bootstrap["credential"].(map[string]any); !ok {
		t.Fatalf("expected tenant_admin_bootstrap credential object")
	} else if got, _ := credential["owner_type"].(string); got != "bootstrap" {
		t.Fatalf("expected tenant_admin_bootstrap credential owner_type bootstrap, got %q", got)
	}
	adminSecret, _ := bootstrap["secret"].(string)
	if strings.TrimSpace(adminSecret) == "" {
		t.Fatalf("expected tenant admin secret in onboarding response")
	}
	readiness, ok := onboarded["readiness"].(map[string]any)
	if !ok {
		t.Fatalf("expected readiness object")
	}
	if got, _ := readiness["status"].(string); got != "pending" {
		t.Fatalf("expected onboarding status pending before pricing, got %q", got)
	}
	tenantReadiness, ok := readiness["tenant"].(map[string]any)
	if !ok {
		t.Fatalf("expected tenant readiness object")
	}
	if got, _ := tenantReadiness["status"].(string); got != "ready" {
		t.Fatalf("expected tenant readiness ready, got %q", got)
	}
	if got, _ := tenantReadiness["tenant_admin_ready"].(bool); !got {
		t.Fatalf("expected tenant_admin_ready=true")
	}
	billingReadiness, ok := readiness["billing_integration"].(map[string]any)
	if !ok {
		t.Fatalf("expected billing integration readiness object")
	}
	if got, _ := billingReadiness["status"].(string); got != "pending" {
		t.Fatalf("expected billing integration readiness pending before pricing, got %q", got)
	}
	if got, _ := billingReadiness["billing_mapping_ready"].(bool); !got {
		t.Fatalf("expected billing_mapping_ready=true")
	}
	if got, _ := billingReadiness["pricing_ready"].(bool); got {
		t.Fatalf("expected pricing_ready=false before pricing bootstrap")
	}
	firstCustomerReadiness, ok := readiness["first_customer"].(map[string]any)
	if !ok {
		t.Fatalf("expected first_customer readiness object")
	}
	if got, _ := firstCustomerReadiness["status"].(string); got != "pending" {
		t.Fatalf("expected first_customer status pending, got %q", got)
	}
	if got, _ := firstCustomerReadiness["managed"].(bool); !got {
		t.Fatalf("expected first_customer managed=true")
	}
	if got, _ := firstCustomerReadiness["customer_exists"].(bool); got {
		t.Fatalf("expected first_customer customer_exists=false before customer bootstrap")
	}

	initialReadiness := getJSON(t, ts.URL+"/internal/onboarding/tenants/tenant_onboard", "platform-admin", http.StatusOK)
	initialReadinessData, ok := initialReadiness["readiness"].(map[string]any)
	if !ok {
		t.Fatalf("expected readiness object from onboarding status endpoint")
	}
	if got, _ := initialReadinessData["status"].(string); got != "pending" {
		t.Fatalf("expected onboarding readiness pending before pricing, got %q", got)
	}

	rule := postJSON(t, ts.URL+"/v1/rating-rules", map[string]any{
		"rule_key":          "tenant_onboard_api_calls",
		"name":              "Tenant Onboard API Calls",
		"version":           1,
		"lifecycle_state":   "active",
		"mode":              "flat",
		"currency":          "USD",
		"flat_amount_cents": 100,
	}, adminSecret, http.StatusCreated)
	ruleID := rule["id"].(string)

	_ = postJSON(t, ts.URL+"/v1/meters", map[string]any{
		"key":                    "tenant_onboard_api_calls",
		"name":                   "Tenant Onboard API Calls",
		"unit":                   "call",
		"aggregation":            "sum",
		"rating_rule_version_id": ruleID,
	}, adminSecret, http.StatusCreated)

	_ = postJSON(t, ts.URL+"/v1/customers", map[string]any{
		"external_id":  "cust_onboard",
		"display_name": "Tenant Onboard Customer",
		"email":        "billing@tenant-onboard.test",
	}, adminSecret, http.StatusCreated)
	_ = putJSON(t, ts.URL+"/v1/customers/cust_onboard/billing-profile", map[string]any{
		"legal_name":            "Tenant Onboard Customer LLC",
		"email":                 "billing@tenant-onboard.test",
		"billing_address_line1": "1 Billing Street",
		"billing_city":          "Bengaluru",
		"billing_postal_code":   "560001",
		"billing_country":       "IN",
		"currency":              "USD",
		"provider_code":         "stripe_onboard",
	}, adminSecret, http.StatusOK)
	checkout := postJSON(t, ts.URL+"/v1/customers/cust_onboard/payment-setup/checkout-url", map[string]any{
		"payment_method_type": "card",
	}, adminSecret, http.StatusOK)
	if got, _ := checkout["checkout_url"].(string); got == "" {
		t.Fatalf("expected checkout_url from customer payment setup")
	}
	refreshed := postJSON(t, ts.URL+"/v1/customers/cust_onboard/payment-setup/refresh", map[string]any{}, adminSecret, http.StatusOK)
	if readiness, ok := refreshed["readiness"].(map[string]any); !ok {
		t.Fatalf("expected readiness in payment setup refresh response")
	} else if got, _ := readiness["status"].(string); got != "ready" {
		t.Fatalf("expected customer readiness ready after payment setup refresh, got %q", got)
	}

	finalReadiness := getJSON(t, ts.URL+"/internal/onboarding/tenants/tenant_onboard", "platform-admin", http.StatusOK)
	finalReadinessData, ok := finalReadiness["readiness"].(map[string]any)
	if !ok {
		t.Fatalf("expected readiness object from onboarding status endpoint")
	}
	if got, _ := finalReadinessData["status"].(string); got != "ready" {
		t.Fatalf("expected onboarding status ready after pricing bootstrap, got %q", got)
	}
	finalBillingReadiness, ok := finalReadinessData["billing_integration"].(map[string]any)
	if !ok {
		t.Fatalf("expected billing integration readiness object from onboarding status endpoint")
	}
	if got, _ := finalBillingReadiness["status"].(string); got != "ready" {
		t.Fatalf("expected billing integration readiness ready after pricing bootstrap, got %q", got)
	}
	if got, _ := finalBillingReadiness["pricing_ready"].(bool); !got {
		t.Fatalf("expected pricing_ready=true after pricing bootstrap")
	}
	finalFirstCustomerReadiness, ok := finalReadinessData["first_customer"].(map[string]any)
	if !ok {
		t.Fatalf("expected first_customer readiness object from onboarding status endpoint")
	}
	if got, _ := finalFirstCustomerReadiness["status"].(string); got != "ready" {
		t.Fatalf("expected first_customer readiness ready after customer bootstrap, got %q", got)
	}
	if got, _ := finalFirstCustomerReadiness["customer_external_id"].(string); got != "cust_onboard" {
		t.Fatalf("expected first_customer customer_external_id=cust_onboard, got %q", got)
	}
}

func TestInternalTenantOnboardingStatusPagesCustomers(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for integration tests")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := store.NewPostgresStore(db)
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	resetTables(t, db)

	mustCreatePlatformAPIKey(t, repo, "platform-admin")

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}
	lagoTransport, lagoCleanup := newTestLagoTransport(t)
	defer lagoCleanup()

	ts := httptest.NewServer(api.NewServer(repo, api.WithAPIKeyAuthorizer(authorizer), api.WithMeterSyncAdapter(service.NewLagoMeterSyncAdapter(lagoTransport)), api.WithInvoiceBillingAdapter(service.NewLagoInvoiceAdapter(lagoTransport))).Handler())
	defer ts.Close()

	onboarded := postJSON(t, ts.URL+"/internal/onboarding/tenants", map[string]any{
		"id":                         "tenant_onboard_paged",
		"name":                       "Tenant Onboard Paged",
		"lago_organization_id":       "org_onboard_paged",
		"lago_billing_provider_code": "stripe_onboard_paged",
		"admin_key_name":             "tenant-onboard-paged-admin",
	}, "platform-admin", http.StatusCreated)
	bootstrap, ok := onboarded["tenant_admin_bootstrap"].(map[string]any)
	if !ok {
		t.Fatalf("expected tenant_admin_bootstrap object")
	}
	if _, ok := bootstrap["service_account"].(map[string]any); !ok {
		t.Fatalf("expected tenant_admin_bootstrap service_account object")
	}
	adminSecret, _ := bootstrap["secret"].(string)
	if strings.TrimSpace(adminSecret) == "" {
		t.Fatalf("expected tenant admin secret in onboarding response")
	}

	rule := postJSON(t, ts.URL+"/v1/rating-rules", map[string]any{
		"rule_key":          "tenant_onboard_paged_api_calls",
		"name":              "Tenant Onboard Paged API Calls",
		"version":           1,
		"lifecycle_state":   "active",
		"mode":              "flat",
		"currency":          "USD",
		"flat_amount_cents": 100,
	}, adminSecret, http.StatusCreated)
	ruleID := rule["id"].(string)

	_ = postJSON(t, ts.URL+"/v1/meters", map[string]any{
		"key":                    "tenant_onboard_paged_api_calls",
		"name":                   "Tenant Onboard Paged API Calls",
		"unit":                   "call",
		"aggregation":            "sum",
		"rating_rule_version_id": ruleID,
	}, adminSecret, http.StatusCreated)

	for i := 0; i < 100; i++ {
		externalID := fmt.Sprintf("cust_paged_%03d", i)
		_ = postJSON(t, ts.URL+"/v1/customers", map[string]any{
			"external_id":  externalID,
			"display_name": fmt.Sprintf("Paged Customer %03d", i),
			"email":        fmt.Sprintf("billing+%03d@tenant-onboard-paged.test", i),
		}, adminSecret, http.StatusCreated)
	}

	_ = postJSON(t, ts.URL+"/v1/customers", map[string]any{
		"external_id":  "cust_paged_ready",
		"display_name": "Paged Ready Customer",
		"email":        "billing@tenant-onboard-paged.test",
	}, adminSecret, http.StatusCreated)
	_ = putJSON(t, ts.URL+"/v1/customers/cust_paged_ready/billing-profile", map[string]any{
		"legal_name":            "Paged Ready Customer LLC",
		"email":                 "billing@tenant-onboard-paged.test",
		"billing_address_line1": "1 Billing Street",
		"billing_city":          "Bengaluru",
		"billing_postal_code":   "560001",
		"billing_country":       "IN",
		"currency":              "USD",
		"provider_code":         "stripe_onboard_paged",
	}, adminSecret, http.StatusOK)
	checkout := postJSON(t, ts.URL+"/v1/customers/cust_paged_ready/payment-setup/checkout-url", map[string]any{
		"payment_method_type": "card",
	}, adminSecret, http.StatusOK)
	if got, _ := checkout["checkout_url"].(string); got == "" {
		t.Fatalf("expected checkout_url from customer payment setup")
	}
	_ = postJSON(t, ts.URL+"/v1/customers/cust_paged_ready/payment-setup/refresh", map[string]any{}, adminSecret, http.StatusOK)

	status := getJSON(t, ts.URL+"/internal/onboarding/tenants/tenant_onboard_paged", "platform-admin", http.StatusOK)
	readiness, ok := status["readiness"].(map[string]any)
	if !ok {
		t.Fatalf("expected readiness object from onboarding status endpoint")
	}
	if got, _ := readiness["status"].(string); got != "ready" {
		t.Fatalf("expected onboarding readiness ready with billing-ready customer after first page, got %q", got)
	}
	firstCustomer, ok := readiness["first_customer"].(map[string]any)
	if !ok {
		t.Fatalf("expected first_customer readiness object")
	}
	if got, _ := firstCustomer["status"].(string); got != "ready" {
		t.Fatalf("expected first_customer readiness ready, got %q", got)
	}
	if got, _ := firstCustomer["customer_external_id"].(string); got != "cust_paged_ready" {
		t.Fatalf("expected first_customer customer_external_id=cust_paged_ready, got %q", got)
	}
}

func TestAuditExportToS3(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for integration tests")
	}
	s3Endpoint := strings.TrimSpace(os.Getenv("TEST_S3_ENDPOINT"))
	s3Bucket := strings.TrimSpace(os.Getenv("TEST_S3_BUCKET"))
	s3AccessKey := strings.TrimSpace(os.Getenv("TEST_S3_ACCESS_KEY_ID"))
	s3Secret := strings.TrimSpace(os.Getenv("TEST_S3_SECRET_ACCESS_KEY"))
	if s3Endpoint == "" || s3Bucket == "" || s3AccessKey == "" || s3Secret == "" {
		t.Skip("TEST_S3_ENDPOINT, TEST_S3_BUCKET, TEST_S3_ACCESS_KEY_ID, TEST_S3_SECRET_ACCESS_KEY are required for S3 export integration test")
	}
	s3Region := strings.TrimSpace(os.Getenv("TEST_S3_REGION"))
	if s3Region == "" {
		s3Region = "us-east-1"
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := store.NewPostgresStore(db)
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	resetTables(t, db)

	mustCreateAPIKey(t, repo, "tenant-export-admin", api.RoleAdmin, "tenant_export")

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}
	lagoTransport, lagoCleanup := newTestLagoTransport(t)
	defer lagoCleanup()

	objectStore, err := service.NewS3ObjectStore(context.Background(), service.S3Config{
		Region:          s3Region,
		Bucket:          s3Bucket,
		Endpoint:        s3Endpoint,
		AccessKeyID:     s3AccessKey,
		SecretAccessKey: s3Secret,
		ForcePathStyle:  true,
	})
	if err != nil {
		t.Fatalf("new s3 object store: %v", err)
	}
	if err := objectStore.EnsureBucket(context.Background()); err != nil {
		t.Fatalf("ensure bucket: %v", err)
	}

	auditExportSvc := service.NewAuditExportService(repo, objectStore, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	worker := service.NewAuditExportWorker(auditExportSvc, 20*time.Millisecond)
	go worker.Run(ctx)

	ts := httptest.NewServer(api.NewServer(
		repo,
		api.WithAPIKeyAuthorizer(authorizer),
		api.WithAuditExportService(auditExportSvc),
		api.WithMeterSyncAdapter(service.NewLagoMeterSyncAdapter(lagoTransport)), api.WithInvoiceBillingAdapter(service.NewLagoInvoiceAdapter(lagoTransport)),
	).Handler())
	defer ts.Close()

	created := postJSON(t, ts.URL+"/v1/api-keys", map[string]any{
		"name": "tenant-export-reader",
		"role": "reader",
	}, "tenant-export-admin", http.StatusCreated)
	createdID := nestedID(t, created, "api_key")

	createExportResp := postJSON(t, ts.URL+"/v1/api-keys/audit/exports", map[string]any{
		"idempotency_key": "exp_1",
		"action":          "created",
	}, "tenant-export-admin", http.StatusCreated)
	jobMap, ok := createExportResp["job"].(map[string]any)
	if !ok {
		t.Fatalf("expected export response to include job object")
	}
	jobID, _ := jobMap["id"].(string)
	if jobID == "" {
		t.Fatalf("expected export job id")
	}

	idemResp := postJSON(t, ts.URL+"/v1/api-keys/audit/exports", map[string]any{
		"idempotency_key": "exp_1",
		"action":          "created",
	}, "tenant-export-admin", http.StatusOK)
	if idem, _ := idemResp["idempotent_request"].(bool); !idem {
		t.Fatalf("expected idempotent_request=true on duplicate idempotency key")
	}
	createExportResp2 := postJSON(t, ts.URL+"/v1/api-keys/audit/exports", map[string]any{
		"idempotency_key": "exp_2",
		"action":          "created",
	}, "tenant-export-admin", http.StatusCreated)
	jobMap2, ok := createExportResp2["job"].(map[string]any)
	if !ok {
		t.Fatalf("expected second export response to include job object")
	}
	jobID2, _ := jobMap2["id"].(string)
	if jobID2 == "" {
		t.Fatalf("expected second export job id")
	}

	exportListPageOne := getJSON(t, ts.URL+"/v1/api-keys/audit/exports?limit=1", "tenant-export-admin", http.StatusOK)
	exportListPageOneItems := listItemsFromResponse(t, exportListPageOne)
	if len(exportListPageOneItems) != 1 {
		t.Fatalf("expected first export jobs page size 1, got %d", len(exportListPageOneItems))
	}
	exportNextCursor, _ := exportListPageOne["next_cursor"].(string)
	if strings.TrimSpace(exportNextCursor) == "" {
		t.Fatalf("expected export jobs next_cursor on first page")
	}
	exportListPageTwo := getJSON(t, ts.URL+"/v1/api-keys/audit/exports?limit=1&cursor="+url.QueryEscape(exportNextCursor), "tenant-export-admin", http.StatusOK)
	exportListPageTwoItems := listItemsFromResponse(t, exportListPageTwo)
	if len(exportListPageTwoItems) != 1 {
		t.Fatalf("expected second export jobs page size 1, got %d", len(exportListPageTwoItems))
	}
	exportCursorOffsetErr := getJSON(
		t,
		ts.URL+"/v1/api-keys/audit/exports?limit=1&offset=1&cursor="+url.QueryEscape(exportNextCursor),
		"tenant-export-admin",
		http.StatusBadRequest,
	)
	if got, _ := exportCursorOffsetErr["error"].(string); !strings.Contains(got, "offset cannot be used with cursor") {
		t.Fatalf("expected export offset/cursor validation error, got %q", got)
	}
	exportBadCursor := getJSON(t, ts.URL+"/v1/api-keys/audit/exports?cursor=invalid", "tenant-export-admin", http.StatusBadRequest)
	if got, _ := exportBadCursor["error"].(string); !strings.Contains(got, "invalid cursor") {
		t.Fatalf("expected export invalid cursor error, got %q", got)
	}
	exportBadStatus := getJSON(t, ts.URL+"/v1/api-keys/audit/exports?status=unknown", "tenant-export-admin", http.StatusBadRequest)
	if got, _ := exportBadStatus["error"].(string); !strings.Contains(got, "invalid status") {
		t.Fatalf("expected export invalid status error, got %q", got)
	}

	var downloadURL string
	for i := 0; i < 100; i++ {
		statusResp := getJSON(t, ts.URL+"/v1/api-keys/audit/exports/"+jobID, "tenant-export-admin", http.StatusOK)
		job, ok := statusResp["job"].(map[string]any)
		if !ok {
			t.Fatalf("expected job object in status response")
		}
		status, _ := job["status"].(string)
		if status == "done" {
			downloadURL, _ = statusResp["download_url"].(string)
			break
		}
		if status == "failed" {
			t.Fatalf("export job failed: %v", job["error"])
		}
		time.Sleep(50 * time.Millisecond)
	}
	if strings.TrimSpace(downloadURL) == "" {
		t.Fatalf("expected export download_url when job is done")
	}
	for i := 0; i < 100; i++ {
		statusResp := getJSON(t, ts.URL+"/v1/api-keys/audit/exports/"+jobID2, "tenant-export-admin", http.StatusOK)
		job, ok := statusResp["job"].(map[string]any)
		if !ok {
			t.Fatalf("expected job object in second status response")
		}
		status, _ := job["status"].(string)
		if status == "done" {
			break
		}
		if status == "failed" {
			t.Fatalf("second export job failed: %v", job["error"])
		}
		time.Sleep(50 * time.Millisecond)
	}

	exportDoneOnly := getJSON(t, ts.URL+"/v1/api-keys/audit/exports?status=done&limit=10", "tenant-export-admin", http.StatusOK)
	exportDoneItems := listItemsFromResponse(t, exportDoneOnly)
	if len(exportDoneItems) == 0 {
		t.Fatalf("expected at least one done export job in filtered list")
	}

	csvBody, csvHeaders := getRaw(t, downloadURL, "", http.StatusOK)
	if !strings.Contains(strings.ToLower(csvHeaders.Get("Content-Type")), "text/csv") {
		t.Fatalf("expected csv content type, got %q", csvHeaders.Get("Content-Type"))
	}
	if !strings.Contains(csvBody, "id,tenant_id,api_key_id,actor_api_key_id,action,metadata,created_at") {
		t.Fatalf("expected csv header in downloaded export")
	}
	if !strings.Contains(csvBody, createdID) {
		t.Fatalf("expected downloaded export CSV to include created api key id=%s", createdID)
	}
}

func newTestLagoTransport(t *testing.T) (*service.LagoHTTPTransport, func()) {
	t.Helper()

	baseURL := strings.TrimSpace(os.Getenv("TEST_LAGO_API_URL"))
	apiKey := strings.TrimSpace(os.Getenv("TEST_LAGO_API_KEY"))
	if baseURL == "" || apiKey == "" {
		t.Skip("TEST_LAGO_API_URL and TEST_LAGO_API_KEY are required for Lago-backed API tests")
	}

	lagoTransport, err := service.NewLagoHTTPTransport(service.LagoClientConfig{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Timeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}

	return lagoTransport, func() {}
}

func startTemporalReplayRuntime(t *testing.T, repo *store.PostgresStore) (func() map[string]any, func()) {
	t.Helper()

	temporalAddress := strings.TrimSpace(os.Getenv("TEST_TEMPORAL_ADDRESS"))
	if temporalAddress == "" {
		temporalAddress = "127.0.0.1:17233"
	}
	temporalNamespace := strings.TrimSpace(os.Getenv("TEST_TEMPORAL_NAMESPACE"))
	if temporalNamespace == "" {
		temporalNamespace = "default"
	}
	taskQueue := fmt.Sprintf(
		"alpha-replay-it-%d",
		time.Now().UTC().UnixNano(),
	)

	temporalClient, err := temporalclient.Dial(temporalclient.Options{
		HostPort:  temporalAddress,
		Namespace: temporalNamespace,
	})
	if err != nil {
		t.Fatalf("dial temporal (%s): %v", temporalAddress, err)
	}
	ensureCtx, ensureCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer ensureCancel()
	if err := temporalutil.EnsureNamespaceReady(ensureCtx, temporalClient, temporalNamespace, 24*time.Hour); err != nil {
		temporalClient.Close()
		t.Fatalf("ensure temporal namespace %q: %v", temporalNamespace, err)
	}

	temporalWorker := temporalsdkworker.New(temporalClient, taskQueue, temporalsdkworker.Options{})
	replay.RegisterTemporalReplayWorker(temporalWorker, repo)
	if err := temporalWorker.Start(); err != nil {
		temporalClient.Close()
		t.Fatalf("start temporal worker: %v", err)
	}

	dispatcherCtx, dispatcherCancel := context.WithCancel(context.Background())
	dispatcher := replay.NewTemporalDispatcher(repo, temporalClient, taskQueue, 10*time.Millisecond, 100)
	go dispatcher.Run(dispatcherCtx)

	metricsProvider := func() map[string]any {
		return map[string]any{
			"replay_execution_mode":      "temporal",
			"replay_temporal_dispatcher": dispatcher.Stats(),
		}
	}
	cleanup := func() {
		dispatcherCancel()
		temporalWorker.Stop()
		temporalClient.Close()
	}
	return metricsProvider, cleanup
}

func TestCustomerCRUDAndReadiness(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for integration tests")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := store.NewPostgresStore(db)
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	resetTables(t, db)

	mustCreateAPIKey(t, repo, "customer-reader-key", api.RoleReader, "default")
	mustCreateAPIKey(t, repo, "customer-writer-key", api.RoleWriter, "default")

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	customerPaymentMethodReady := false
	lagoMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/customers":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"customer":{"lago_id":"lago_cust_alpha","external_id":"cust_alpha","billing_configuration":{"payment_provider":"stripe","payment_provider_code":"stripe_test","provider_customer_id":"pcus_123","provider_payment_methods":["card"]}}}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/customers/cust_alpha/payment_methods":
			w.Header().Set("Content-Type", "application/json")
			if customerPaymentMethodReady {
				_, _ = w.Write([]byte(`{"payment_methods":[{"lago_id":"pm_lago_alpha","is_default":true,"provider_method_id":"pm_123"}]}`))
				return
			}
			_, _ = w.Write([]byte(`{"payment_methods":[]}`))
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/customers/cust_alpha/checkout_url":
			customerPaymentMethodReady = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"customer":{"external_customer_id":"cust_alpha","checkout_url":"https://checkout.example.test/cust_alpha"}}`))
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer lagoMock.Close()
	lagoTransport, err := service.NewLagoHTTPTransport(service.LagoClientConfig{
		BaseURL: lagoMock.URL,
		APIKey:  "test-api-key",
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}
	now := time.Now().UTC()
	connectedAt := now
	lastSyncedAt := now
	connection, err := repo.CreateBillingProviderConnection(domain.BillingProviderConnection{
		ID:                 "bpc_onboarding_default",
		ProviderType:       domain.BillingProviderTypeStripe,
		Environment:        "test",
		DisplayName:        "Stripe Platform",
		Scope:              domain.BillingProviderConnectionScopePlatform,
		Status:             domain.BillingProviderConnectionStatusConnected,
		LagoOrganizationID: "org_default",
		LagoProviderCode:   "stripe_test",
		SecretRef:          "memory://billing-provider-connections/bpc_onboarding_default/seed",
		ConnectedAt:        &connectedAt,
		LastSyncedAt:       &lastSyncedAt,
		CreatedByType:      "platform_api_key",
		CreatedByID:        "pkey_seed",
		CreatedAt:          now,
		UpdatedAt:          now,
	})
	if err != nil {
		t.Fatalf("create onboarding billing provider connection: %v", err)
	}
	_, err = repo.CreateWorkspaceBillingBinding(domain.WorkspaceBillingBinding{
		ID:                          "wbb_onboarding_default",
		WorkspaceID:                 "default",
		BillingProviderConnectionID: connection.ID,
		Backend:                     domain.WorkspaceBillingBackendLago,
		BackendOrganizationID:       connection.LagoOrganizationID,
		BackendProviderCode:         connection.LagoProviderCode,
		IsolationMode:               domain.WorkspaceBillingIsolationModeShared,
		Status:                      domain.WorkspaceBillingBindingStatusConnected,
		ConnectedAt:                 &connectedAt,
		CreatedByType:               "platform_api_key",
		CreatedByID:                 "pkey_seed",
		CreatedAt:                   now,
		UpdatedAt:                   now,
	})
	if err != nil {
		t.Fatalf("create onboarding workspace billing binding: %v", err)
	}

	ts := httptest.NewServer(api.NewServer(
		repo,
		api.WithAPIKeyAuthorizer(authorizer),
		api.WithCustomerBillingAdapter(service.NewLagoCustomerBillingAdapter(lagoTransport)),
	).Handler())
	defer ts.Close()

	readerForbidden := postJSON(t, ts.URL+"/v1/customers", map[string]any{
		"external_id":  "cust_alpha",
		"display_name": "Alpha Co",
	}, "customer-reader-key", http.StatusForbidden)
	if got, _ := readerForbidden["error"].(string); !strings.Contains(strings.ToLower(got), "forbidden") {
		t.Fatalf("expected reader create forbidden, got %q", got)
	}

	customer := postJSON(t, ts.URL+"/v1/customers", map[string]any{
		"external_id":  "cust_alpha",
		"display_name": "Alpha Co",
		"email":        "billing@alpha.test",
	}, "customer-writer-key", http.StatusCreated)
	if got, _ := customer["external_id"].(string); got != "cust_alpha" {
		t.Fatalf("expected external_id cust_alpha, got %q", got)
	}
	if got, _ := customer["status"].(string); got != "active" {
		t.Fatalf("expected status active, got %q", got)
	}

	duplicate := postJSON(t, ts.URL+"/v1/customers", map[string]any{
		"external_id": "cust_alpha",
	}, "customer-writer-key", http.StatusConflict)
	if got, _ := duplicate["error"].(string); !strings.Contains(strings.ToLower(got), "already exists") {
		t.Fatalf("expected duplicate customer conflict, got %q", got)
	}

	customers := getJSONArray(t, ts.URL+"/v1/customers?limit=10", "customer-reader-key", http.StatusOK)
	if len(customers) != 1 {
		t.Fatalf("expected customer list size 1, got %d", len(customers))
	}

	gotCustomer := getJSON(t, ts.URL+"/v1/customers/cust_alpha", "customer-reader-key", http.StatusOK)
	if got, _ := gotCustomer["display_name"].(string); got != "Alpha Co" {
		t.Fatalf("expected display_name Alpha Co, got %q", got)
	}

	profileMissing := getJSON(t, ts.URL+"/v1/customers/cust_alpha/billing-profile", "customer-reader-key", http.StatusOK)
	if got, _ := profileMissing["profile_status"].(string); got != "missing" {
		t.Fatalf("expected billing profile status missing, got %q", got)
	}
	setupMissing := getJSON(t, ts.URL+"/v1/customers/cust_alpha/payment-setup", "customer-reader-key", http.StatusOK)
	if got, _ := setupMissing["setup_status"].(string); got != "missing" {
		t.Fatalf("expected payment setup status missing, got %q", got)
	}
	readinessPending := getJSON(t, ts.URL+"/v1/customers/cust_alpha/readiness", "customer-reader-key", http.StatusOK)
	if got, _ := readinessPending["status"].(string); got != "pending" {
		t.Fatalf("expected readiness pending, got %q", got)
	}
	assertStringArrayContains(t, readinessPending["missing_steps"], "billing_profile_ready")
	assertStringArrayContains(t, readinessPending["missing_steps"], "payment_setup_ready")

	updated := patchJSON(t, ts.URL+"/v1/customers/cust_alpha", map[string]any{
		"display_name":     "Alpha Company",
		"lago_customer_id": "lago_cust_alpha",
	}, "customer-writer-key", http.StatusOK)
	if got, _ := updated["display_name"].(string); got != "Alpha Company" {
		t.Fatalf("expected updated display_name Alpha Company, got %q", got)
	}

	profileIncomplete := putJSON(t, ts.URL+"/v1/customers/cust_alpha/billing-profile", map[string]any{
		"legal_name": "Alpha Company Pvt Ltd",
		"email":      "billing@alpha.test",
		"currency":   "usd",
	}, "customer-writer-key", http.StatusOK)
	if got, _ := profileIncomplete["profile_status"].(string); got != "incomplete" {
		t.Fatalf("expected billing profile status incomplete, got %q", got)
	}

	readinessStillPending := getJSON(t, ts.URL+"/v1/customers/cust_alpha/readiness", "customer-reader-key", http.StatusOK)
	if got, _ := readinessStillPending["status"].(string); got != "pending" {
		t.Fatalf("expected readiness pending after partial setup, got %q", got)
	}

	profileReady := putJSON(t, ts.URL+"/v1/customers/cust_alpha/billing-profile", map[string]any{
		"legal_name":            "Alpha Company Pvt Ltd",
		"email":                 "billing@alpha.test",
		"billing_address_line1": "1 Billing St",
		"billing_city":          "Bengaluru",
		"billing_postal_code":   "560001",
		"billing_country":       "IN",
		"currency":              "USD",
		"provider_code":         "stripe_default",
	}, "customer-writer-key", http.StatusOK)
	if got, _ := profileReady["profile_status"].(string); got != "ready" {
		t.Fatalf("expected billing profile status ready, got %q", got)
	}
	gotCustomerAfterSync := getJSON(t, ts.URL+"/v1/customers/cust_alpha", "customer-reader-key", http.StatusOK)
	if got, _ := gotCustomerAfterSync["lago_customer_id"].(string); got != "lago_cust_alpha" {
		t.Fatalf("expected lago_customer_id lago_cust_alpha after sync, got %q", got)
	}

	checkout := postJSON(t, ts.URL+"/v1/customers/cust_alpha/payment-setup/checkout-url", map[string]any{
		"payment_method_type": "card",
	}, "customer-writer-key", http.StatusOK)
	if got, _ := checkout["checkout_url"].(string); got != "https://checkout.example.test/cust_alpha" {
		t.Fatalf("expected checkout_url response, got %q", got)
	}
	setupPending := getJSON(t, ts.URL+"/v1/customers/cust_alpha/payment-setup", "customer-reader-key", http.StatusOK)
	if got, _ := setupPending["setup_status"].(string); got != "pending" {
		t.Fatalf("expected payment setup status pending immediately after checkout generation, got %q", got)
	}
	readinessAfterCheckout := getJSON(t, ts.URL+"/v1/customers/cust_alpha/readiness", "customer-reader-key", http.StatusOK)
	if got, _ := readinessAfterCheckout["status"].(string); got != "pending" {
		t.Fatalf("expected readiness to remain pending before explicit refresh, got %q", got)
	}
	refresh := postJSON(t, ts.URL+"/v1/customers/cust_alpha/payment-setup/refresh", map[string]any{}, "customer-writer-key", http.StatusOK)
	if got, _ := refresh["external_id"].(string); got != "cust_alpha" {
		t.Fatalf("expected refresh external_id cust_alpha, got %q", got)
	}
	refreshReadiness, ok := refresh["readiness"].(map[string]any)
	if !ok {
		t.Fatalf("expected readiness in refresh response")
	}
	if got, _ := refreshReadiness["status"].(string); got != "ready" {
		t.Fatalf("expected refresh readiness ready, got %q", got)
	}

	readinessReady := getJSON(t, ts.URL+"/v1/customers/cust_alpha/readiness", "customer-reader-key", http.StatusOK)
	if got, _ := readinessReady["status"].(string); got != "ready" {
		t.Fatalf("expected readiness ready, got %q", got)
	}
	if got, _ := readinessReady["billing_provider_configured"].(bool); !got {
		t.Fatalf("expected billing_provider_configured=true")
	}
	if got, _ := readinessReady["lago_customer_synced"].(bool); !got {
		t.Fatalf("expected lago_customer_synced=true")
	}
	if got, _ := readinessReady["default_payment_method_verified"].(bool); !got {
		t.Fatalf("expected default_payment_method_verified=true")
	}
	if missingRaw, ok := readinessReady["missing_steps"].([]any); !ok || len(missingRaw) != 0 {
		t.Fatalf("expected readiness missing_steps empty, got %v", readinessReady["missing_steps"])
	}
}

func TestCustomerOnboardingWorkflow(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for integration tests")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := store.NewPostgresStore(db)
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	resetTables(t, db)

	mustCreateAPIKey(t, repo, "customer-reader-key", api.RoleReader, "default")
	mustCreateAPIKey(t, repo, "customer-writer-key", api.RoleWriter, "default")

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	customerPaymentMethodReady := false
	lagoMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/customers":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"customer":{"lago_id":"lago_cust_flow","external_id":"cust_flow","billing_configuration":{"payment_provider":"stripe","payment_provider_code":"stripe_test","provider_customer_id":"pcus_flow","provider_payment_methods":["card"]}}}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/customers/cust_flow/payment_methods":
			w.Header().Set("Content-Type", "application/json")
			if customerPaymentMethodReady {
				_, _ = w.Write([]byte(`{"payment_methods":[{"lago_id":"pm_lago_flow","is_default":true,"provider_method_id":"pm_flow"}]}`))
				return
			}
			_, _ = w.Write([]byte(`{"payment_methods":[]}`))
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/customers/cust_flow/checkout_url":
			customerPaymentMethodReady = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"customer":{"external_customer_id":"cust_flow","checkout_url":"https://checkout.example.test/cust_flow"}}`))
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer lagoMock.Close()
	lagoTransport, err := service.NewLagoHTTPTransport(service.LagoClientConfig{
		BaseURL: lagoMock.URL,
		APIKey:  "test-api-key",
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}
	now := time.Now().UTC()
	connectedAt := now
	lastSyncedAt := now
	connection, err := repo.CreateBillingProviderConnection(domain.BillingProviderConnection{
		ID:                 "bpc_onboarding_default",
		ProviderType:       domain.BillingProviderTypeStripe,
		Environment:        "test",
		DisplayName:        "Stripe Platform",
		Scope:              domain.BillingProviderConnectionScopePlatform,
		Status:             domain.BillingProviderConnectionStatusConnected,
		LagoOrganizationID: "org_default",
		LagoProviderCode:   "stripe_test",
		SecretRef:          "memory://billing-provider-connections/bpc_onboarding_default/seed",
		ConnectedAt:        &connectedAt,
		LastSyncedAt:       &lastSyncedAt,
		CreatedByType:      "platform_api_key",
		CreatedByID:        "pkey_seed",
		CreatedAt:          now,
		UpdatedAt:          now,
	})
	if err != nil {
		t.Fatalf("create onboarding billing provider connection: %v", err)
	}
	_, err = repo.CreateWorkspaceBillingBinding(domain.WorkspaceBillingBinding{
		ID:                          "wbb_onboarding_default",
		WorkspaceID:                 "default",
		BillingProviderConnectionID: connection.ID,
		Backend:                     domain.WorkspaceBillingBackendLago,
		BackendOrganizationID:       connection.LagoOrganizationID,
		BackendProviderCode:         connection.LagoProviderCode,
		IsolationMode:               domain.WorkspaceBillingIsolationModeShared,
		Status:                      domain.WorkspaceBillingBindingStatusConnected,
		ConnectedAt:                 &connectedAt,
		CreatedByType:               "platform_api_key",
		CreatedByID:                 "pkey_seed",
		CreatedAt:                   now,
		UpdatedAt:                   now,
	})
	if err != nil {
		t.Fatalf("create onboarding workspace billing binding: %v", err)
	}

	ts := httptest.NewServer(api.NewServer(
		repo,
		api.WithAPIKeyAuthorizer(authorizer),
		api.WithCustomerBillingAdapter(service.NewLagoCustomerBillingAdapter(lagoTransport)),
	).Handler())
	defer ts.Close()

	created := postJSON(t, ts.URL+"/v1/customer-onboarding", map[string]any{
		"external_id":         "cust_flow",
		"display_name":        "Flow Co",
		"email":               "billing@flow.test",
		"start_payment_setup": true,
		"payment_method_type": "card",
		"billing_profile": map[string]any{
			"legal_name":            "Flow Co Pvt Ltd",
			"email":                 "billing@flow.test",
			"billing_address_line1": "1 Flow Street",
			"billing_city":          "Bengaluru",
			"billing_postal_code":   "560001",
			"billing_country":       "IN",
			"currency":              "USD",
			"provider_code":         "stripe_test",
		},
	}, "customer-writer-key", http.StatusCreated)
	if got, _ := created["customer_created"].(bool); !got {
		t.Fatalf("expected customer_created=true")
	}
	if got, _ := created["billing_profile_applied"].(bool); !got {
		t.Fatalf("expected billing_profile_applied=true")
	}
	if got, _ := created["payment_setup_started"].(bool); !got {
		t.Fatalf("expected payment_setup_started=true")
	}
	if got, _ := created["checkout_url"].(string); got != "https://checkout.example.test/cust_flow" {
		t.Fatalf("expected checkout_url, got %q", got)
	}
	createdCustomer, ok := created["customer"].(map[string]any)
	if !ok {
		t.Fatalf("expected customer object in onboarding response")
	}
	if got, _ := createdCustomer["lago_customer_id"].(string); got != "lago_cust_flow" {
		t.Fatalf("expected lago_customer_id lago_cust_flow, got %q", got)
	}
	createdReadiness, ok := created["readiness"].(map[string]any)
	if !ok {
		t.Fatalf("expected readiness object in onboarding response")
	}
	if got, _ := createdReadiness["status"].(string); got != "pending" {
		t.Fatalf("expected onboarding readiness pending before refresh, got %q", got)
	}

	reconciled := postJSON(t, ts.URL+"/v1/customer-onboarding", map[string]any{
		"external_id":  "cust_flow",
		"display_name": "Flow Company",
	}, "customer-writer-key", http.StatusOK)
	if got, _ := reconciled["customer_created"].(bool); got {
		t.Fatalf("expected customer_created=false on reconcile")
	}
	reconciledCustomer, ok := reconciled["customer"].(map[string]any)
	if !ok {
		t.Fatalf("expected reconciled customer object")
	}
	if got, _ := reconciledCustomer["display_name"].(string); got != "Flow Company" {
		t.Fatalf("expected reconciled display_name Flow Company, got %q", got)
	}
	if got, _ := reconciledCustomer["email"].(string); got != "billing.test" {
		t.Fatalf("expected reconciled email to be preserved, got %q", got)
	}

	refresh := postJSON(t, ts.URL+"/v1/customers/cust_flow/payment-setup/refresh", map[string]any{}, "customer-writer-key", http.StatusOK)
	refreshReadiness, ok := refresh["readiness"].(map[string]any)
	if !ok {
		t.Fatalf("expected readiness in refresh response")
	}
	if got, _ := refreshReadiness["status"].(string); got != "ready" {
		t.Fatalf("expected refresh readiness ready, got %q", got)
	}
}

func TestCustomerPaymentSetupRequestAndResend(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for integration tests")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := store.NewPostgresStore(db)
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	resetTables(t, db)

	mustCreateAPIKey(t, repo, "customer-reader-key", api.RoleReader, "default")
	mustCreateAPIKey(t, repo, "customer-writer-key", api.RoleWriter, "default")

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	customerPaymentMethodReady := false
	lagoMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/customers":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"customer":{"lago_id":"lago_cust_alpha","external_id":"cust_alpha","billing_configuration":{"payment_provider":"stripe","payment_provider_code":"stripe_test","provider_customer_id":"pcus_123","provider_payment_methods":["card"]}}}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/customers/cust_alpha/payment_methods":
			w.Header().Set("Content-Type", "application/json")
			if customerPaymentMethodReady {
				_, _ = w.Write([]byte(`{"payment_methods":[{"lago_id":"pm_lago_alpha","is_default":true,"provider_method_id":"pm_123"}]}`))
				return
			}
			_, _ = w.Write([]byte(`{"payment_methods":[]}`))
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/customers/cust_alpha/checkout_url":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"customer":{"external_customer_id":"cust_alpha","checkout_url":"https://checkout.example.test/cust_alpha"}}`))
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer lagoMock.Close()
	lagoTransport, err := service.NewLagoHTTPTransport(service.LagoClientConfig{
		BaseURL: lagoMock.URL,
		APIKey:  "test-api-key",
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}
	mustSetTenantMappings(t, repo, "default", "org_default", "stripe_test")

	emailSender := &fakeCustomerPaymentSetupRequestEmailSender{}
	ts := httptest.NewServer(api.NewServer(
		repo,
		api.WithAPIKeyAuthorizer(authorizer),
		api.WithCustomerBillingAdapter(service.NewLagoCustomerBillingAdapter(lagoTransport)),
		api.WithNotificationService(service.NewNotificationService(nil, nil, emailSender, nil)),
	).Handler())
	defer ts.Close()

	_ = postJSON(t, ts.URL+"/v1/customers", map[string]any{
		"external_id":  "cust_alpha",
		"display_name": "Alpha Co",
		"email":        "billing@alpha.test",
	}, "customer-writer-key", http.StatusCreated)
	_ = putJSON(t, ts.URL+"/v1/customers/cust_alpha/billing-profile", map[string]any{
		"legal_name":            "Alpha Company Pvt Ltd",
		"email":                 "billing@alpha.test",
		"billing_address_line1": "1 Billing St",
		"billing_city":          "Bengaluru",
		"billing_postal_code":   "560001",
		"billing_country":       "IN",
		"currency":              "USD",
		"provider_code":         "stripe_test",
	}, "customer-writer-key", http.StatusOK)

	requested := postJSON(t, ts.URL+"/v1/customers/cust_alpha/payment-setup/request", map[string]any{
		"payment_method_type": "card",
	}, "customer-writer-key", http.StatusOK)
	if got, _ := requested["action"].(string); got != "requested" {
		t.Fatalf("expected action requested, got %q", got)
	}
	if got, _ := requested["checkout_url"].(string); got != "https://checkout.example.test/cust_alpha" {
		t.Fatalf("expected checkout_url from request flow, got %q", got)
	}
	if len(emailSender.inputs) != 1 {
		t.Fatalf("expected one payment setup request email, got %d", len(emailSender.inputs))
	}
	if got := emailSender.inputs[0].ToEmail; got != "billing@alpha.test" {
		t.Fatalf("expected request recipient billing@alpha.test, got %q", got)
	}
	if got := emailSender.inputs[0].RequestKind; got != "requested" {
		t.Fatalf("expected request kind requested, got %q", got)
	}

	resent := postJSON(t, ts.URL+"/v1/customers/cust_alpha/payment-setup/resend", map[string]any{
		"payment_method_type": "card",
	}, "customer-writer-key", http.StatusOK)
	if got, _ := resent["action"].(string); got != "resent" {
		t.Fatalf("expected action resent, got %q", got)
	}
	if len(emailSender.inputs) != 2 {
		t.Fatalf("expected two payment setup request emails after resend, got %d", len(emailSender.inputs))
	}
	if got := emailSender.inputs[1].RequestKind; got != "resent" {
		t.Fatalf("expected resend request kind resent, got %q", got)
	}

	setup := getJSON(t, ts.URL+"/v1/customers/cust_alpha/payment-setup", "customer-reader-key", http.StatusOK)
	if got, _ := setup["last_request_status"].(string); got != "sent" {
		t.Fatalf("expected last_request_status sent, got %q", got)
	}
	if got, _ := setup["last_request_kind"].(string); got != "resent" {
		t.Fatalf("expected last_request_kind resent, got %q", got)
	}
	if got, _ := setup["last_request_to_email"].(string); got != "billing@alpha.test" {
		t.Fatalf("expected last_request_to_email billing@alpha.test, got %q", got)
	}
	if got, _ := setup["last_request_sent_at"].(string); strings.TrimSpace(got) == "" {
		t.Fatalf("expected last_request_sent_at to be set")
	}

	auditPage, err := repo.ListTenantAuditEvents(store.TenantAuditFilter{TenantID: "default", Limit: 20})
	if err != nil {
		t.Fatalf("list tenant audit events: %v", err)
	}
	var sawRequested, sawResent bool
	for _, item := range auditPage.Items {
		if item.TenantID != "default" {
			continue
		}
		if item.Action == "customer.payment_setup_requested" {
			sawRequested = true
		}
		if item.Action == "customer.payment_setup_resent" {
			sawResent = true
		}
	}
	if !sawRequested {
		t.Fatalf("expected customer.payment_setup_requested tenant audit event")
	}
	if !sawResent {
		t.Fatalf("expected customer.payment_setup_resent tenant audit event")
	}
}

func TestCustomerBillingProfileRetrySync(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for integration tests")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := store.NewPostgresStore(db)
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	resetTables(t, db)

	mustCreateAPIKey(t, repo, "customer-reader-key", api.RoleReader, "default")
	mustCreateAPIKey(t, repo, "customer-writer-key", api.RoleWriter, "default")

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	syncHealthy := false
	lagoMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/customers":
			if !syncHealthy {
				http.Error(w, `{"error":"temporary failure"}`, http.StatusBadGateway)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"customer":{"lago_id":"lago_cust_retry","external_id":"cust_retry","billing_configuration":{"payment_provider":"stripe","payment_provider_code":"stripe_test","provider_customer_id":"pcus_retry","provider_payment_methods":["card"]}}}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/customers/cust_retry/payment_methods":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"payment_methods":[]}`))
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer lagoMock.Close()
	lagoTransport, err := service.NewLagoHTTPTransport(service.LagoClientConfig{
		BaseURL: lagoMock.URL,
		APIKey:  "test-api-key",
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}
	mustSetTenantMappings(t, repo, "default", "org_default", "stripe_test")

	ts := httptest.NewServer(api.NewServer(
		repo,
		api.WithAPIKeyAuthorizer(authorizer),
		api.WithCustomerBillingAdapter(service.NewLagoCustomerBillingAdapter(lagoTransport)),
	).Handler())
	defer ts.Close()

	_ = postJSON(t, ts.URL+"/v1/customers", map[string]any{
		"external_id":  "cust_retry",
		"display_name": "Retry Co",
		"email":        "billing@retry.test",
	}, "customer-writer-key", http.StatusCreated)

	failed := putJSON(t, ts.URL+"/v1/customers/cust_retry/billing-profile", map[string]any{
		"legal_name":            "Retry Co Pvt Ltd",
		"email":                 "billing@retry.test",
		"billing_address_line1": "1 Retry Street",
		"billing_city":          "Bengaluru",
		"billing_postal_code":   "560001",
		"billing_country":       "IN",
		"currency":              "USD",
		"provider_code":         "stripe_test",
	}, "customer-writer-key", http.StatusInternalServerError)
	if got, _ := failed["error"].(string); !strings.Contains(strings.ToLower(got), "temporary failure") {
		t.Fatalf("expected sync failure response, got %q", got)
	}

	profile := getJSON(t, ts.URL+"/v1/customers/cust_retry/billing-profile", "customer-reader-key", http.StatusOK)
	if got, _ := profile["profile_status"].(string); got != "sync_error" {
		t.Fatalf("expected billing profile status sync_error after failed sync, got %q", got)
	}
	if got, _ := profile["last_sync_error"].(string); got == "" {
		t.Fatalf("expected last_sync_error after failed sync")
	}

	syncHealthy = true
	retried := postJSON(t, ts.URL+"/v1/customers/cust_retry/billing-profile/retry-sync", map[string]any{}, "customer-writer-key", http.StatusOK)
	if got, _ := retried["external_id"].(string); got != "cust_retry" {
		t.Fatalf("expected external_id cust_retry, got %q", got)
	}
	retriedProfile, ok := retried["billing_profile"].(map[string]any)
	if !ok {
		t.Fatalf("expected billing_profile object in retry response")
	}
	if got, _ := retriedProfile["profile_status"].(string); got != "ready" {
		t.Fatalf("expected billing profile status ready after retry, got %q", got)
	}
	if got, _ := retriedProfile["last_sync_error"].(string); got != "" {
		t.Fatalf("expected last_sync_error cleared after retry, got %q", got)
	}

	customer := getJSON(t, ts.URL+"/v1/customers/cust_retry", "customer-reader-key", http.StatusOK)
	if got, _ := customer["lago_customer_id"].(string); got != "lago_cust_retry" {
		t.Fatalf("expected lago_customer_id lago_cust_retry after retry, got %q", got)
	}
}

func mustCreateAPIKey(t *testing.T, repo *store.PostgresStore, rawKey string, role api.Role, tenantID string) {
	t.Helper()

	if _, err := repo.CreateTenant(domain.Tenant{
		ID:     tenantID,
		Name:   tenantID,
		Status: domain.TenantStatusActive,
	}); err != nil && err != store.ErrAlreadyExists && err != store.ErrDuplicateKey {
		t.Fatalf("create tenant %q: %v", tenantID, err)
	}

	hashed := api.HashAPIKey(rawKey)
	prefix := api.KeyPrefixFromHash(hashed)
	_, err := repo.CreateAPIKey(domain.APIKey{
		KeyPrefix: prefix,
		KeyHash:   hashed,
		Name:      "test-" + string(role),
		Role:      string(role),
		TenantID:  tenantID,
	})
	if err != nil {
		t.Fatalf("create api key %q: %v", rawKey, err)
	}
}

func mustCreatePlatformAPIKey(t *testing.T, repo *store.PostgresStore, rawKey string) {
	t.Helper()

	hashed := api.HashAPIKey(rawKey)
	prefix := api.KeyPrefixFromHash(hashed)
	_, err := repo.CreatePlatformAPIKey(domain.PlatformAPIKey{
		KeyPrefix: prefix,
		KeyHash:   hashed,
		Name:      "test-platform-admin",
		Role:      string(api.PlatformRoleAdmin),
	})
	if err != nil {
		t.Fatalf("create platform api key %q: %v", rawKey, err)
	}
}

func mustSetTenantMappings(t *testing.T, repo *store.PostgresStore, tenantID, lagoOrganizationID, lagoBillingProviderCode string) {
	t.Helper()

	tenant, err := repo.GetTenant(tenantID)
	if err != nil {
		t.Fatalf("get tenant %q: %v", tenantID, err)
	}
	tenant.LagoOrganizationID = lagoOrganizationID
	tenant.LagoBillingProviderCode = lagoBillingProviderCode
	tenant.UpdatedAt = time.Now().UTC()
	if _, err := repo.UpdateTenant(tenant); err != nil {
		t.Fatalf("update tenant %q mappings: %v", tenantID, err)
	}
}

func postJSON(t *testing.T, url string, body any, apiKey string, expectedStatus int) map[string]any {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
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

func patchJSON(t *testing.T, url string, body any, apiKey string, expectedStatus int) map[string]any {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("patch request failed: %v", err)
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

func putJSON(t *testing.T, url string, body any, apiKey string, expectedStatus int) map[string]any {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("put request failed: %v", err)
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

func postJSONWithHeaders(t *testing.T, url string, body any, headers map[string]string, apiKey string, expectedStatus int) map[string]any {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
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

func getJSON(t *testing.T, url string, apiKey string, expectedStatus int) map[string]any {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	resp, err := http.DefaultClient.Do(req)
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

func getJSONArray(t *testing.T, url string, apiKey string, expectedStatus int) []any {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != expectedStatus {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected status %d, expected %d, body=%s", resp.StatusCode, expectedStatus, string(b))
	}

	var out []any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}

func containsID(items []any, id string) bool {
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if got, _ := m["id"].(string); got == id {
			return true
		}
	}
	return false
}

func listItemsFromResponse(t *testing.T, resp map[string]any) []any {
	t.Helper()
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("expected response items field to be an array, got %T", resp["items"])
	}
	return items
}

func containsActionForKey(items []any, apiKeyID, action string) bool {
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		gotKeyID, _ := m["api_key_id"].(string)
		gotAction, _ := m["action"].(string)
		if gotKeyID == apiKeyID && gotAction == action {
			return true
		}
	}
	return false
}

func containsTenantAuditAction(items []any, tenantID, action string) bool {
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		gotTenantID, _ := m["tenant_id"].(string)
		gotAction, _ := m["action"].(string)
		gotEventCode, _ := m["event_code"].(string)
		if gotTenantID == tenantID && (gotAction == action || gotEventCode == action) {
			return true
		}
	}
	return false
}

func nestedID(t *testing.T, payload map[string]any, field string) string {
	t.Helper()
	obj, ok := payload[field].(map[string]any)
	if !ok {
		t.Fatalf("expected payload field %q to be object", field)
	}
	id, _ := obj["id"].(string)
	if id == "" {
		t.Fatalf("expected payload field %q to contain id", field)
	}
	return id
}

func getRaw(t *testing.T, url string, apiKey string, expectedStatus int) (string, http.Header) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get request failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body failed: %v", err)
	}

	if resp.StatusCode != expectedStatus {
		t.Fatalf("unexpected status %d, expected %d, body=%s", resp.StatusCode, expectedStatus, string(bodyBytes))
	}

	return string(bodyBytes), resp.Header
}

func assertStringArrayContains(t *testing.T, raw any, want string) {
	t.Helper()
	items, ok := raw.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", raw)
	}
	for _, item := range items {
		if got, _ := item.(string); got == want {
			return
		}
	}
	t.Fatalf("expected %q in %v", want, raw)
}

func envIntOrDefault(t *testing.T, key string, defaultValue int) int {
	t.Helper()

	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultValue
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		t.Fatalf("invalid %s=%q: %v", key, raw, err)
	}
	return parsed
}

func TestInvoiceResendEmailDelegatesThroughNotificationService(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for integration tests")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := store.NewPostgresStore(db)
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	resetTables(t, db)

	mustCreateAPIKey(t, repo, "tenant-a-writer", api.RoleWriter, "tenant_a")

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	var (
		resendCalls int
		lastBody    map[string]any
	)
	lagoMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/invoices/inv_123/resend_email" {
			resendCalls++
			defer r.Body.Close()
			rawBody, _ := io.ReadAll(r.Body)
			if len(rawBody) > 0 {
				_ = json.Unmarshal(rawBody, &lastBody)
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer lagoMock.Close()

	lagoTransport, err := service.NewLagoHTTPTransport(service.LagoClientConfig{
		BaseURL: lagoMock.URL,
		APIKey:  "test-api-key",
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}
	invoiceAdapter := service.NewLagoInvoiceAdapter(lagoTransport)

	ts := httptest.NewServer(api.NewServer(
		repo,
		api.WithAPIKeyAuthorizer(authorizer),
		api.WithInvoiceBillingAdapter(invoiceAdapter),
		api.WithNotificationService(service.NewNotificationService(nil, nil, nil, invoiceAdapter)),
	).Handler())
	defer ts.Close()

	resp := postJSON(
		t,
		ts.URL+"/v1/invoices/inv_123/resend-email",
		map[string]any{"to": []string{"billing@acme.test"}},
		"tenant-a-writer",
		http.StatusAccepted,
	)

	if resendCalls != 1 {
		t.Fatalf("expected exactly one resend call to lago, got %d", resendCalls)
	}
	if got, _ := resp["action"].(string); got != "resend_invoice_email" {
		t.Fatalf("expected action resend_invoice_email, got %q", got)
	}
	if got, _ := resp["backend"].(string); got != "lago" {
		t.Fatalf("expected backend lago, got %q", got)
	}
	if got, _ := resp["dispatched"].(bool); !got {
		t.Fatalf("expected dispatched=true")
	}
	toRaw, ok := lastBody["to"].([]any)
	if !ok || len(toRaw) != 1 {
		t.Fatalf("expected one custom recipient, got %#v", lastBody["to"])
	}
	if got, _ := toRaw[0].(string); got != "billing@acme.test" {
		t.Fatalf("expected recipient billing@acme.test, got %q", got)
	}
}

func TestBillingDocumentResendEmailDelegatesThroughNotificationService(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for integration tests")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := store.NewPostgresStore(db)
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	resetTables(t, db)

	mustCreateAPIKey(t, repo, "tenant-a-writer", api.RoleWriter, "tenant_a")

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	var (
		paymentReceiptCalls int
		creditNoteCalls     int
		lastReceiptBody     map[string]any
		lastCreditNoteBody  map[string]any
	)
	lagoMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		rawBody, _ := io.ReadAll(r.Body)
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/payment_receipts/pr_123/resend_email":
			paymentReceiptCalls++
			if len(rawBody) > 0 {
				_ = json.Unmarshal(rawBody, &lastReceiptBody)
			}
			w.WriteHeader(http.StatusOK)
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/credit_notes/cn_123/resend_email":
			creditNoteCalls++
			if len(rawBody) > 0 {
				_ = json.Unmarshal(rawBody, &lastCreditNoteBody)
			}
			w.WriteHeader(http.StatusOK)
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer lagoMock.Close()

	lagoTransport, err := service.NewLagoHTTPTransport(service.LagoClientConfig{
		BaseURL: lagoMock.URL,
		APIKey:  "test-api-key",
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}
	invoiceAdapter := service.NewLagoInvoiceAdapter(lagoTransport)

	ts := httptest.NewServer(api.NewServer(
		repo,
		api.WithAPIKeyAuthorizer(authorizer),
		api.WithInvoiceBillingAdapter(invoiceAdapter),
		api.WithNotificationService(service.NewNotificationService(nil, nil, nil, invoiceAdapter)),
	).Handler())
	defer ts.Close()

	receiptResp := postJSON(
		t,
		ts.URL+"/v1/payment-receipts/pr_123/resend-email",
		map[string]any{"cc": []string{"finance@acme.test"}},
		"tenant-a-writer",
		http.StatusAccepted,
	)
	if paymentReceiptCalls != 1 {
		t.Fatalf("expected exactly one payment receipt resend call to lago, got %d", paymentReceiptCalls)
	}
	if got, _ := receiptResp["action"].(string); got != "resend_payment_receipt_email" {
		t.Fatalf("expected action resend_payment_receipt_email, got %q", got)
	}
	ccRaw, ok := lastReceiptBody["cc"].([]any)
	if !ok || len(ccRaw) != 1 {
		t.Fatalf("expected one cc recipient, got %#v", lastReceiptBody["cc"])
	}
	if got, _ := ccRaw[0].(string); got != "finance@acme.test" {
		t.Fatalf("expected cc recipient finance@acme.test, got %q", got)
	}

	creditResp := postJSON(
		t,
		ts.URL+"/v1/credit-notes/cn_123/resend-email",
		map[string]any{"bcc": []string{"audit@acme.test"}},
		"tenant-a-writer",
		http.StatusAccepted,
	)
	if creditNoteCalls != 1 {
		t.Fatalf("expected exactly one credit note resend call to lago, got %d", creditNoteCalls)
	}
	if got, _ := creditResp["action"].(string); got != "resend_credit_note_email" {
		t.Fatalf("expected action resend_credit_note_email, got %q", got)
	}
	bccRaw, ok := lastCreditNoteBody["bcc"].([]any)
	if !ok || len(bccRaw) != 1 {
		t.Fatalf("expected one bcc recipient, got %#v", lastCreditNoteBody["bcc"])
	}
	if got, _ := bccRaw[0].(string); got != "audit@acme.test" {
		t.Fatalf("expected bcc recipient audit@acme.test, got %q", got)
	}
}

func TestRetryPaymentMaterializesPendingProjectionWithoutWebhook(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for integration tests")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := store.NewPostgresStore(db)
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	resetTables(t, db)

	mustCreateAPIKey(t, repo, "tenant-a-writer", api.RoleWriter, "default")
	mustCreateAPIKey(t, repo, "tenant-a-reader", api.RoleReader, "default")

	now := time.Now().UTC().Truncate(time.Second)
	customer, err := repo.CreateCustomer(domain.Customer{
		TenantID:    "default",
		ExternalID:  "cust_retry_pending",
		DisplayName: "Retry Pending Corp",
		Email:       "billing@retry-pending.test",
		Status:      domain.CustomerStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		t.Fatalf("create customer: %v", err)
	}
	if _, err := repo.UpsertCustomerBillingProfile(domain.CustomerBillingProfile{
		CustomerID:    customer.ID,
		TenantID:      "default",
		LegalName:     "Retry Pending Corp",
		Email:         "billing@retry-pending.test",
		Currency:      "USD",
		ProviderCode:  "stripe_default",
		ProfileStatus: domain.BillingProfileStatusReady,
		LastSyncedAt:  &now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}); err != nil {
		t.Fatalf("upsert billing profile: %v", err)
	}
	if _, err := repo.UpsertCustomerPaymentSetup(domain.CustomerPaymentSetup{
		CustomerID:                  customer.ID,
		TenantID:                    "default",
		SetupStatus:                 domain.PaymentSetupStatusPending,
		DefaultPaymentMethodPresent: false,
		PaymentMethodType:           "card",
		CreatedAt:                   now,
		UpdatedAt:                   now,
	}); err != nil {
		t.Fatalf("upsert payment setup: %v", err)
	}

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	retryCalls := 0
	getInvoiceCalls := 0
	lagoMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/invoices/inv_pending_1/retry_payment":
			retryCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"invoice":{"lago_id":"inv_pending_1","payment_status":"pending"}}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/invoices/inv_pending_1":
			getInvoiceCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"invoice":{"lago_id":"inv_pending_1","billing_entity_code":"org_test_1","status":"finalized","payment_status":"pending","payment_overdue":false,"number":"INV-PENDING-1","currency":"USD","total_amount_cents":199,"total_due_amount_cents":199,"total_paid_amount_cents":0,"updated_at":"2026-03-22T12:00:00Z","created_at":"2026-03-22T11:59:00Z","customer":{"external_id":"cust_retry_pending"}}}`))
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer lagoMock.Close()

	lagoTransport, err := service.NewLagoHTTPTransport(service.LagoClientConfig{
		BaseURL: lagoMock.URL,
		APIKey:  "test-api-key",
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new lago transport: %v", err)
	}

	lagoWebhookSvc := service.NewLagoWebhookService(repo, service.NoopLagoWebhookVerifier{}, nil, nil)

	ts := httptest.NewServer(api.NewServer(
		repo,
		api.WithAPIKeyAuthorizer(authorizer),
		api.WithInvoiceBillingAdapter(service.NewLagoInvoiceAdapter(lagoTransport)),
		api.WithLagoWebhookService(lagoWebhookSvc),
	).Handler())
	defer ts.Close()

	retryResp := postJSON(t, ts.URL+"/v1/invoices/inv_pending_1/retry-payment", map[string]any{}, "tenant-a-writer", http.StatusOK)
	retryInvoice, ok := retryResp["invoice"].(map[string]any)
	if !ok {
		t.Fatalf("expected retry response to include invoice object")
	}
	if got, _ := retryInvoice["payment_status"].(string); got != "pending" {
		t.Fatalf("expected retry payment_status pending, got %q", got)
	}
	if retryCalls != 1 {
		t.Fatalf("expected exactly one retry call to lago, got %d", retryCalls)
	}
	if getInvoiceCalls != 1 {
		t.Fatalf("expected exactly one get-invoice call to lago, got %d", getInvoiceCalls)
	}

	paymentResp := getJSON(t, ts.URL+"/v1/payments/inv_pending_1", "tenant-a-reader", http.StatusOK)
	if got, _ := paymentResp["payment_status"].(string); got != "pending" {
		t.Fatalf("expected payment_status pending, got %q", got)
	}
	lifecycle, ok := paymentResp["lifecycle"].(map[string]any)
	if !ok {
		t.Fatalf("expected lifecycle in payment response")
	}
	if got, _ := lifecycle["recommended_action"].(string); got != "collect_payment" {
		t.Fatalf("expected collect_payment recommended_action, got %q", got)
	}
	if got, _ := lifecycle["payment_status"].(string); got != "pending" {
		t.Fatalf("expected lifecycle payment_status pending, got %q", got)
	}
	dunning, ok := paymentResp["dunning"].(map[string]any)
	if !ok {
		t.Fatalf("expected dunning summary in payment response")
	}
	if got, _ := dunning["state"].(string); got != string(domain.DunningRunStateAwaitingPaymentSetup) {
		t.Fatalf("expected dunning state awaiting_payment_setup, got %q", got)
	}
}

func envInt64OrDefault(t *testing.T, key string, defaultValue int64) int64 {
	t.Helper()

	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultValue
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		t.Fatalf("invalid %s=%q: %v", key, raw, err)
	}
	return parsed
}
