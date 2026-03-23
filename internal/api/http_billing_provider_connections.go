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
	LinkedWorkspaceCount int                                    `json:"linked_workspace_count"`
	LagoOrganizationID   string                                 `json:"lago_organization_id,omitempty"`
	LagoProviderCode     string                                 `json:"lago_provider_code,omitempty"`
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
		return "Connected and ready for workspace assignment."
	case "failed":
		if msg := strings.TrimSpace(item.LastSyncError); msg != "" {
			return msg
		}
		return "The last sync failed. Review provider configuration and try again."
	case "disabled":
		return "Connection is disabled and cannot be assigned to new workspaces."
	case "never_synced":
		return "Connection has not been synced yet. Run the first sync before assigning it to workspaces."
	default:
		return "Connection is waiting for a successful provider sync."
	}
}

func newBillingProviderConnectionResponse(item domain.BillingProviderConnection, linkedWorkspaceCount int) billingProviderConnectionResponse {
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
		LinkedWorkspaceCount: linkedWorkspaceCount,
		LagoOrganizationID:   item.LagoOrganizationID,
		LagoProviderCode:     item.LagoProviderCode,
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

func (s *Server) handleInternalBillingProviderConnections(w http.ResponseWriter, r *http.Request) {
	if !s.isOperatorRequest(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if s.billingProviderConnectionService == nil {
		writeError(w, http.StatusServiceUnavailable, "billing provider connections are not configured")
		return
	}

	switch r.Method {
	case http.MethodPost:
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
		writeJSON(w, http.StatusCreated, map[string]any{"connection": newBillingProviderConnectionResponse(created, 0)})
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
			resp = append(resp, newBillingProviderConnectionResponse(item, counts[item.ID]))
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": resp})
	default:
		writeMethodNotAllowed(w)
	}
}

func (s *Server) handleInternalBillingProviderConnectionByID(w http.ResponseWriter, r *http.Request) {
	if !s.isOperatorRequest(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if s.billingProviderConnectionService == nil {
		writeError(w, http.StatusServiceUnavailable, "billing provider connections are not configured")
		return
	}

	id, action := parseInternalBillingProviderConnectionPath(r.URL.Path)
	if id == "" {
		writeError(w, http.StatusBadRequest, "connection id is required")
		return
	}
	switch action {
	case "", "sync", "disable", "rotate-secret":
	default:
		writeError(w, http.StatusBadRequest, "unsupported billing provider connection subresource")
		return
	}

	loadCounts := func() (map[string]int, bool) {
		counts, err := s.repo.CountTenantsByBillingProviderConnections([]string{id})
		if err != nil {
			writeDomainError(w, err)
			return nil, false
		}
		return counts, true
	}

	switch {
	case action == "sync":
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w)
			return
		}
		item, err := s.billingProviderConnectionService.SyncBillingProviderConnection(r.Context(), id)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		counts, ok := loadCounts()
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"connection": newBillingProviderConnectionResponse(item, counts[id])})
	case action == "disable":
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w)
			return
		}
		item, err := s.billingProviderConnectionService.DisableBillingProviderConnection(id)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		counts, ok := loadCounts()
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"connection": newBillingProviderConnectionResponse(item, counts[id])})
	case action == "rotate-secret":
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w)
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
		counts, ok := loadCounts()
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"connection": newBillingProviderConnectionResponse(item, counts[id])})
	default:
		switch r.Method {
		case http.MethodGet:
			item, err := s.billingProviderConnectionService.GetBillingProviderConnection(id)
			if err != nil {
				writeDomainError(w, err)
				return
			}
			counts, ok := loadCounts()
			if !ok {
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"connection": newBillingProviderConnectionResponse(item, counts[id])})
		case http.MethodPatch:
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
			counts, ok := loadCounts()
			if !ok {
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"connection": newBillingProviderConnectionResponse(item, counts[id])})
		default:
			writeMethodNotAllowed(w)
		}
	}
}

func parseInternalBillingProviderConnectionPath(path string) (connectionID string, action string) {
	tail := strings.Trim(strings.TrimPrefix(path, "/internal/billing-provider-connections/"), "/")
	if tail == "" {
		return "", ""
	}
	parts := strings.Split(tail, "/")
	connectionID = strings.TrimSpace(parts[0])
	if len(parts) > 1 {
		action = strings.TrimSpace(parts[1])
	}
	return connectionID, action
}
