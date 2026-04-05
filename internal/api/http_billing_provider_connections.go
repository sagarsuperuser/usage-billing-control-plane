package api

import (
	"net/http"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/service"
)

type billingProviderConnectionResponse struct {
	ID                   string                                 `json:"id"`
	ProviderType         domain.BillingProviderType             `json:"provider_type"`
	Environment          string                                 `json:"environment"`
	DisplayName          string                                 `json:"display_name"`
	Scope                domain.BillingProviderConnectionScope  `json:"scope"`
	OwnerTenantID        string                                 `json:"owner_tenant_id,omitempty"`
	Status               domain.BillingProviderConnectionStatus `json:"status"`
	WorkspaceReady       bool                                   `json:"workspace_ready"`
	SyncState            string                                 `json:"sync_state"`
	SyncSummary          string                                 `json:"sync_summary"`
	CheckReady           bool                                   `json:"check_ready"`
	CheckBlockerCode     string                                 `json:"check_blocker_code,omitempty"`
	CheckBlockerSummary  string                                 `json:"check_blocker_summary,omitempty"`
	LinkedWorkspaceCount int                                    `json:"linked_workspace_count"`
	SecretConfigured     bool                                   `json:"secret_configured"`
	LastSyncedAt         *string                                `json:"last_synced_at,omitempty"`
	LastSyncError        string                                 `json:"last_sync_error,omitempty"`
	ConnectedAt          *string                                `json:"connected_at,omitempty"`
	DisabledAt           *string                                `json:"disabled_at,omitempty"`
	CreatedByType        string                                 `json:"created_by_type"`
	CreatedByID          string                                 `json:"created_by_id,omitempty"`
	CreatedAt            string                                 `json:"created_at"`
	UpdatedAt            string                                 `json:"updated_at"`
}

type rotateBillingProviderConnectionSecretRequest struct {
	StripeSecretKey string `json:"stripe_secret_key"`
}

func billingConnectionSyncState(item domain.BillingProviderConnection) string {
	switch item.Status {
	case domain.BillingProviderConnectionStatusDisabled:
		return "disabled"
	case domain.BillingProviderConnectionStatusConnected:
		return "healthy"
	case domain.BillingProviderConnectionStatusSyncError:
		return "failed"
	case domain.BillingProviderConnectionStatusPending:
		if item.ConnectedAt != nil || item.LastSyncedAt != nil {
			return "pending"
		}
		return "never_synced"
	default:
		if item.LastSyncedAt == nil {
			return "never_synced"
		}
		return "pending"
	}
}

func billingConnectionSyncSummary(item domain.BillingProviderConnection) string {
	switch billingConnectionSyncState(item) {
	case "healthy":
		return "Stripe credentials are verified and ready for workspace assignment."
	case "failed":
		if msg := strings.TrimSpace(item.LastSyncError); msg != "" {
			return msg
		}
		return "Stripe needs attention before it can be used."
	case "disabled":
		return "Connection is disabled."
	case "never_synced":
		return "Run the first connection check before assigning this connection to a workspace."
	default:
		return "Run another connection check before using this connection."
	}
}

func (s *Server) describeBillingProviderConnectionCheck(item domain.BillingProviderConnection) (bool, string, string) {
	if item.Status == domain.BillingProviderConnectionStatusDisabled {
		return false, "disabled", "Disabled connections cannot be checked."
	}
	if strings.TrimSpace(item.SecretRef) == "" {
		return false, "missing_secret", "Add a Stripe secret before checking this connection."
	}
	return true, "", ""
}

func (s *Server) newBillingProviderConnectionResponse(item domain.BillingProviderConnection, linkedWorkspaceCount int) billingProviderConnectionResponse {
	checkReady, checkBlockerCode, checkBlockerSummary := s.describeBillingProviderConnectionCheck(item)
	out := billingProviderConnectionResponse{
		ID:                   item.ID,
		ProviderType:         item.ProviderType,
		Environment:          item.Environment,
		DisplayName:          item.DisplayName,
		Scope:                item.Scope,
		OwnerTenantID:        item.OwnerTenantID,
		Status:               item.Status,
		WorkspaceReady:       item.Status == domain.BillingProviderConnectionStatusConnected,
		SyncState:            billingConnectionSyncState(item),
		SyncSummary:          billingConnectionSyncSummary(item),
		CheckReady:           checkReady,
		CheckBlockerCode:     checkBlockerCode,
		CheckBlockerSummary:  checkBlockerSummary,
		LinkedWorkspaceCount: linkedWorkspaceCount,
		SecretConfigured:     strings.TrimSpace(item.SecretRef) != "",
		LastSyncError:        item.LastSyncError,
		CreatedByType:        item.CreatedByType,
		CreatedByID:          item.CreatedByID,
		CreatedAt:            item.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:            item.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if item.LastSyncedAt != nil {
		value := item.LastSyncedAt.UTC().Format(time.RFC3339Nano)
		out.LastSyncedAt = &value
	}
	if item.ConnectedAt != nil {
		value := item.ConnectedAt.UTC().Format(time.RFC3339Nano)
		out.ConnectedAt = &value
	}
	if item.DisabledAt != nil {
		value := item.DisabledAt.UTC().Format(time.RFC3339Nano)
		out.DisabledAt = &value
	}
	return out
}

func (s *Server) listBillingProviderConnections(w http.ResponseWriter, r *http.Request) {
	if !s.isOperatorRequest(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if s.billingProviderConnectionService == nil {
		writeError(w, http.StatusServiceUnavailable, "billing provider connections are not configured")
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
	items, err := s.billingProviderConnectionService.ListBillingProviderConnections(service.ListBillingProviderConnectionsRequest{
		ProviderType:  r.URL.Query().Get("provider_type"),
		Environment:   r.URL.Query().Get("environment"),
		Status:        r.URL.Query().Get("status"),
		Scope:         r.URL.Query().Get("scope"),
		OwnerTenantID: r.URL.Query().Get("owner_tenant_id"),
		Limit:         limit,
		Offset:        offset,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	counts, err := s.repo.CountTenantsByBillingProviderConnections(ids)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	resp := make([]billingProviderConnectionResponse, 0, len(items))
	for _, item := range items {
		resp = append(resp, s.newBillingProviderConnectionResponse(item, counts[item.ID]))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": resp})
}

func (s *Server) createBillingProviderConnection(w http.ResponseWriter, r *http.Request) {
	if !s.isOperatorRequest(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if s.billingProviderConnectionService == nil {
		writeError(w, http.StatusServiceUnavailable, "billing provider connections are not configured")
		return
	}
	var req service.CreateBillingProviderConnectionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	created, err := s.billingProviderConnectionService.CreateBillingProviderConnection(r.Context(), req, "platform_api_key", requestActorAPIKeyID(r))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"connection": s.newBillingProviderConnectionResponse(created, 0)})
}

func (s *Server) billingProviderConnectionLoadCounts(w http.ResponseWriter, id string) (map[string]int, bool) {
	counts, err := s.repo.CountTenantsByBillingProviderConnections([]string{id})
	if err != nil {
		writeDomainError(w, err)
		return nil, false
	}
	return counts, true
}

func (s *Server) getBillingProviderConnection(w http.ResponseWriter, r *http.Request) {
	if !s.isOperatorRequest(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if s.billingProviderConnectionService == nil {
		writeError(w, http.StatusServiceUnavailable, "billing provider connections are not configured")
		return
	}
	id := urlParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "connection id is required")
		return
	}
	item, err := s.billingProviderConnectionService.GetBillingProviderConnection(id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	counts, ok := s.billingProviderConnectionLoadCounts(w, id)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"connection": s.newBillingProviderConnectionResponse(item, counts[id])})
}

func (s *Server) updateBillingProviderConnection(w http.ResponseWriter, r *http.Request) {
	if !s.isOperatorRequest(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if s.billingProviderConnectionService == nil {
		writeError(w, http.StatusServiceUnavailable, "billing provider connections are not configured")
		return
	}
	id := urlParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "connection id is required")
		return
	}
	var req service.UpdateBillingProviderConnectionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	item, err := s.billingProviderConnectionService.UpdateBillingProviderConnection(id, req)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	counts, ok := s.billingProviderConnectionLoadCounts(w, id)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"connection": s.newBillingProviderConnectionResponse(item, counts[id])})
}

func (s *Server) syncBillingProviderConnection(w http.ResponseWriter, r *http.Request) {
	if !s.isOperatorRequest(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if s.billingProviderConnectionService == nil {
		writeError(w, http.StatusServiceUnavailable, "billing provider connections are not configured")
		return
	}
	id := urlParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "connection id is required")
		return
	}
	item, err := s.billingProviderConnectionService.SyncBillingProviderConnection(r.Context(), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	counts, ok := s.billingProviderConnectionLoadCounts(w, id)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"connection": s.newBillingProviderConnectionResponse(item, counts[id])})
}

func (s *Server) disableBillingProviderConnection(w http.ResponseWriter, r *http.Request) {
	if !s.isOperatorRequest(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if s.billingProviderConnectionService == nil {
		writeError(w, http.StatusServiceUnavailable, "billing provider connections are not configured")
		return
	}
	id := urlParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "connection id is required")
		return
	}
	item, err := s.billingProviderConnectionService.DisableBillingProviderConnection(id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	counts, ok := s.billingProviderConnectionLoadCounts(w, id)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"connection": s.newBillingProviderConnectionResponse(item, counts[id])})
}

func (s *Server) rotateBillingProviderConnectionSecret(w http.ResponseWriter, r *http.Request) {
	if !s.isOperatorRequest(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if s.billingProviderConnectionService == nil {
		writeError(w, http.StatusServiceUnavailable, "billing provider connections are not configured")
		return
	}
	id := urlParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "connection id is required")
		return
	}
	var req rotateBillingProviderConnectionSecretRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	item, err := s.billingProviderConnectionService.RotateBillingProviderConnectionSecret(r.Context(), id, req.StripeSecretKey)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	counts, ok := s.billingProviderConnectionLoadCounts(w, id)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"connection": s.newBillingProviderConnectionResponse(item, counts[id])})
}
