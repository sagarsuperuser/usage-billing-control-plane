package api

import (
	"net/http"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/service"
)

type tenantAuditEventResponse struct {
	ID            string         `json:"id"`
	TenantID      string         `json:"tenant_id"`
	ActorAPIKeyID string         `json:"actor_api_key_id,omitempty"`
	Action        string         `json:"action"`
	EventCode     string         `json:"event_code"`
	EventCategory string         `json:"event_category"`
	EventTitle    string         `json:"event_title"`
	EventSummary  string         `json:"event_summary"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	CreatedAt     string         `json:"created_at"`
}

type tenantAuditResultResponse struct {
	Items  []tenantAuditEventResponse `json:"items"`
	Total  int                        `json:"total"`
	Limit  int                        `json:"limit"`
	Offset int                        `json:"offset"`
}

type workspaceBillingResponse struct {
	Configured                bool   `json:"configured"`
	Connected                 bool   `json:"connected"`
	ActiveBillingConnectionID string `json:"active_billing_connection_id,omitempty"`
	Status                    string `json:"status"`
	Source                    string `json:"source,omitempty"`
	IsolationMode             string `json:"isolation_mode,omitempty"`
	ConnectionStatus          string `json:"connection_status,omitempty"`
	ConnectionSyncState       string `json:"connection_sync_state,omitempty"`
	ProvisioningError         string `json:"provisioning_error,omitempty"`
	LastSyncError             string `json:"last_sync_error,omitempty"`
	DiagnosisCode             string `json:"diagnosis_code,omitempty"`
	DiagnosisSummary          string `json:"diagnosis_summary,omitempty"`
	NextAction                string `json:"next_action,omitempty"`
}

type workspaceBillingSettingsResponse struct {
	WorkspaceID            string     `json:"workspace_id"`
	BillingEntityCode      string     `json:"billing_entity_code,omitempty"`
	NetPaymentTermDays     *int       `json:"net_payment_term_days,omitempty"`
	TaxCodes               []string   `json:"tax_codes,omitempty"`
	InvoiceMemo            string     `json:"invoice_memo,omitempty"`
	InvoiceFooter          string     `json:"invoice_footer,omitempty"`
	DocumentLocale         string     `json:"document_locale,omitempty"`
	InvoiceGracePeriodDays *int       `json:"invoice_grace_period_days,omitempty"`
	DocumentNumbering      string     `json:"document_numbering,omitempty"`
	DocumentNumberPrefix   string     `json:"document_number_prefix,omitempty"`
	HasOverrides           bool       `json:"has_overrides"`
	UpdatedAt              *time.Time `json:"updated_at,omitempty"`
}

type tenantResponse struct {
	ID                          string                           `json:"id"`
	Name                        string                           `json:"name"`
	Status                      domain.TenantStatus              `json:"status"`
	BillingProviderConnectionID string                           `json:"billing_provider_connection_id,omitempty"`
	WorkspaceBilling            workspaceBillingResponse         `json:"workspace_billing"`
	WorkspaceBillingSettings    workspaceBillingSettingsResponse `json:"workspace_billing_settings"`
	CreatedAt                   time.Time                        `json:"created_at"`
	UpdatedAt                   time.Time                        `json:"updated_at"`
}

func (s *Server) newTenantResponse(tenant domain.Tenant) tenantResponse {
	return tenantResponse{
		ID:                          tenant.ID,
		Name:                        tenant.Name,
		Status:                      tenant.Status,
		BillingProviderConnectionID: tenant.BillingProviderConnectionID,
		WorkspaceBilling:            s.buildWorkspaceBillingResponse(tenant),
		WorkspaceBillingSettings:    s.buildWorkspaceBillingSettingsResponse(tenant.ID),
		CreatedAt:                   tenant.CreatedAt,
		UpdatedAt:                   tenant.UpdatedAt,
	}
}

func (s *Server) newTenantResponses(items []domain.Tenant) []tenantResponse {
	out := make([]tenantResponse, 0, len(items))
	for _, tenant := range items {
		out = append(out, s.newTenantResponse(tenant))
	}
	return out
}

func (s *Server) buildWorkspaceBillingResponse(tenant domain.Tenant) workspaceBillingResponse {
	resp := workspaceBillingResponse{
		Status:           "missing",
		DiagnosisCode:    "missing",
		DiagnosisSummary: "No billing connection is assigned to this workspace.",
		NextAction:       "Select an active billing connection and save the workspace binding before handoff.",
	}
	if connectionID := strings.TrimSpace(tenant.BillingProviderConnectionID); connectionID != "" {
		resp.Configured = true
		resp.ActiveBillingConnectionID = connectionID
		resp.Status = "pending"
		resp.DiagnosisCode = "pending_verification"
		resp.DiagnosisSummary = "A billing connection is attached, but verification is still pending."
		resp.NextAction = "Run or wait for a successful connection sync before treating this workspace as billing-ready."
	}
	if s == nil || s.workspaceBillingBindingService == nil {
		return resp
	}
	if diagnosis, err := s.workspaceBillingBindingService.DescribeWorkspaceBilling(tenant.ID); err == nil {
		resp.Configured = diagnosis.Configured
		resp.Connected = diagnosis.Connected
		resp.ActiveBillingConnectionID = diagnosis.ActiveBillingConnectionID
		resp.Status = diagnosis.Status
		resp.Source = diagnosis.Source
		resp.IsolationMode = string(diagnosis.IsolationMode)
		resp.ConnectionStatus = string(diagnosis.ConnectionStatus)
		resp.ConnectionSyncState = diagnosis.ConnectionSyncState
		resp.ProvisioningError = diagnosis.ProvisioningError
		resp.LastSyncError = diagnosis.LastSyncError
		resp.DiagnosisCode = diagnosis.DiagnosisCode
		resp.DiagnosisSummary = diagnosis.DiagnosisSummary
		resp.NextAction = diagnosis.NextAction
		return resp
	}
	if binding, err := s.workspaceBillingBindingService.GetWorkspaceBillingBinding(tenant.ID); err == nil {
		resp.Configured = true
		resp.ActiveBillingConnectionID = binding.BillingProviderConnectionID
		resp.Status = string(binding.Status)
		resp.Source = "binding"
		resp.IsolationMode = string(binding.IsolationMode)
		resp.Connected = binding.Status == domain.WorkspaceBillingBindingStatusConnected
		resp.ProvisioningError = strings.TrimSpace(binding.ProvisioningError)
		if resp.ProvisioningError != "" {
			resp.DiagnosisCode = "verification_failed"
			resp.DiagnosisSummary = resp.ProvisioningError
			resp.NextAction = "Correct the billing connection or override values, rerun sync, then verify the workspace binding again."
		}
	}
	return resp
}

func (s *Server) buildWorkspaceBillingSettingsResponse(workspaceID string) workspaceBillingSettingsResponse {
	resp := workspaceBillingSettingsResponse{
		WorkspaceID: workspaceID,
	}
	if s == nil || s.workspaceBillingSettingsService == nil {
		return resp
	}
	settings, err := s.workspaceBillingSettingsService.GetWorkspaceBillingSettings(workspaceID)
	if err != nil {
		return resp
	}
	resp.WorkspaceID = settings.WorkspaceID
	resp.BillingEntityCode = settings.BillingEntityCode
	resp.NetPaymentTermDays = settings.NetPaymentTermDays
	resp.TaxCodes = settings.TaxCodes
	resp.InvoiceMemo = settings.InvoiceMemo
	resp.InvoiceFooter = settings.InvoiceFooter
	resp.DocumentLocale = settings.DocumentLocale
	resp.InvoiceGracePeriodDays = settings.InvoiceGracePeriodDays
	resp.DocumentNumbering = settings.DocumentNumbering
	resp.DocumentNumberPrefix = settings.DocumentNumberPrefix
	resp.HasOverrides = settings.BillingEntityCode != "" || settings.NetPaymentTermDays != nil || len(settings.TaxCodes) > 0 || settings.InvoiceMemo != "" || settings.InvoiceFooter != "" || settings.DocumentLocale != "" || settings.InvoiceGracePeriodDays != nil || settings.DocumentNumbering != "" || settings.DocumentNumberPrefix != ""
	if !settings.UpdatedAt.IsZero() {
		updatedAt := settings.UpdatedAt.UTC()
		resp.UpdatedAt = &updatedAt
	}
	return resp
}

func parseInternalTenantPath(path string) (tenantID string, action string, actionID string, subaction string) {
	tail := strings.Trim(strings.TrimPrefix(path, "/internal/tenants/"), "/")
	if tail == "" {
		return "", "", "", ""
	}
	parts := strings.Split(tail, "/")
	tenantID = strings.TrimSpace(parts[0])
	if len(parts) > 1 {
		action = strings.TrimSpace(parts[1])
	}
	if len(parts) > 2 {
		actionID = strings.TrimSpace(parts[2])
	}
	if len(parts) > 3 {
		subaction = strings.TrimSpace(parts[3])
	}
	return tenantID, action, actionID, subaction
}

func newTenantAuditEventResponse(event domain.TenantAuditEvent) tenantAuditEventResponse {
	presentation := presentTenantAuditEvent(event)
	return tenantAuditEventResponse{
		ID:            event.ID,
		TenantID:      event.TenantID,
		ActorAPIKeyID: event.ActorAPIKeyID,
		Action:        event.Action,
		EventCode:     presentation.Code,
		EventCategory: presentation.Category,
		EventTitle:    presentation.Title,
		EventSummary:  presentation.Summary,
		Metadata:      event.Metadata,
		CreatedAt:     event.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func (s *Server) handleInternalTenants(w http.ResponseWriter, r *http.Request) {
	if !s.isOperatorRequest(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	switch r.Method {
	case http.MethodPost:
		var req service.EnsureTenantRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		tenant, err := s.tenantService.CreateTenant(req, requestActorAPIKeyID(r))
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"tenant":  s.newTenantResponse(tenant),
			"created": true,
		})
	case http.MethodGet:
		tenants, err := s.tenantService.ListTenants(service.ListTenantsRequest{
			Status: r.URL.Query().Get("status"),
		})
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, s.newTenantResponses(tenants))
	default:
		writeMethodNotAllowed(w)
	}
}

func (s *Server) handleInternalTenantByID(w http.ResponseWriter, r *http.Request) {
	if !s.isOperatorRequest(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	id, action, actionID, subaction := parseInternalTenantPath(r.URL.Path)
	if id == "" {
		writeError(w, http.StatusBadRequest, "tenant id is required")
		return
	}
	if action != "" && action != "bootstrap-admin-key" && action != "workspace-billing" && action != "workspace-billing-settings" && action != "members" && action != "invitations" {
		writeError(w, http.StatusBadRequest, "unsupported tenant subresource")
		return
	}
	if action == "bootstrap-admin-key" {
		s.handleInternalTenantBootstrapAdminKey(w, r, id)
		return
	}
	if action == "workspace-billing" {
		s.handleInternalTenantWorkspaceBilling(w, r, id)
		return
	}
	if action == "workspace-billing-settings" {
		s.handleInternalTenantWorkspaceBillingSettings(w, r, id)
		return
	}
	if action == "members" {
		s.handleInternalTenantMembers(w, r, id, actionID)
		return
	}
	if action == "invitations" {
		s.handleInternalTenantInvitations(w, r, id, actionID, subaction)
		return
	}

	switch r.Method {
	case http.MethodGet:
		tenant, err := s.tenantService.GetTenant(id)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, s.newTenantResponse(tenant))
	case http.MethodPatch:
		var req struct {
			Name                        *string              `json:"name,omitempty"`
			Status                      *domain.TenantStatus `json:"status,omitempty"`
			BillingProviderConnectionID *string              `json:"billing_provider_connection_id,omitempty"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		tenant, err := s.tenantService.UpdateTenant(id, service.UpdateTenantRequest{
			Name:                        req.Name,
			Status:                      req.Status,
			BillingProviderConnectionID: req.BillingProviderConnectionID,
		}, requestActorAPIKeyID(r))
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, s.newTenantResponse(tenant))
	default:
		writeMethodNotAllowed(w)
	}
}

func (s *Server) handleInternalTenantWorkspaceBilling(w http.ResponseWriter, r *http.Request, tenantID string) {
	if !s.isOperatorRequest(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	switch r.Method {
	case http.MethodGet:
		tenant, err := s.tenantService.GetTenant(tenantID)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"workspace_billing": s.buildWorkspaceBillingResponse(tenant),
		})
	case http.MethodPatch:
		var req struct {
			BillingProviderConnectionID string `json:"billing_provider_connection_id"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		connectionID := strings.TrimSpace(req.BillingProviderConnectionID)
		if connectionID == "" {
			writeError(w, http.StatusBadRequest, "billing_provider_connection_id is required")
			return
		}
		tenant, err := s.tenantService.UpdateTenant(tenantID, service.UpdateTenantRequest{
			BillingProviderConnectionID: &connectionID,
		}, requestActorAPIKeyID(r))
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"tenant": s.newTenantResponse(tenant),
		})
	default:
		writeMethodNotAllowed(w)
	}
}

func (s *Server) handleInternalTenantWorkspaceBillingSettings(w http.ResponseWriter, r *http.Request, tenantID string) {
	if !s.isOperatorRequest(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if s.workspaceBillingSettingsService == nil {
		writeError(w, http.StatusServiceUnavailable, "workspace billing settings are not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		settings, err := s.workspaceBillingSettingsService.GetWorkspaceBillingSettings(tenantID)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"workspace_billing_settings": s.buildWorkspaceBillingSettingsResponse(settings.WorkspaceID),
		})
	case http.MethodPatch:
		var req service.UpdateWorkspaceBillingSettingsRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		settings, err := s.workspaceBillingSettingsService.UpsertWorkspaceBillingSettings(tenantID, req)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"workspace_billing_settings": s.buildWorkspaceBillingSettingsResponse(settings.WorkspaceID),
		})
	default:
		writeMethodNotAllowed(w)
	}
}

func (s *Server) handleInternalTenantMembers(w http.ResponseWriter, r *http.Request, tenantID, userID string) {
	if !s.isOperatorRequest(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if s.workspaceAccessService == nil {
		writeError(w, http.StatusServiceUnavailable, "workspace access is not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		if userID != "" {
			writeMethodNotAllowed(w)
			return
		}
		items, err := s.workspaceAccessService.ListWorkspaceMembers(tenantID)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	case http.MethodPatch:
		if strings.TrimSpace(userID) == "" {
			writeError(w, http.StatusBadRequest, "user id is required")
			return
		}
		var req struct {
			Role   string `json:"role"`
			Reason string `json:"reason,omitempty"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		principal, _ := principalFromContext(r.Context())
		member, err := s.workspaceAccessService.UpdateWorkspaceMemberRoleWithAudit(tenantID, userID, req.Role, workspaceAccessAuditActorFromPrincipal(principal, req.Reason))
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"member": member})
	case http.MethodDelete:
		if strings.TrimSpace(userID) == "" {
			writeError(w, http.StatusBadRequest, "user id is required")
			return
		}
		principal, _ := principalFromContext(r.Context())
		if err := s.workspaceAccessService.RemoveWorkspaceMemberWithAudit(tenantID, userID, workspaceAccessAuditActorFromPrincipal(principal, strings.TrimSpace(r.URL.Query().Get("reason")))); err != nil {
			writeDomainError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeMethodNotAllowed(w)
	}
}

func (s *Server) handleInternalTenantInvitations(w http.ResponseWriter, r *http.Request, tenantID, invitationID, subaction string) {
	if !s.isOperatorRequest(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if s.workspaceAccessService == nil {
		writeError(w, http.StatusServiceUnavailable, "workspace access is not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		if invitationID != "" || subaction != "" {
			writeMethodNotAllowed(w)
			return
		}
		items, err := s.workspaceAccessService.ListWorkspaceInvitations(tenantID, strings.TrimSpace(r.URL.Query().Get("status")))
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": newWorkspaceInvitationResponses(items)})
	case http.MethodPost:
		if invitationID != "" || subaction != "" {
			if invitationID != "" && subaction == "revoke" {
				var req struct {
					Reason string `json:"reason,omitempty"`
				}
				if r.ContentLength != 0 {
					if err := decodeJSON(r, &req); err != nil {
						writeError(w, http.StatusBadRequest, err.Error())
						return
					}
				}
				principal, _ := principalFromContext(r.Context())
				invite, err := s.workspaceAccessService.RevokeWorkspaceInvitationWithAudit(tenantID, invitationID, workspaceAccessAuditActorFromPrincipal(principal, req.Reason))
				if err != nil {
					writeDomainError(w, err)
					return
				}
				writeJSON(w, http.StatusOK, map[string]any{"invitation": newWorkspaceInvitationResponse(invite)})
				return
			}
			writeMethodNotAllowed(w)
			return
		}
		var req struct {
			Email string `json:"email"`
			Role  string `json:"role"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		principal, _ := principalFromContext(r.Context())
		invitedByUserID := ""
		if strings.TrimSpace(principal.SubjectType) == "user" {
			invitedByUserID = strings.TrimSpace(principal.SubjectID)
		}
		issued, err := s.workspaceAccessService.IssueWorkspaceInvitation(service.CreateWorkspaceInvitationRequest{
			WorkspaceID:           tenantID,
			Email:                 req.Email,
			Role:                  req.Role,
			InvitedByUserID:       invitedByUserID,
			InvitedByPlatformUser: principal.Scope == ScopePlatform,
		})
		if err != nil {
			writeDomainError(w, err)
			return
		}
		acceptPath, acceptURL := s.workspaceInvitationAcceptURL(issued.Token)
		s.sendWorkspaceInvitationEmail(tenantID, principal.UserEmail, issued)
		writeJSON(w, http.StatusCreated, map[string]any{
			"invitation":  newWorkspaceInvitationResponseWithAcceptURL(issued.Invitation, acceptURL),
			"accept_url":  acceptURL,
			"accept_path": acceptPath,
		})
	default:
		writeMethodNotAllowed(w)
	}
}

type internalTenantBootstrapAdminKeyRequest struct {
	Name                    string     `json:"name"`
	ExpiresAt               *time.Time `json:"expires_at,omitempty"`
	AllowExistingActiveKeys bool       `json:"allow_existing_active_keys,omitempty"`
}

func (s *Server) handleInternalTenantBootstrapAdminKey(w http.ResponseWriter, r *http.Request, tenantID string) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	var req internalTenantBootstrapAdminKeyRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		req.Name = "bootstrap-admin-" + normalizeTenantID(tenantID)
	}

	activeKeys, err := s.apiKeyService.ListAPIKeys(tenantID, service.ListAPIKeysRequest{
		State: "active",
		Limit: 1,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if activeKeys.Total > 0 && !req.AllowExistingActiveKeys {
		writeError(w, http.StatusConflict, "tenant already has active keys")
		return
	}

	created, err := s.serviceAccountService.IssueBootstrapWorkspaceServiceAccountCredential(tenantID, req.Name, service.APICredentialActor{
		PlatformAPIKeyID:  requestActorAPIKeyID(r),
		CreatedByPlatform: true,
	}, req.ExpiresAt)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"service_account":      created.ServiceAccount,
		"credential":           created.Credential,
		"secret":               created.Secret,
		"existing_active_keys": activeKeys.Total,
	})
}

func (s *Server) handleInternalTenantAudit(w http.ResponseWriter, r *http.Request) {
	if !s.isOperatorRequest(r) {
		writeError(w, http.StatusForbidden, "forbidden")
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
	events, err := s.tenantService.ListTenantAuditEvents(service.ListTenantAuditEventsRequest{
		TenantID:      r.URL.Query().Get("tenant_id"),
		ActorAPIKeyID: r.URL.Query().Get("actor_api_key_id"),
		Action:        r.URL.Query().Get("action"),
		Limit:         limit,
		Offset:        offset,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	resp := tenantAuditResultResponse{
		Items:  make([]tenantAuditEventResponse, 0, len(events.Items)),
		Total:  events.Total,
		Limit:  events.Limit,
		Offset: events.Offset,
	}
	for _, event := range events.Items {
		resp.Items = append(resp.Items, newTenantAuditEventResponse(event))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleInternalOnboardingTenants(w http.ResponseWriter, r *http.Request) {
	if !s.isOperatorRequest(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	var req service.TenantOnboardingRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := s.onboardingService.OnboardTenant(req, requestActorAPIKeyID(r))
	if err != nil {
		s.logOnboardingFailure(r, req, err)
		writeDomainError(w, err)
		return
	}
	status := http.StatusOK
	if result.TenantCreated {
		status = http.StatusCreated
	}
	writeJSON(w, status, map[string]any{
		"tenant":                 s.newTenantResponse(result.Tenant),
		"tenant_created":         result.TenantCreated,
		"tenant_admin_bootstrap": result.TenantAdminBootstrap,
		"readiness":              result.Readiness,
	})
}

func (s *Server) handleInternalOnboardingTenantByID(w http.ResponseWriter, r *http.Request) {
	if !s.isOperatorRequest(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/internal/onboarding/tenants/"), "/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "tenant id is required")
		return
	}
	readiness, err := s.onboardingService.GetTenantReadiness(id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	tenant, err := s.tenantService.GetTenant(id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"tenant":    s.newTenantResponse(tenant),
		"readiness": readiness,
		"tenant_id": normalizeTenantID(id),
	})
}

func workspaceAccessAuditActorFromPrincipal(principal Principal, reason string) service.WorkspaceAccessAuditActor {
	return service.WorkspaceAccessAuditActor{
		SubjectType:  strings.TrimSpace(principal.SubjectType),
		SubjectID:    strings.TrimSpace(principal.SubjectID),
		UserEmail:    strings.TrimSpace(principal.UserEmail),
		Scope:        strings.TrimSpace(string(principal.Scope)),
		PlatformRole: strings.TrimSpace(string(principal.PlatformRole)),
		APIKeyID:     strings.TrimSpace(principal.APIKeyID),
		Reason:       strings.TrimSpace(reason),
	}
}
