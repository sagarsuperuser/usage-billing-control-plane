package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/service"
)

type workspaceServiceAccountResponse struct {
	ID                    string          `json:"id"`
	WorkspaceID           string          `json:"workspace_id"`
	Name                  string          `json:"name"`
	Description           string          `json:"description,omitempty"`
	Role                  string          `json:"role"`
	Status                string          `json:"status"`
	Purpose               string          `json:"purpose,omitempty"`
	Environment           string          `json:"environment,omitempty"`
	CreatedByUserID       string          `json:"created_by_user_id,omitempty"`
	CreatedByPlatformUser bool            `json:"created_by_platform_user,omitempty"`
	CreatedAt             time.Time       `json:"created_at"`
	UpdatedAt             time.Time       `json:"updated_at"`
	DisabledAt            *time.Time      `json:"disabled_at,omitempty"`
	ActiveCredentialCount int             `json:"active_credential_count"`
	Credentials           []domain.APIKey `json:"credentials"`
}

type workspaceServiceAccountAuditExportJobResponse struct {
	Job         domain.APIKeyAuditExportJob `json:"job"`
	DownloadURL string                      `json:"download_url,omitempty"`
}

func newWorkspaceServiceAccountResponse(item service.WorkspaceServiceAccount) workspaceServiceAccountResponse {
	return workspaceServiceAccountResponse{
		ID:                    item.ServiceAccount.ID,
		WorkspaceID:           item.ServiceAccount.TenantID,
		Name:                  item.ServiceAccount.Name,
		Description:           item.ServiceAccount.Description,
		Role:                  item.ServiceAccount.Role,
		Status:                item.ServiceAccount.Status,
		Purpose:               item.ServiceAccount.Purpose,
		Environment:           item.ServiceAccount.Environment,
		CreatedByUserID:       item.ServiceAccount.CreatedByUserID,
		CreatedByPlatformUser: item.ServiceAccount.CreatedByPlatformUser,
		CreatedAt:             item.ServiceAccount.CreatedAt,
		UpdatedAt:             item.ServiceAccount.UpdatedAt,
		DisabledAt:            item.ServiceAccount.DisabledAt,
		ActiveCredentialCount: item.ActiveCredentialCount,
		Credentials:           item.Credentials,
	}
}

func newWorkspaceServiceAccountResponses(items []service.WorkspaceServiceAccount) []workspaceServiceAccountResponse {
	if len(items) == 0 {
		return []workspaceServiceAccountResponse{}
	}
	out := make([]workspaceServiceAccountResponse, 0, len(items))
	for _, item := range items {
		out = append(out, newWorkspaceServiceAccountResponse(item))
	}
	return out
}

type workspaceInvitationResponse struct {
	ID                    string     `json:"id"`
	WorkspaceID           string     `json:"workspace_id"`
	Email                 string     `json:"email"`
	Role                  string     `json:"role"`
	Status                string     `json:"status"`
	ExpiresAt             time.Time  `json:"expires_at"`
	AcceptedAt            *time.Time `json:"accepted_at,omitempty"`
	AcceptedByUserID      string     `json:"accepted_by_user_id,omitempty"`
	InvitedByUserID       string     `json:"invited_by_user_id,omitempty"`
	InvitedByPlatformUser bool       `json:"invited_by_platform_user"`
	RevokedAt             *time.Time `json:"revoked_at,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
	AcceptURL             string     `json:"accept_url,omitempty"`
}

func newWorkspaceInvitationResponse(invite domain.WorkspaceInvitation) workspaceInvitationResponse {
	return workspaceInvitationResponse{
		ID:                    invite.ID,
		WorkspaceID:           invite.WorkspaceID,
		Email:                 invite.Email,
		Role:                  invite.Role,
		Status:                string(invite.Status),
		ExpiresAt:             invite.ExpiresAt,
		AcceptedAt:            invite.AcceptedAt,
		AcceptedByUserID:      invite.AcceptedByUserID,
		InvitedByUserID:       invite.InvitedByUserID,
		InvitedByPlatformUser: invite.InvitedByPlatformUser,
		RevokedAt:             invite.RevokedAt,
		CreatedAt:             invite.CreatedAt,
		UpdatedAt:             invite.UpdatedAt,
	}
}

func newWorkspaceInvitationResponseWithAcceptURL(invite domain.WorkspaceInvitation, acceptURL string) workspaceInvitationResponse {
	resp := newWorkspaceInvitationResponse(invite)
	resp.AcceptURL = strings.TrimSpace(acceptURL)
	return resp
}

func newWorkspaceInvitationResponses(items []domain.WorkspaceInvitation) []workspaceInvitationResponse {
	out := make([]workspaceInvitationResponse, 0, len(items))
	for _, item := range items {
		out = append(out, newWorkspaceInvitationResponse(item))
	}
	return out
}

// ── Workspace Members ───────────────────────────────────────────────

func (s *Server) requireWorkspaceAccess(w http.ResponseWriter, r *http.Request) (Principal, bool) {
	if s.workspaceAccessService == nil {
		writeError(w, http.StatusServiceUnavailable, "workspace access is not configured")
		return Principal{}, false
	}
	principal, ok := principalFromContext(r.Context())
	if !ok || principal.Scope != ScopeTenant {
		writeError(w, http.StatusForbidden, "forbidden")
		return Principal{}, false
	}
	return principal, true
}

func (s *Server) listWorkspaceMembers(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.requireWorkspaceAccess(w, r)
	if !ok {
		return
	}
	items, err := s.workspaceAccessService.ListWorkspaceMembers(principal.TenantID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) updateWorkspaceMember(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.requireWorkspaceAccess(w, r)
	if !ok {
		return
	}
	userID := urlParam(r, "id")
	if userID == "" {
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
	member, err := s.workspaceAccessService.UpdateWorkspaceMemberRoleWithAudit(principal.TenantID, userID, req.Role, workspaceAccessAuditActorFromPrincipal(principal, req.Reason))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"member": member})
}

func (s *Server) removeWorkspaceMember(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.requireWorkspaceAccess(w, r)
	if !ok {
		return
	}
	userID := urlParam(r, "id")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "user id is required")
		return
	}
	if err := s.workspaceAccessService.RemoveWorkspaceMemberWithAudit(principal.TenantID, userID, workspaceAccessAuditActorFromPrincipal(principal, strings.TrimSpace(r.URL.Query().Get("reason")))); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Workspace Invitations ───────────────────────────────────────────

func (s *Server) listWorkspaceInvitations(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.requireWorkspaceAccess(w, r)
	if !ok {
		return
	}
	items, err := s.workspaceAccessService.ListWorkspaceInvitations(principal.TenantID, strings.TrimSpace(r.URL.Query().Get("status")))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": newWorkspaceInvitationResponses(items)})
}

func (s *Server) createWorkspaceInvitation(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.requireWorkspaceAccess(w, r)
	if !ok {
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
	issued, err := s.workspaceAccessService.IssueWorkspaceInvitation(service.CreateWorkspaceInvitationRequest{
		WorkspaceID:           principal.TenantID,
		Email:                 req.Email,
		Role:                  req.Role,
		InvitedByUserID:       principal.SubjectID,
		InvitedByPlatformUser: false,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	acceptPath, acceptURL := s.workspaceInvitationAcceptURL(issued.Token)
	s.sendWorkspaceInvitationEmail(principal.TenantID, principal.UserEmail, issued)
	writeJSON(w, http.StatusCreated, map[string]any{
		"invitation":  newWorkspaceInvitationResponseWithAcceptURL(issued.Invitation, acceptURL),
		"accept_url":  acceptURL,
		"accept_path": acceptPath,
	})
}

func (s *Server) revokeWorkspaceInvitation(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.requireWorkspaceAccess(w, r)
	if !ok {
		return
	}
	invitationID := urlParam(r, "id")
	if invitationID == "" {
		writeError(w, http.StatusBadRequest, "invitation id is required")
		return
	}
	invite, err := s.workspaceAccessService.RevokeWorkspaceInvitation(principal.TenantID, invitationID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"invitation": newWorkspaceInvitationResponse(invite)})
}

// ── Workspace Service Accounts ──────────────────────────────────────

func (s *Server) requireServiceAccounts(w http.ResponseWriter, r *http.Request) (Principal, bool) {
	if s.serviceAccountService == nil {
		writeError(w, http.StatusServiceUnavailable, "service accounts are not configured")
		return Principal{}, false
	}
	principal, ok := principalFromContext(r.Context())
	if !ok || principal.Scope != ScopeTenant {
		writeError(w, http.StatusForbidden, "forbidden")
		return Principal{}, false
	}
	return principal, true
}

func (s *Server) listServiceAccounts(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.requireServiceAccounts(w, r)
	if !ok {
		return
	}
	items, err := s.serviceAccountService.ListWorkspaceServiceAccounts(principal.TenantID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": newWorkspaceServiceAccountResponses(items)})
}

func (s *Server) createServiceAccount(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.requireServiceAccounts(w, r)
	if !ok {
		return
	}
	var req struct {
		Name                   string     `json:"name"`
		Description            string     `json:"description"`
		Role                   string     `json:"role"`
		Purpose                string     `json:"purpose"`
		Environment            string     `json:"environment"`
		IssueInitialCredential *bool      `json:"issue_initial_credential,omitempty"`
		CredentialName         string     `json:"credential_name,omitempty"`
		ExpiresAt              *time.Time `json:"expires_at,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	issueInitial := true
	if req.IssueInitialCredential != nil {
		issueInitial = *req.IssueInitialCredential
	}
	created, err := s.serviceAccountService.CreateWorkspaceServiceAccount(principal.TenantID, service.APICredentialActor{
		UserID: strings.TrimSpace(principal.SubjectID),
	}, service.CreateServiceAccountRequest{
		Name:                   req.Name,
		Description:            req.Description,
		Role:                   req.Role,
		Purpose:                req.Purpose,
		Environment:            req.Environment,
		IssueInitialCredential: issueInitial,
		CredentialName:         req.CredentialName,
		ExpiresAt:              req.ExpiresAt,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	payload := map[string]any{"service_account": newWorkspaceServiceAccountResponse(service.WorkspaceServiceAccount{ServiceAccount: created.ServiceAccount})}
	if created.Credential != nil {
		payload["credential"] = created.Credential
		payload["secret"] = created.Secret
	}
	writeJSON(w, http.StatusCreated, payload)
}

func (s *Server) updateServiceAccountStatus(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.requireServiceAccounts(w, r)
	if !ok {
		return
	}
	serviceAccountID := urlParam(r, "id")
	if serviceAccountID == "" {
		writeError(w, http.StatusBadRequest, "service account id is required")
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated, err := s.serviceAccountService.SetWorkspaceServiceAccountStatus(principal.TenantID, serviceAccountID, req.Status)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"service_account": newWorkspaceServiceAccountResponse(service.WorkspaceServiceAccount{ServiceAccount: updated}),
	})
}

func (s *Server) listServiceAccountAudit(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.requireServiceAccounts(w, r)
	if !ok {
		return
	}
	serviceAccountID := urlParam(r, "id")
	if serviceAccountID == "" {
		writeError(w, http.StatusBadRequest, "service account id is required")
		return
	}
	account, err := s.serviceAccountService.GetWorkspaceServiceAccount(principal.TenantID, serviceAccountID)
	if err != nil {
		writeDomainError(w, err)
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
	req := service.ListAPIKeyAuditEventsRequest{
		Action:  r.URL.Query().Get("action"),
		OwnerID: serviceAccountID,
		Limit:   limit,
		Offset:  offset,
		Cursor:  r.URL.Query().Get("cursor"),
	}
	if strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("format")), "csv") {
		csvData, err := s.apiKeyService.GenerateAPIKeyAuditCSV(principal.TenantID, req)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=service_account_%s_audit.csv", account.ID))
		_, _ = w.Write([]byte(csvData))
		return
	}
	events, err := s.apiKeyService.ListAPIKeyAuditEvents(principal.TenantID, req)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"service_account": newWorkspaceServiceAccountResponse(service.WorkspaceServiceAccount{ServiceAccount: account}),
		"items":           events.Items,
		"total":           events.Total,
		"limit":           events.Limit,
		"offset":          events.Offset,
		"next_cursor":     events.NextCursor,
	})
}

func (s *Server) listServiceAccountAuditExports(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.requireServiceAccounts(w, r)
	if !ok {
		return
	}
	if s.auditExportSvc == nil {
		writeError(w, http.StatusNotImplemented, "audit export service not configured")
		return
	}
	serviceAccountID := urlParam(r, "id")
	if serviceAccountID == "" {
		writeError(w, http.StatusBadRequest, "service account id is required")
		return
	}
	account, err := s.serviceAccountService.GetWorkspaceServiceAccount(principal.TenantID, serviceAccountID)
	if err != nil {
		writeDomainError(w, err)
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
	list, err := s.auditExportSvc.ListJobs(principal.TenantID, service.ListAuditExportJobsRequest{
		Status:  r.URL.Query().Get("status"),
		OwnerID: serviceAccountID,
		Limit:   limit,
		Offset:  offset,
		Cursor:  r.URL.Query().Get("cursor"),
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"service_account": newWorkspaceServiceAccountResponse(service.WorkspaceServiceAccount{ServiceAccount: account}),
		"items":           list.Items,
		"total":           list.Total,
		"limit":           list.Limit,
		"offset":          list.Offset,
		"next_cursor":     list.NextCursor,
	})
}

func (s *Server) getServiceAccountAuditExport(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.requireServiceAccounts(w, r)
	if !ok {
		return
	}
	if s.auditExportSvc == nil {
		writeError(w, http.StatusNotImplemented, "audit export service not configured")
		return
	}
	serviceAccountID := urlParam(r, "id")
	if serviceAccountID == "" {
		writeError(w, http.StatusBadRequest, "service account id is required")
		return
	}
	account, err := s.serviceAccountService.GetWorkspaceServiceAccount(principal.TenantID, serviceAccountID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	jobID := urlParam(r, "exportId")
	if jobID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	resp, err := s.auditExportSvc.GetJob(principal.TenantID, jobID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if strings.TrimSpace(resp.Job.Filters.OwnerID) != serviceAccountID {
		writeError(w, http.StatusNotFound, "audit export job not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"service_account": newWorkspaceServiceAccountResponse(service.WorkspaceServiceAccount{ServiceAccount: account}),
		"job":             resp.Job,
		"download_url":    resp.DownloadURL,
	})
}

func (s *Server) createServiceAccountAuditExport(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.requireServiceAccounts(w, r)
	if !ok {
		return
	}
	if s.auditExportSvc == nil {
		writeError(w, http.StatusNotImplemented, "audit export service not configured")
		return
	}
	serviceAccountID := urlParam(r, "id")
	if serviceAccountID == "" {
		writeError(w, http.StatusBadRequest, "service account id is required")
		return
	}
	account, err := s.serviceAccountService.GetWorkspaceServiceAccount(principal.TenantID, serviceAccountID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	var req struct {
		IdempotencyKey string `json:"idempotency_key"`
		Action         string `json:"action,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	job, idempotent, err := s.auditExportSvc.CreateJob(principal.TenantID, requestActorAPIKeyID(r), service.CreateAuditExportJobRequest{
		IdempotencyKey: req.IdempotencyKey,
		Action:         req.Action,
		OwnerID:        serviceAccountID,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	status := http.StatusCreated
	if idempotent {
		status = http.StatusOK
	}
	writeJSON(w, status, map[string]any{
		"service_account":    newWorkspaceServiceAccountResponse(service.WorkspaceServiceAccount{ServiceAccount: account}),
		"idempotent_request": idempotent,
		"job":                job,
	})
}

func (s *Server) issueServiceAccountCredential(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.requireServiceAccounts(w, r)
	if !ok {
		return
	}
	serviceAccountID := urlParam(r, "id")
	if serviceAccountID == "" {
		writeError(w, http.StatusBadRequest, "service account id is required")
		return
	}
	var req struct {
		Name      string     `json:"name,omitempty"`
		ExpiresAt *time.Time `json:"expires_at,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	issued, err := s.serviceAccountService.IssueWorkspaceServiceAccountCredential(principal.TenantID, serviceAccountID, service.APICredentialActor{
		UserID: strings.TrimSpace(principal.SubjectID),
	}, service.IssueServiceAccountCredentialRequest{
		Name:      req.Name,
		ExpiresAt: req.ExpiresAt,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"credential": issued.APIKey,
		"secret":     issued.Secret,
	})
}

func (s *Server) rotateServiceAccountCredential(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.requireServiceAccounts(w, r)
	if !ok {
		return
	}
	serviceAccountID := urlParam(r, "id")
	if serviceAccountID == "" {
		writeError(w, http.StatusBadRequest, "service account id is required")
		return
	}
	credentialID := urlParam(r, "credentialId")
	if credentialID == "" {
		writeError(w, http.StatusBadRequest, "credential id is required")
		return
	}
	rotated, err := s.serviceAccountService.RotateWorkspaceServiceAccountCredential(principal.TenantID, serviceAccountID, credentialID, service.APICredentialActor{UserID: strings.TrimSpace(principal.SubjectID)})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"credential": rotated.APIKey, "secret": rotated.Secret})
}

func (s *Server) revokeServiceAccountCredential(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.requireServiceAccounts(w, r)
	if !ok {
		return
	}
	serviceAccountID := urlParam(r, "id")
	if serviceAccountID == "" {
		writeError(w, http.StatusBadRequest, "service account id is required")
		return
	}
	credentialID := urlParam(r, "credentialId")
	if credentialID == "" {
		writeError(w, http.StatusBadRequest, "credential id is required")
		return
	}
	revoked, err := s.serviceAccountService.RevokeWorkspaceServiceAccountCredential(principal.TenantID, serviceAccountID, credentialID, service.APICredentialActor{UserID: strings.TrimSpace(principal.SubjectID)})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"credential": revoked})
}

// ---------------------------------------------------------------------------
// Workspace settings (tenant-scoped)
// ---------------------------------------------------------------------------

func (s *Server) getWorkspaceSettings(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok || principal.TenantID == "" {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	tenant, err := s.repo.GetTenant(principal.TenantID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	resp := map[string]any{
		"workspace": map[string]any{
			"id":         tenant.ID,
			"name":       tenant.Name,
			"status":     tenant.Status,
			"created_at": tenant.CreatedAt,
			"updated_at": tenant.UpdatedAt,
		},
	}
	if s.workspaceBillingSettingsService != nil {
		resp["billing_settings"] = s.buildWorkspaceBillingSettingsResponse(principal.TenantID)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) updateWorkspaceSettings(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok || principal.TenantID == "" {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	var req struct {
		Name                 *string  `json:"name,omitempty"`
		BillingEntityCode    *string  `json:"billing_entity_code,omitempty"`
		NetPaymentTermDays   *int     `json:"net_payment_term_days,omitempty"`
		TaxCodes             []string `json:"tax_codes,omitempty"`
		InvoiceMemo          *string  `json:"invoice_memo,omitempty"`
		InvoiceFooter        *string  `json:"invoice_footer,omitempty"`
		DocumentLocale       *string  `json:"document_locale,omitempty"`
		InvoiceGracePeriodDays *int   `json:"invoice_grace_period_days,omitempty"`
		DocumentNumbering    *string  `json:"document_numbering,omitempty"`
		DocumentNumberPrefix *string  `json:"document_number_prefix,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Update workspace name if provided.
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			writeError(w, http.StatusBadRequest, "workspace name cannot be empty")
			return
		}
		tenant, err := s.repo.GetTenant(principal.TenantID)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		tenant.Name = name
		tenant.UpdatedAt = time.Now().UTC()
		if _, err := s.repo.UpdateTenant(tenant); err != nil {
			writeDomainError(w, err)
			return
		}
	}

	// Update billing settings if any billing field is provided.
	if s.workspaceBillingSettingsService != nil {
		ds := func(p *string) string {
			if p == nil {
				return ""
			}
			return *p
		}
		billingReq := service.UpdateWorkspaceBillingSettingsRequest{
			BillingEntityCode:      ds(req.BillingEntityCode),
			NetPaymentTermDays:     req.NetPaymentTermDays,
			TaxCodes:               req.TaxCodes,
			InvoiceMemo:            ds(req.InvoiceMemo),
			InvoiceFooter:          ds(req.InvoiceFooter),
			DocumentLocale:         ds(req.DocumentLocale),
			InvoiceGracePeriodDays: req.InvoiceGracePeriodDays,
			DocumentNumbering:      ds(req.DocumentNumbering),
			DocumentNumberPrefix:   ds(req.DocumentNumberPrefix),
		}
		if _, err := s.workspaceBillingSettingsService.UpsertWorkspaceBillingSettings(r.Context(), principal.TenantID, billingReq); err != nil {
			writeDomainError(w, err)
			return
		}
	}

	// Return fresh state.
	s.getWorkspaceSettings(w, r)
}

// ---------------------------------------------------------------------------
// Workspace billing connection (tenant-scoped Stripe setup)
// ---------------------------------------------------------------------------

type billingConnectionResponse struct {
	ID            string     `json:"id"`
	ProviderType  string     `json:"provider_type"`
	Environment   string     `json:"environment"`
	DisplayName   string     `json:"display_name"`
	Status        string     `json:"status"`
	SecretSet     bool       `json:"secret_configured"`
	ConnectedAt   *time.Time `json:"connected_at,omitempty"`
	LastSyncedAt  *time.Time `json:"last_synced_at,omitempty"`
	LastSyncError string     `json:"last_sync_error,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

func buildBillingConnectionResponse(c domain.BillingProviderConnection) billingConnectionResponse {
	return billingConnectionResponse{
		ID:            c.ID,
		ProviderType:  string(c.ProviderType),
		Environment:   c.Environment,
		DisplayName:   c.DisplayName,
		Status:        string(c.Status),
		SecretSet:     c.SecretRef != "",
		ConnectedAt:   c.ConnectedAt,
		LastSyncedAt:  c.LastSyncedAt,
		LastSyncError: c.LastSyncError,
		CreatedAt:     c.CreatedAt,
	}
}

func (s *Server) getWorkspaceBillingConnection(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok || principal.TenantID == "" {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if s.billingProviderConnectionService == nil {
		writeJSON(w, http.StatusOK, map[string]any{"connection": nil})
		return
	}

	// Find the tenant's billing provider connection.
	tenant, err := s.repo.GetTenant(principal.TenantID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if tenant.BillingProviderConnectionID == "" {
		writeJSON(w, http.StatusOK, map[string]any{"connection": nil})
		return
	}
	conn, err := s.billingProviderConnectionService.GetBillingProviderConnection(tenant.BillingProviderConnectionID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"connection": nil})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"connection": buildBillingConnectionResponse(conn)})
}

func (s *Server) createWorkspaceBillingConnection(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok || principal.TenantID == "" {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if s.billingProviderConnectionService == nil {
		writeError(w, http.StatusServiceUnavailable, "billing provider connections are not configured")
		return
	}

	var req struct {
		StripeSecretKey string `json:"stripe_secret_key"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	key := strings.TrimSpace(req.StripeSecretKey)
	if key == "" {
		writeError(w, http.StatusBadRequest, "stripe_secret_key is required")
		return
	}
	if !strings.HasPrefix(key, "sk_test_") && !strings.HasPrefix(key, "sk_live_") {
		writeError(w, http.StatusBadRequest, "stripe_secret_key must start with sk_test_ or sk_live_")
		return
	}

	env := "test"
	if strings.HasPrefix(key, "sk_live_") {
		env = "live"
	}

	// 1. Create connection.
	conn, err := s.billingProviderConnectionService.CreateBillingProviderConnection(r.Context(), service.CreateBillingProviderConnectionRequest{
		ProviderType:    string(domain.BillingProviderTypeStripe),
		Environment:     env,
		DisplayName:     fmt.Sprintf("Stripe (%s)", env),
		Scope:           string(domain.BillingProviderConnectionScopeTenant),
		OwnerTenantID:   principal.TenantID,
		StripeSecretKey: key,
	}, strings.TrimSpace(principal.SubjectType), strings.TrimSpace(principal.SubjectID))
	if err != nil {
		writeDomainError(w, err)
		return
	}

	// 2. Verify (sync) the connection.
	conn, err = s.billingProviderConnectionService.SyncBillingProviderConnection(r.Context(), conn.ID)
	if err != nil {
		// Connection created but verification failed — still return the connection.
		s.logWarn("billing connection verification failed",
			"component", "api", "connection_id", conn.ID, "tenant_id", principal.TenantID, "error", err)
	}

	// 3. If connected, bind to workspace + update tenant.
	if conn.Status == domain.BillingProviderConnectionStatusConnected {
		if s.workspaceBillingBindingService != nil {
			_, _, bindErr := s.workspaceBillingBindingService.EnsureWorkspaceBillingBinding(service.EnsureWorkspaceBillingBindingRequest{
				WorkspaceID:                 principal.TenantID,
				BillingProviderConnectionID: conn.ID,
				Backend:                     "stripe",
				IsolationMode:               "shared",
			})
			if bindErr != nil {
				s.logWarn("billing connection binding failed",
					"component", "api", "connection_id", conn.ID, "tenant_id", principal.TenantID, "error", bindErr)
			}
		}
		// Update tenant reference.
		if tenant, err := s.repo.GetTenant(principal.TenantID); err == nil {
			tenant.BillingProviderConnectionID = conn.ID
			tenant.UpdatedAt = time.Now().UTC()
			_, _ = s.repo.UpdateTenant(tenant)
		}
	}

	writeJSON(w, http.StatusCreated, map[string]any{"connection": buildBillingConnectionResponse(conn)})
}

func (s *Server) verifyWorkspaceBillingConnection(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok || principal.TenantID == "" {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if s.billingProviderConnectionService == nil {
		writeError(w, http.StatusServiceUnavailable, "billing provider connections are not configured")
		return
	}

	tenant, err := s.repo.GetTenant(principal.TenantID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if tenant.BillingProviderConnectionID == "" {
		writeError(w, http.StatusNotFound, "no billing connection configured")
		return
	}

	conn, err := s.billingProviderConnectionService.SyncBillingProviderConnection(r.Context(), tenant.BillingProviderConnectionID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"connection": buildBillingConnectionResponse(conn)})
}
