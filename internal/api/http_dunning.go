package api

import (
	"net/http"
	"strings"

	"usage-billing-control-plane/internal/service"
)

func (s *Server) handleDunningPolicy(w http.ResponseWriter, r *http.Request) {
	if s.dunningService == nil {
		writeError(w, http.StatusServiceUnavailable, "dunning service is required")
		return
	}
	switch r.Method {
	case http.MethodGet:
		policy, err := s.dunningService.GetPolicy(requestTenantID(r))
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
	case http.MethodPut:
		var req service.PutDunningPolicyRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		policy, err := s.dunningService.PutPolicy(requestTenantID(r), req)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
	default:
		writeMethodNotAllowed(w)
	}
}

func (s *Server) handleDunningRuns(w http.ResponseWriter, r *http.Request) {
	if s.dunningService == nil {
		writeError(w, http.StatusServiceUnavailable, "dunning service is required")
		return
	}
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
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
	activeOnly, err := parseOptionalQueryBool(r, "active_only")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req := service.ListDunningRunsRequest{
		InvoiceID:          strings.TrimSpace(r.URL.Query().Get("invoice_id")),
		CustomerExternalID: strings.TrimSpace(r.URL.Query().Get("customer_external_id")),
		State:              strings.TrimSpace(r.URL.Query().Get("state")),
		Limit:              limit,
		Offset:             offset,
		ActiveOnly:         activeOnly == nil || *activeOnly,
	}
	items, err := s.dunningService.ListRuns(requestTenantID(r), req)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":  items,
		"limit":  req.Limit,
		"offset": req.Offset,
		"filters": map[string]any{
			"invoice_id":           req.InvoiceID,
			"customer_external_id": req.CustomerExternalID,
			"state":                req.State,
			"active_only":          req.ActiveOnly,
		},
	})
}

func (s *Server) handleDunningRunByID(w http.ResponseWriter, r *http.Request) {
	if s.dunningService == nil {
		writeError(w, http.StatusServiceUnavailable, "dunning service is required")
		return
	}
	tail := strings.TrimPrefix(r.URL.Path, "/v1/dunning/runs/")
	parts := strings.Split(strings.Trim(tail, "/"), "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		writeError(w, http.StatusBadRequest, "run id is required")
		return
	}
	runID := strings.TrimSpace(parts[0])
	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w)
			return
		}
		detail, err := s.dunningService.GetRunDetail(requestTenantID(r), runID)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, detail)
		return
	}
	if len(parts) == 2 && strings.EqualFold(parts[1], "collect-payment-reminder") {
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w)
			return
		}
		result, err := s.dunningService.DispatchCollectPaymentReminder(requestTenantID(r), runID)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
		return
	}
	if len(parts) == 2 && strings.EqualFold(parts[1], "retry-now") {
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w)
			return
		}
		result, err := s.dunningService.DispatchRetryPayment(requestTenantID(r), runID)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
		return
	}
	if len(parts) == 2 && strings.EqualFold(parts[1], "pause") {
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w)
			return
		}
		result, err := s.dunningService.PauseRun(requestTenantID(r), runID)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
		return
	}
	if len(parts) == 2 && strings.EqualFold(parts[1], "resume") {
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w)
			return
		}
		result, err := s.dunningService.ResumeRun(requestTenantID(r), runID)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
		return
	}
	if len(parts) == 2 && strings.EqualFold(parts[1], "resolve") {
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w)
			return
		}
		result, err := s.dunningService.ResolveRun(requestTenantID(r), runID)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
		return
	}
	writeMethodNotAllowed(w)
}
