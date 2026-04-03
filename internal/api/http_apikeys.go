package api

import (
	"net/http"
	"strings"

	"usage-billing-control-plane/internal/service"
)

func (s *Server) handleAPIKeys(w http.ResponseWriter, r *http.Request) {
	setCompatibilityDeprecationHeaders(w, "/v1/workspace/service-accounts")
	tenantID := requestTenantID(r)
	actorAPIKeyID := requestActorAPIKeyID(r)

	switch r.Method {
	case http.MethodPost:
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
	case http.MethodGet:
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
	default:
		writeMethodNotAllowed(w)
	}
}

func (s *Server) handleAPIKeyByID(w http.ResponseWriter, r *http.Request) {
	setCompatibilityDeprecationHeaders(w, "/v1/workspace/service-accounts")
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	tail := strings.TrimPrefix(r.URL.Path, "/v1/api-keys/")
	parts := strings.Split(strings.Trim(tail, "/"), "/")
	if len(parts) != 2 {
		writeError(w, http.StatusBadRequest, "expected /v1/api-keys/{id}/revoke or /v1/api-keys/{id}/rotate")
		return
	}

	id := strings.TrimSpace(parts[0])
	action := strings.ToLower(strings.TrimSpace(parts[1]))
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	tenantID := requestTenantID(r)
	actorAPIKeyID := requestActorAPIKeyID(r)
	switch action {
	case "revoke":
		key, err := s.apiKeyService.RevokeAPIKey(tenantID, actorAPIKeyID, id)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, key)
	case "rotate":
		result, err := s.apiKeyService.RotateAPIKey(tenantID, actorAPIKeyID, id)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	default:
		writeError(w, http.StatusBadRequest, "unsupported action")
	}
}

func (s *Server) handleAPIKeyAuditEvents(w http.ResponseWriter, r *http.Request) {
	setCompatibilityDeprecationHeaders(w, "/v1/workspace/service-accounts/{id}/audit")
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

func (s *Server) handleAPIKeyAuditExports(w http.ResponseWriter, r *http.Request) {
	setCompatibilityDeprecationHeaders(w, "/v1/workspace/service-accounts/{id}/audit/exports")
	if s.auditExportSvc == nil {
		writeError(w, http.StatusNotImplemented, "audit export service not configured")
		return
	}
	switch r.Method {
	case http.MethodPost:
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
	case http.MethodGet:
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
	default:
		writeMethodNotAllowed(w)
	}
}

func (s *Server) handleAPIKeyAuditExportByID(w http.ResponseWriter, r *http.Request) {
	setCompatibilityDeprecationHeaders(w, "/v1/workspace/service-accounts/{id}/audit/exports/{job_id}")
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if s.auditExportSvc == nil {
		writeError(w, http.StatusNotImplemented, "audit export service not configured")
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/v1/api-keys/audit/exports/")
	id = strings.TrimSpace(id)
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
