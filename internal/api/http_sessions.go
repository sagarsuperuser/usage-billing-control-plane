package api

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/service"
	"usage-billing-control-plane/internal/store"
)

type browserWorkspaceOptionResponse struct {
	TenantID string `json:"tenant_id"`
	Name     string `json:"name"`
	Role     string `json:"role"`
}

type passwordResetRequestedResponse struct {
	Requested bool `json:"requested"`
}

type workspaceInvitationPreviewResponse struct {
	Invitation          workspaceInvitationResponse `json:"invitation"`
	WorkspaceName       string                      `json:"workspace_name"`
	RequiresLogin       bool                        `json:"requires_login"`
	Authenticated       bool                        `json:"authenticated"`
	CurrentUserEmail    string                      `json:"current_user_email,omitempty"`
	EmailMatchesSession bool                        `json:"email_matches_session"`
	CanAccept           bool                        `json:"can_accept"`
	AccountExists       bool                        `json:"account_exists"`
}

type uiSessionLoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
	TenantID string `json:"tenant_id"`
	Next     string `json:"next"`
}

type uiInvitationRegisterRequest struct {
	DisplayName string `json:"display_name"`
	Password    string `json:"password" validate:"required"`
}

type uiPasswordForgotRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type uiPasswordResetRequest struct {
	Token    string `json:"token" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type uiWorkspaceSelectRequest struct {
	TenantID string `json:"tenant_id" validate:"required"`
}

func newBrowserWorkspaceOptionResponses(items []service.BrowserWorkspaceOption) []browserWorkspaceOptionResponse {
	out := make([]browserWorkspaceOptionResponse, 0, len(items))
	for _, item := range items {
		out = append(out, browserWorkspaceOptionResponse{
			TenantID: item.TenantID,
			Name:     item.Name,
			Role:     item.Role,
		})
	}
	return out
}

func parseUISSOPath(path string) (providerKey string, action string) {
	tail := strings.Trim(strings.TrimPrefix(path, "/v1/ui/auth/sso/"), "/")
	if tail == "" {
		return "", ""
	}
	parts := strings.Split(tail, "/")
	if len(parts) < 2 {
		return strings.TrimSpace(parts[0]), ""
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}

func parseUIInvitationPath(path string) (token string, action string) {
	tail := strings.Trim(strings.TrimPrefix(path, "/v1/ui/invitations/"), "/")
	if tail == "" {
		return "", ""
	}
	parts := strings.Split(tail, "/")
	if len(parts) < 2 {
		return strings.TrimSpace(parts[0]), ""
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}

func invitationTokenFromNextPath(nextPath string) string {
	nextPath = normalizeUINextPath(nextPath)
	if strings.HasPrefix(nextPath, "/invite/") {
		token := strings.TrimSpace(strings.TrimPrefix(nextPath, "/invite/"))
		if token != "" && !strings.Contains(token, "/") {
			return token
		}
		return ""
	}
	if !strings.HasPrefix(nextPath, "/v1/ui/invitations/") {
		return ""
	}
	token, action := parseUIInvitationPath(nextPath)
	if token == "" {
		return ""
	}
	if action != "" && action != "accept" && action != "register" {
		return ""
	}
	return token
}

func normalizeUINextPath(nextPath string) string {
	nextPath = strings.TrimSpace(nextPath)
	if nextPath == "" || nextPath == "/" {
		return "/"
	}
	if strings.HasPrefix(nextPath, "http://") || strings.HasPrefix(nextPath, "https://") || strings.HasPrefix(nextPath, "//") {
		return "/"
	}
	if !strings.HasPrefix(nextPath, "/") {
		nextPath = "/" + nextPath
	}
	return nextPath
}

func (s *Server) handleUIPreAuthRateLimitProbe(w http.ResponseWriter, r *http.Request) {
	if s.rateLimiter != nil {
		if !s.enforceRateLimit(w, r, RateLimitPolicyPreAuthLogin, preAuthLoginRateLimitIdentifier(r), "", s.rateLimitLoginFailOpen) {
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleUIAuthProviders(w http.ResponseWriter, r *http.Request) {
	providers := make([]map[string]any, 0)
	if s.browserSSOService != nil {
		for _, provider := range s.browserSSOService.ListProviders() {
			providers = append(providers, map[string]any{
				"key":          strings.TrimSpace(provider.Key),
				"display_name": strings.TrimSpace(provider.DisplayName),
				"type":         strings.TrimSpace(string(provider.Type)),
			})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"password_enabled":       true,
		"password_reset_enabled": s.passwordResetService != nil && s.canSendPasswordResetEmail(),
		"sso_providers":          providers,
	})
}

func (s *Server) handleUIPasswordForgot(w http.ResponseWriter, r *http.Request) {
	if s.rateLimiter != nil {
		if !s.enforceRateLimit(w, r, RateLimitPolicyPreAuthLogin, preAuthLoginRateLimitIdentifier(r), "", s.rateLimitLoginFailOpen) {
			return
		}
	}
	if s.passwordResetService == nil || !s.canSendPasswordResetEmail() {
		writeError(w, http.StatusServiceUnavailable, "password reset is not configured")
		return
	}
	var req uiPasswordForgotRequest
	if err := decodeAndValidate(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	issued, err := s.passwordResetService.IssuePasswordReset(req.Email)
	switch {
	case err == nil:
		s.sendPasswordResetEmail(issued)
	case errors.Is(err, store.ErrNotFound):
		// Keep the response neutral to avoid leaking account existence.
	default:
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, passwordResetRequestedResponse{Requested: true})
}

func (s *Server) handleUIPasswordReset(w http.ResponseWriter, r *http.Request) {
	if s.rateLimiter != nil {
		if !s.enforceRateLimit(w, r, RateLimitPolicyPreAuthLogin, preAuthLoginRateLimitIdentifier(r), "", s.rateLimitLoginFailOpen) {
			return
		}
	}
	if s.passwordResetService == nil {
		writeError(w, http.StatusServiceUnavailable, "password reset is not configured")
		return
	}
	var req uiPasswordResetRequest
	if err := decodeAndValidate(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Token = strings.TrimSpace(req.Token)
	req.Password = strings.TrimSpace(req.Password)
	user, err := s.passwordResetService.ResetPassword(req.Token, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusNotFound, "password reset token not found")
		case errors.Is(err, service.ErrPasswordResetTokenExpired):
			writeError(w, http.StatusGone, "password reset token expired")
		case errors.Is(err, service.ErrPasswordResetTokenUsed):
			writeError(w, http.StatusGone, "password reset token already used")
		default:
			writeDomainError(w, err)
		}
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"reset": true,
		"user": map[string]any{
			"email":        user.Email,
			"display_name": user.DisplayName,
		},
	})
}

func (s *Server) handleUISessionWorkspaces(w http.ResponseWriter, r *http.Request) {
	if s.sessionManager == nil || s.browserUserAuthService == nil {
		writeError(w, http.StatusServiceUnavailable, "ui sessions are not configured")
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	items, err := s.browserUserAuthService.ListWorkspaceOptions(principal.SubjectID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":             newBrowserWorkspaceOptionResponses(items),
		"current_tenant_id": normalizeTenantID(principal.TenantID),
	})
}

func (s *Server) handleUISessionSwitchWorkspace(w http.ResponseWriter, r *http.Request) {
	if s.sessionManager == nil || s.browserUserAuthService == nil {
		writeError(w, http.StatusServiceUnavailable, "ui sessions are not configured")
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	expectedCSRF := strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionCSRFKey))
	providedCSRF := strings.TrimSpace(r.Header.Get(csrfHeaderName))
	if expectedCSRF == "" || providedCSRF == "" || subtle.ConstantTimeCompare([]byte(expectedCSRF), []byte(providedCSRF)) != 1 {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	var req uiWorkspaceSelectRequest
	if err := decodeAndValidate(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	user, err := s.repo.GetUser(principal.SubjectID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	resolved, err := s.browserUserAuthService.ResolveUserPrincipal(user, req.TenantID)
	if err != nil {
		if errors.Is(err, service.ErrBrowserTenantAccessDenied) {
			writeError(w, http.StatusForbidden, "workspace access denied")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to switch workspace")
		return
	}
	if err := s.sessionManager.RenewToken(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to renew session")
		return
	}
	csrfToken, err := randomHexToken(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to initialize session")
		return
	}
	newPrincipal := Principal{
		SubjectType: "user",
		SubjectID:   resolved.User.ID,
		UserEmail:   resolved.User.Email,
		Scope:       ScopeTenant,
		Role:        Role(resolved.Role),
		TenantID:    normalizeTenantID(resolved.TenantID),
	}
	s.putUISessionPrincipal(r.Context(), newPrincipal, csrfToken)
	writeJSON(w, http.StatusOK, buildUISessionResponse(newPrincipal, csrfToken, time.Now().UTC().Add(s.sessionManager.Lifetime)))
}

func (s *Server) handleUIInvitations(w http.ResponseWriter, r *http.Request) {
	token, action := parseUIInvitationPath(r.URL.Path)
	if token == "" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	switch action {
	case "":
		s.handleUIInvitationPreview(w, r, token)
	case "accept":
		s.handleUIInvitationAccept(w, r, token)
	case "register":
		s.handleUIInvitationRegister(w, r, token)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func (s *Server) handleUIInvitationPreview(w http.ResponseWriter, r *http.Request, token string) {
	if s.workspaceAccessService == nil {
		writeError(w, http.StatusServiceUnavailable, "workspace access is not configured")
		return
	}
	currentUserEmail := ""
	if s.sessionManager != nil {
		currentUserEmail = strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionUserEmailKey))
	}
	preview, err := s.workspaceAccessService.PreviewWorkspaceInvitation(token, currentUserEmail)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusNotFound, "workspace invitation not found")
		case errors.Is(err, service.ErrWorkspaceInvitationExpired):
			writeError(w, http.StatusGone, "workspace invitation expired")
		case errors.Is(err, service.ErrWorkspaceInvitationRevoked):
			writeError(w, http.StatusGone, "workspace invitation revoked")
		case errors.Is(err, service.ErrWorkspaceInvitationAccepted):
			writeError(w, http.StatusGone, "workspace invitation already accepted")
		default:
			writeDomainError(w, err)
		}
		return
	}
	writeJSON(w, http.StatusOK, workspaceInvitationPreviewResponse{
		Invitation:          newWorkspaceInvitationResponse(preview.Invitation),
		WorkspaceName:       preview.WorkspaceName,
		RequiresLogin:       preview.RequiresLogin,
		Authenticated:       preview.Authenticated,
		CurrentUserEmail:    preview.CurrentUserEmail,
		EmailMatchesSession: preview.EmailMatchesSession,
		CanAccept:           preview.CanAccept,
		AccountExists:       preview.AccountExists,
	})
}

func (s *Server) handleUIInvitationAccept(w http.ResponseWriter, r *http.Request, token string) {
	if s.workspaceAccessService == nil || s.browserUserAuthService == nil || s.sessionManager == nil {
		writeError(w, http.StatusServiceUnavailable, "workspace access is not configured")
		return
	}
	expectedCSRF := strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionCSRFKey))
	providedCSRF := strings.TrimSpace(r.Header.Get(csrfHeaderName))
	if expectedCSRF == "" || providedCSRF == "" || subtle.ConstantTimeCompare([]byte(expectedCSRF), []byte(providedCSRF)) != 1 {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	userID := strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionSubjectIDKey))
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	invite, _, err := s.workspaceAccessService.AcceptWorkspaceInvitation(token, userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrWorkspaceInvitationEmailMismatch):
			writeError(w, http.StatusForbidden, "workspace invitation email mismatch")
		case errors.Is(err, service.ErrWorkspaceInvitationExpired):
			writeError(w, http.StatusGone, "workspace invitation expired")
		case errors.Is(err, service.ErrWorkspaceInvitationRevoked):
			writeError(w, http.StatusGone, "workspace invitation revoked")
		case errors.Is(err, service.ErrWorkspaceInvitationAccepted):
			writeError(w, http.StatusGone, "workspace invitation already accepted")
		default:
			writeDomainError(w, err)
		}
		return
	}
	user, err := s.repo.GetUser(userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	authResult, err := s.browserUserAuthService.ResolveUserPrincipal(user, invite.WorkspaceID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if err := s.sessionManager.RenewToken(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to renew session")
		return
	}
	csrfToken, err := randomHexToken(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to initialize session")
		return
	}
	sessionPrincipal := Principal{
		SubjectType: "user",
		SubjectID:   authResult.User.ID,
		UserEmail:   authResult.User.Email,
		Scope:       Scope(authResult.Scope),
	}
	if sessionPrincipal.Scope == ScopePlatform {
		sessionPrincipal.PlatformRole = PlatformRole(authResult.PlatformRole)
	} else {
		sessionPrincipal.Role = Role(authResult.Role)
		sessionPrincipal.TenantID = normalizeTenantID(authResult.TenantID)
	}
	s.putUISessionPrincipal(r.Context(), sessionPrincipal, csrfToken)
	writeJSON(w, http.StatusCreated, map[string]any{
		"invitation": newWorkspaceInvitationResponse(invite),
		"session":    buildUISessionResponse(sessionPrincipal, csrfToken, time.Now().UTC().Add(s.sessionManager.Lifetime)),
	})
}

func (s *Server) handleUIInvitationRegister(w http.ResponseWriter, r *http.Request, token string) {
	if s.workspaceAccessService == nil || s.browserUserAuthService == nil || s.sessionManager == nil || s.repo == nil {
		writeError(w, http.StatusServiceUnavailable, "workspace access is not configured")
		return
	}
	if s.rateLimiter != nil {
		if !s.enforceRateLimit(w, r, RateLimitPolicyPreAuthLogin, preAuthLoginRateLimitIdentifier(r), "", s.rateLimitLoginFailOpen) {
			return
		}
	}

	var req uiInvitationRegisterRequest
	if err := decodeAndValidate(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	req.Password = strings.TrimSpace(req.Password)

	user, invite, _, err := s.workspaceAccessService.RegisterInvitedUser(token, req.DisplayName, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrWorkspaceInvitationAccountExists):
			writeError(w, http.StatusConflict, "workspace invitation account already exists")
		case errors.Is(err, service.ErrWorkspaceInvitationExpired):
			writeError(w, http.StatusGone, "workspace invitation expired")
		case errors.Is(err, service.ErrWorkspaceInvitationRevoked):
			writeError(w, http.StatusGone, "workspace invitation revoked")
		case errors.Is(err, service.ErrWorkspaceInvitationAccepted):
			writeError(w, http.StatusGone, "workspace invitation already accepted")
		default:
			writeDomainError(w, err)
		}
		return
	}

	authResult, err := s.browserUserAuthService.ResolveUserPrincipal(user, invite.WorkspaceID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if err := s.sessionManager.RenewToken(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to renew session")
		return
	}
	csrfToken, err := randomHexToken(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to initialize session")
		return
	}
	sessionPrincipal := Principal{
		SubjectType: "user",
		SubjectID:   authResult.User.ID,
		UserEmail:   authResult.User.Email,
		Scope:       Scope(authResult.Scope),
	}
	if sessionPrincipal.Scope == ScopePlatform {
		sessionPrincipal.PlatformRole = PlatformRole(authResult.PlatformRole)
	} else {
		sessionPrincipal.Role = Role(authResult.Role)
		sessionPrincipal.TenantID = normalizeTenantID(authResult.TenantID)
	}
	s.putUISessionPrincipal(r.Context(), sessionPrincipal, csrfToken)
	writeJSON(w, http.StatusCreated, map[string]any{
		"invitation": newWorkspaceInvitationResponse(invite),
		"session":    buildUISessionResponse(sessionPrincipal, csrfToken, time.Now().UTC().Add(s.sessionManager.Lifetime)),
	})
}

func (s *Server) handleUISSO(w http.ResponseWriter, r *http.Request) {
	providerKey, action := parseUISSOPath(r.URL.Path)
	switch action {
	case "start":
		s.handleUISSOStart(w, r, providerKey)
	case "callback":
		s.handleUISSOCallback(w, r, providerKey)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func (s *Server) handleUISSOStart(w http.ResponseWriter, r *http.Request, providerKey string) {
	if s.sessionManager == nil || s.browserSSOService == nil {
		writeError(w, http.StatusNotFound, "sso is not configured")
		return
	}
	if s.rateLimiter != nil {
		if !s.enforceRateLimit(w, r, RateLimitPolicyPreAuthLogin, preAuthLoginRateLimitIdentifier(r), "", s.rateLimitLoginFailOpen) {
			return
		}
	}

	state, err := randomURLToken(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to initialize sso")
		return
	}
	nonce, err := randomURLToken(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to initialize sso")
		return
	}
	codeVerifier, err := randomURLToken(48)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to initialize sso")
		return
	}
	redirectURI := s.uiSSOCallbackURL(r, providerKey)
	authURL, err := s.browserSSOService.BuildStartURL(providerKey, state, nonce, service.BuildPKCECodeChallenge(codeVerifier), redirectURI)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrBrowserSSOProviderNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, "failed to initialize sso provider")
		return
	}
	if err := s.sessionManager.RenewToken(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to initialize sso session")
		return
	}
	s.sessionManager.Put(r.Context(), sessionSSOStateKey, state)
	s.sessionManager.Put(r.Context(), sessionSSOProviderKey, strings.ToLower(strings.TrimSpace(providerKey)))
	s.sessionManager.Put(r.Context(), sessionSSONonceKey, nonce)
	s.sessionManager.Put(r.Context(), sessionSSOPKCEKey, codeVerifier)
	s.sessionManager.Put(r.Context(), sessionSSONextKey, normalizeUINextPath(strings.TrimSpace(r.URL.Query().Get("next"))))
	s.sessionManager.Put(r.Context(), sessionSSOTenantIDKey, strings.TrimSpace(r.URL.Query().Get("tenant_id")))
	http.Redirect(w, r, authURL, http.StatusFound)
}

func (s *Server) handleUISSOCallback(w http.ResponseWriter, r *http.Request, providerKey string) {
	if s.sessionManager == nil || s.browserSSOService == nil || strings.TrimSpace(s.uiPublicBaseURL) == "" {
		writeError(w, http.StatusNotFound, "sso is not configured")
		return
	}

	query := r.URL.Query()
	if errCode := strings.TrimSpace(query.Get("error")); errCode != "" {
		s.redirectUISSOFailure(w, r, strings.TrimSpace(providerKey), "sso_denied")
		return
	}
	code := strings.TrimSpace(query.Get("code"))
	state := strings.TrimSpace(query.Get("state"))
	if code == "" || state == "" {
		s.redirectUISSOFailure(w, r, strings.TrimSpace(providerKey), "sso_invalid_callback")
		return
	}

	expectedState := strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionSSOStateKey))
	expectedProvider := strings.ToLower(strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionSSOProviderKey)))
	nonce := strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionSSONonceKey))
	codeVerifier := strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionSSOPKCEKey))
	tenantID := strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionSSOTenantIDKey))
	nextPath := normalizeUINextPath(strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionSSONextKey)))
	if expectedState == "" || subtle.ConstantTimeCompare([]byte(expectedState), []byte(state)) != 1 || expectedProvider != strings.ToLower(strings.TrimSpace(providerKey)) || codeVerifier == "" {
		s.clearSSOSessionState(r.Context())
		s.redirectUISSOFailure(w, r, strings.TrimSpace(providerKey), "sso_state_invalid")
		return
	}

	principal, err := s.browserSSOService.AuthenticateCallback(r.Context(), providerKey, code, codeVerifier, nonce, tenantID, s.uiSSOCallbackURL(r, providerKey), invitationTokenFromNextPath(nextPath))
	s.clearSSOSessionState(r.Context())
	if err != nil {
		var accessDeniedErr service.BrowserTenantAccessDeniedError
		if errors.As(err, &accessDeniedErr) && strings.HasPrefix(nextPath, "/invite/") {
			// Invitation screen handles its own auth — redirect directly.
			http.Redirect(w, r, s.uiNextURL(nextPath), http.StatusFound)
			return
		}
		s.redirectUISSOFailure(w, r, strings.TrimSpace(providerKey), s.uiSSOErrorCode(err))
		return
	}

	sessionPrincipal := Principal{
		SubjectType: "user",
		SubjectID:   principal.User.ID,
		UserEmail:   principal.User.Email,
		Scope:       Scope(principal.Scope),
	}
	if sessionPrincipal.Scope == ScopePlatform {
		sessionPrincipal.PlatformRole = PlatformRole(principal.PlatformRole)
	} else {
		sessionPrincipal.Role = Role(principal.Role)
		sessionPrincipal.TenantID = normalizeTenantID(principal.TenantID)
	}
	if err := s.sessionManager.RenewToken(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to renew session")
		return
	}
	csrfToken, err := randomHexToken(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to initialize session")
		return
	}
	s.putUISessionPrincipal(r.Context(), sessionPrincipal, csrfToken)
	http.Redirect(w, r, s.uiNextURL(nextPath), http.StatusFound)
}

func (s *Server) handleUISessionLogin(w http.ResponseWriter, r *http.Request) {
	if s.rateLimiter != nil {
		if !s.enforceRateLimit(w, r, RateLimitPolicyPreAuthLogin, preAuthLoginRateLimitIdentifier(r), "", s.rateLimitLoginFailOpen) {
			return
		}
	}
	if s.sessionManager == nil {
		writeError(w, http.StatusServiceUnavailable, "ui sessions are not configured")
		return
	}

	var req uiSessionLoginRequest
	if err := decodeAndValidate(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Password = strings.TrimSpace(req.Password)
	req.TenantID = strings.TrimSpace(req.TenantID)
	req.Next = normalizeUINextPath(strings.TrimSpace(req.Next))

	if s.browserUserAuthService == nil {
		writeError(w, http.StatusServiceUnavailable, "browser user auth is not configured")
		return
	}
	user, err := s.browserUserAuthService.AuthenticateIdentity(req.Email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidBrowserCredentials), errors.Is(err, service.ErrBrowserPasswordUnavailable):
			writeError(w, http.StatusUnauthorized, "invalid credentials")
		case errors.Is(err, service.ErrBrowserUserDisabled):
			writeError(w, http.StatusForbidden, "user disabled")
		default:
			writeError(w, http.StatusInternalServerError, "failed to authenticate browser user")
		}
		return
	}
	authResult, err := s.browserUserAuthService.ResolveUserPrincipal(user, req.TenantID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrBrowserTenantAccessDenied):
			writeError(w, http.StatusForbidden, "tenant access denied")
		default:
			writeError(w, http.StatusInternalServerError, "failed to authenticate browser user")
		}
		return
	}
	principal := Principal{
		SubjectType: "user",
		SubjectID:   authResult.User.ID,
		UserEmail:   authResult.User.Email,
		Scope:       Scope(authResult.Scope),
	}
	if principal.Scope == ScopePlatform {
		principal.PlatformRole = PlatformRole(authResult.PlatformRole)
	} else {
		principal.Role = Role(authResult.Role)
		principal.TenantID = normalizeTenantID(authResult.TenantID)
	}

	if err := s.sessionManager.RenewToken(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to renew session")
		return
	}

	csrfToken, err := randomHexToken(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to initialize session")
		return
	}

	s.putUISessionPrincipal(r.Context(), principal, csrfToken)

	writeJSON(w, http.StatusCreated, buildUISessionResponse(principal, csrfToken, time.Now().UTC().Add(s.sessionManager.Lifetime)))
}

func (s *Server) handleUISessionMe(w http.ResponseWriter, r *http.Request) {
	if s.sessionManager == nil {
		writeError(w, http.StatusServiceUnavailable, "ui sessions are not configured")
		return
	}

	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	csrfToken := strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionCSRFKey))
	if csrfToken == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	writeJSON(w, http.StatusOK, buildUISessionResponse(principal, csrfToken, time.Time{}))
}

func (s *Server) handleUISessionLogout(w http.ResponseWriter, r *http.Request) {
	if s.sessionManager == nil {
		writeError(w, http.StatusServiceUnavailable, "ui sessions are not configured")
		return
	}
	s.sessionManager.Destroy(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"logged_out": true})
}

func buildUISessionResponse(principal Principal, csrfToken string, expiresAt time.Time) map[string]any {
	resp := map[string]any{
		"authenticated": true,
		"subject_type":  strings.TrimSpace(principal.SubjectType),
		"subject_id":    strings.TrimSpace(principal.SubjectID),
		"user_email":    strings.TrimSpace(principal.UserEmail),
		"scope":         principal.Scope,
		"api_key_id":    strings.TrimSpace(principal.APIKeyID),
		"csrf_token":    csrfToken,
	}
	if !expiresAt.IsZero() {
		resp["expires_at"] = expiresAt
	}
	if principal.Scope == ScopePlatform {
		resp["platform_role"] = principal.PlatformRole
	} else {
		resp["role"] = principal.Role
		resp["tenant_id"] = normalizeTenantID(principal.TenantID)
	}
	return resp
}

func (s *Server) putUISessionPrincipal(ctx context.Context, principal Principal, csrfToken string) {
	s.sessionManager.Put(ctx, sessionSubjectTypeKey, strings.TrimSpace(principal.SubjectType))
	s.sessionManager.Put(ctx, sessionSubjectIDKey, strings.TrimSpace(principal.SubjectID))
	s.sessionManager.Put(ctx, sessionUserEmailKey, strings.TrimSpace(principal.UserEmail))
	s.sessionManager.Put(ctx, sessionScopeKey, string(principal.Scope))
	if principal.Scope == ScopePlatform {
		s.sessionManager.Remove(ctx, sessionRoleKey)
		s.sessionManager.Put(ctx, sessionPlatformRoleKey, string(principal.PlatformRole))
		s.sessionManager.Remove(ctx, sessionTenantIDKey)
	} else {
		s.sessionManager.Put(ctx, sessionRoleKey, string(principal.Role))
		s.sessionManager.Remove(ctx, sessionPlatformRoleKey)
		s.sessionManager.Put(ctx, sessionTenantIDKey, normalizeTenantID(principal.TenantID))
	}
	s.sessionManager.Put(ctx, sessionAPIKeyIDKey, strings.TrimSpace(principal.APIKeyID))
	s.sessionManager.Put(ctx, sessionCSRFKey, csrfToken)
}

func (s *Server) clearSSOSessionState(ctx context.Context) {
	s.sessionManager.Remove(ctx, sessionSSOStateKey)
	s.sessionManager.Remove(ctx, sessionSSOProviderKey)
	s.sessionManager.Remove(ctx, sessionSSONonceKey)
	s.sessionManager.Remove(ctx, sessionSSOPKCEKey)
	s.sessionManager.Remove(ctx, sessionSSONextKey)
	s.sessionManager.Remove(ctx, sessionSSOTenantIDKey)
}

func (s *Server) uiSSOCallbackURL(r *http.Request, providerKey string) string {
	baseURL := externalBaseURL(r)
	return strings.TrimRight(baseURL, "/") + "/v1/ui/auth/sso/" + url.PathEscape(strings.ToLower(strings.TrimSpace(providerKey))) + "/callback"
}

func (s *Server) uiNextURL(nextPath string) string {
	return strings.TrimRight(s.uiPublicBaseURL, "/") + normalizeUINextPath(nextPath)
}

func (s *Server) workspaceInvitationAcceptURL(token string) (string, string) {
	acceptPath := "/invite/" + url.PathEscape(strings.TrimSpace(token))
	acceptURL := strings.TrimRight(s.uiPublicBaseURL, "/") + acceptPath
	if strings.TrimSpace(s.uiPublicBaseURL) == "" {
		acceptURL = acceptPath
	}
	return acceptPath, acceptURL
}

func (s *Server) uiPasswordResetURL(token string) string {
	target, err := url.Parse(strings.TrimRight(s.uiPublicBaseURL, "/") + "/reset-password")
	if err != nil {
		return "/reset-password?token=" + url.QueryEscape(strings.TrimSpace(token))
	}
	query := target.Query()
	query.Set("token", strings.TrimSpace(token))
	target.RawQuery = query.Encode()
	return target.String()
}

func (s *Server) uiSSOErrorCode(err error) string {
	switch {
	case errors.Is(err, service.ErrBrowserSSOProviderNotFound):
		return "sso_provider_not_found"
	case errors.Is(err, service.ErrBrowserSSOEmailRequired):
		return "sso_email_required"
	case errors.Is(err, service.ErrBrowserSSOEmailNotVerified):
		return "sso_email_not_verified"
	case errors.Is(err, service.ErrBrowserSSOInviteEmailMismatch):
		return "workspace_invitation_email_mismatch"
	case errors.Is(err, service.ErrBrowserSSOUserNotProvisioned):
		return "sso_user_not_provisioned"
	case errors.Is(err, service.ErrBrowserTenantAccessDenied):
		return "tenant_access_denied"
	case errors.Is(err, service.ErrBrowserUserDisabled):
		return "user_disabled"
	default:
		return "sso_failed"
	}
}

func (s *Server) canSendWorkspaceInvitationEmail() bool {
	if s == nil {
		return false
	}
	if s.notificationService != nil && s.notificationService.CanSendWorkspaceInvitations() {
		return true
	}
	return s.workspaceInvitationEmailSender != nil
}

func (s *Server) canSendPasswordResetEmail() bool {
	if s == nil {
		return false
	}
	if s.notificationService != nil && s.notificationService.CanSendPasswordReset() {
		return true
	}
	return s.passwordResetEmailSender != nil
}

func (s *Server) sendWorkspaceInvitationEmail(workspaceID, invitedByEmail string, issued service.IssuedWorkspaceInvitation) {
	if s == nil || !s.canSendWorkspaceInvitationEmail() {
		return
	}
	workspaceName := workspaceID
	if s.repo != nil {
		if tenant, err := s.repo.GetTenant(workspaceID); err == nil {
			if trimmed := strings.TrimSpace(tenant.Name); trimmed != "" {
				workspaceName = trimmed
			}
		}
	}
	_, acceptURL := s.workspaceInvitationAcceptURL(issued.Token)
	input := service.WorkspaceInvitationEmail{
		ToEmail:        issued.Invitation.Email,
		WorkspaceName:  workspaceName,
		Role:           issued.Invitation.Role,
		AcceptURL:      acceptURL,
		ExpiresAt:      issued.Invitation.ExpiresAt,
		InvitedByEmail: invitedByEmail,
	}
	var err error
	if s.notificationService != nil && s.notificationService.CanSendWorkspaceInvitations() {
		err = s.notificationService.SendWorkspaceInvitation(input)
	} else {
		err = s.workspaceInvitationEmailSender.SendWorkspaceInvitation(input)
	}
	if err != nil && s.logger != nil {
		s.logger.Warn(
			"workspace invitation email delivery failed",
			"component", "server",
			"workspace_id", workspaceID,
			"invitation_id", issued.Invitation.ID,
			"email", issued.Invitation.Email,
			"error", err,
		)
	}
}

func (s *Server) sendPasswordResetEmail(issued service.PasswordResetIssueResult) {
	if s == nil || !s.canSendPasswordResetEmail() {
		return
	}
	input := service.PasswordResetEmail{
		ToEmail:   issued.UserEmail,
		ResetURL:  s.uiPasswordResetURL(issued.RawToken),
		ExpiresAt: issued.Token.ExpiresAt,
	}
	var err error
	if s.notificationService != nil && s.notificationService.CanSendPasswordReset() {
		err = s.notificationService.SendPasswordReset(input)
	} else {
		err = s.passwordResetEmailSender.SendPasswordReset(input)
	}
	if err != nil && s.logger != nil {
		s.logger.Warn(
			"password reset email delivery failed",
			"component", "server",
			"user_email", issued.UserEmail,
			"reset_token_id", issued.Token.ID,
			"error", err,
		)
	}
}

// ---------------------------------------------------------------------------
// Self-registration + workspace creation (self-serve tenant flow)
// ---------------------------------------------------------------------------

func (s *Server) handleUIRegister(w http.ResponseWriter, r *http.Request) {
	if s.browserUserAuthService == nil || s.repo == nil {
		writeError(w, http.StatusServiceUnavailable, "registration is not configured")
		return
	}

	var req struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))
	password := strings.TrimSpace(req.Password)
	displayName := strings.TrimSpace(req.DisplayName)
	if email == "" || !strings.Contains(email, "@") {
		writeError(w, http.StatusBadRequest, "valid email is required")
		return
	}
	if len(password) < 12 {
		writeError(w, http.StatusBadRequest, "password must be at least 12 characters")
		return
	}
	if displayName == "" {
		displayName = strings.Split(email, "@")[0]
	}

	// Create user (or recover orphaned user from a previously failed registration).
	user, err := s.repo.CreateUser(domain.User{
		Email:       email,
		DisplayName: displayName,
		Status:      domain.UserStatusActive,
	})
	if err != nil {
		if errors.Is(err, store.ErrAlreadyExists) {
			// Check if user has any workspace — if not, this is an orphaned
			// registration (user created but tenant failed). Resume the flow.
			existing, lookupErr := s.browserUserAuthService.AuthenticateIdentity(email, password)
			if lookupErr != nil {
				writeError(w, http.StatusConflict, "an account with this email already exists")
				return
			}
			memberships, _ := s.repo.ListUserTenantMemberships(existing.ID)
			if len(memberships) > 0 {
				writeError(w, http.StatusConflict, "an account with this email already exists")
				return
			}
			// Orphaned user — continue with workspace creation
			user = existing
		} else {
			writeDomainError(w, err)
			return
		}
	} else {
		// New user — set password.
		passwordHash, hashErr := service.HashPassword(password)
		if hashErr != nil {
			writeDomainError(w, hashErr)
			return
		}
		_, err = s.repo.UpsertUserPasswordCredential(domain.UserPasswordCredential{
			UserID:       user.ID,
			PasswordHash: passwordHash,
		})
		if err != nil {
			writeDomainError(w, err)
			return
		}
	}

	// Auto-create workspace.
	workspaceName := displayName + "'s workspace"
	tenant, err := s.tenantService.CreateTenant(service.EnsureTenantRequest{
		Name: workspaceName,
	}, "")
	if err != nil {
		writeDomainError(w, err)
		return
	}

	// Auto-create admin membership.
	_, err = s.repo.UpsertUserTenantMembership(domain.UserTenantMembership{
		UserID:   user.ID,
		TenantID: tenant.ID,
		Role:     "admin",
		Status:   "active",
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}

	// Create session with tenant scope.
	if err := s.sessionManager.RenewToken(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}
	csrfToken, err := randomHexToken(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate csrf token")
		return
	}
	s.putUISessionPrincipal(r.Context(), Principal{
		SubjectType:  "user",
		SubjectID:    user.ID,
		UserEmail:    user.Email,
		Scope:        ScopeTenant,
		TenantID:     tenant.ID,
		Role:         RoleAdmin,
	}, csrfToken)

	writeJSON(w, http.StatusCreated, map[string]any{
		"registered":   true,
		"user_id":      user.ID,
		"email":        user.Email,
		"workspace_id": tenant.ID,
		"csrf_token":   csrfToken,
		"next_path":    "/control-plane",
	})
}

func (s *Server) handleUISelfServeCreateWorkspace(w http.ResponseWriter, r *http.Request) {

	principal, ok := principalFromContext(r.Context())
	if !ok || principal.SubjectID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "workspace name is required")
		return
	}

	// Create workspace.
	tenant, err := s.tenantService.CreateTenant(service.EnsureTenantRequest{
		Name: name,
	}, "")
	if err != nil {
		writeDomainError(w, err)
		return
	}

	// Create admin membership for the current user.
	_, err = s.repo.UpsertUserTenantMembership(domain.UserTenantMembership{
		UserID:   principal.SubjectID,
		TenantID: tenant.ID,
		Role:     "admin",
		Status:   "active",
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}

	// Upgrade session to tenant scope.
	if err := s.sessionManager.RenewToken(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to upgrade session")
		return
	}
	csrfToken, err := randomHexToken(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate csrf token")
		return
	}
	s.putUISessionPrincipal(r.Context(), Principal{
		SubjectType:  principal.SubjectType,
		SubjectID:    principal.SubjectID,
		UserEmail:    principal.UserEmail,
		Scope:        ScopeTenant,
		TenantID:     tenant.ID,
		Role:         RoleAdmin,
	}, csrfToken)

	writeJSON(w, http.StatusCreated, map[string]any{
		"workspace_id":   tenant.ID,
		"workspace_name": tenant.Name,
		"csrf_token":     csrfToken,
		"next_path":      "/control-plane",
	})
}

func (s *Server) redirectUISSOFailure(w http.ResponseWriter, r *http.Request, providerKey, errorCode string) {
	target, err := url.Parse(strings.TrimRight(s.uiPublicBaseURL, "/") + "/login")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to redirect to login")
		return
	}
	query := target.Query()
	if providerKey = strings.TrimSpace(providerKey); providerKey != "" {
		query.Set("provider", strings.ToLower(providerKey))
	}
	if errorCode = strings.TrimSpace(errorCode); errorCode != "" {
		query.Set("error", errorCode)
	}
	target.RawQuery = query.Encode()
	http.Redirect(w, r, target.String(), http.StatusFound)
}
