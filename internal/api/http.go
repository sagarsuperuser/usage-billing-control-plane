package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"lago-usage-billing-alpha/internal/domain"
	"lago-usage-billing-alpha/internal/reconcile"
	"lago-usage-billing-alpha/internal/replay"
	"lago-usage-billing-alpha/internal/service"
	"lago-usage-billing-alpha/internal/store"
)

type Server struct {
	ratingService  *service.RatingService
	meterService   *service.MeterService
	invoiceService *service.InvoiceService
	usageService   *service.UsageService
	replayService  *replay.Service
	recService     *reconcile.Service
	metricsFn      func() map[string]any
	mux            *http.ServeMux
}

type ServerOption func(*Server)

func WithMetricsProvider(provider func() map[string]any) ServerOption {
	return func(s *Server) {
		s.metricsFn = provider
	}
}

func NewServer(repo store.Repository, opts ...ServerOption) *Server {
	s := &Server{
		ratingService:  service.NewRatingService(repo),
		meterService:   service.NewMeterService(repo),
		invoiceService: service.NewInvoiceService(repo),
		usageService:   service.NewUsageService(repo),
		replayService:  replay.NewService(repo),
		recService:     reconcile.NewService(repo),
		mux:            http.NewServeMux(),
	}
	for _, opt := range opts {
		opt(s)
	}
	s.registerRoutes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("/health", s.handleHealth)

	s.mux.HandleFunc("/v1/rating-rules", s.handleRatingRules)
	s.mux.HandleFunc("/v1/rating-rules/", s.handleRatingRuleByID)

	s.mux.HandleFunc("/v1/meters", s.handleMeters)
	s.mux.HandleFunc("/v1/meters/", s.handleMeterByID)

	s.mux.HandleFunc("/v1/invoices/preview", s.handleInvoicePreview)

	s.mux.HandleFunc("/v1/usage-events", s.handleUsageEvents)
	s.mux.HandleFunc("/v1/billed-entries", s.handleBilledEntries)

	s.mux.HandleFunc("/v1/replay-jobs", s.handleReplayJobs)
	s.mux.HandleFunc("/v1/replay-jobs/", s.handleReplayJobByID)

	s.mux.HandleFunc("/v1/reconciliation-report", s.handleReconciliationReport)
	s.mux.HandleFunc("/internal/metrics", s.handleInternalMetrics)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleRatingRules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var req domain.RatingRuleVersion
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		rule, err := s.ratingService.CreateRuleVersion(req)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, rule)
	case http.MethodGet:
		rules, err := s.ratingService.ListRuleVersions()
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, rules)
	default:
		writeMethodNotAllowed(w)
	}
}

func (s *Server) handleRatingRuleByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/v1/rating-rules/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	rule, err := s.ratingService.GetRuleVersion(id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

func (s *Server) handleMeters(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var req domain.Meter
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		meter, err := s.meterService.CreateMeter(req)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, meter)
	case http.MethodGet:
		meters, err := s.meterService.ListMeters()
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, meters)
	default:
		writeMethodNotAllowed(w)
	}
}

func (s *Server) handleMeterByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/v1/meters/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	switch r.Method {
	case http.MethodPut:
		var req domain.Meter
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		meter, err := s.meterService.UpdateMeter(id, req)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, meter)
	case http.MethodGet:
		meter, err := s.meterService.GetMeter(id)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, meter)
	default:
		writeMethodNotAllowed(w)
	}
}

func (s *Server) handleInvoicePreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	var req domain.InvoicePreviewRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	preview, err := s.invoiceService.Preview(req)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (s *Server) handleUsageEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	var req domain.UsageEvent
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	event, err := s.usageService.CreateUsageEvent(req)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, event)
}

func (s *Server) handleBilledEntries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	var req domain.BilledEntry
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	entry, err := s.usageService.CreateBilledEntry(req)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, entry)
}

func (s *Server) handleReplayJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	var req replay.CreateReplayJobRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

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
		"job":               job,
	})
}

func (s *Server) handleReplayJobByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/v1/replay-jobs/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	job, err := s.replayService.GetJob(id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) handleReconciliationReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	filter, err := parseFilter(r)
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

func (s *Server) handleInternalMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	if s.metricsFn == nil {
		writeError(w, http.StatusNotFound, "metrics not configured")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"generated_at": time.Now().UTC(),
		"metrics":      s.metricsFn(),
	})
}

func parseFilter(r *http.Request) (reconcile.Filter, error) {
	filter := reconcile.Filter{CustomerID: strings.TrimSpace(r.URL.Query().Get("customer_id"))}

	fromStr := strings.TrimSpace(r.URL.Query().Get("from"))
	toStr := strings.TrimSpace(r.URL.Query().Get("to"))

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

func parseTime(v string) (time.Time, error) {
	if unixSec, err := strconv.ParseInt(v, 10, 64); err == nil {
		return time.Unix(unixSec, 0).UTC(), nil
	}
	return time.Parse(time.RFC3339, v)
}

func decodeJSON(r *http.Request, target any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeMethodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func writeDomainError(w http.ResponseWriter, err error) {
	if err == nil {
		writeError(w, http.StatusInternalServerError, "unknown error")
		return
	}
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if strings.Contains(strings.ToLower(err.Error()), "not found") {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if errors.Is(err, store.ErrDuplicateKey) {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if strings.Contains(strings.ToLower(err.Error()), "validation") || strings.Contains(strings.ToLower(err.Error()), "required") || strings.Contains(strings.ToLower(err.Error()), "invalid") {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error())
}
