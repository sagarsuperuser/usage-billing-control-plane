package api

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/reconcile"
	"usage-billing-control-plane/internal/replay"
	"usage-billing-control-plane/internal/service"
)

func (s *Server) createUsageEvent(w http.ResponseWriter, r *http.Request) {
	var req domain.UsageEvent
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.TenantID = requestTenantID(r)

	event, idempotent, err := s.usageService.CreateUsageEventWithIdempotency(req)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	status := http.StatusCreated
	if idempotent {
		status = http.StatusOK
	}
	writeJSON(w, status, event)
}

func (s *Server) listUsageEvents(w http.ResponseWriter, r *http.Request) {
	limit, err := parseQueryInt(r, "limit")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	offset, err := parseQueryInt(r, "offset")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	from, err := parseOptionalTime(r.URL.Query().Get("from"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid from: "+err.Error())
		return
	}
	to, err := parseOptionalTime(r.URL.Query().Get("to"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid to: "+err.Error())
		return
	}

	events, err := s.usageService.ListUsageEvents(requestTenantID(r), service.ListUsageEventsRequest{
		CustomerID: r.URL.Query().Get("customer_id"),
		MeterID:    r.URL.Query().Get("meter_id"),
		Order:      r.URL.Query().Get("order"),
		From:       from,
		To:         to,
		Limit:      limit,
		Offset:     offset,
		Cursor:     r.URL.Query().Get("cursor"),
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, events)
}

func (s *Server) createBilledEntry(w http.ResponseWriter, r *http.Request) {
	var req domain.BilledEntry
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.TenantID = requestTenantID(r)

	entry, idempotent, err := s.usageService.CreateBilledEntryWithIdempotency(req)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	status := http.StatusCreated
	if idempotent {
		status = http.StatusOK
	}
	writeJSON(w, status, entry)
}

func (s *Server) listBilledEntries(w http.ResponseWriter, r *http.Request) {
	limit, err := parseQueryInt(r, "limit")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	offset, err := parseQueryInt(r, "offset")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	from, err := parseOptionalTime(r.URL.Query().Get("from"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid from: "+err.Error())
		return
	}
	to, err := parseOptionalTime(r.URL.Query().Get("to"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid to: "+err.Error())
		return
	}

	entries, err := s.usageService.ListBilledEntries(requestTenantID(r), service.ListBilledEntriesRequest{
		CustomerID:        r.URL.Query().Get("customer_id"),
		MeterID:           r.URL.Query().Get("meter_id"),
		BilledSource:      r.URL.Query().Get("billed_source"),
		BilledReplayJobID: r.URL.Query().Get("billed_replay_job_id"),
		Order:             r.URL.Query().Get("order"),
		From:              from,
		To:                to,
		Limit:             limit,
		Offset:            offset,
		Cursor:            r.URL.Query().Get("cursor"),
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

func (s *Server) createReplayJob(w http.ResponseWriter, r *http.Request) {
	var req replay.CreateReplayJobRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.TenantID = requestTenantID(r)

	job, idempotent, err := s.replayService.CreateJob(req)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	status := http.StatusCreated
	if idempotent {
		status = http.StatusOK
	}
	writeJSON(w, status, map[string]any{
		"idempotent_replay": idempotent,
		"job":               s.decorateReplayJob(r, job),
	})
}

func (s *Server) listReplayJobs(w http.ResponseWriter, r *http.Request) {
	limit, err := parseQueryInt(r, "limit")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	offset, err := parseQueryInt(r, "offset")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	jobs, err := s.replayService.ListJobs(requestTenantID(r), replay.ListReplayJobsRequest{
		CustomerID: r.URL.Query().Get("customer_id"),
		MeterID:    r.URL.Query().Get("meter_id"),
		Status:     r.URL.Query().Get("status"),
		Limit:      limit,
		Offset:     offset,
		Cursor:     r.URL.Query().Get("cursor"),
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(jobs.Items))
	for _, job := range jobs.Items {
		items = append(items, s.decorateReplayJob(r, job))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":       items,
		"total":       jobs.Total,
		"limit":       jobs.Limit,
		"offset":      jobs.Offset,
		"next_cursor": jobs.NextCursor,
	})
}

func (s *Server) getReplayJob(w http.ResponseWriter, r *http.Request) {
	id := urlParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	job, err := s.replayService.GetJob(requestTenantID(r), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.decorateReplayJob(r, job))
}

func (s *Server) getReplayJobEvents(w http.ResponseWriter, r *http.Request) {
	id := urlParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	diag, err := s.replayService.GetJobDiagnostics(requestTenantID(r), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, diag)
}

func (s *Server) getReplayJobArtifact(w http.ResponseWriter, r *http.Request) {
	id := urlParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	artifactName := urlParam(r, "artifactName")
	if artifactName == "" {
		writeError(w, http.StatusBadRequest, "artifact name is required")
		return
	}
	s.handleReplayJobArtifact(w, r, id, artifactName)
}

func (s *Server) retryReplayJob(w http.ResponseWriter, r *http.Request) {
	id := urlParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	job, err := s.replayService.RetryJob(requestTenantID(r), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.decorateReplayJob(r, job))
}

func (s *Server) handleReplayJobArtifact(w http.ResponseWriter, r *http.Request, jobID, artifactName string) {
	tenantID := requestTenantID(r)
	diag, err := s.replayService.GetJobDiagnostics(tenantID, jobID)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	artifactName = strings.ToLower(strings.TrimSpace(artifactName))
	switch artifactName {
	case "report.json":
		payload := map[string]any{
			"job_id":               diag.Job.ID,
			"tenant_id":            diag.Job.TenantID,
			"status":               diag.Job.Status,
			"customer_id":          diag.Job.CustomerID,
			"meter_id":             diag.Job.MeterID,
			"from":                 diag.Job.From,
			"to":                   diag.Job.To,
			"processed_records":    diag.Job.ProcessedRecords,
			"attempt_count":        diag.Job.AttemptCount,
			"usage_events_count":   diag.UsageEventsCount,
			"usage_quantity":       diag.UsageQuantity,
			"billed_entries_count": diag.BilledEntriesCount,
			"billed_amount_cents":  diag.BilledAmountCents,
			"error":                diag.Job.Error,
			"generated_at":         time.Now().UTC(),
		}
		writeJSON(w, http.StatusOK, payload)
	case "report.csv":
		body, buildErr := replayDiagnosticsCSV(diag)
		if buildErr != nil {
			writeError(w, http.StatusInternalServerError, "failed to generate replay csv artifact")
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=replay_%s_report.csv", jobID))
		_, _ = w.Write([]byte(body))
	case "dataset_digest.txt":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=replay_%s_dataset_digest.txt", jobID))
		_, _ = w.Write([]byte(replayDatasetDigest(diag) + "\n"))
	default:
		writeError(w, http.StatusNotFound, "artifact not found")
	}
}

func (s *Server) decorateReplayJob(r *http.Request, job domain.ReplayJob) map[string]any {
	out := make(map[string]any)
	encoded, err := json.Marshal(job)
	if err != nil {
		out["id"] = job.ID
		out["status"] = job.Status
	} else {
		_ = json.Unmarshal(encoded, &out)
	}
	out["workflow_telemetry"] = replayWorkflowTelemetry(job)
	out["artifact_links"] = replayArtifactLinks(r, job.ID)
	return out
}

func replayWorkflowTelemetry(job domain.ReplayJob) map[string]any {
	currentStep := "queued"
	progressPercent := 0

	switch job.Status {
	case domain.ReplayQueued:
		currentStep = "queued"
		progressPercent = 0
	case domain.ReplayRunning:
		currentStep = "replay_processing"
		progressPercent = 50
	case domain.ReplayDone:
		currentStep = "completed"
		progressPercent = 100
	case domain.ReplayFailed:
		currentStep = "failed"
		progressPercent = 100
	}

	updatedAt := job.CreatedAt
	if job.CompletedAt != nil {
		updatedAt = job.CompletedAt.UTC()
	} else if job.StartedAt != nil {
		updatedAt = job.StartedAt.UTC()
	}

	return map[string]any{
		"current_step":      currentStep,
		"progress_percent":  progressPercent,
		"attempt_count":     job.AttemptCount,
		"last_attempt_at":   job.LastAttemptAt,
		"processed_records": job.ProcessedRecords,
		"updated_at":        updatedAt,
	}
}

func replayArtifactLinks(r *http.Request, jobID string) map[string]string {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return map[string]string{}
	}

	return map[string]string{
		"report_json":    replayArtifactURL(r, jobID, "report.json"),
		"report_csv":     replayArtifactURL(r, jobID, "report.csv"),
		"dataset_digest": replayArtifactURL(r, jobID, "dataset_digest.txt"),
	}
}

func replayArtifactURL(r *http.Request, jobID, artifact string) string {
	base := externalBaseURL(r)
	escapedID := url.PathEscape(strings.TrimSpace(jobID))
	escapedArtifact := url.PathEscape(strings.TrimSpace(artifact))
	return fmt.Sprintf("%s/v1/replay-jobs/%s/artifacts/%s", base, escapedID, escapedArtifact)
}

func replayDiagnosticsCSV(diag replay.ReplayJobDiagnostics) (string, error) {
	var b strings.Builder
	writer := csv.NewWriter(&b)
	header := []string{
		"job_id",
		"status",
		"customer_id",
		"meter_id",
		"from",
		"to",
		"processed_records",
		"attempt_count",
		"usage_events_count",
		"usage_quantity",
		"billed_entries_count",
		"billed_amount_cents",
		"error",
	}
	if err := writer.Write(header); err != nil {
		return "", err
	}

	row := []string{
		diag.Job.ID,
		string(diag.Job.Status),
		diag.Job.CustomerID,
		diag.Job.MeterID,
		formatOptionalTime(diag.Job.From),
		formatOptionalTime(diag.Job.To),
		strconv.FormatInt(diag.Job.ProcessedRecords, 10),
		strconv.Itoa(diag.Job.AttemptCount),
		strconv.Itoa(diag.UsageEventsCount),
		strconv.FormatInt(diag.UsageQuantity, 10),
		strconv.Itoa(diag.BilledEntriesCount),
		strconv.FormatInt(diag.BilledAmountCents, 10),
		diag.Job.Error,
	}
	if err := writer.Write(row); err != nil {
		return "", err
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}
	return b.String(), nil
}

func replayDatasetDigest(diag replay.ReplayJobDiagnostics) string {
	payload := struct {
		JobID              string    `json:"job_id"`
		TenantID           string    `json:"tenant_id"`
		CustomerID         string    `json:"customer_id"`
		MeterID            string    `json:"meter_id"`
		From               string    `json:"from,omitempty"`
		To                 string    `json:"to,omitempty"`
		ProcessedRecords   int64     `json:"processed_records"`
		AttemptCount       int       `json:"attempt_count"`
		UsageEventsCount   int       `json:"usage_events_count"`
		UsageQuantity      int64     `json:"usage_quantity"`
		BilledEntriesCount int       `json:"billed_entries_count"`
		BilledAmountCents  int64     `json:"billed_amount_cents"`
		Status             string    `json:"status"`
		CompletedAt        time.Time `json:"completed_at,omitempty"`
	}{
		JobID:              diag.Job.ID,
		TenantID:           diag.Job.TenantID,
		CustomerID:         diag.Job.CustomerID,
		MeterID:            diag.Job.MeterID,
		From:               formatOptionalTime(diag.Job.From),
		To:                 formatOptionalTime(diag.Job.To),
		ProcessedRecords:   diag.Job.ProcessedRecords,
		AttemptCount:       diag.Job.AttemptCount,
		UsageEventsCount:   diag.UsageEventsCount,
		UsageQuantity:      diag.UsageQuantity,
		BilledEntriesCount: diag.BilledEntriesCount,
		BilledAmountCents:  diag.BilledAmountCents,
		Status:             string(diag.Job.Status),
	}
	if diag.Job.CompletedAt != nil {
		payload.CompletedAt = diag.Job.CompletedAt.UTC()
	}
	encoded, _ := json.Marshal(payload)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func (s *Server) handleReconciliationReport(w http.ResponseWriter, r *http.Request) {
	filter, err := parseFilter(r, requestTenantID(r))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	report, err := s.recService.GenerateReport(filter)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	format := strings.ToLower(r.URL.Query().Get("format"))
	if format == "csv" {
		csvData, err := s.recService.GenerateCSV(report)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename=reconciliation_report.csv")
		_, _ = w.Write([]byte(csvData))
		return
	}

	writeJSON(w, http.StatusOK, report)
}

func parseFilter(r *http.Request, tenantID string) (reconcile.Filter, error) {
	filter := reconcile.Filter{
		TenantID:   normalizeTenantID(tenantID),
		CustomerID: strings.TrimSpace(r.URL.Query().Get("customer_id")),
	}
	filter.BilledReplayJobID = strings.TrimSpace(r.URL.Query().Get("billed_replay_job_id"))

	rawBilledSource := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("billed_source")))
	if rawBilledSource != "" {
		billedSource := domain.BilledEntrySource(rawBilledSource)
		switch billedSource {
		case domain.BilledEntrySourceAPI, domain.BilledEntrySourceReplayAdjustment:
			filter.BilledSource = billedSource
		default:
			return reconcile.Filter{}, fmt.Errorf("invalid billed_source: must be api or replay_adjustment")
		}
	}

	fromStr := strings.TrimSpace(r.URL.Query().Get("from"))
	toStr := strings.TrimSpace(r.URL.Query().Get("to"))
	absDeltaGTERaw := strings.TrimSpace(r.URL.Query().Get("abs_delta_gte"))
	if absDeltaGTERaw != "" {
		v, err := strconv.ParseInt(absDeltaGTERaw, 10, 64)
		if err != nil {
			return reconcile.Filter{}, fmt.Errorf("invalid abs_delta_gte: must be integer")
		}
		if v < 0 {
			return reconcile.Filter{}, fmt.Errorf("invalid abs_delta_gte: must be >= 0")
		}
		filter.AbsDeltaGTE = v
	}
	mismatchOnly, err := parseQueryBool(r, "mismatch_only")
	if err != nil {
		return reconcile.Filter{}, err
	}
	filter.MismatchOnly = mismatchOnly

	if fromStr != "" {
		from, err := parseTime(fromStr)
		if err != nil {
			return reconcile.Filter{}, fmt.Errorf("invalid from: %w", err)
		}
		filter.From = &from
	}
	if toStr != "" {
		to, err := parseTime(toStr)
		if err != nil {
			return reconcile.Filter{}, fmt.Errorf("invalid to: %w", err)
		}
		filter.To = &to
	}
	if filter.From != nil && filter.To != nil && filter.From.After(*filter.To) {
		return reconcile.Filter{}, fmt.Errorf("from must be <= to")
	}
	return filter, nil
}

func (s *Server) createRatingRule(w http.ResponseWriter, r *http.Request) {
	tenantID := requestTenantID(r)
	var req domain.RatingRuleVersion
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.TenantID = tenantID
	rule, err := s.ratingService.CreateRuleVersion(req)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, rule)
}

func (s *Server) listRatingRules(w http.ResponseWriter, r *http.Request) {
	tenantID := requestTenantID(r)
	latestOnly, err := parseQueryBool(r, "latest_only")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	allRules, err := s.ratingService.ListRuleVersions(tenantID, service.ListRuleVersionsRequest{
		RuleKey:        r.URL.Query().Get("rule_key"),
		LifecycleState: r.URL.Query().Get("lifecycle_state"),
		LatestOnly:     latestOnly,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	rules := make([]domain.RatingRuleVersion, 0, len(allRules))
	for _, rule := range allRules {
		rules = append(rules, rule)
	}
	writeJSON(w, http.StatusOK, rules)
}

func (s *Server) getRatingRule(w http.ResponseWriter, r *http.Request) {
	id := urlParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	rule, err := s.ratingService.GetRuleVersion(requestTenantID(r), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

func (s *Server) createMeter(w http.ResponseWriter, r *http.Request) {
	if s.meterSyncAdapter == nil {
		writeError(w, http.StatusServiceUnavailable, "Pricing updates are unavailable right now.")
		return
	}
	tenantID := requestTenantID(r)
	var req domain.Meter
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.TenantID = tenantID
	meter, err := s.meterService.CreateMeter(req)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if err := s.meterSyncAdapter.SyncMeter(r.Context(), meter); err != nil {
		writeError(w, http.StatusBadGateway, "Pricing metric changes could not be applied right now.")
		return
	}
	writeJSON(w, http.StatusCreated, meter)
}

func (s *Server) listMeters(w http.ResponseWriter, r *http.Request) {
	if s.meterSyncAdapter == nil {
		writeError(w, http.StatusServiceUnavailable, "Pricing updates are unavailable right now.")
		return
	}
	tenantID := requestTenantID(r)
	allMeters, err := s.meterService.ListMeters(tenantID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	meters := make([]domain.Meter, 0, len(allMeters))
	for _, meter := range allMeters {
		meters = append(meters, meter)
	}
	writeJSON(w, http.StatusOK, meters)
}

func (s *Server) getMeter(w http.ResponseWriter, r *http.Request) {
	if s.meterSyncAdapter == nil {
		writeError(w, http.StatusServiceUnavailable, "Pricing updates are unavailable right now.")
		return
	}
	tenantID := requestTenantID(r)
	id := urlParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	meter, err := s.meterService.GetMeter(tenantID, id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, meter)
}

func (s *Server) updateMeter(w http.ResponseWriter, r *http.Request) {
	if s.meterSyncAdapter == nil {
		writeError(w, http.StatusServiceUnavailable, "Pricing updates are unavailable right now.")
		return
	}
	tenantID := requestTenantID(r)
	id := urlParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	var req domain.Meter
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.TenantID = tenantID
	meter, err := s.meterService.UpdateMeter(tenantID, id, req)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if err := s.meterSyncAdapter.SyncMeter(r.Context(), meter); err != nil {
		writeError(w, http.StatusBadGateway, "Pricing metric changes could not be applied right now.")
		return
	}
	writeJSON(w, http.StatusOK, meter)
}

func (s *Server) handleInvoicePreview(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	tenantID := normalizeTenantID(principal.TenantID)

	subscriptionID := strings.TrimSpace(r.URL.Query().Get("subscription_id"))
	if subscriptionID == "" {
		writeError(w, http.StatusBadRequest, "subscription_id is required")
		return
	}

	if s.invoiceGenerationService == nil {
		writeError(w, http.StatusServiceUnavailable, "invoice generation is not configured")
		return
	}

	result, err := s.invoiceGenerationService.Preview(r.Context(), tenantID, subscriptionID)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	type lineItemResponse struct {
		LineType         string  `json:"line_type"`
		Description      string  `json:"description"`
		Quantity         int64   `json:"quantity"`
		UnitAmountCents  int64   `json:"unit_amount_cents"`
		AmountCents      int64   `json:"amount_cents"`
		TotalAmountCents int64   `json:"total_amount_cents"`
		TaxRate          float64 `json:"tax_rate,omitempty"`
	}

	items := make([]lineItemResponse, 0, len(result.LineItems))
	for _, li := range result.LineItems {
		items = append(items, lineItemResponse{
			LineType:         string(li.LineType),
			Description:      li.Description,
			Quantity:         li.Quantity,
			UnitAmountCents:  li.UnitAmountCents,
			AmountCents:      li.AmountCents,
			TotalAmountCents: li.TotalAmountCents,
			TaxRate:          li.TaxRate,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"preview":            true,
		"subscription_id":    subscriptionID,
		"customer_id":        result.Invoice.CustomerID,
		"currency":           result.Invoice.Currency,
		"subtotal_cents":     result.Invoice.SubtotalCents,
		"discount_cents":     result.Invoice.DiscountCents,
		"tax_amount_cents":   result.Invoice.TaxAmountCents,
		"total_amount_cents": result.Invoice.TotalAmountCents,
		"billing_period_start": result.Invoice.BillingPeriodStart,
		"billing_period_end":   result.Invoice.BillingPeriodEnd,
		"due_at":               result.Invoice.DueAt,
		"line_items":           items,
		"generated_at":         time.Now().UTC(),
	})
}

func (s *Server) getInvoice(w http.ResponseWriter, r *http.Request) {
	id := urlParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "invoice id is required")
		return
	}
	if s.invoiceBillingAdapter == nil {
		writeError(w, http.StatusServiceUnavailable, "invoice billing adapter is required")
		return
	}
	statusCode, body, detail, err := s.loadInvoiceDetail(r.Context(), requestTenantID(r), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if statusCode < 200 || statusCode >= 300 {
		writeTranslatedUpstreamError(w, statusCode, "Invoice details could not be loaded right now.", body)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) getInvoicePaymentReceipts(w http.ResponseWriter, r *http.Request) {
	id := urlParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "invoice id is required")
		return
	}
	s.handleInvoicePaymentReceipts(w, r, id)
}

func (s *Server) getInvoiceCreditNotes(w http.ResponseWriter, r *http.Request) {
	id := urlParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "invoice id is required")
		return
	}
	s.handleInvoiceCreditNotes(w, r, id)
}

func (s *Server) retryInvoicePayment(w http.ResponseWriter, r *http.Request) {
	id := urlParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "invoice id is required")
		return
	}
	if s.invoiceBillingAdapter == nil {
		writeError(w, http.StatusServiceUnavailable, "invoice billing adapter is required")
		return
	}

	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(strings.TrimSpace(string(rawBody))) == 0 {
		rawBody = []byte("{}")
	}

	ctx := service.ContextWithBillingTenant(r.Context(), requestTenantID(r))
	statusCode, body, err := s.invoiceBillingAdapter.RetryInvoicePayment(ctx, id, rawBody)
	if err != nil {
		s.writeInternalError(w, r, http.StatusBadGateway, "payment retry failed", err)
		return
	}
	if statusCode >= 200 && statusCode < 300 {
		if syncErr := s.materializeRetryPaymentProjection(r.Context(), requestTenantID(r), id); syncErr != nil && s.logger != nil {
			s.logger.Warn("materialize retry payment projection failed", "invoice_id", id, "tenant_id", requestTenantID(r), "error", syncErr)
		}
	}
	if statusCode < 200 || statusCode >= 300 {
		writeTranslatedUpstreamError(w, statusCode, "Payment retry could not be started right now.", body)
		return
	}
	writeJSONRaw(w, statusCode, body)
}

func (s *Server) resendInvoiceEmail(w http.ResponseWriter, r *http.Request) {
	id := urlParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "invoice id is required")
		return
	}
	s.handleInvoiceResendEmail(w, r, id)
}

func (s *Server) getInvoiceExplainability(w http.ResponseWriter, r *http.Request) {
	id := urlParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "invoice id is required")
		return
	}
	if s.invoiceBillingAdapter == nil {
		writeError(w, http.StatusServiceUnavailable, "invoice billing adapter is required")
		return
	}

	feeTypes := make([]string, 0, 8)
	feeTypes = append(feeTypes, splitCommaSeparatedValues(r.URL.Query().Get("fee_types"))...)
	feeTypes = append(feeTypes, r.URL.Query()["fee_type"]...)
	lineItemSort := r.URL.Query().Get("line_item_sort")
	page, err := parseQueryInt(r, "page")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	limit, err := parseQueryInt(r, "limit")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	options, err := service.NewInvoiceExplainabilityOptions(feeTypes, lineItemSort, page, limit)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	ctx := service.ContextWithBillingTenant(r.Context(), requestTenantID(r))
	statusCode, body, err := s.invoiceBillingAdapter.GetInvoice(ctx, id)
	if err != nil {
		s.writeInternalError(w, r, http.StatusBadGateway, "failed to fetch invoice", err)
		return
	}
	if statusCode < 200 || statusCode >= 300 {
		writeTranslatedUpstreamError(w, statusCode, "Invoice explainability is unavailable right now.", body)
		return
	}

	explainability, err := service.BuildInvoiceExplainability(body, options)
	if err != nil {
		s.writeInternalError(w, r, http.StatusBadGateway, "failed to compute invoice explainability", err)
		return
	}
	writeJSON(w, http.StatusOK, explainability)
}

func (s *Server) handleInvoicePaymentStatuses(w http.ResponseWriter, r *http.Request) {
	if s.paymentStatusSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "payment status service is required")
		return
	}

	limit, err := parseQueryInt(r, "limit")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	offset, err := parseQueryInt(r, "offset")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	paymentOverdue, err := parseOptionalQueryBool(r, "payment_overdue")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	items, err := s.paymentStatusSvc.ListInvoicePaymentStatusViews(
		requestTenantID(r),
		service.ListInvoicePaymentStatusViewsRequest{
			OrganizationID: r.URL.Query().Get("organization_id"),
			PaymentStatus:  r.URL.Query().Get("payment_status"),
			InvoiceStatus:  r.URL.Query().Get("invoice_status"),
			PaymentOverdue: paymentOverdue,
			SortBy:         r.URL.Query().Get("sort_by"),
			Order:          r.URL.Query().Get("order"),
			Limit:          limit,
			Offset:         offset,
		},
	)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":  items,
		"limit":  limit,
		"offset": offset,
		"filters": map[string]any{
			"organization_id": r.URL.Query().Get("organization_id"),
			"payment_status":  r.URL.Query().Get("payment_status"),
			"invoice_status":  r.URL.Query().Get("invoice_status"),
			"payment_overdue": paymentOverdue,
			"sort_by":         r.URL.Query().Get("sort_by"),
			"order":           r.URL.Query().Get("order"),
		},
	})
}

func (s *Server) getInvoicePaymentStatusSummary(w http.ResponseWriter, r *http.Request) {
	if s.paymentStatusSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "payment status service is required")
		return
	}
	staleAfterSec, err := parseQueryInt(r, "stale_after_sec")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	summary, err := s.paymentStatusSvc.GetInvoicePaymentStatusSummary(
		requestTenantID(r),
		service.GetInvoicePaymentStatusSummaryRequest{
			OrganizationID:    r.URL.Query().Get("organization_id"),
			StaleAfterSeconds: staleAfterSec,
		},
	)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) getInvoicePaymentStatus(w http.ResponseWriter, r *http.Request) {
	if s.paymentStatusSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "payment status service is required")
		return
	}
	invoiceID := urlParam(r, "id")
	if invoiceID == "" {
		writeError(w, http.StatusBadRequest, "invoice id is required")
		return
	}
	item, err := s.paymentStatusSvc.GetInvoicePaymentStatusView(requestTenantID(r), invoiceID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) getInvoicePaymentStatusEvents(w http.ResponseWriter, r *http.Request) {
	if s.paymentStatusSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "payment status service is required")
		return
	}
	invoiceID := urlParam(r, "id")
	if invoiceID == "" {
		writeError(w, http.StatusBadRequest, "invoice id is required")
		return
	}
	limit, err := parseQueryInt(r, "limit")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	offset, err := parseQueryInt(r, "offset")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	events, err := s.paymentStatusSvc.ListBillingEvents(
		requestTenantID(r),
		service.ListBillingEventsRequest{
			OrganizationID: r.URL.Query().Get("organization_id"),
			InvoiceID:      invoiceID,
			WebhookType:    r.URL.Query().Get("webhook_type"),
			SortBy:         r.URL.Query().Get("sort_by"),
			Order:          r.URL.Query().Get("order"),
			Limit:          limit,
			Offset:         offset,
		},
	)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":      events,
		"limit":      limit,
		"offset":     offset,
		"invoice_id": invoiceID,
	})
}

func (s *Server) getInvoicePaymentStatusLifecycle(w http.ResponseWriter, r *http.Request) {
	if s.paymentStatusSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "payment status service is required")
		return
	}
	invoiceID := urlParam(r, "id")
	if invoiceID == "" {
		writeError(w, http.StatusBadRequest, "invoice id is required")
		return
	}
	eventLimit, err := parseQueryInt(r, "event_limit")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	view, err := s.paymentStatusSvc.GetInvoicePaymentStatusView(requestTenantID(r), invoiceID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	lifecycle, err := s.paymentStatusSvc.GetInvoicePaymentLifecycle(requestTenantID(r), invoiceID, eventLimit)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	lifecycle, err = s.enrichPaymentLifecycleWithCustomerReadiness(requestTenantID(r), view.CustomerExternalID, lifecycle)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, lifecycle)
}

func (s *Server) handleStripeWebhooks(w http.ResponseWriter, r *http.Request) {
	if s.stripeWebhookSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "stripe webhook service is required")
		return
	}
	if s.stripeWebhookSecret == "" {
		writeError(w, http.StatusServiceUnavailable, "stripe webhook secret is not configured")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	sigHeader := r.Header.Get("Stripe-Signature")
	if sigHeader == "" {
		writeError(w, http.StatusBadRequest, "Stripe-Signature header is required")
		return
	}

	// Tenant ID is extracted from PaymentIntent metadata by the webhook service
	// after signature verification. For events without metadata (e.g. customer.*),
	// we fall back to X-Tenant-ID header if provided.
	tenantID := r.Header.Get("X-Tenant-ID")

	result, err := s.stripeWebhookSvc.Ingest(r.Context(), body, sigHeader, s.stripeWebhookSecret, tenantID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	status := http.StatusAccepted
	if result.Idempotent {
		status = http.StatusOK
	}
	writeJSON(w, status, map[string]any{
		"idempotent": result.Idempotent,
		"event":      result.Event,
	})
}
