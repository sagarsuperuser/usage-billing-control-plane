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

func parseTenantWorkspaceSubresource(path, prefix string) (id string, action string) {
	tail := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	if tail == "" {
		return "", ""
	}
	parts := strings.Split(tail, "/")
	id = strings.TrimSpace(parts[0])
	if len(parts) > 1 {
		action = strings.TrimSpace(parts[1])
	}
	return id, action
}

func parseTenantWorkspaceServiceAccountSubresource(path string) (id string, remainder []string) {
	tail := strings.Trim(strings.TrimPrefix(path, "/v1/workspace/service-accounts"), "/")
	if tail == "" {
		return "", nil
	}
	parts := strings.Split(tail, "/")
	id = strings.TrimSpace(parts[0])
	if len(parts) > 1 {
		remainder = parts[1:]
	}
	return id, remainder
}

func (s *Server) handleTenantWorkspaceMembers(w http.ResponseWriter, r *http.Request) {
	if s.workspaceAccessService == nil {
		writeError(w, http.StatusServiceUnavailable, "workspace access is not configured")
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok || principal.Scope != ScopeTenant {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	userID, _ := parseTenantWorkspaceSubresource(r.URL.Path, "/v1/workspace/members")
	switch r.Method {
	case http.MethodGet:
		if userID != "" {
			writeMethodNotAllowed(w)
			return
		}
		items, err := s.workspaceAccessService.ListWorkspaceMembers(principal.TenantID)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	case http.MethodPatch:
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
	case http.MethodDelete:
		if userID == "" {
			writeError(w, http.StatusBadRequest, "user id is required")
			return
		}
		if err := s.workspaceAccessService.RemoveWorkspaceMemberWithAudit(principal.TenantID, userID, workspaceAccessAuditActorFromPrincipal(principal, strings.TrimSpace(r.URL.Query().Get("reason")))); err != nil {
			writeDomainError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeMethodNotAllowed(w)
	}
}

func (s *Server) handleTenantWorkspaceServiceAccounts(w http.ResponseWriter, r *http.Request) {
	if s.serviceAccountService == nil {
		writeError(w, http.StatusServiceUnavailable, "service accounts are not configured")
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok || principal.Scope != ScopeTenant {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	serviceAccountID, remainder := parseTenantWorkspaceServiceAccountSubresource(r.URL.Path)
	switch r.Method {
	case http.MethodGet:
		if serviceAccountID == "" && len(remainder) == 0 {
			items, err := s.serviceAccountService.ListWorkspaceServiceAccounts(principal.TenantID)
			if err != nil {
				writeDomainError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"items": newWorkspaceServiceAccountResponses(items)})
			return
		}
		if serviceAccountID != "" && len(remainder) == 1 && remainder[0] == "audit" {
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
			return
		}
		if serviceAccountID != "" && len(remainder) == 2 && remainder[0] == "audit" && remainder[1] == "exports" {
			if s.auditExportSvc == nil {
				writeError(w, http.StatusNotImplemented, "audit export service not configured")
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
			return
		}
		if serviceAccountID != "" && len(remainder) == 3 && remainder[0] == "audit" && remainder[1] == "exports" {
			if s.auditExportSvc == nil {
				writeError(w, http.StatusNotImplemented, "audit export service not configured")
				return
			}
			account, err := s.serviceAccountService.GetWorkspaceServiceAccount(principal.TenantID, serviceAccountID)
			if err != nil {
				writeDomainError(w, err)
				return
			}
			jobID := strings.TrimSpace(remainder[2])
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
			return
		}
		writeMethodNotAllowed(w)
	case http.MethodPost:
		if serviceAccountID == "" {
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
			return
		}
		if len(remainder) == 2 && remainder[0] == "audit" && remainder[1] == "exports" {
			if s.auditExportSvc == nil {
				writeError(w, http.StatusNotImplemented, "audit export service not configured")
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
			return
		}
		if len(remainder) == 1 && remainder[0] == "credentials" {
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
			return
		}
		if len(remainder) == 3 && remainder[0] == "credentials" {
			credentialID := strings.TrimSpace(remainder[1])
			action := strings.TrimSpace(remainder[2])
			switch action {
			case "rotate":
				rotated, err := s.serviceAccountService.RotateWorkspaceServiceAccountCredential(principal.TenantID, serviceAccountID, credentialID, service.APICredentialActor{UserID: strings.TrimSpace(principal.SubjectID)})
				if err != nil {
					writeDomainError(w, err)
					return
				}
				writeJSON(w, http.StatusOK, map[string]any{"credential": rotated.APIKey, "secret": rotated.Secret})
				return
			case "revoke":
				revoked, err := s.serviceAccountService.RevokeWorkspaceServiceAccountCredential(principal.TenantID, serviceAccountID, credentialID, service.APICredentialActor{UserID: strings.TrimSpace(principal.SubjectID)})
				if err != nil {
					writeDomainError(w, err)
					return
				}
				writeJSON(w, http.StatusOK, map[string]any{"credential": revoked})
				return
			}
		}
		writeMethodNotAllowed(w)
	case http.MethodPatch:
		if serviceAccountID == "" || len(remainder) > 0 {
			writeMethodNotAllowed(w)
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
	default:
		writeMethodNotAllowed(w)
	}
}

func (s *Server) handleTenantWorkspaceInvitations(w http.ResponseWriter, r *http.Request) {
	if s.workspaceAccessService == nil {
		writeError(w, http.StatusServiceUnavailable, "workspace access is not configured")
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok || principal.Scope != ScopeTenant {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	invitationID, action := parseTenantWorkspaceSubresource(r.URL.Path, "/v1/workspace/invitations")
	switch r.Method {
	case http.MethodGet:
		if invitationID != "" || action != "" {
			writeMethodNotAllowed(w)
			return
		}
		items, err := s.workspaceAccessService.ListWorkspaceInvitations(principal.TenantID, strings.TrimSpace(r.URL.Query().Get("status")))
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": newWorkspaceInvitationResponses(items)})
	case http.MethodPost:
		if invitationID != "" || action != "" {
			if invitationID != "" && action == "revoke" {
				invite, err := s.workspaceAccessService.RevokeWorkspaceInvitation(principal.TenantID, invitationID)
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
	default:
		writeMethodNotAllowed(w)
	}
}
