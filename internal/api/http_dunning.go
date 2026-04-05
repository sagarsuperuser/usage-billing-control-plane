package api

import (
	"net/http"
	"strings"

	"usage-billing-control-plane/internal/service"
)

func (s *Server) getDunningPolicy(w http.ResponseWriter, r *http.Request) {
	if s.dunningService == nil {
		writeError(w, http.StatusServiceUnavailable, "dunning service is required")
		return
	}
	policy, err := s.dunningService.GetPolicy(requestTenantID(r))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func (s *Server) putDunningPolicy(w http.ResponseWriter, r *http.Request) {
	if s.dunningService == nil {
		writeError(w, http.StatusServiceUnavailable, "dunning service is required")
		return
	}
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
}

func (s *Server) listDunningRuns(w http.ResponseWriter, r *http.Request) {
	if s.dunningService == nil {
		writeError(w, http.StatusServiceUnavailable, "dunning service is required")
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

func (s *Server) getDunningRun(w http.ResponseWriter, r *http.Request) {
	if s.dunningService == nil {
		writeError(w, http.StatusServiceUnavailable, "dunning service is required")
		return
	}
	runID := urlParam(r, "id")
	if runID == "" {
		writeError(w, http.StatusBadRequest, "run id is required")
		return
	}
	detail, err := s.dunningService.GetRunDetail(requestTenantID(r), runID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) collectPaymentReminder(w http.ResponseWriter, r *http.Request) {
	if s.dunningService == nil {
		writeError(w, http.StatusServiceUnavailable, "dunning service is required")
		return
	}
	runID := urlParam(r, "id")
	if runID == "" {
		writeError(w, http.StatusBadRequest, "run id is required")
		return
	}
	result, err := s.dunningService.DispatchCollectPaymentReminder(requestTenantID(r), runID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) retryDunningPayment(w http.ResponseWriter, r *http.Request) {
	if s.dunningService == nil {
		writeError(w, http.StatusServiceUnavailable, "dunning service is required")
		return
	}
	runID := urlParam(r, "id")
	if runID == "" {
		writeError(w, http.StatusBadRequest, "run id is required")
		return
	}
	result, err := s.dunningService.DispatchRetryPayment(requestTenantID(r), runID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) pauseDunningRun(w http.ResponseWriter, r *http.Request) {
	if s.dunningService == nil {
		writeError(w, http.StatusServiceUnavailable, "dunning service is required")
		return
	}
	runID := urlParam(r, "id")
	if runID == "" {
		writeError(w, http.StatusBadRequest, "run id is required")
		return
	}
	result, err := s.dunningService.PauseRun(requestTenantID(r), runID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) resumeDunningRun(w http.ResponseWriter, r *http.Request) {
	if s.dunningService == nil {
		writeError(w, http.StatusServiceUnavailable, "dunning service is required")
		return
	}
	runID := urlParam(r, "id")
	if runID == "" {
		writeError(w, http.StatusBadRequest, "run id is required")
		return
	}
	result, err := s.dunningService.ResumeRun(requestTenantID(r), runID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) resolveDunningRun(w http.ResponseWriter, r *http.Request) {
	if s.dunningService == nil {
		writeError(w, http.StatusServiceUnavailable, "dunning service is required")
		return
	}
	runID := urlParam(r, "id")
	if runID == "" {
		writeError(w, http.StatusBadRequest, "run id is required")
		return
	}
	result, err := s.dunningService.ResolveRun(requestTenantID(r), runID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
