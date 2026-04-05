package api

import (
	"net/http"
	"strings"

	"usage-billing-control-plane/internal/service"
)

func (s *Server) listAPIKeys(w http.ResponseWriter, r *http.Request) {
	setCompatibilityDeprecationHeaders(w, "/v1/workspace/service-accounts")
	tenantID := requestTenantID(r)

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
	keys, err := s.apiKeyService.ListAPIKeys(tenantID, service.ListAPIKeysRequest{
		Role:         r.URL.Query().Get("role"),
		State:        r.URL.Query().Get("state"),
		NameContains: r.URL.Query().Get("name_contains"),
		Limit:        limit,
		Offset:       offset,
		Cursor:       r.URL.Query().Get("cursor"),
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, keys)
}

func (s *Server) createAPIKey(w http.ResponseWriter, r *http.Request) {
	setCompatibilityDeprecationHeaders(w, "/v1/workspace/service-accounts")
	tenantID := requestTenantID(r)
	actorAPIKeyID := requestActorAPIKeyID(r)

	var req service.CreateAPIKeyRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	created, err := s.apiKeyService.CreateAPIKey(tenantID, actorAPIKeyID, req)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) revokeAPIKey(w http.ResponseWriter, r *http.Request) {
	setCompatibilityDeprecationHeaders(w, "/v1/workspace/service-accounts")
	id := urlParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	tenantID := requestTenantID(r)
	actorAPIKeyID := requestActorAPIKeyID(r)
	key, err := s.apiKeyService.RevokeAPIKey(tenantID, actorAPIKeyID, id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, key)
}

func (s *Server) rotateAPIKey(w http.ResponseWriter, r *http.Request) {
	setCompatibilityDeprecationHeaders(w, "/v1/workspace/service-accounts")
	id := urlParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	tenantID := requestTenantID(r)
	actorAPIKeyID := requestActorAPIKeyID(r)
	result, err := s.apiKeyService.RotateAPIKey(tenantID, actorAPIKeyID, id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) listAPIKeyAuditEvents(w http.ResponseWriter, r *http.Request) {
	setCompatibilityDeprecationHeaders(w, "/v1/workspace/service-accounts/{id}/audit")

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
	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))

	req := service.ListAPIKeyAuditEventsRequest{
		APIKeyID:      r.URL.Query().Get("api_key_id"),
		ActorAPIKeyID: r.URL.Query().Get("actor_api_key_id"),
		Action:        r.URL.Query().Get("action"),
		OwnerType:     r.URL.Query().Get("owner_type"),
		OwnerID:       r.URL.Query().Get("owner_id"),
		Limit:         limit,
		Offset:        offset,
		Cursor:        r.URL.Query().Get("cursor"),
	}
	if format == "csv" {
		csvData, err := s.apiKeyService.GenerateAPIKeyAuditCSV(requestTenantID(r), req)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename=api_key_audit.csv")
		_, _ = w.Write([]byte(csvData))
		return
	}

	events, err := s.apiKeyService.ListAPIKeyAuditEvents(requestTenantID(r), req)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, events)
}

func (s *Server) listAPIKeyAuditExports(w http.ResponseWriter, r *http.Request) {
	setCompatibilityDeprecationHeaders(w, "/v1/workspace/service-accounts/{id}/audit/exports")
	if s.auditExportSvc == nil {
		writeError(w, http.StatusNotImplemented, "audit export service not configured")
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
	list, err := s.auditExportSvc.ListJobs(requestTenantID(r), service.ListAuditExportJobsRequest{
		Status:              r.URL.Query().Get("status"),
		RequestedByAPIKeyID: r.URL.Query().Get("requested_by_api_key_id"),
		OwnerType:           r.URL.Query().Get("owner_type"),
		OwnerID:             r.URL.Query().Get("owner_id"),
		Limit:               limit,
		Offset:              offset,
		Cursor:              r.URL.Query().Get("cursor"),
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) createAPIKeyAuditExport(w http.ResponseWriter, r *http.Request) {
	setCompatibilityDeprecationHeaders(w, "/v1/workspace/service-accounts/{id}/audit/exports")
	if s.auditExportSvc == nil {
		writeError(w, http.StatusNotImplemented, "audit export service not configured")
		return
	}
	var req service.CreateAuditExportJobRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	job, idempotent, err := s.auditExportSvc.CreateJob(requestTenantID(r), requestActorAPIKeyID(r), req)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	status := http.StatusCreated
	if idempotent {
		status = http.StatusOK
	}
	writeJSON(w, status, map[string]any{
		"idempotent_request": idempotent,
		"job":                job,
	})
}

func (s *Server) getAPIKeyAuditExport(w http.ResponseWriter, r *http.Request) {
	setCompatibilityDeprecationHeaders(w, "/v1/workspace/service-accounts/{id}/audit/exports/{job_id}")
	if s.auditExportSvc == nil {
		writeError(w, http.StatusNotImplemented, "audit export service not configured")
		return
	}

	id := urlParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	resp, err := s.auditExportSvc.GetJob(requestTenantID(r), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
