package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alexedwards/scs/v2"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/reconcile"
	"usage-billing-control-plane/internal/replay"
	"usage-billing-control-plane/internal/service"
	"usage-billing-control-plane/internal/store"
)

type Server struct {
	repo                               store.Repository
	ratingService                      *service.RatingService
	tenantService                      *service.TenantService
	customerService                    *service.CustomerService
	customerPaymentSetupRequestService *service.CustomerPaymentSetupRequestService
	customerOnboardingService          *service.CustomerOnboardingService
	billingProviderConnectionService   *service.BillingProviderConnectionService
	workspaceBillingBindingService     *service.WorkspaceBillingBindingService
	workspaceBillingSettingsService    *service.WorkspaceBillingSettingsService
	workspaceAccessService             *service.WorkspaceAccessService
	serviceAccountService              *service.ServiceAccountService
	notificationService                *service.NotificationService
	dunningService                     *service.DunningService
	workspaceInvitationEmailSender     service.WorkspaceInvitationEmailSender
	passwordResetService               *service.PasswordResetService
	passwordResetEmailSender           service.PasswordResetEmailSender
	browserUserAuthService             *service.BrowserUserAuthService
	browserSSOService                  *service.BrowserSSOService
	pricingMetricService               *service.PricingMetricService
	taxService                         *service.TaxService
	addOnService                       *service.AddOnService
	couponService                      *service.CouponService
	planService                        *service.PlanService
	subscriptionService                *service.SubscriptionService
	meterService                       *service.MeterService
	usageService                       *service.UsageService
	apiKeyService                      *service.APIKeyService
	onboardingService                  *service.TenantOnboardingService
	auditExportSvc                     *service.AuditExportService
	lagoOrganizationBootstrapper       service.LagoOrganizationBootstrapper
	meterSyncAdapter                   service.MeterSyncAdapter
	taxSyncAdapter                     service.TaxSyncAdapter
	planSyncAdapter                    service.PlanSyncAdapter
	subscriptionSyncAdapter            service.SubscriptionSyncAdapter
	usageSyncAdapter                   service.UsageSyncAdapter
	invoiceBillingAdapter              service.InvoiceBillingAdapter
	customerBillingAdapter             service.CustomerBillingAdapter
	lagoWebhookSvc                     *service.LagoWebhookService
	replayService                      *replay.Service
	recService                         *reconcile.Service
	authorizer                         APIKeyAuthorizer
	sessionManager                     *scs.SessionManager
	metricsFn                          func() map[string]any
	readinessFn                        func() error
	requestMetrics                     *requestMetricsCollector
	logger                             *slog.Logger
	rateLimiter                        RateLimiter
	rateLimitFailOpen                  bool
	rateLimitLoginFailOpen             bool
	requireSessionOriginCheck          bool
	allowedSessionOrigins              map[string]struct{}
	uiPublicBaseURL                    string
	mux                                *http.ServeMux
}

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

type browserWorkspaceOptionResponse struct {
	TenantID string `json:"tenant_id"`
	Name     string `json:"name"`
	Role     string `json:"role"`
}

type workspaceSelectionResponse struct {
	Required  bool                             `json:"required"`
	UserEmail string                           `json:"user_email,omitempty"`
	Items     []browserWorkspaceOptionResponse `json:"items,omitempty"`
	CSRFToken string                           `json:"csrf_token,omitempty"`
}

type pendingInvitationLoginResponse struct {
	PendingInvitation bool   `json:"pending_invitation"`
	NextPath          string `json:"next_path,omitempty"`
}

type passwordResetRequestedResponse struct {
	Requested bool `json:"requested"`
}

type billingNotificationDispatchResponse struct {
	DispatchedAt time.Time `json:"dispatched_at"`
	Dispatched   bool      `json:"dispatched"`
	Action       string    `json:"action"`
	Domain       string    `json:"domain"`
	Backend      string    `json:"backend"`
}

type resendInvoiceEmailRequest struct {
	To  []string `json:"to"`
	Cc  []string `json:"cc"`
	Bcc []string `json:"bcc"`
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

func newWorkspaceInvitationResponses(items []domain.WorkspaceInvitation) []workspaceInvitationResponse {
	out := make([]workspaceInvitationResponse, 0, len(items))
	for _, item := range items {
		out = append(out, newWorkspaceInvitationResponse(item))
	}
	return out
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

const (
	sessionSubjectTypeKey      = "principal_subject_type"
	sessionSubjectIDKey        = "principal_subject_id"
	sessionUserEmailKey        = "principal_user_email"
	sessionScopeKey            = "principal_scope"
	sessionRoleKey             = "principal_role"
	sessionPlatformRoleKey     = "principal_platform_role"
	sessionTenantIDKey         = "principal_tenant_id"
	sessionAPIKeyIDKey         = "principal_api_key_id"
	sessionCSRFKey             = "csrf_token"
	sessionSSOStateKey         = "ui_sso_state"
	sessionSSOProviderKey      = "ui_sso_provider"
	sessionSSONonceKey         = "ui_sso_nonce"
	sessionSSOPKCEKey          = "ui_sso_pkce_verifier"
	sessionSSONextKey          = "ui_sso_next"
	sessionSSOTenantIDKey      = "ui_sso_tenant_id"
	sessionPendingUserIDKey    = "ui_pending_workspace_user_id"
	sessionPendingUserEmailKey = "ui_pending_workspace_user_email"
	sessionPendingNextKey      = "ui_pending_workspace_next"
	csrfHeaderName             = "X-CSRF-Token"
	requestIDHeaderKey         = "X-Request-ID"
)

type requestMetricsCollector struct {
	mu                   sync.Mutex
	counts               map[string]int64
	tenantCounts         map[string]int64
	authDeniedCounts     map[string]map[string]int64
	rateLimitedCounts    map[string]map[string]int64
	rateLimitErrorCounts map[string]map[string]int64
}

func newRequestMetricsCollector() *requestMetricsCollector {
	return &requestMetricsCollector{
		counts:               make(map[string]int64),
		tenantCounts:         make(map[string]int64),
		authDeniedCounts:     make(map[string]map[string]int64),
		rateLimitedCounts:    make(map[string]map[string]int64),
		rateLimitErrorCounts: make(map[string]map[string]int64),
	}
}

func (c *requestMetricsCollector) Inc(method, route string, statusCode int) {
	if c == nil {
		return
	}
	key := fmt.Sprintf("%s %s %d", strings.ToUpper(strings.TrimSpace(method)), strings.TrimSpace(route), statusCode)
	c.mu.Lock()
	c.counts[key]++
	c.mu.Unlock()
}

func (c *requestMetricsCollector) Snapshot() map[string]int64 {
	if c == nil {
		return map[string]int64{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]int64, len(c.counts))
	for k, v := range c.counts {
		out[k] = v
	}
	return out
}

func (c *requestMetricsCollector) TenantSnapshot() map[string]int64 {
	if c == nil {
		return map[string]int64{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]int64, len(c.tenantCounts))
	for k, v := range c.tenantCounts {
		out[k] = v
	}
	return out
}

func (c *requestMetricsCollector) AuthDeniedSnapshot() map[string]map[string]int64 {
	if c == nil {
		return map[string]map[string]int64{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return cloneNestedCounterMap(c.authDeniedCounts)
}

func (c *requestMetricsCollector) RateLimitedSnapshot() map[string]map[string]int64 {
	if c == nil {
		return map[string]map[string]int64{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return cloneNestedCounterMap(c.rateLimitedCounts)
}

func (c *requestMetricsCollector) RateLimitErrorSnapshot() map[string]map[string]int64 {
	if c == nil {
		return map[string]map[string]int64{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return cloneNestedCounterMap(c.rateLimitErrorCounts)
}

func (c *requestMetricsCollector) IncTenant(tenantID string) {
	if c == nil {
		return
	}
	tenantID = normalizeTenantID(strings.TrimSpace(tenantID))
	c.mu.Lock()
	c.tenantCounts[tenantID]++
	c.mu.Unlock()
}

func (c *requestMetricsCollector) IncAuthDenied(tenantID, reason string) {
	if c == nil {
		return
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		tenantID = "unknown"
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "unknown"
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.authDeniedCounts[tenantID]; !ok {
		c.authDeniedCounts[tenantID] = make(map[string]int64)
	}
	c.authDeniedCounts[tenantID][reason]++
}

func (c *requestMetricsCollector) IncRateLimited(tenantID, policy string) {
	if c == nil {
		return
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		tenantID = "unknown"
	}
	policy = strings.TrimSpace(policy)
	if policy == "" {
		policy = "unknown"
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.rateLimitedCounts[tenantID]; !ok {
		c.rateLimitedCounts[tenantID] = make(map[string]int64)
	}
	c.rateLimitedCounts[tenantID][policy]++
}

func (c *requestMetricsCollector) IncRateLimitError(tenantID, policy string) {
	if c == nil {
		return
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		tenantID = "unknown"
	}
	policy = strings.TrimSpace(policy)
	if policy == "" {
		policy = "unknown"
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.rateLimitErrorCounts[tenantID]; !ok {
		c.rateLimitErrorCounts[tenantID] = make(map[string]int64)
	}
	c.rateLimitErrorCounts[tenantID][policy]++
}

func cloneNestedCounterMap(src map[string]map[string]int64) map[string]map[string]int64 {
	out := make(map[string]map[string]int64, len(src))
	for key, inner := range src {
		innerCopy := make(map[string]int64, len(inner))
		for innerKey, value := range inner {
			innerCopy[innerKey] = value
		}
		out[key] = innerCopy
	}
	return out
}

type statusCapturingResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (w *statusCapturingResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *statusCapturingResponseWriter) Write(p []byte) (int, error) {
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytesWritten += n
	return n, err
}

type ServerOption func(*Server)

func WithMetricsProvider(provider func() map[string]any) ServerOption {
	return func(s *Server) {
		s.metricsFn = provider
	}
}

func WithReadinessCheck(check func() error) ServerOption {
	return func(s *Server) {
		s.readinessFn = check
	}
}

func WithAPIKeyAuthorizer(authorizer APIKeyAuthorizer) ServerOption {
	return func(s *Server) {
		s.authorizer = authorizer
	}
}

func WithSessionManager(sessionManager *scs.SessionManager) ServerOption {
	return func(s *Server) {
		s.sessionManager = sessionManager
	}
}

func WithAuditExportService(auditExportSvc *service.AuditExportService) ServerOption {
	return func(s *Server) {
		s.auditExportSvc = auditExportSvc
	}
}

func WithLagoOrganizationBootstrapper(bootstrapper service.LagoOrganizationBootstrapper) ServerOption {
	return func(s *Server) {
		s.lagoOrganizationBootstrapper = bootstrapper
	}
}

func WithMeterSyncAdapter(adapter service.MeterSyncAdapter) ServerOption {
	return func(s *Server) {
		s.meterSyncAdapter = adapter
	}
}

func WithInvoiceBillingAdapter(adapter service.InvoiceBillingAdapter) ServerOption {
	return func(s *Server) {
		s.invoiceBillingAdapter = adapter
	}
}

func WithCustomerBillingAdapter(adapter service.CustomerBillingAdapter) ServerOption {
	return func(s *Server) {
		s.customerBillingAdapter = adapter
	}
}

func WithPlanSyncAdapter(adapter service.PlanSyncAdapter) ServerOption {
	return func(s *Server) {
		s.planSyncAdapter = adapter
	}
}

func WithSubscriptionSyncAdapter(adapter service.SubscriptionSyncAdapter) ServerOption {
	return func(s *Server) {
		s.subscriptionSyncAdapter = adapter
	}
}

func WithUsageSyncAdapter(adapter service.UsageSyncAdapter) ServerOption {
	return func(s *Server) {
		s.usageSyncAdapter = adapter
	}
}

func WithBillingProviderConnectionService(svc *service.BillingProviderConnectionService) ServerOption {
	return func(s *Server) {
		s.billingProviderConnectionService = svc
	}
}

func WithBrowserUserAuthService(svc *service.BrowserUserAuthService) ServerOption {
	return func(s *Server) {
		s.browserUserAuthService = svc
	}
}

func WithBrowserSSOService(svc *service.BrowserSSOService) ServerOption {
	return func(s *Server) {
		s.browserSSOService = svc
	}
}

func WithWorkspaceAccessService(svc *service.WorkspaceAccessService) ServerOption {
	return func(s *Server) {
		s.workspaceAccessService = svc
	}
}

func WithWorkspaceInvitationEmailSender(sender service.WorkspaceInvitationEmailSender) ServerOption {
	return func(s *Server) {
		s.workspaceInvitationEmailSender = sender
	}
}

func WithNotificationService(svc *service.NotificationService) ServerOption {
	return func(s *Server) {
		s.notificationService = svc
	}
}

func WithPasswordResetService(svc *service.PasswordResetService) ServerOption {
	return func(s *Server) {
		s.passwordResetService = svc
	}
}

func WithPasswordResetEmailSender(sender service.PasswordResetEmailSender) ServerOption {
	return func(s *Server) {
		s.passwordResetEmailSender = sender
	}
}

func WithUIPublicBaseURL(baseURL string) ServerOption {
	return func(s *Server) {
		s.uiPublicBaseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	}
}

func WithLagoWebhookService(lagoWebhookSvc *service.LagoWebhookService) ServerOption {
	return func(s *Server) {
		s.lagoWebhookSvc = lagoWebhookSvc
	}
}

func WithLogger(logger *slog.Logger) ServerOption {
	return func(s *Server) {
		s.logger = logger
	}
}

func WithTaxSyncAdapter(adapter service.TaxSyncAdapter) ServerOption {
	return func(s *Server) {
		s.taxSyncAdapter = adapter
	}
}

func WithRateLimiter(rateLimiter RateLimiter, failOpen bool, loginFailOpen bool) ServerOption {
	return func(s *Server) {
		s.rateLimiter = rateLimiter
		s.rateLimitFailOpen = failOpen
		s.rateLimitLoginFailOpen = loginFailOpen
	}
}

func WithSessionOriginPolicy(require bool, allowedOrigins []string) ServerOption {
	return func(s *Server) {
		s.requireSessionOriginCheck = require
		s.allowedSessionOrigins = make(map[string]struct{}, len(allowedOrigins))
		for _, origin := range allowedOrigins {
			normalized, ok := normalizeAbsoluteOrigin(origin)
			if !ok {
				continue
			}
			s.allowedSessionOrigins[normalized] = struct{}{}
		}
	}
}

type requestIDContextKey struct{}

var httpRequestIDContextKey requestIDContextKey

func NewServer(repo store.Repository, opts ...ServerOption) *Server {
	s := &Server{
		repo:                   repo,
		replayService:          replay.NewService(repo),
		recService:             reconcile.NewService(repo),
		requestMetrics:         newRequestMetricsCollector(),
		logger:                 slog.Default(),
		rateLimitFailOpen:      true,
		rateLimitLoginFailOpen: false,
		allowedSessionOrigins:  make(map[string]struct{}),
		mux:                    http.NewServeMux(),
	}
	for _, opt := range opts {
		opt(s)
	}
	s.ratingService = service.NewRatingService(repo)
	s.workspaceBillingBindingService = service.NewWorkspaceBillingBindingService(repo)
	s.workspaceBillingSettingsService = service.NewWorkspaceBillingSettingsService(repo)
	if adapter, ok := any(s.customerBillingAdapter).(service.BillingEntitySettingsSyncAdapter); ok {
		s.workspaceBillingSettingsService = s.workspaceBillingSettingsService.WithBillingEntitySyncAdapter(adapter)
	}
	s.workspaceAccessService = service.NewWorkspaceAccessService(repo)
	s.tenantService = service.NewTenantService(repo).
		WithWorkspaceBillingBindingService(s.workspaceBillingBindingService).
		WithBillingProviderConnectionService(s.billingProviderConnectionService).
		WithLagoOrganizationBootstrapper(s.lagoOrganizationBootstrapper)
	s.customerService = service.NewCustomerService(repo, s.customerBillingAdapter).WithWorkspaceBillingBindingService(s.workspaceBillingBindingService)
	s.customerPaymentSetupRequestService = service.NewCustomerPaymentSetupRequestService(repo, s.customerService, s.notificationService)
	if dunningSvc, err := service.NewDunningService(repo); err == nil {
		s.dunningService = dunningSvc.WithPaymentSetupRequestSender(s.customerPaymentSetupRequestService).WithInvoiceRetryExecutor(s.invoiceBillingAdapter)
	}
	s.customerOnboardingService = service.NewCustomerOnboardingService(s.customerService)
	s.meterService = service.NewMeterService(repo)
	s.pricingMetricService = service.NewPricingMetricService(s.ratingService, s.meterService)
	s.taxService = service.NewTaxService(repo).WithSyncAdapter(s.taxSyncAdapter)
	s.addOnService = service.NewAddOnService(repo)
	s.couponService = service.NewCouponService(repo)
	s.planService = service.NewPlanService(repo).WithPlanSyncAdapter(s.planSyncAdapter)
	s.subscriptionService = service.NewSubscriptionService(repo, s.customerService).WithSubscriptionSyncAdapter(s.subscriptionSyncAdapter)
	s.usageService = service.NewUsageService(repo).WithUsageSyncAdapter(s.usageSyncAdapter)
	s.apiKeyService = service.NewAPIKeyService(repo)
	s.serviceAccountService = service.NewServiceAccountService(repo, s.apiKeyService)
	if s.browserUserAuthService == nil {
		s.browserUserAuthService, _ = service.NewBrowserUserAuthService(repo)
	}
	s.onboardingService = service.NewTenantOnboardingService(s.tenantService, s.workspaceBillingBindingService, s.customerService, s.apiKeyService, s.serviceAccountService, s.ratingService, s.meterService)
	s.registerRoutes()
	return s
}

func (s *Server) Handler() http.Handler {
	var handler http.Handler = s.mux
	handler = s.accessLogMiddleware(handler)
	handler = s.authMiddleware(handler)
	handler = s.corsMiddleware(handler)
	handler = s.instrumentMiddleware(handler)
	handler = s.requestIDMiddleware(handler)
	return handler
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}

		normalizedOrigin, ok := normalizeAbsoluteOrigin(origin)
		if !ok || !s.isAllowedOrigin(normalizedOrigin, r) {
			if r.Method == http.MethodOptions {
				writeError(w, http.StatusForbidden, "forbidden")
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Add("Vary", "Origin")
		w.Header().Add("Vary", "Access-Control-Request-Method")
		w.Header().Add("Vary", "Access-Control-Request-Headers")
		w.Header().Set("Access-Control-Allow-Origin", normalizedOrigin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")

		requestedHeaders := strings.TrimSpace(r.Header.Get("Access-Control-Request-Headers"))
		if requestedHeaders == "" {
			requestedHeaders = "Content-Type, X-CSRF-Token, X-API-Key"
		}
		w.Header().Set("Access-Control-Allow-Headers", requestedHeaders)
		w.Header().Set("Access-Control-Expose-Headers", requestIDHeaderKey)

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) instrumentMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &statusCapturingResponseWriter{ResponseWriter: w}
		next.ServeHTTP(recorder, r)
		statusCode := recorder.statusCode
		if statusCode == 0 {
			statusCode = http.StatusOK
		}
		s.requestMetrics.Inc(r.Method, normalizeMetricsRoute(r.URL.Path), statusCode)
	})
}

func (s *Server) requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := normalizeRequestID(r.Header.Get(requestIDHeaderKey))
		if requestID == "" {
			token, err := randomHexToken(12)
			if err != nil {
				requestID = strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
			} else {
				requestID = token
			}
		}

		w.Header().Set(requestIDHeaderKey, requestID)
		next.ServeHTTP(w, r.WithContext(withRequestID(r.Context(), requestID)))
	})
}

func (s *Server) accessLogMiddleware(next http.Handler) http.Handler {
	if s.logger == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now().UTC()
		recorder := &statusCapturingResponseWriter{ResponseWriter: w}
		next.ServeHTTP(recorder, r)

		statusCode := recorder.statusCode
		if statusCode == 0 {
			statusCode = http.StatusOK
		}
		durationMs := time.Since(start).Milliseconds()
		attrs := []any{
			"component", "api",
			"event", "http_request",
			"request_id", requestIDFromContext(r.Context()),
			"method", r.Method,
			"route", normalizeMetricsRoute(r.URL.Path),
			"path", r.URL.Path,
			"status", statusCode,
			"duration_ms", durationMs,
			"bytes", recorder.bytesWritten,
			"auth_method", inferAuthMethod(r),
		}

		principal, ok := principalFromContext(r.Context())
		if ok {
			s.requestMetrics.IncTenant(metricsTenantKey(principal))
			attrs = append(attrs, "scope", string(principal.Scope))
			switch principal.Scope {
			case ScopePlatform:
				attrs = append(attrs, "platform_role", string(principal.PlatformRole))
			default:
				attrs = append(attrs,
					"tenant_id", normalizeTenantID(principal.TenantID),
					"role", string(principal.Role),
				)
			}
			if apiKeyID := strings.TrimSpace(principal.APIKeyID); apiKeyID != "" {
				attrs = append(attrs, "api_key_id", apiKeyID)
			}
		}

		switch {
		case statusCode >= http.StatusInternalServerError:
			s.logger.Error("http request", attrs...)
		case statusCode >= http.StatusBadRequest:
			s.logger.Warn("http request", attrs...)
		default:
			s.logger.Info("http request", attrs...)
		}
	})
}

func (s *Server) logAuthFailure(r *http.Request, statusCode int, reason string, err error) {
	if s.logger == nil {
		return
	}

	attrs := []any{
		"component", "api",
		"event", "auth_denied",
		"request_id", requestIDFromContext(r.Context()),
		"method", r.Method,
		"route", normalizeMetricsRoute(r.URL.Path),
		"path", r.URL.Path,
		"status", statusCode,
		"reason", reason,
		"auth_method", inferAuthMethod(r),
	}
	if err != nil {
		attrs = append(attrs, "error", err.Error())
	}
	tenantID := ""
	if principal, ok := principalFromContext(r.Context()); ok {
		tenantID = normalizeTenantID(principal.TenantID)
	} else {
		var blocked tenantBlockedError
		if errors.As(err, &blocked) {
			tenantID = normalizeTenantID(blocked.TenantID)
		}
	}
	s.requestMetrics.IncAuthDenied(tenantID, reason)
	if tenantID != "" {
		attrs = append(attrs, "tenant_id", tenantID)
	}

	if statusCode >= http.StatusInternalServerError {
		s.logger.Error("http auth denied", attrs...)
		return
	}
	s.logger.Warn("http auth denied", attrs...)
}

func (s *Server) logOnboardingFailure(r *http.Request, req service.TenantOnboardingRequest, err error) {
	if s.logger == nil || err == nil {
		return
	}

	bootstrapAdminKey := true
	if req.BootstrapAdminKey != nil {
		bootstrapAdminKey = *req.BootstrapAdminKey
	}

	attrs := []any{
		"component", "api",
		"event", "tenant_onboarding_failed",
		"request_id", requestIDFromContext(r.Context()),
		"route", normalizeMetricsRoute(r.URL.Path),
		"path", r.URL.Path,
		"method", r.Method,
		"auth_method", inferAuthMethod(r),
		"tenant_id", normalizeTenantID(req.ID),
		"billing_provider_connection_id", strings.TrimSpace(req.BillingProviderConnectionID),
		"bootstrap_admin_key", bootstrapAdminKey,
		"error_class", classifyDomainErrorKind(err),
		"error", err.Error(),
	}
	if apiKeyID := strings.TrimSpace(requestActorAPIKeyID(r)); apiKeyID != "" {
		attrs = append(attrs, "actor_api_key_id", apiKeyID)
	}
	var staged *service.TenantOnboardingStageError
	if errors.As(err, &staged) && strings.TrimSpace(staged.Stage) != "" {
		attrs = append(attrs, "stage", staged.Stage)
	}
	if principal, ok := principalFromContext(r.Context()); ok {
		attrs = append(attrs, "scope", string(principal.Scope))
		switch principal.Scope {
		case ScopePlatform:
			attrs = append(attrs, "platform_role", string(principal.PlatformRole))
		default:
			attrs = append(attrs,
				"tenant_scope_id", normalizeTenantID(principal.TenantID),
				"role", string(principal.Role),
			)
		}
	}
	s.logger.Error("tenant onboarding failed", attrs...)
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	if s.authorizer == nil && s.sessionManager == nil {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requiredRole, protected := requiredRoleForRequest(r)
		requiresPlatform := requiresPlatformScope(r)
		if policy, identifier, failOpen, ok := s.preAuthRateLimitTarget(r, protected); ok {
			if !s.enforceRateLimit(w, r, policy, identifier, "", failOpen) {
				return
			}
		}
		if !protected {
			next.ServeHTTP(w, r)
			return
		}

		principal, usingSession, err := s.authorizePrincipal(r)
		if err != nil {
			statusCode := http.StatusInternalServerError
			reason := "authorization_failed"
			if errors.Is(err, errUnauthorized) {
				statusCode = http.StatusUnauthorized
				reason = "unauthorized"
			} else if errors.Is(err, errTenantBlocked) {
				statusCode = http.StatusForbidden
				reason = "tenant_blocked"
			}
			s.logAuthFailure(r, statusCode, reason, err)
			writeAuthError(w, err)
			return
		}
		if requiresPlatform {
			if principal.Scope != ScopePlatform {
				s.logAuthFailure(r, http.StatusForbidden, "platform_scope_required", nil)
				writeError(w, http.StatusForbidden, "forbidden")
				return
			}
			if principal.PlatformRole != PlatformRoleAdmin {
				s.logAuthFailure(r, http.StatusForbidden, "insufficient_platform_role", nil)
				writeError(w, http.StatusForbidden, "forbidden")
				return
			}
		} else if isUISessionSelfRoute(r.URL.Path) {
			switch principal.Scope {
			case ScopePlatform:
				if principal.PlatformRole != PlatformRoleAdmin {
					s.logAuthFailure(r, http.StatusForbidden, "insufficient_platform_role", nil)
					writeError(w, http.StatusForbidden, "forbidden")
					return
				}
			case ScopeTenant:
				if roleRank(principal.Role) == 0 {
					s.logAuthFailure(r, http.StatusUnauthorized, "invalid_role", nil)
					writeError(w, http.StatusUnauthorized, "unauthorized")
					return
				}
				if !roleAllows(principal.Role, requiredRole) {
					s.logAuthFailure(r, http.StatusForbidden, "insufficient_role", nil)
					writeError(w, http.StatusForbidden, "forbidden")
					return
				}
			default:
				s.logAuthFailure(r, http.StatusForbidden, "session_scope_required", nil)
				writeError(w, http.StatusForbidden, "forbidden")
				return
			}
		} else {
			if principal.Scope != ScopeTenant {
				s.logAuthFailure(r, http.StatusForbidden, "tenant_scope_required", nil)
				writeError(w, http.StatusForbidden, "forbidden")
				return
			}
			if roleRank(principal.Role) == 0 {
				s.logAuthFailure(r, http.StatusUnauthorized, "invalid_role", nil)
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			if !roleAllows(principal.Role, requiredRole) {
				s.logAuthFailure(r, http.StatusForbidden, "insufficient_role", nil)
				writeError(w, http.StatusForbidden, "forbidden")
				return
			}
		}
		if usingSession && isUnsafeMethod(r.Method) {
			if s.requireSessionOriginCheck && !s.isAllowedSessionOrigin(r) {
				s.logAuthFailure(r, http.StatusForbidden, "origin_mismatch", nil)
				writeError(w, http.StatusForbidden, "forbidden")
				return
			}
			expectedCSRF := strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionCSRFKey))
			providedCSRF := strings.TrimSpace(r.Header.Get(csrfHeaderName))
			if expectedCSRF == "" || providedCSRF == "" || subtleConstantTimeMatch(expectedCSRF, providedCSRF) == false {
				s.logAuthFailure(r, http.StatusForbidden, "csrf_mismatch", nil)
				writeError(w, http.StatusForbidden, "forbidden")
				return
			}
		}

		if policy, identifier, failOpen, ok := s.authRateLimitTarget(r, principal, requiredRole, usingSession); ok {
			if !s.enforceRateLimit(w, r, policy, identifier, principal.TenantID, failOpen) {
				return
			}
		}

		next.ServeHTTP(w, r.WithContext(withPrincipal(r.Context(), principal)))
	})
}

func (s *Server) preAuthRateLimitTarget(r *http.Request, protected bool) (policy string, identifier string, failOpen bool, ok bool) {
	if s.rateLimiter == nil {
		return "", "", false, false
	}

	path := strings.TrimSpace(r.URL.Path)
	switch {
	case path == "/health":
		return "", "", false, false
	case path == "/internal/lago/webhooks":
		return RateLimitPolicyWebhook, "ip:" + requestClientIP(r), s.rateLimitFailOpen, true
	case protected:
		return RateLimitPolicyPreAuthProtected, "ip:" + requestClientIP(r) + ":route:" + normalizeMetricsRoute(path), s.rateLimitFailOpen, true
	default:
		return "", "", false, false
	}
}

func (s *Server) authRateLimitTarget(r *http.Request, principal Principal, requiredRole Role, usingSession bool) (policy string, identifier string, failOpen bool, ok bool) {
	if s.rateLimiter == nil {
		return "", "", false, false
	}

	identifier = authRateLimitIdentifier(r, principal, usingSession)
	if identifier == "" {
		return "", "", false, false
	}

	policy = authRateLimitPolicy(r, requiredRole)
	return policy, identifier, s.rateLimitFailOpen, true
}

func authRateLimitPolicy(r *http.Request, requiredRole Role) string {
	path := strings.TrimSpace(r.URL.Path)
	if strings.HasPrefix(path, "/internal/") {
		return RateLimitPolicyAuthInternal
	}
	switch requiredRole {
	case RoleAdmin:
		return RateLimitPolicyAuthAdmin
	case RoleWriter:
		return RateLimitPolicyAuthWrite
	default:
		return RateLimitPolicyAuthRead
	}
}

func authRateLimitIdentifier(r *http.Request, principal Principal, usingSession bool) string {
	base := "tenant:" + normalizeTenantID(principal.TenantID)
	if principal.Scope == ScopePlatform {
		base = "platform"
	}

	if usingSession {
		return base + ":session_ip:" + requestClientIP(r)
	}

	if apiKeyID := strings.TrimSpace(principal.APIKeyID); apiKeyID != "" {
		return base + ":api_key_id:" + apiKeyID
	}

	rawAPIKey := strings.TrimSpace(r.Header.Get(apiKeyHeader))
	if rawAPIKey != "" {
		return base + ":api_key_prefix:" + KeyPrefixFromHash(HashAPIKey(rawAPIKey))
	}

	role := strings.TrimSpace(strings.ToLower(string(principal.Role)))
	if role == "" {
		role = "unknown"
	}
	return base + ":role:" + role
}

func preAuthLoginRateLimitIdentifier(r *http.Request) string {
	return "ip:" + requestClientIP(r) + ":route:" + normalizeMetricsRoute(r.URL.Path)
}

func requestClientIP(r *http.Request) string {
	if r == nil {
		return "unknown"
	}

	if forwarded := strings.TrimSpace(forwardedValue(r.Header.Get("X-Forwarded-For"))); forwarded != "" {
		return sanitizeClientIP(forwarded)
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return sanitizeClientIP(realIP)
	}

	host := strings.TrimSpace(r.RemoteAddr)
	if host == "" {
		return "unknown"
	}
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		return sanitizeClientIP(parsedHost)
	}
	return sanitizeClientIP(host)
}

func sanitizeClientIP(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return "unknown"
	}
	return strings.ReplaceAll(raw, " ", "")
}

func (s *Server) enforceRateLimit(w http.ResponseWriter, r *http.Request, policy, identifier, tenantID string, failOpen bool) bool {
	if s.rateLimiter == nil {
		return true
	}

	decision, err := s.rateLimiter.Allow(r.Context(), RateLimitRequest{
		Policy:     policy,
		Identifier: identifier,
	})
	if err != nil {
		if s.logger != nil {
			s.logger.Warn(
				"rate limit check failed",
				"component", "api",
				"event", "rate_limit_error",
				"request_id", requestIDFromContext(r.Context()),
				"policy", policy,
				"fail_open", failOpen,
				"error", err.Error(),
			)
		}
		s.requestMetrics.IncRateLimitError(tenantID, policy)
		if failOpen {
			return true
		}
		writeError(w, http.StatusServiceUnavailable, "rate limiter unavailable")
		return false
	}

	writeRateLimitHeaders(w, decision)
	if decision.Allowed {
		return true
	}

	if retryAfter := retryAfterSeconds(decision.ResetAt); retryAfter > 0 {
		w.Header().Set("Retry-After", strconv.FormatInt(retryAfter, 10))
	}
	if s.logger != nil {
		s.logger.Warn(
			"rate limit exceeded",
			"component", "api",
			"event", "rate_limited",
			"request_id", requestIDFromContext(r.Context()),
			"policy", policy,
			"path", r.URL.Path,
			"method", r.Method,
		)
	}
	s.requestMetrics.IncRateLimited(tenantID, policy)
	writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
	return false
}

func writeRateLimitHeaders(w http.ResponseWriter, decision RateLimitDecision) {
	w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(decision.Limit, 10))
	w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(decision.Remaining, 10))
	if !decision.ResetAt.IsZero() {
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(decision.ResetAt.Unix(), 10))
	}
}

func retryAfterSeconds(resetAt time.Time) int64 {
	if resetAt.IsZero() {
		return 1
	}
	remaining := time.Until(resetAt)
	if remaining <= 0 {
		return 1
	}
	seconds := int64(remaining.Seconds())
	if remaining%time.Second != 0 {
		seconds++
	}
	if seconds < 1 {
		return 1
	}
	return seconds
}

func normalizeRequestID(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 128 {
		return ""
	}
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			continue
		}
		return ""
	}
	return raw
}

func withRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, httpRequestIDContextKey, requestID)
}

func requestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	requestID, _ := ctx.Value(httpRequestIDContextKey).(string)
	return strings.TrimSpace(requestID)
}

func inferAuthMethod(r *http.Request) string {
	if strings.TrimSpace(r.Header.Get(apiKeyHeader)) != "" {
		return "api_key"
	}
	if strings.TrimSpace(r.Header.Get("Cookie")) != "" {
		return "session"
	}
	return "none"
}

func ParseAllowedOrigins(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, item := range parts {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		normalized, ok := normalizeAbsoluteOrigin(item)
		if !ok {
			return nil, fmt.Errorf("invalid origin %q", item)
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	sort.Strings(out)
	return out, nil
}

func (s *Server) isAllowedSessionOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin != "" {
		normalized, ok := normalizeAbsoluteOrigin(origin)
		if !ok {
			return false
		}
		return s.isAllowedOrigin(normalized, r)
	}

	referer := strings.TrimSpace(r.Header.Get("Referer"))
	if referer == "" {
		return false
	}
	refURL, err := url.Parse(referer)
	if err != nil {
		return false
	}
	if refURL.Scheme == "" || refURL.Host == "" {
		return false
	}
	normalized, ok := normalizeAbsoluteOrigin(refURL.Scheme + "://" + refURL.Host)
	if !ok {
		return false
	}
	return s.isAllowedOrigin(normalized, r)
}

func (s *Server) isAllowedOrigin(origin string, r *http.Request) bool {
	if _, ok := s.allowedSessionOrigins[origin]; ok {
		return true
	}

	requestOrigin, ok := effectiveRequestOrigin(r)
	if !ok {
		return false
	}
	return strings.EqualFold(origin, requestOrigin)
}

func effectiveRequestOrigin(r *http.Request) (string, bool) {
	scheme := forwardedValue(r.Header.Get("X-Forwarded-Proto"))
	if scheme == "" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	scheme = strings.ToLower(strings.TrimSpace(scheme))
	if scheme != "http" && scheme != "https" {
		return "", false
	}

	host := forwardedValue(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = strings.TrimSpace(r.Host)
	}
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return "", false
	}

	return scheme + "://" + host, true
}

func forwardedValue(raw string) string {
	if raw == "" {
		return ""
	}
	parts := strings.Split(raw, ",")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}

func normalizeAbsoluteOrigin(raw string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", false
	}
	if u == nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", false
	}
	scheme := strings.ToLower(strings.TrimSpace(u.Scheme))
	if scheme != "http" && scheme != "https" {
		return "", false
	}
	host := strings.ToLower(strings.TrimSpace(u.Host))
	if host == "" {
		return "", false
	}
	path := strings.TrimSpace(u.EscapedPath())
	if path != "" && path != "/" {
		return "", false
	}
	return scheme + "://" + host, true
}

func (s *Server) authorizePrincipal(r *http.Request) (Principal, bool, error) {
	rawAPIKey := strings.TrimSpace(r.Header.Get(apiKeyHeader))
	if rawAPIKey != "" {
		if s.authorizer == nil {
			return Principal{}, false, errUnauthorized
		}
		principal, err := s.authorizer.Authorize(r)
		return principal, false, err
	}
	if s.sessionManager == nil {
		return Principal{}, false, errUnauthorized
	}

	subjectType := strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionSubjectTypeKey))
	subjectID := strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionSubjectIDKey))
	userEmail := strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionUserEmailKey))
	scopeRaw := strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionScopeKey))
	roleRaw := strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionRoleKey))
	platformRoleRaw := strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionPlatformRoleKey))
	tenantID := strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionTenantIDKey))
	apiKeyID := strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionAPIKeyIDKey))
	switch Scope(scopeRaw) {
	case ScopePlatform:
		platformRole, err := ParsePlatformRole(platformRoleRaw)
		if err != nil {
			return Principal{}, true, errUnauthorized
		}
		return Principal{
			SubjectType:  subjectType,
			SubjectID:    subjectID,
			UserEmail:    userEmail,
			Scope:        ScopePlatform,
			PlatformRole: platformRole,
			APIKeyID:     apiKeyID,
		}, true, nil
	case ScopeTenant, "":
		if roleRaw == "" || tenantID == "" {
			return Principal{}, true, errUnauthorized
		}
		role, err := ParseRole(roleRaw)
		if err != nil {
			return Principal{}, true, errUnauthorized
		}
		return Principal{
			SubjectType: subjectType,
			SubjectID:   subjectID,
			UserEmail:   userEmail,
			Scope:       ScopeTenant,
			Role:        role,
			TenantID:    normalizeTenantID(tenantID),
			APIKeyID:    apiKeyID,
		}, true, nil
	default:
		return Principal{}, true, errUnauthorized
	}
}

func isUnsafeMethod(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func subtleConstantTimeMatch(expected, provided string) bool {
	if len(expected) == 0 || len(expected) != len(provided) {
		return false
	}
	var diff byte
	for i := 0; i < len(expected); i++ {
		diff |= expected[i] ^ provided[i]
	}
	return diff == 0
}

func writeAuthError(w http.ResponseWriter, err error) {
	if errors.Is(err, errUnauthorized) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if errors.Is(err, errTenantBlocked) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	writeError(w, http.StatusInternalServerError, "authorization failed")
}

func requiredRoleForRequest(r *http.Request) (Role, bool) {
	path := r.URL.Path

	if path == "/health" {
		return "", false
	}
	if path == "/internal/metrics" {
		return RoleAdmin, true
	}
	if path == "/internal/ready" {
		return RoleAdmin, true
	}
	if path == "/internal/lago/webhooks" {
		return "", false
	}
	if path == "/internal/tenants/audit" {
		return RoleAdmin, true
	}
	if path == "/internal/billing-provider-connections" {
		return RoleAdmin, true
	}
	if strings.HasPrefix(path, "/internal/billing-provider-connections/") {
		return RoleAdmin, true
	}
	if path == "/internal/onboarding/tenants" {
		return RoleAdmin, true
	}
	if strings.HasPrefix(path, "/internal/onboarding/tenants/") {
		return RoleAdmin, true
	}
	if path == "/internal/tenants" {
		return RoleAdmin, true
	}
	if strings.HasPrefix(path, "/internal/tenants/") {
		return RoleAdmin, true
	}
	if path == "/v1/ui/sessions/login" {
		return "", false
	}
	if path == "/v1/ui/auth/providers" {
		return "", false
	}
	if path == "/v1/ui/password/forgot" {
		return "", false
	}
	if path == "/v1/ui/password/reset" {
		return "", false
	}
	if path == "/v1/ui/workspaces/pending" {
		return "", false
	}
	if path == "/v1/ui/workspaces/select" {
		return "", false
	}
	if strings.HasPrefix(path, "/v1/ui/invitations/") {
		return "", false
	}
	if strings.HasPrefix(path, "/v1/ui/auth/sso/") {
		return "", false
	}
	if path == "/v1/ui/sessions/rate-limit-probe" {
		return "", false
	}
	if path == "/v1/ui/sessions/me" {
		return RoleReader, true
	}
	if path == "/v1/ui/sessions/logout" {
		return RoleReader, true
	}
	if path == "/v1/customer-onboarding" {
		return RoleWriter, true
	}
	if path == "/v1/workspace/members" || strings.HasPrefix(path, "/v1/workspace/members/") {
		return RoleAdmin, true
	}
	if path == "/v1/workspace/invitations" || strings.HasPrefix(path, "/v1/workspace/invitations/") {
		return RoleAdmin, true
	}
	if path == "/v1/workspace/service-accounts" || strings.HasPrefix(path, "/v1/workspace/service-accounts/") {
		return RoleAdmin, true
	}

	switch {
	case path == "/v1/customers":
		if r.Method == http.MethodPost {
			return RoleWriter, true
		}
		return RoleReader, true
	case strings.HasPrefix(path, "/v1/customers/"):
		tail := strings.Trim(strings.TrimPrefix(path, "/v1/customers/"), "/")
		if strings.HasSuffix(tail, "/billing-profile/retry-sync") {
			if r.Method == http.MethodPost {
				return RoleWriter, true
			}
			return RoleReader, true
		}
		if strings.HasSuffix(tail, "/billing-profile") {
			if r.Method == http.MethodPut {
				return RoleWriter, true
			}
			return RoleReader, true
		}
		if strings.HasSuffix(tail, "/payment-setup/checkout-url") {
			if r.Method == http.MethodPost {
				return RoleWriter, true
			}
			return RoleReader, true
		}
		if strings.HasSuffix(tail, "/payment-setup/request") || strings.HasSuffix(tail, "/payment-setup/resend") {
			if r.Method == http.MethodPost {
				return RoleWriter, true
			}
			return RoleReader, true
		}
		if strings.HasSuffix(tail, "/payment-setup/refresh") {
			if r.Method == http.MethodPost {
				return RoleWriter, true
			}
			return RoleReader, true
		}
		if strings.HasSuffix(tail, "/payment-setup") {
			return RoleReader, true
		}
		if r.Method == http.MethodPatch {
			return RoleWriter, true
		}
		return RoleReader, true
	case path == "/v1/rating-rules":
		if r.Method == http.MethodPost {
			return RoleWriter, true
		}
		return RoleReader, true
	case strings.HasPrefix(path, "/v1/rating-rules/"):
		return RoleReader, true
	case path == "/v1/meters":
		if r.Method == http.MethodPost {
			return RoleWriter, true
		}
		return RoleReader, true
	case strings.HasPrefix(path, "/v1/meters/"):
		if r.Method == http.MethodPut {
			return RoleWriter, true
		}
		return RoleReader, true
	case path == "/v1/pricing/metrics":
		if r.Method == http.MethodPost {
			return RoleWriter, true
		}
		return RoleReader, true
	case strings.HasPrefix(path, "/v1/pricing/metrics/"):
		return RoleReader, true
	case path == "/v1/taxes":
		if r.Method == http.MethodPost {
			return RoleWriter, true
		}
		return RoleReader, true
	case strings.HasPrefix(path, "/v1/taxes/"):
		return RoleReader, true
	case path == "/v1/plans":
		if r.Method == http.MethodPost {
			return RoleWriter, true
		}
		return RoleReader, true
	case strings.HasPrefix(path, "/v1/plans/"):
		if r.Method == http.MethodPatch {
			return RoleWriter, true
		}
		return RoleReader, true
	case path == "/v1/subscriptions":
		if r.Method == http.MethodPost {
			return RoleWriter, true
		}
		return RoleReader, true
	case strings.HasPrefix(path, "/v1/subscriptions/"):
		if r.Method == http.MethodPost || r.Method == http.MethodPatch {
			return RoleWriter, true
		}
		return RoleReader, true
	case path == "/v1/invoices":
		return RoleReader, true
	case path == "/v1/payments":
		return RoleReader, true
	case path == "/v1/invoices/preview":
		return RoleReader, true
	case strings.HasPrefix(path, "/v1/invoices/"):
		if r.Method == http.MethodPost && (strings.HasSuffix(strings.Trim(path, "/"), "/retry-payment") || strings.HasSuffix(strings.Trim(path, "/"), "/resend-email")) {
			return RoleWriter, true
		}
		return RoleReader, true
	case strings.HasPrefix(path, "/v1/payment-receipts/"):
		if r.Method == http.MethodPost && strings.HasSuffix(strings.Trim(path, "/"), "/resend-email") {
			return RoleWriter, true
		}
		return RoleReader, true
	case strings.HasPrefix(path, "/v1/credit-notes/"):
		if r.Method == http.MethodPost && strings.HasSuffix(strings.Trim(path, "/"), "/resend-email") {
			return RoleWriter, true
		}
		return RoleReader, true
	case strings.HasPrefix(path, "/v1/payments/"):
		if r.Method == http.MethodPost && strings.HasSuffix(strings.Trim(path, "/"), "/retry") {
			return RoleWriter, true
		}
		return RoleReader, true
	case path == "/v1/reconciliation-report":
		return RoleReader, true
	case path == "/v1/invoice-payment-statuses":
		return RoleReader, true
	case path == "/v1/invoice-payment-statuses/summary":
		return RoleReader, true
	case strings.HasPrefix(path, "/v1/invoice-payment-statuses/"):
		return RoleReader, true
	case path == "/v1/dunning/policy":
		if r.Method == http.MethodPut {
			return RoleWriter, true
		}
		return RoleReader, true
	case path == "/v1/dunning/runs":
		return RoleReader, true
	case strings.HasPrefix(path, "/v1/dunning/runs/"):
		if r.Method == http.MethodPost && strings.HasSuffix(strings.Trim(path, "/"), "/collect-payment-reminder") {
			return RoleWriter, true
		}
		return RoleReader, true
	case path == "/v1/replay-jobs":
		if r.Method == http.MethodPost {
			return RoleWriter, true
		}
		return RoleReader, true
	case strings.HasPrefix(path, "/v1/replay-jobs/"):
		if r.Method == http.MethodPost && strings.HasSuffix(strings.Trim(path, "/"), "/retry") {
			return RoleWriter, true
		}
		return RoleReader, true
	case path == "/v1/api-keys":
		return RoleAdmin, true
	case path == "/v1/api-keys/audit":
		return RoleAdmin, true
	case path == "/v1/api-keys/audit/exports":
		return RoleAdmin, true
	case strings.HasPrefix(path, "/v1/api-keys/audit/exports/"):
		return RoleAdmin, true
	case strings.HasPrefix(path, "/v1/api-keys/"):
		return RoleAdmin, true
	case path == "/v1/usage-events":
		if r.Method == http.MethodPost {
			return RoleWriter, true
		}
		return RoleReader, true
	case path == "/v1/billed-entries":
		if r.Method == http.MethodPost {
			return RoleWriter, true
		}
		return RoleReader, true
	case strings.HasPrefix(path, "/v1/"):
		return RoleReader, true
	default:
		return "", false
	}
}

func requiresPlatformScope(r *http.Request) bool {
	path := strings.TrimSpace(r.URL.Path)
	switch {
	case path == "/internal/metrics":
		return true
	case path == "/internal/ready":
		return true
	case path == "/internal/billing-provider-connections":
		return true
	case strings.HasPrefix(path, "/internal/billing-provider-connections/"):
		return true
	case path == "/internal/onboarding/tenants":
		return true
	case strings.HasPrefix(path, "/internal/onboarding/tenants/"):
		return true
	case path == "/internal/tenants":
		return true
	case path == "/internal/tenants/audit":
		return true
	case strings.HasPrefix(path, "/internal/tenants/"):
		return true
	default:
		return false
	}
}

func isUISessionSelfRoute(path string) bool {
	path = strings.TrimSpace(path)
	switch path {
	case "/v1/ui/sessions/me", "/v1/ui/sessions/logout":
		return true
	default:
		return strings.HasSuffix(strings.TrimRight(path, "/"), "/accept") && strings.HasPrefix(path, "/v1/ui/invitations/")
	}
}

func normalizeMetricsRoute(path string) string {
	switch {
	case path == "/health":
		return "/health"
	case path == "/internal/metrics":
		return "/internal/metrics"
	case path == "/internal/ready":
		return "/internal/ready"
	case path == "/internal/onboarding/tenants":
		return "/internal/onboarding/tenants"
	case strings.HasPrefix(path, "/internal/onboarding/tenants/"):
		return "/internal/onboarding/tenants/{id}"
	case path == "/internal/lago/webhooks":
		return "/internal/lago/webhooks"
	case path == "/internal/tenants/audit":
		return "/internal/tenants/audit"
	case path == "/internal/billing-provider-connections":
		return "/internal/billing-provider-connections"
	case strings.HasPrefix(path, "/internal/billing-provider-connections/"):
		return "/internal/billing-provider-connections/{id}"
	case path == "/internal/tenants":
		return "/internal/tenants"
	case strings.HasPrefix(path, "/internal/tenants/"):
		return "/internal/tenants/{id}"
	case path == "/v1/ui/sessions/login":
		return "/v1/ui/sessions/login"
	case path == "/v1/ui/auth/providers":
		return "/v1/ui/auth/providers"
	case path == "/v1/ui/password/forgot":
		return "/v1/ui/password/forgot"
	case path == "/v1/ui/password/reset":
		return "/v1/ui/password/reset"
	case strings.HasPrefix(path, "/v1/ui/auth/sso/"):
		tail := strings.Trim(strings.TrimPrefix(path, "/v1/ui/auth/sso/"), "/")
		if strings.HasSuffix(tail, "/start") {
			return "/v1/ui/auth/sso/{provider}/start"
		}
		if strings.HasSuffix(tail, "/callback") {
			return "/v1/ui/auth/sso/{provider}/callback"
		}
		return "/v1/ui/auth/sso/{provider}"
	case path == "/v1/ui/sessions/rate-limit-probe":
		return "/v1/ui/sessions/rate-limit-probe"
	case path == "/v1/ui/sessions/me":
		return "/v1/ui/sessions/me"
	case path == "/v1/ui/sessions/logout":
		return "/v1/ui/sessions/logout"
	case path == "/v1/customer-onboarding":
		return "/v1/customer-onboarding"
	case path == "/v1/customers":
		return "/v1/customers"
	case strings.HasPrefix(path, "/v1/customers/"):
		tail := strings.Trim(strings.TrimPrefix(path, "/v1/customers/"), "/")
		if strings.HasSuffix(tail, "/billing-profile/retry-sync") {
			return "/v1/customers/{id}/billing-profile/retry-sync"
		}
		if strings.HasSuffix(tail, "/billing-profile") {
			return "/v1/customers/{id}/billing-profile"
		}
		if strings.HasSuffix(tail, "/payment-setup/checkout-url") {
			return "/v1/customers/{id}/payment-setup/checkout-url"
		}
		if strings.HasSuffix(tail, "/payment-setup/request") {
			return "/v1/customers/{id}/payment-setup/request"
		}
		if strings.HasSuffix(tail, "/payment-setup/resend") {
			return "/v1/customers/{id}/payment-setup/resend"
		}
		if strings.HasSuffix(tail, "/payment-setup/refresh") {
			return "/v1/customers/{id}/payment-setup/refresh"
		}
		if strings.HasSuffix(tail, "/payment-setup") {
			return "/v1/customers/{id}/payment-setup"
		}
		if strings.HasSuffix(tail, "/readiness") {
			return "/v1/customers/{id}/readiness"
		}
		return "/v1/customers/{id}"
	case path == "/v1/rating-rules":
		return "/v1/rating-rules"
	case strings.HasPrefix(path, "/v1/rating-rules/"):
		return "/v1/rating-rules/{id}"
	case path == "/v1/meters":
		return "/v1/meters"
	case strings.HasPrefix(path, "/v1/meters/"):
		return "/v1/meters/{id}"
	case path == "/v1/pricing/metrics":
		return "/v1/pricing/metrics"
	case strings.HasPrefix(path, "/v1/pricing/metrics/"):
		return "/v1/pricing/metrics/{id}"
	case path == "/v1/taxes":
		return "/v1/taxes"
	case strings.HasPrefix(path, "/v1/taxes/"):
		return "/v1/taxes/{id}"
	case path == "/v1/plans":
		return "/v1/plans"
	case strings.HasPrefix(path, "/v1/plans/"):
		return "/v1/plans/{id}"
	case path == "/v1/subscriptions":
		return "/v1/subscriptions"
	case strings.HasPrefix(path, "/v1/subscriptions/"):
		tail := strings.Trim(strings.TrimPrefix(path, "/v1/subscriptions/"), "/")
		if strings.HasSuffix(tail, "/payment-setup/request") {
			return "/v1/subscriptions/{id}/payment-setup/request"
		}
		if strings.HasSuffix(tail, "/payment-setup/resend") {
			return "/v1/subscriptions/{id}/payment-setup/resend"
		}
		return "/v1/subscriptions/{id}"
	case path == "/v1/invoices":
		return "/v1/invoices"
	case path == "/v1/payments":
		return "/v1/payments"
	case path == "/v1/invoices/preview":
		return "/v1/invoices/preview"
	case strings.HasPrefix(path, "/v1/invoices/"):
		tail := strings.Trim(strings.TrimPrefix(path, "/v1/invoices/"), "/")
		if strings.HasSuffix(tail, "/retry-payment") {
			return "/v1/invoices/{id}/retry-payment"
		}
		if strings.HasSuffix(tail, "/resend-email") {
			return "/v1/invoices/{id}/resend-email"
		}
		if strings.HasSuffix(tail, "/payment-receipts") {
			return "/v1/invoices/{id}/payment-receipts"
		}
		if strings.HasSuffix(tail, "/credit-notes") {
			return "/v1/invoices/{id}/credit-notes"
		}
		if strings.HasSuffix(tail, "/explainability") {
			return "/v1/invoices/{id}/explainability"
		}
		return "/v1/invoices/{id}"
	case strings.HasPrefix(path, "/v1/payment-receipts/"):
		tail := strings.Trim(strings.TrimPrefix(path, "/v1/payment-receipts/"), "/")
		if strings.HasSuffix(tail, "/resend-email") {
			return "/v1/payment-receipts/{id}/resend-email"
		}
		return "/v1/payment-receipts/{id}"
	case strings.HasPrefix(path, "/v1/credit-notes/"):
		tail := strings.Trim(strings.TrimPrefix(path, "/v1/credit-notes/"), "/")
		if strings.HasSuffix(tail, "/resend-email") {
			return "/v1/credit-notes/{id}/resend-email"
		}
		return "/v1/credit-notes/{id}"
	case strings.HasPrefix(path, "/v1/payments/"):
		tail := strings.Trim(strings.TrimPrefix(path, "/v1/payments/"), "/")
		if strings.HasSuffix(tail, "/events") {
			return "/v1/payments/{id}/events"
		}
		if strings.HasSuffix(tail, "/retry") {
			return "/v1/payments/{id}/retry"
		}
		return "/v1/payments/{id}"
	case path == "/v1/usage-events":
		return "/v1/usage-events"
	case path == "/v1/billed-entries":
		return "/v1/billed-entries"
	case path == "/v1/replay-jobs":
		return "/v1/replay-jobs"
	case strings.HasPrefix(path, "/v1/replay-jobs/"):
		tail := strings.Trim(strings.TrimPrefix(path, "/v1/replay-jobs/"), "/")
		if strings.HasSuffix(tail, "/events") {
			return "/v1/replay-jobs/{id}/events"
		}
		if strings.Contains(tail, "/artifacts/") {
			return "/v1/replay-jobs/{id}/artifacts/{artifact}"
		}
		if strings.HasSuffix(tail, "/retry") {
			return "/v1/replay-jobs/{id}/retry"
		}
		return "/v1/replay-jobs/{id}"
	case path == "/v1/reconciliation-report":
		return "/v1/reconciliation-report"
	case path == "/v1/invoice-payment-statuses":
		return "/v1/invoice-payment-statuses"
	case path == "/v1/invoice-payment-statuses/summary":
		return "/v1/invoice-payment-statuses/summary"
	case strings.HasPrefix(path, "/v1/invoice-payment-statuses/"):
		tail := strings.Trim(strings.TrimPrefix(path, "/v1/invoice-payment-statuses/"), "/")
		if strings.HasSuffix(tail, "/events") {
			return "/v1/invoice-payment-statuses/{id}/events"
		}
		if strings.HasSuffix(tail, "/lifecycle") {
			return "/v1/invoice-payment-statuses/{id}/lifecycle"
		}
		return "/v1/invoice-payment-statuses/{id}"
	case path == "/v1/dunning/policy":
		return "/v1/dunning/policy"
	case path == "/v1/dunning/runs":
		return "/v1/dunning/runs"
	case strings.HasPrefix(path, "/v1/dunning/runs/"):
		tail := strings.Trim(strings.TrimPrefix(path, "/v1/dunning/runs/"), "/")
		if strings.HasSuffix(tail, "/collect-payment-reminder") {
			return "/v1/dunning/runs/{id}/collect-payment-reminder"
		}
		return "/v1/dunning/runs/{id}"
	case path == "/v1/api-keys":
		return "/v1/api-keys"
	case path == "/v1/api-keys/audit":
		return "/v1/api-keys/audit"
	case path == "/v1/api-keys/audit/exports":
		return "/v1/api-keys/audit/exports"
	case strings.HasPrefix(path, "/v1/api-keys/audit/exports/"):
		return "/v1/api-keys/audit/exports/{id}"
	case strings.HasPrefix(path, "/v1/api-keys/"):
		return "/v1/api-keys/{id}/{action}"
	case path == "/v1/workspace/service-accounts":
		return "/v1/workspace/service-accounts"
	case strings.HasPrefix(path, "/v1/workspace/service-accounts/"):
		tail := strings.Trim(strings.TrimPrefix(path, "/v1/workspace/service-accounts/"), "/")
		if strings.HasSuffix(tail, "/audit/exports") {
			return "/v1/workspace/service-accounts/{id}/audit/exports"
		}
		if strings.Contains(tail, "/audit/exports/") {
			return "/v1/workspace/service-accounts/{id}/audit/exports/{job_id}"
		}
		if strings.HasSuffix(tail, "/audit") {
			return "/v1/workspace/service-accounts/{id}/audit"
		}
		if strings.Contains(tail, "/credentials/") {
			return "/v1/workspace/service-accounts/{id}/credentials/{credential_id}/{action}"
		}
		if strings.HasSuffix(tail, "/credentials") {
			return "/v1/workspace/service-accounts/{id}/credentials"
		}
		return "/v1/workspace/service-accounts/{id}"
	case strings.HasPrefix(path, "/v1/"):
		return "/v1/*"
	default:
		return path
	}
}

func requestTenantID(r *http.Request) string {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		return defaultTenantID
	}
	if principal.Scope != ScopeTenant {
		return ""
	}
	return normalizeTenantID(principal.TenantID)
}

func requestActorAPIKeyID(r *http.Request) string {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		return ""
	}
	return strings.TrimSpace(principal.APIKeyID)
}

func isTenantMatch(resourceTenantID, requestTenantID string) bool {
	return normalizeTenantID(resourceTenantID) == normalizeTenantID(requestTenantID)
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/internal/lago/webhooks", s.handleLagoWebhooks)
	s.mux.HandleFunc("/internal/onboarding/tenants", s.handleInternalOnboardingTenants)
	s.mux.HandleFunc("/internal/onboarding/tenants/", s.handleInternalOnboardingTenantByID)
	s.mux.HandleFunc("/internal/billing-provider-connections", s.handleInternalBillingProviderConnections)
	s.mux.HandleFunc("/internal/billing-provider-connections/", s.handleInternalBillingProviderConnectionByID)
	s.mux.HandleFunc("/internal/tenants/audit", s.handleInternalTenantAudit)
	s.mux.HandleFunc("/internal/tenants", s.handleInternalTenants)
	s.mux.HandleFunc("/internal/tenants/", s.handleInternalTenantByID)
	s.mux.HandleFunc("/v1/ui/auth/providers", s.handleUIAuthProviders)
	s.mux.HandleFunc("/v1/ui/workspaces/pending", s.handleUIWorkspaceSelectionPending)
	s.mux.HandleFunc("/v1/ui/workspaces/select", s.handleUIWorkspaceSelectionSelect)
	s.mux.HandleFunc("/v1/ui/invitations/", s.handleUIInvitations)
	s.mux.HandleFunc("/v1/ui/auth/sso/", s.handleUISSO)
	s.mux.HandleFunc("/v1/ui/password/forgot", s.handleUIPasswordForgot)
	s.mux.HandleFunc("/v1/ui/password/reset", s.handleUIPasswordReset)
	s.mux.HandleFunc("/v1/ui/sessions/login", s.handleUISessionLogin)
	s.mux.HandleFunc("/v1/ui/sessions/rate-limit-probe", s.handleUIPreAuthRateLimitProbe)
	s.mux.HandleFunc("/v1/ui/sessions/me", s.handleUISessionMe)
	s.mux.HandleFunc("/v1/ui/sessions/logout", s.handleUISessionLogout)
	s.mux.HandleFunc("/v1/workspace/members", s.handleTenantWorkspaceMembers)
	s.mux.HandleFunc("/v1/workspace/members/", s.handleTenantWorkspaceMembers)
	s.mux.HandleFunc("/v1/workspace/invitations", s.handleTenantWorkspaceInvitations)
	s.mux.HandleFunc("/v1/workspace/invitations/", s.handleTenantWorkspaceInvitations)
	s.mux.HandleFunc("/v1/workspace/service-accounts", s.handleTenantWorkspaceServiceAccounts)
	s.mux.HandleFunc("/v1/workspace/service-accounts/", s.handleTenantWorkspaceServiceAccounts)
	s.mux.HandleFunc("/v1/customer-onboarding", s.handleCustomerOnboarding)

	s.mux.HandleFunc("/v1/customers", s.handleCustomers)
	s.mux.HandleFunc("/v1/customers/", s.handleCustomerByExternalID)

	s.mux.HandleFunc("/v1/rating-rules", s.handleRatingRules)
	s.mux.HandleFunc("/v1/rating-rules/", s.handleRatingRuleByID)

	s.mux.HandleFunc("/v1/meters", s.handleMeters)
	s.mux.HandleFunc("/v1/meters/", s.handleMeterByID)
	s.mux.HandleFunc("/v1/pricing/metrics", s.handlePricingMetrics)
	s.mux.HandleFunc("/v1/pricing/metrics/", s.handlePricingMetricByID)
	s.mux.HandleFunc("/v1/taxes", s.handleTaxes)
	s.mux.HandleFunc("/v1/taxes/", s.handleTaxByID)
	s.mux.HandleFunc("/v1/add-ons", s.handleAddOns)
	s.mux.HandleFunc("/v1/add-ons/", s.handleAddOnByID)
	s.mux.HandleFunc("/v1/coupons", s.handleCoupons)
	s.mux.HandleFunc("/v1/coupons/", s.handleCouponByID)
	s.mux.HandleFunc("/v1/plans", s.handlePlans)
	s.mux.HandleFunc("/v1/plans/", s.handlePlanByID)
	s.mux.HandleFunc("/v1/subscriptions", s.handleSubscriptions)
	s.mux.HandleFunc("/v1/subscriptions/", s.handleSubscriptionByID)

	s.mux.HandleFunc("/v1/invoices", s.handleInvoices)
	s.mux.HandleFunc("/v1/invoices/preview", s.handleInvoicePreview)
	s.mux.HandleFunc("/v1/invoices/", s.handleInvoiceByID)
	s.mux.HandleFunc("/v1/payment-receipts/", s.handlePaymentReceiptByID)
	s.mux.HandleFunc("/v1/credit-notes/", s.handleCreditNoteByID)
	s.mux.HandleFunc("/v1/payments", s.handlePayments)
	s.mux.HandleFunc("/v1/payments/", s.handlePaymentByID)

	s.mux.HandleFunc("/v1/usage-events", s.handleUsageEvents)
	s.mux.HandleFunc("/v1/billed-entries", s.handleBilledEntries)

	s.mux.HandleFunc("/v1/replay-jobs", s.handleReplayJobs)
	s.mux.HandleFunc("/v1/replay-jobs/", s.handleReplayJobByID)
	s.mux.HandleFunc("/v1/invoice-payment-statuses", s.handleInvoicePaymentStatuses)
	s.mux.HandleFunc("/v1/invoice-payment-statuses/", s.handleInvoicePaymentStatusByID)
	s.mux.HandleFunc("/v1/dunning/policy", s.handleDunningPolicy)
	s.mux.HandleFunc("/v1/dunning/runs", s.handleDunningRuns)
	s.mux.HandleFunc("/v1/dunning/runs/", s.handleDunningRunByID)

	s.mux.HandleFunc("/v1/reconciliation-report", s.handleReconciliationReport)
	s.mux.HandleFunc("/v1/api-keys/audit/exports", s.handleAPIKeyAuditExports)
	s.mux.HandleFunc("/v1/api-keys/audit/exports/", s.handleAPIKeyAuditExportByID)
	s.mux.HandleFunc("/v1/api-keys/audit", s.handleAPIKeyAuditEvents)
	s.mux.HandleFunc("/v1/api-keys", s.handleAPIKeys)
	s.mux.HandleFunc("/v1/api-keys/", s.handleAPIKeyByID)
	s.mux.HandleFunc("/internal/metrics", s.handleInternalMetrics)
	s.mux.HandleFunc("/internal/ready", s.handleInternalReady)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) isOperatorRequest(r *http.Request) bool {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		return false
	}
	return principal.Scope == ScopePlatform && principal.PlatformRole == PlatformRoleAdmin
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
			LagoOrganizationID          *string              `json:"lago_organization_id,omitempty"`
			LagoBillingProviderCode     *string              `json:"lago_billing_provider_code,omitempty"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		tenant, err := s.tenantService.UpdateTenant(id, service.UpdateTenantRequest{
			Name:                        req.Name,
			Status:                      req.Status,
			BillingProviderConnectionID: req.BillingProviderConnectionID,
			LagoOrganizationID:          req.LagoOrganizationID,
			LagoBillingProviderCode:     req.LagoBillingProviderCode,
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

type uiSessionLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	TenantID string `json:"tenant_id"`
	Next     string `json:"next"`
}

type uiInvitationRegisterRequest struct {
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
}

type uiPasswordForgotRequest struct {
	Email string `json:"email"`
}

type uiPasswordResetRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

type uiWorkspaceSelectRequest struct {
	TenantID string `json:"tenant_id"`
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

func (s *Server) handleUIPreAuthRateLimitProbe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if s.rateLimiter != nil {
		if !s.enforceRateLimit(w, r, RateLimitPolicyPreAuthLogin, preAuthLoginRateLimitIdentifier(r), "", s.rateLimitLoginFailOpen) {
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleUIAuthProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
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
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
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
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}
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
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
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
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Token = strings.TrimSpace(req.Token)
	req.Password = strings.TrimSpace(req.Password)
	if req.Token == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "token and password are required")
		return
	}
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

func (s *Server) handleUIWorkspaceSelectionPending(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if s.sessionManager == nil || s.browserUserAuthService == nil {
		writeError(w, http.StatusServiceUnavailable, "workspace selection is not configured")
		return
	}
	userID, userEmail := s.pendingWorkspaceSelectionState(r)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "workspace selection not pending")
		return
	}
	resp, err := s.buildWorkspaceSelectionResponse(r, userID, userEmail)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleUIWorkspaceSelectionSelect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if s.sessionManager == nil || s.browserUserAuthService == nil {
		writeError(w, http.StatusServiceUnavailable, "workspace selection is not configured")
		return
	}
	expectedCSRF := strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionCSRFKey))
	providedCSRF := strings.TrimSpace(r.Header.Get(csrfHeaderName))
	if expectedCSRF == "" || providedCSRF == "" || !subtleConstantTimeMatch(expectedCSRF, providedCSRF) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	userID, _ := s.pendingWorkspaceSelectionState(r)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "workspace selection not pending")
		return
	}
	var req uiWorkspaceSelectRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	user, err := s.repo.GetUser(userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	principal, err := s.browserUserAuthService.ResolveUserPrincipal(user, req.TenantID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrBrowserTenantAccessDenied):
			writeError(w, http.StatusForbidden, "tenant access denied")
		case errors.Is(err, service.ErrBrowserTenantSelection):
			resp, buildErr := s.buildWorkspaceSelectionResponse(r, user.ID, user.Email)
			if buildErr != nil {
				writeError(w, http.StatusInternalServerError, "failed to resolve workspace options")
				return
			}
			writeJSON(w, http.StatusConflict, resp)
		default:
			writeError(w, http.StatusInternalServerError, "failed to initialize session")
		}
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
	s.clearUIPendingWorkspaceSelection(r.Context())
	s.putUISessionPrincipal(r.Context(), Principal{
		SubjectType: "user",
		SubjectID:   principal.User.ID,
		UserEmail:   principal.User.Email,
		Scope:       ScopeTenant,
		Role:        Role(principal.Role),
		TenantID:    normalizeTenantID(principal.TenantID),
	}, csrfToken)
	writeJSON(w, http.StatusCreated, buildUISessionResponse(Principal{
		SubjectType: "user",
		SubjectID:   principal.User.ID,
		UserEmail:   principal.User.Email,
		Scope:       ScopeTenant,
		Role:        Role(principal.Role),
		TenantID:    normalizeTenantID(principal.TenantID),
	}, csrfToken, time.Now().UTC().Add(s.sessionManager.Lifetime)))
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
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if s.workspaceAccessService == nil {
		writeError(w, http.StatusServiceUnavailable, "workspace access is not configured")
		return
	}
	currentUserEmail := ""
	if s.sessionManager != nil {
		currentUserEmail = strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionUserEmailKey))
		if currentUserEmail == "" {
			_, currentUserEmail = s.pendingWorkspaceSelectionState(r)
		}
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
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if s.workspaceAccessService == nil || s.browserUserAuthService == nil || s.sessionManager == nil {
		writeError(w, http.StatusServiceUnavailable, "workspace access is not configured")
		return
	}
	expectedCSRF := strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionCSRFKey))
	providedCSRF := strings.TrimSpace(r.Header.Get(csrfHeaderName))
	if expectedCSRF == "" || providedCSRF == "" || !subtleConstantTimeMatch(expectedCSRF, providedCSRF) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	userID := strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionSubjectIDKey))
	if userID == "" {
		userID, _ = s.pendingWorkspaceSelectionState(r)
	}
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
	s.clearUIPendingWorkspaceSelection(r.Context())
	s.putUISessionPrincipal(r.Context(), sessionPrincipal, csrfToken)
	writeJSON(w, http.StatusCreated, map[string]any{
		"invitation": newWorkspaceInvitationResponse(invite),
		"session":    buildUISessionResponse(sessionPrincipal, csrfToken, time.Now().UTC().Add(s.sessionManager.Lifetime)),
	})
}

func (s *Server) handleUIInvitationRegister(w http.ResponseWriter, r *http.Request, token string) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
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
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	req.Password = strings.TrimSpace(req.Password)
	if req.Password == "" {
		writeError(w, http.StatusBadRequest, "password is required")
		return
	}

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
	s.clearUIPendingWorkspaceSelection(r.Context())
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
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
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
	s.sessionManager.Put(r.Context(), sessionSSOTenantIDKey, normalizeOptionalTenantID(strings.TrimSpace(r.URL.Query().Get("tenant_id"))))
	http.Redirect(w, r, authURL, http.StatusFound)
}

func (s *Server) handleUISSOCallback(w http.ResponseWriter, r *http.Request, providerKey string) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
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
	tenantID := normalizeOptionalTenantID(strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionSSOTenantIDKey)))
	nextPath := normalizeUINextPath(strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionSSONextKey)))
	if expectedState == "" || !subtleConstantTimeMatch(expectedState, state) || expectedProvider != strings.ToLower(strings.TrimSpace(providerKey)) || codeVerifier == "" {
		s.clearSSOSessionState(r.Context())
		s.redirectUISSOFailure(w, r, strings.TrimSpace(providerKey), "sso_state_invalid")
		return
	}

	principal, err := s.browserSSOService.AuthenticateCallback(r.Context(), providerKey, code, codeVerifier, nonce, tenantID, s.uiSSOCallbackURL(r, providerKey), invitationTokenFromNextPath(nextPath))
	s.clearSSOSessionState(r.Context())
	if err != nil {
		var selectionErr service.BrowserTenantSelectionError
		var accessDeniedErr service.BrowserTenantAccessDeniedError
		if errors.As(err, &selectionErr) {
			if _, pendingErr := s.beginUIPendingWorkspaceSelection(r, selectionErr.User, nextPath); pendingErr == nil {
				http.Redirect(w, r, s.uiWorkspaceSelectURL(nextPath), http.StatusFound)
				return
			}
		}
		if errors.As(err, &accessDeniedErr) && strings.HasPrefix(nextPath, "/invite/") {
			if _, pendingErr := s.beginUIPendingWorkspaceSelection(r, accessDeniedErr.User, nextPath); pendingErr == nil {
				http.Redirect(w, r, s.uiNextURL(nextPath), http.StatusFound)
				return
			}
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
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
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
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Password = strings.TrimSpace(req.Password)
	req.TenantID = strings.TrimSpace(req.TenantID)
	req.Next = normalizeUINextPath(strings.TrimSpace(req.Next))
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

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
		var selectionErr service.BrowserTenantSelectionError
		var accessDeniedErr service.BrowserTenantAccessDeniedError
		switch {
		case errors.As(err, &selectionErr):
			resp, selectionSetupErr := s.beginUIPendingWorkspaceSelection(r, selectionErr.User, req.Next)
			if selectionSetupErr != nil {
				writeError(w, http.StatusInternalServerError, "failed to initialize workspace selection")
				return
			}
			writeJSON(w, http.StatusConflict, resp)
		case errors.As(err, &accessDeniedErr) && strings.HasPrefix(req.Next, "/invite/"):
			if _, pendingErr := s.beginUIPendingWorkspaceSelection(r, accessDeniedErr.User, req.Next); pendingErr != nil {
				writeError(w, http.StatusInternalServerError, "failed to initialize invitation session")
				return
			}
			writeJSON(w, http.StatusAccepted, pendingInvitationLoginResponse{
				PendingInvitation: true,
				NextPath:          req.Next,
			})
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

func (s *Server) putUIPendingWorkspaceSelection(ctx context.Context, user domain.User, nextPath, csrfToken string) {
	s.sessionManager.Put(ctx, sessionPendingUserIDKey, strings.TrimSpace(user.ID))
	s.sessionManager.Put(ctx, sessionPendingUserEmailKey, strings.ToLower(strings.TrimSpace(user.Email)))
	s.sessionManager.Put(ctx, sessionPendingNextKey, normalizeUINextPath(nextPath))
	s.sessionManager.Put(ctx, sessionCSRFKey, csrfToken)
	s.sessionManager.Remove(ctx, sessionSubjectTypeKey)
	s.sessionManager.Remove(ctx, sessionSubjectIDKey)
	s.sessionManager.Remove(ctx, sessionUserEmailKey)
	s.sessionManager.Remove(ctx, sessionScopeKey)
	s.sessionManager.Remove(ctx, sessionRoleKey)
	s.sessionManager.Remove(ctx, sessionPlatformRoleKey)
	s.sessionManager.Remove(ctx, sessionTenantIDKey)
	s.sessionManager.Remove(ctx, sessionAPIKeyIDKey)
}

func (s *Server) clearUIPendingWorkspaceSelection(ctx context.Context) {
	s.sessionManager.Remove(ctx, sessionPendingUserIDKey)
	s.sessionManager.Remove(ctx, sessionPendingUserEmailKey)
	s.sessionManager.Remove(ctx, sessionPendingNextKey)
}

func (s *Server) uiSSOCallbackURL(r *http.Request, providerKey string) string {
	baseURL := externalBaseURL(r)
	return strings.TrimRight(baseURL, "/") + "/v1/ui/auth/sso/" + url.PathEscape(strings.ToLower(strings.TrimSpace(providerKey))) + "/callback"
}

func (s *Server) uiNextURL(nextPath string) string {
	return strings.TrimRight(s.uiPublicBaseURL, "/") + normalizeUINextPath(nextPath)
}

func (s *Server) uiWorkspaceSelectURL(nextPath string) string {
	target, err := url.Parse(strings.TrimRight(s.uiPublicBaseURL, "/") + "/workspace-select")
	if err != nil {
		return s.uiNextURL("/workspace-select")
	}
	nextPath = normalizeUINextPath(nextPath)
	if nextPath != "" && nextPath != "/" {
		query := target.Query()
		query.Set("next", nextPath)
		target.RawQuery = query.Encode()
	}
	return target.String()
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

func (s *Server) uiSSOErrorCode(err error) string {
	switch {
	case errors.Is(err, service.ErrBrowserSSOProviderNotFound):
		return "sso_provider_not_found"
	case errors.Is(err, service.ErrBrowserSSOEmailRequired):
		return "sso_email_required"
	case errors.Is(err, service.ErrBrowserSSOEmailNotVerified):
		return "sso_email_not_verified"
	case errors.Is(err, service.ErrBrowserSSOUserNotProvisioned):
		return "sso_user_not_provisioned"
	case errors.Is(err, service.ErrBrowserTenantSelection):
		return "tenant_selection_required"
	case errors.Is(err, service.ErrBrowserTenantAccessDenied):
		return "tenant_access_denied"
	case errors.Is(err, service.ErrBrowserUserDisabled):
		return "user_disabled"
	default:
		return "sso_failed"
	}
}

func (s *Server) pendingWorkspaceSelectionState(r *http.Request) (string, string) {
	if s == nil || s.sessionManager == nil {
		return "", ""
	}
	return strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionPendingUserIDKey)),
		strings.ToLower(strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionPendingUserEmailKey)))
}

func (s *Server) buildWorkspaceSelectionResponse(r *http.Request, userID, userEmail string) (workspaceSelectionResponse, error) {
	userID = strings.TrimSpace(userID)
	userEmail = strings.ToLower(strings.TrimSpace(userEmail))
	if userID == "" {
		return workspaceSelectionResponse{}, errUnauthorized
	}
	items, err := s.browserUserAuthService.ListWorkspaceOptions(userID)
	if err != nil {
		return workspaceSelectionResponse{}, err
	}
	return workspaceSelectionResponse{
		Required:  len(items) > 1,
		UserEmail: userEmail,
		Items:     newBrowserWorkspaceOptionResponses(items),
		CSRFToken: strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionCSRFKey)),
	}, nil
}

func (s *Server) beginUIPendingWorkspaceSelection(r *http.Request, user domain.User, nextPath string) (workspaceSelectionResponse, error) {
	if s == nil || s.sessionManager == nil || s.browserUserAuthService == nil {
		return workspaceSelectionResponse{}, fmt.Errorf("ui workspace selection is not configured")
	}
	if err := s.sessionManager.RenewToken(r.Context()); err != nil {
		return workspaceSelectionResponse{}, err
	}
	csrfToken, err := randomHexToken(32)
	if err != nil {
		return workspaceSelectionResponse{}, err
	}
	s.putUIPendingWorkspaceSelection(r.Context(), user, nextPath, csrfToken)
	return s.buildWorkspaceSelectionResponse(r, user.ID, user.Email)
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

func parseCustomerPath(path string) (externalID string, action string, subaction string) {
	tail := strings.Trim(strings.TrimPrefix(path, "/v1/customers/"), "/")
	if tail == "" {
		return "", "", ""
	}
	parts := strings.Split(tail, "/")
	externalID = strings.TrimSpace(parts[0])
	if len(parts) > 1 {
		action = strings.TrimSpace(parts[1])
	}
	if len(parts) > 2 {
		subaction = strings.TrimSpace(parts[2])
	}
	return externalID, action, subaction
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

func metricsTenantKey(principal Principal) string {
	if principal.Scope == ScopePlatform {
		return "platform"
	}
	return normalizeTenantID(principal.TenantID)
}

func (s *Server) handleUISessionMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
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

func (s *Server) handleCustomerOnboarding(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	tenantID := requestTenantID(r)
	var req service.CustomerOnboardingRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := s.customerOnboardingService.OnboardCustomer(tenantID, req)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	status := http.StatusOK
	if result.CustomerCreated {
		status = http.StatusCreated
	}
	writeJSON(w, status, result)
}

func (s *Server) handleCustomers(w http.ResponseWriter, r *http.Request) {
	tenantID := requestTenantID(r)
	switch r.Method {
	case http.MethodPost:
		var req service.CreateCustomerRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		customer, err := s.customerService.CreateCustomer(tenantID, req)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, customer)
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
		customers, err := s.customerService.ListCustomers(tenantID, service.ListCustomersRequest{
			Status:     r.URL.Query().Get("status"),
			ExternalID: r.URL.Query().Get("external_id"),
			Limit:      limit,
			Offset:     offset,
		})
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, customers)
	default:
		writeMethodNotAllowed(w)
	}
}

func (s *Server) handleCustomerByExternalID(w http.ResponseWriter, r *http.Request) {
	tenantID := requestTenantID(r)
	externalID, action, subaction := parseCustomerPath(r.URL.Path)
	if externalID == "" {
		writeError(w, http.StatusBadRequest, "customer external_id is required")
		return
	}

	switch action {
	case "":
		switch r.Method {
		case http.MethodGet:
			customer, err := s.customerService.GetCustomerByExternalID(tenantID, externalID)
			if err != nil {
				writeDomainError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, customer)
		case http.MethodPatch:
			var req service.UpdateCustomerRequest
			if err := decodeJSON(r, &req); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			customer, err := s.customerService.UpdateCustomer(tenantID, externalID, req)
			if err != nil {
				writeDomainError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, customer)
		default:
			writeMethodNotAllowed(w)
		}
	case "billing-profile":
		if subaction == "retry-sync" {
			if r.Method != http.MethodPost {
				writeMethodNotAllowed(w)
				return
			}
			result, err := s.customerService.RetryCustomerBillingProfileSync(tenantID, externalID)
			if err != nil {
				writeDomainError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
			return
		}
		switch r.Method {
		case http.MethodGet:
			profile, err := s.customerService.GetCustomerBillingProfile(tenantID, externalID)
			if err != nil {
				writeDomainError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, profile)
		case http.MethodPut:
			var req service.UpsertCustomerBillingProfileRequest
			if err := decodeJSON(r, &req); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			profile, err := s.customerService.UpsertCustomerBillingProfile(tenantID, externalID, req)
			if err != nil {
				writeDomainError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, profile)
		default:
			writeMethodNotAllowed(w)
		}
	case "payment-setup":
		if subaction == "request" || subaction == "resend" {
			if r.Method != http.MethodPost {
				writeMethodNotAllowed(w)
				return
			}
			if s.customerPaymentSetupRequestService == nil || s.notificationService == nil || !s.notificationService.CanSendCustomerPaymentSetupRequest() {
				writeError(w, http.StatusNotImplemented, "payment setup request delivery is not configured")
				return
			}
			var req service.BeginCustomerPaymentSetupRequest
			if err := decodeJSON(r, &req); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			principal, _ := principalFromContext(r.Context())
			actor := service.CustomerPaymentSetupRequestActor{
				SubjectType:   strings.TrimSpace(principal.SubjectType),
				SubjectID:     strings.TrimSpace(principal.SubjectID),
				UserEmail:     strings.TrimSpace(principal.UserEmail),
				ActorAPIKeyID: strings.TrimSpace(principal.APIKeyID),
			}
			var (
				result service.CustomerPaymentSetupRequestResult
				err    error
			)
			if subaction == "resend" {
				result, err = s.customerPaymentSetupRequestService.Resend(tenantID, externalID, actor, req.PaymentMethodType)
			} else {
				result, err = s.customerPaymentSetupRequestService.Request(tenantID, externalID, actor, req.PaymentMethodType)
			}
			attrs := []any{
				"request_id", requestIDFromContext(r.Context()),
				"tenant_id", tenantID,
				"customer_external_id", externalID,
				"request_kind", subaction,
			}
			if err != nil {
				attrs = append(attrs, "error", err.Error())
				s.logger.Warn("customer payment setup request dispatch failed", attrs...)
				writeDomainError(w, err)
				return
			}
			attrs = append(attrs,
				"recipient_email", strings.TrimSpace(result.PaymentSetup.LastRequestToEmail),
				"backend", result.Dispatch.Backend,
				"action", result.Dispatch.Action,
				"domain", result.Dispatch.Domain,
			)
			s.logger.Info("customer payment setup request dispatched", attrs...)
			writeJSON(w, http.StatusOK, result)
			return
		}
		if subaction == "checkout-url" {
			if r.Method != http.MethodPost {
				writeMethodNotAllowed(w)
				return
			}
			var req service.BeginCustomerPaymentSetupRequest
			if err := decodeJSON(r, &req); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			result, err := s.customerService.BeginCustomerPaymentSetup(tenantID, externalID, req)
			if err != nil {
				writeDomainError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
			return
		}
		if subaction == "refresh" {
			if r.Method != http.MethodPost {
				writeMethodNotAllowed(w)
				return
			}
			result, err := s.customerService.RefreshCustomerPaymentSetup(tenantID, externalID)
			if err != nil {
				writeDomainError(w, err)
				return
			}
			if s.dunningService != nil {
				if _, err := s.dunningService.RefreshRunsForCustomer(tenantID, externalID); err != nil {
					writeDomainError(w, err)
					return
				}
			}
			writeJSON(w, http.StatusOK, result)
			return
		}
		switch r.Method {
		case http.MethodGet:
			setup, err := s.customerService.GetCustomerPaymentSetup(tenantID, externalID)
			if err != nil {
				writeDomainError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, setup)
		default:
			writeMethodNotAllowed(w)
		}
	case "readiness":
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w)
			return
		}
		readiness, err := s.customerService.GetCustomerReadiness(tenantID, externalID)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, readiness)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func (s *Server) handleUISessionLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if s.sessionManager == nil {
		writeError(w, http.StatusServiceUnavailable, "ui sessions are not configured")
		return
	}
	s.sessionManager.Destroy(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"logged_out": true})
}

func randomHexToken(numBytes int) (string, error) {
	if numBytes <= 0 {
		numBytes = 16
	}
	buf := make([]byte, numBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func randomURLToken(numBytes int) (string, error) {
	if numBytes <= 0 {
		numBytes = 16
	}
	buf := make([]byte, numBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return strings.TrimRight(base64.RawURLEncoding.EncodeToString(buf), "="), nil
}

func (s *Server) handleLagoWebhooks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if s.lagoWebhookSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "lago webhook service is required")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := s.lagoWebhookSvc.Ingest(r.Context(), r.Header, body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	status := http.StatusAccepted
	if result.Idempotent {
		status = http.StatusOK
	}
	writeJSON(w, status, map[string]any{
		"idempotent": result.Idempotent,
		"event":      result.Event,
	})
}

func (s *Server) handleInvoicePaymentStatuses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if s.lagoWebhookSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "lago webhook service is required")
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
	paymentOverdue, err := parseOptionalQueryBool(r, "payment_overdue")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	items, err := s.lagoWebhookSvc.ListInvoicePaymentStatusViews(
		requestTenantID(r),
		service.ListInvoicePaymentStatusViewsRequest{
			OrganizationID: r.URL.Query().Get("organization_id"),
			PaymentStatus:  r.URL.Query().Get("payment_status"),
			InvoiceStatus:  r.URL.Query().Get("invoice_status"),
			PaymentOverdue: paymentOverdue,
			SortBy:         r.URL.Query().Get("sort_by"),
			Order:          r.URL.Query().Get("order"),
			Limit:          limit,
			Offset:         offset,
		},
	)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":  items,
		"limit":  limit,
		"offset": offset,
		"filters": map[string]any{
			"organization_id": r.URL.Query().Get("organization_id"),
			"payment_status":  r.URL.Query().Get("payment_status"),
			"invoice_status":  r.URL.Query().Get("invoice_status"),
			"payment_overdue": paymentOverdue,
			"sort_by":         r.URL.Query().Get("sort_by"),
			"order":           r.URL.Query().Get("order"),
		},
	})
}

func (s *Server) handleInvoicePaymentStatusByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if s.lagoWebhookSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "lago webhook service is required")
		return
	}

	tail := strings.TrimPrefix(r.URL.Path, "/v1/invoice-payment-statuses/")
	parts := strings.Split(strings.Trim(tail, "/"), "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		writeError(w, http.StatusBadRequest, "invoice id is required")
		return
	}
	invoiceID := strings.TrimSpace(parts[0])
	if len(parts) == 1 && strings.EqualFold(invoiceID, "summary") {
		staleAfterSec, err := parseQueryInt(r, "stale_after_sec")
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		summary, err := s.lagoWebhookSvc.GetInvoicePaymentStatusSummary(
			requestTenantID(r),
			service.GetInvoicePaymentStatusSummaryRequest{
				OrganizationID:    r.URL.Query().Get("organization_id"),
				StaleAfterSeconds: staleAfterSec,
			},
		)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, summary)
		return
	}

	if len(parts) == 1 {
		item, err := s.lagoWebhookSvc.GetInvoicePaymentStatusView(requestTenantID(r), invoiceID)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, item)
		return
	}

	if len(parts) == 2 && strings.EqualFold(strings.TrimSpace(parts[1]), "events") {
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
		events, err := s.lagoWebhookSvc.ListLagoWebhookEvents(
			requestTenantID(r),
			service.ListLagoWebhookEventsRequest{
				OrganizationID: r.URL.Query().Get("organization_id"),
				InvoiceID:      invoiceID,
				WebhookType:    r.URL.Query().Get("webhook_type"),
				SortBy:         r.URL.Query().Get("sort_by"),
				Order:          r.URL.Query().Get("order"),
				Limit:          limit,
				Offset:         offset,
			},
		)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"items":      events,
			"limit":      limit,
			"offset":     offset,
			"invoice_id": invoiceID,
		})
		return
	}

	if len(parts) == 2 && strings.EqualFold(strings.TrimSpace(parts[1]), "lifecycle") {
		eventLimit, err := parseQueryInt(r, "event_limit")
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		view, err := s.lagoWebhookSvc.GetInvoicePaymentStatusView(requestTenantID(r), invoiceID)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		lifecycle, err := s.lagoWebhookSvc.GetInvoicePaymentLifecycle(requestTenantID(r), invoiceID, eventLimit)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		lifecycle, err = s.enrichPaymentLifecycleWithCustomerReadiness(requestTenantID(r), view.CustomerExternalID, lifecycle)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, lifecycle)
		return
	}

	writeError(w, http.StatusBadRequest, "unsupported invoice payment status subresource")
}

func (s *Server) handleRatingRules(w http.ResponseWriter, r *http.Request) {
	tenantID := requestTenantID(r)

	switch r.Method {
	case http.MethodPost:
		var req domain.RatingRuleVersion
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		req.TenantID = tenantID
		rule, err := s.ratingService.CreateRuleVersion(req)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, rule)
	case http.MethodGet:
		latestOnly, err := parseQueryBool(r, "latest_only")
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		allRules, err := s.ratingService.ListRuleVersions(tenantID, service.ListRuleVersionsRequest{
			RuleKey:        r.URL.Query().Get("rule_key"),
			LifecycleState: r.URL.Query().Get("lifecycle_state"),
			LatestOnly:     latestOnly,
		})
		if err != nil {
			writeDomainError(w, err)
			return
		}
		rules := make([]domain.RatingRuleVersion, 0, len(allRules))
		for _, rule := range allRules {
			rules = append(rules, rule)
		}
		writeJSON(w, http.StatusOK, rules)
	default:
		writeMethodNotAllowed(w)
	}
}

func (s *Server) handleRatingRuleByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/v1/rating-rules/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	rule, err := s.ratingService.GetRuleVersion(requestTenantID(r), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

func (s *Server) handleMeters(w http.ResponseWriter, r *http.Request) {
	if s.meterSyncAdapter == nil {
		writeError(w, http.StatusServiceUnavailable, "Pricing updates are unavailable right now.")
		return
	}

	tenantID := requestTenantID(r)

	switch r.Method {
	case http.MethodPost:
		var req domain.Meter
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		req.TenantID = tenantID
		meter, err := s.meterService.CreateMeter(req)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		if err := s.meterSyncAdapter.SyncMeter(r.Context(), meter); err != nil {
			writeError(w, http.StatusBadGateway, "Pricing metric changes could not be applied right now.")
			return
		}
		writeJSON(w, http.StatusCreated, meter)
	case http.MethodGet:
		allMeters, err := s.meterService.ListMeters(tenantID)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		meters := make([]domain.Meter, 0, len(allMeters))
		for _, meter := range allMeters {
			meters = append(meters, meter)
		}
		writeJSON(w, http.StatusOK, meters)
	default:
		writeMethodNotAllowed(w)
	}
}

func (s *Server) handleMeterByID(w http.ResponseWriter, r *http.Request) {
	if s.meterSyncAdapter == nil {
		writeError(w, http.StatusServiceUnavailable, "Pricing updates are unavailable right now.")
		return
	}

	tenantID := requestTenantID(r)
	id := strings.TrimPrefix(r.URL.Path, "/v1/meters/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	switch r.Method {
	case http.MethodPut:
		var req domain.Meter
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		req.TenantID = tenantID
		meter, err := s.meterService.UpdateMeter(tenantID, id, req)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		if err := s.meterSyncAdapter.SyncMeter(r.Context(), meter); err != nil {
			writeError(w, http.StatusBadGateway, "Pricing metric changes could not be applied right now.")
			return
		}
		writeJSON(w, http.StatusOK, meter)
	case http.MethodGet:
		meter, err := s.meterService.GetMeter(tenantID, id)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, meter)
	default:
		writeMethodNotAllowed(w)
	}
}

func (s *Server) handleInvoicePreview(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "invoice preview is not available in the current alpha release")
}

func (s *Server) handleInvoiceByID(w http.ResponseWriter, r *http.Request) {
	tail := strings.TrimPrefix(r.URL.Path, "/v1/invoices/")
	parts := strings.Split(strings.Trim(tail, "/"), "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		writeError(w, http.StatusBadRequest, "invoice id is required")
		return
	}

	invoiceID := strings.TrimSpace(parts[0])
	if len(parts) == 2 && strings.EqualFold(strings.TrimSpace(parts[1]), "payment-receipts") {
		s.handleInvoicePaymentReceipts(w, r, invoiceID)
		return
	}
	if len(parts) == 2 && strings.EqualFold(strings.TrimSpace(parts[1]), "credit-notes") {
		s.handleInvoiceCreditNotes(w, r, invoiceID)
		return
	}
	if len(parts) == 2 && strings.EqualFold(strings.TrimSpace(parts[1]), "retry-payment") {
		if s.invoiceBillingAdapter == nil {
			writeError(w, http.StatusServiceUnavailable, "invoice billing adapter is required")
			return
		}
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w)
			return
		}

		rawBody, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if len(strings.TrimSpace(string(rawBody))) == 0 {
			rawBody = []byte("{}")
		}

		ctx := service.ContextWithLagoTenant(r.Context(), requestTenantID(r))
		statusCode, body, err := s.invoiceBillingAdapter.RetryInvoicePayment(ctx, invoiceID, rawBody)
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to proxy retry payment to lago: "+err.Error())
			return
		}
		if statusCode >= 200 && statusCode < 300 {
			if syncErr := s.materializeRetryPaymentProjection(r.Context(), requestTenantID(r), invoiceID); syncErr != nil && s.logger != nil {
				s.logger.Warn("materialize retry payment projection failed", "invoice_id", invoiceID, "tenant_id", requestTenantID(r), "error", syncErr)
			}
		}
		if statusCode < 200 || statusCode >= 300 {
			writeTranslatedUpstreamError(w, statusCode, "Payment retry could not be started right now.", body)
			return
		}
		writeJSONRaw(w, statusCode, body)
		return
	}
	if len(parts) == 2 && strings.EqualFold(strings.TrimSpace(parts[1]), "resend-email") {
		s.handleInvoiceResendEmail(w, r, invoiceID)
		return
	}
	if len(parts) == 2 && strings.EqualFold(strings.TrimSpace(parts[1]), "explainability") {
		if s.invoiceBillingAdapter == nil {
			writeError(w, http.StatusServiceUnavailable, "invoice billing adapter is required")
			return
		}
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w)
			return
		}

		feeTypes := make([]string, 0, 8)
		feeTypes = append(feeTypes, splitCommaSeparatedValues(r.URL.Query().Get("fee_types"))...)
		feeTypes = append(feeTypes, r.URL.Query()["fee_type"]...)
		lineItemSort := r.URL.Query().Get("line_item_sort")
		page, err := parseQueryInt(r, "page")
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		limit, err := parseQueryInt(r, "limit")
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		options, err := service.NewInvoiceExplainabilityOptions(feeTypes, lineItemSort, page, limit)
		if err != nil {
			writeDomainError(w, err)
			return
		}

		ctx := service.ContextWithLagoTenant(r.Context(), requestTenantID(r))
		statusCode, body, err := s.invoiceBillingAdapter.GetInvoice(ctx, invoiceID)
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to fetch invoice from lago: "+err.Error())
			return
		}
		if statusCode < 200 || statusCode >= 300 {
			writeTranslatedUpstreamError(w, statusCode, "Invoice explainability is unavailable right now.", body)
			return
		}

		explainability, err := service.BuildInvoiceExplainabilityFromLago(body, options)
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to compute explainability from lago invoice: "+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, explainability)
		return
	}

	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w)
			return
		}
		if s.invoiceBillingAdapter == nil {
			writeError(w, http.StatusServiceUnavailable, "invoice billing adapter is required")
			return
		}
		statusCode, body, detail, err := s.loadInvoiceDetail(r.Context(), requestTenantID(r), invoiceID)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		if statusCode < 200 || statusCode >= 300 {
			writeTranslatedUpstreamError(w, statusCode, "Invoice details could not be loaded right now.", body)
			return
		}
		writeJSON(w, http.StatusOK, detail)
		return
	}

	writeError(w, http.StatusBadRequest, "unsupported invoice subresource")
}

func (s *Server) handleUsageEvents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var req domain.UsageEvent
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		req.TenantID = requestTenantID(r)

		event, idempotent, err := s.usageService.CreateUsageEventWithIdempotency(req)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		status := http.StatusCreated
		if idempotent {
			status = http.StatusOK
		}
		writeJSON(w, status, event)
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
		from, err := parseOptionalTime(r.URL.Query().Get("from"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid from: "+err.Error())
			return
		}
		to, err := parseOptionalTime(r.URL.Query().Get("to"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid to: "+err.Error())
			return
		}

		events, err := s.usageService.ListUsageEvents(requestTenantID(r), service.ListUsageEventsRequest{
			CustomerID: r.URL.Query().Get("customer_id"),
			MeterID:    r.URL.Query().Get("meter_id"),
			Order:      r.URL.Query().Get("order"),
			From:       from,
			To:         to,
			Limit:      limit,
			Offset:     offset,
			Cursor:     r.URL.Query().Get("cursor"),
		})
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, events)
	default:
		writeMethodNotAllowed(w)
	}
}

func (s *Server) handleBilledEntries(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var req domain.BilledEntry
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		req.TenantID = requestTenantID(r)

		entry, idempotent, err := s.usageService.CreateBilledEntryWithIdempotency(req)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		status := http.StatusCreated
		if idempotent {
			status = http.StatusOK
		}
		writeJSON(w, status, entry)
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

		from, err := parseOptionalTime(r.URL.Query().Get("from"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid from: "+err.Error())
			return
		}
		to, err := parseOptionalTime(r.URL.Query().Get("to"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid to: "+err.Error())
			return
		}

		entries, err := s.usageService.ListBilledEntries(requestTenantID(r), service.ListBilledEntriesRequest{
			CustomerID:        r.URL.Query().Get("customer_id"),
			MeterID:           r.URL.Query().Get("meter_id"),
			BilledSource:      r.URL.Query().Get("billed_source"),
			BilledReplayJobID: r.URL.Query().Get("billed_replay_job_id"),
			Order:             r.URL.Query().Get("order"),
			From:              from,
			To:                to,
			Limit:             limit,
			Offset:            offset,
			Cursor:            r.URL.Query().Get("cursor"),
		})
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, entries)
	default:
		writeMethodNotAllowed(w)
	}
}

func (s *Server) handleReplayJobs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var req replay.CreateReplayJobRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		req.TenantID = requestTenantID(r)

		job, idempotent, err := s.replayService.CreateJob(req)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		status := http.StatusCreated
		if idempotent {
			status = http.StatusOK
		}
		writeJSON(w, status, map[string]any{
			"idempotent_replay": idempotent,
			"job":               s.decorateReplayJob(r, job),
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
		jobs, err := s.replayService.ListJobs(requestTenantID(r), replay.ListReplayJobsRequest{
			CustomerID: r.URL.Query().Get("customer_id"),
			MeterID:    r.URL.Query().Get("meter_id"),
			Status:     r.URL.Query().Get("status"),
			Limit:      limit,
			Offset:     offset,
			Cursor:     r.URL.Query().Get("cursor"),
		})
		if err != nil {
			writeDomainError(w, err)
			return
		}
		items := make([]map[string]any, 0, len(jobs.Items))
		for _, job := range jobs.Items {
			items = append(items, s.decorateReplayJob(r, job))
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"items":       items,
			"total":       jobs.Total,
			"limit":       jobs.Limit,
			"offset":      jobs.Offset,
			"next_cursor": jobs.NextCursor,
		})
	default:
		writeMethodNotAllowed(w)
	}
}

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

func (s *Server) handleReplayJobByID(w http.ResponseWriter, r *http.Request) {
	tail := strings.TrimPrefix(r.URL.Path, "/v1/replay-jobs/")
	parts := strings.Split(strings.Trim(tail, "/"), "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	id := strings.TrimSpace(parts[0])

	switch r.Method {
	case http.MethodGet:
		if len(parts) == 1 {
			job, err := s.replayService.GetJob(requestTenantID(r), id)
			if err != nil {
				writeDomainError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, s.decorateReplayJob(r, job))
			return
		}
		if len(parts) == 2 && strings.EqualFold(strings.TrimSpace(parts[1]), "events") {
			diag, err := s.replayService.GetJobDiagnostics(requestTenantID(r), id)
			if err != nil {
				writeDomainError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, diag)
			return
		}
		if len(parts) == 3 && strings.EqualFold(strings.TrimSpace(parts[1]), "artifacts") {
			s.handleReplayJobArtifact(w, r, id, strings.TrimSpace(parts[2]))
			return
		}
		writeError(w, http.StatusBadRequest, "unsupported replay job subresource")
	case http.MethodPost:
		if len(parts) == 2 && strings.EqualFold(strings.TrimSpace(parts[1]), "retry") {
			job, err := s.replayService.RetryJob(requestTenantID(r), id)
			if err != nil {
				writeDomainError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, s.decorateReplayJob(r, job))
			return
		}
		writeError(w, http.StatusBadRequest, "unsupported replay job subresource")
	default:
		writeMethodNotAllowed(w)
	}
}

func (s *Server) handleReplayJobArtifact(w http.ResponseWriter, r *http.Request, jobID, artifactName string) {
	tenantID := requestTenantID(r)
	diag, err := s.replayService.GetJobDiagnostics(tenantID, jobID)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	artifactName = strings.ToLower(strings.TrimSpace(artifactName))
	switch artifactName {
	case "report.json":
		payload := map[string]any{
			"job_id":               diag.Job.ID,
			"tenant_id":            diag.Job.TenantID,
			"status":               diag.Job.Status,
			"customer_id":          diag.Job.CustomerID,
			"meter_id":             diag.Job.MeterID,
			"from":                 diag.Job.From,
			"to":                   diag.Job.To,
			"processed_records":    diag.Job.ProcessedRecords,
			"attempt_count":        diag.Job.AttemptCount,
			"usage_events_count":   diag.UsageEventsCount,
			"usage_quantity":       diag.UsageQuantity,
			"billed_entries_count": diag.BilledEntriesCount,
			"billed_amount_cents":  diag.BilledAmountCents,
			"error":                diag.Job.Error,
			"generated_at":         time.Now().UTC(),
		}
		writeJSON(w, http.StatusOK, payload)
	case "report.csv":
		body, buildErr := replayDiagnosticsCSV(diag)
		if buildErr != nil {
			writeError(w, http.StatusInternalServerError, "failed to generate replay csv artifact")
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=replay_%s_report.csv", jobID))
		_, _ = w.Write([]byte(body))
	case "dataset_digest.txt":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=replay_%s_dataset_digest.txt", jobID))
		_, _ = w.Write([]byte(replayDatasetDigest(diag) + "\n"))
	default:
		writeError(w, http.StatusNotFound, "artifact not found")
	}
}

func (s *Server) decorateReplayJob(r *http.Request, job domain.ReplayJob) map[string]any {
	out := make(map[string]any)
	encoded, err := json.Marshal(job)
	if err != nil {
		out["id"] = job.ID
		out["status"] = job.Status
	} else {
		_ = json.Unmarshal(encoded, &out)
	}
	out["workflow_telemetry"] = replayWorkflowTelemetry(job)
	out["artifact_links"] = replayArtifactLinks(r, job.ID)
	return out
}

func replayWorkflowTelemetry(job domain.ReplayJob) map[string]any {
	currentStep := "queued"
	progressPercent := 0

	switch job.Status {
	case domain.ReplayQueued:
		currentStep = "queued"
		progressPercent = 0
	case domain.ReplayRunning:
		currentStep = "replay_processing"
		progressPercent = 50
	case domain.ReplayDone:
		currentStep = "completed"
		progressPercent = 100
	case domain.ReplayFailed:
		currentStep = "failed"
		progressPercent = 100
	}

	updatedAt := job.CreatedAt
	if job.CompletedAt != nil {
		updatedAt = job.CompletedAt.UTC()
	} else if job.StartedAt != nil {
		updatedAt = job.StartedAt.UTC()
	}

	return map[string]any{
		"current_step":      currentStep,
		"progress_percent":  progressPercent,
		"attempt_count":     job.AttemptCount,
		"last_attempt_at":   job.LastAttemptAt,
		"processed_records": job.ProcessedRecords,
		"updated_at":        updatedAt,
	}
}

func replayArtifactLinks(r *http.Request, jobID string) map[string]string {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return map[string]string{}
	}

	return map[string]string{
		"report_json":    replayArtifactURL(r, jobID, "report.json"),
		"report_csv":     replayArtifactURL(r, jobID, "report.csv"),
		"dataset_digest": replayArtifactURL(r, jobID, "dataset_digest.txt"),
	}
}

func replayArtifactURL(r *http.Request, jobID, artifact string) string {
	base := externalBaseURL(r)
	escapedID := url.PathEscape(strings.TrimSpace(jobID))
	escapedArtifact := url.PathEscape(strings.TrimSpace(artifact))
	return fmt.Sprintf("%s/v1/replay-jobs/%s/artifacts/%s", base, escapedID, escapedArtifact)
}

func externalBaseURL(r *http.Request) string {
	if r == nil {
		return ""
	}
	scheme := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if scheme == "" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	if host == "" {
		host = "localhost"
	}
	return scheme + "://" + host
}

func replayDiagnosticsCSV(diag replay.ReplayJobDiagnostics) (string, error) {
	var b strings.Builder
	writer := csv.NewWriter(&b)
	header := []string{
		"job_id",
		"status",
		"customer_id",
		"meter_id",
		"from",
		"to",
		"processed_records",
		"attempt_count",
		"usage_events_count",
		"usage_quantity",
		"billed_entries_count",
		"billed_amount_cents",
		"error",
	}
	if err := writer.Write(header); err != nil {
		return "", err
	}

	row := []string{
		diag.Job.ID,
		string(diag.Job.Status),
		diag.Job.CustomerID,
		diag.Job.MeterID,
		formatOptionalTime(diag.Job.From),
		formatOptionalTime(diag.Job.To),
		strconv.FormatInt(diag.Job.ProcessedRecords, 10),
		strconv.Itoa(diag.Job.AttemptCount),
		strconv.Itoa(diag.UsageEventsCount),
		strconv.FormatInt(diag.UsageQuantity, 10),
		strconv.Itoa(diag.BilledEntriesCount),
		strconv.FormatInt(diag.BilledAmountCents, 10),
		diag.Job.Error,
	}
	if err := writer.Write(row); err != nil {
		return "", err
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}
	return b.String(), nil
}

func replayDatasetDigest(diag replay.ReplayJobDiagnostics) string {
	payload := struct {
		JobID              string    `json:"job_id"`
		TenantID           string    `json:"tenant_id"`
		CustomerID         string    `json:"customer_id"`
		MeterID            string    `json:"meter_id"`
		From               string    `json:"from,omitempty"`
		To                 string    `json:"to,omitempty"`
		ProcessedRecords   int64     `json:"processed_records"`
		AttemptCount       int       `json:"attempt_count"`
		UsageEventsCount   int       `json:"usage_events_count"`
		UsageQuantity      int64     `json:"usage_quantity"`
		BilledEntriesCount int       `json:"billed_entries_count"`
		BilledAmountCents  int64     `json:"billed_amount_cents"`
		Status             string    `json:"status"`
		CompletedAt        time.Time `json:"completed_at,omitempty"`
	}{
		JobID:              diag.Job.ID,
		TenantID:           diag.Job.TenantID,
		CustomerID:         diag.Job.CustomerID,
		MeterID:            diag.Job.MeterID,
		From:               formatOptionalTime(diag.Job.From),
		To:                 formatOptionalTime(diag.Job.To),
		ProcessedRecords:   diag.Job.ProcessedRecords,
		AttemptCount:       diag.Job.AttemptCount,
		UsageEventsCount:   diag.UsageEventsCount,
		UsageQuantity:      diag.UsageQuantity,
		BilledEntriesCount: diag.BilledEntriesCount,
		BilledAmountCents:  diag.BilledAmountCents,
		Status:             string(diag.Job.Status),
	}
	if diag.Job.CompletedAt != nil {
		payload.CompletedAt = diag.Job.CompletedAt.UTC()
	}
	encoded, _ := json.Marshal(payload)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func formatOptionalTime(v *time.Time) string {
	if v == nil {
		return ""
	}
	return v.UTC().Format(time.RFC3339Nano)
}

func (s *Server) handleReconciliationReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	filter, err := parseFilter(r, requestTenantID(r))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	report, err := s.recService.GenerateReport(filter)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	format := strings.ToLower(r.URL.Query().Get("format"))
	if format == "csv" {
		csvData, err := s.recService.GenerateCSV(report)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename=reconciliation_report.csv")
		_, _ = w.Write([]byte(csvData))
		return
	}

	writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleInternalMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	if s.metricsFn == nil {
		writeError(w, http.StatusNotFound, "metrics not configured")
		return
	}

	metrics := s.metricsFn()
	if metrics == nil {
		metrics = map[string]any{}
	}
	metrics["http_requests_total"] = s.requestMetrics.Snapshot()
	metrics["tenant_http_requests_total"] = s.requestMetrics.TenantSnapshot()
	metrics["tenant_http_auth_denied_total"] = s.requestMetrics.AuthDeniedSnapshot()
	metrics["tenant_http_rate_limited_total"] = s.requestMetrics.RateLimitedSnapshot()
	metrics["tenant_http_rate_limit_errors_total"] = s.requestMetrics.RateLimitErrorSnapshot()

	writeJSON(w, http.StatusOK, map[string]any{
		"generated_at": time.Now().UTC(),
		"metrics":      metrics,
	})
}

func (s *Server) handleInternalReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	if s.readinessFn != nil {
		if err := s.readinessFn(); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{
				"status": "not_ready",
				"error":  err.Error(),
			})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ready",
		"at":     time.Now().UTC(),
	})
}

func parseFilter(r *http.Request, tenantID string) (reconcile.Filter, error) {
	filter := reconcile.Filter{
		TenantID:   normalizeTenantID(tenantID),
		CustomerID: strings.TrimSpace(r.URL.Query().Get("customer_id")),
	}
	filter.BilledReplayJobID = strings.TrimSpace(r.URL.Query().Get("billed_replay_job_id"))

	rawBilledSource := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("billed_source")))
	if rawBilledSource != "" {
		billedSource := domain.BilledEntrySource(rawBilledSource)
		switch billedSource {
		case domain.BilledEntrySourceAPI, domain.BilledEntrySourceReplayAdjustment:
			filter.BilledSource = billedSource
		default:
			return reconcile.Filter{}, fmt.Errorf("invalid billed_source: must be api or replay_adjustment")
		}
	}

	fromStr := strings.TrimSpace(r.URL.Query().Get("from"))
	toStr := strings.TrimSpace(r.URL.Query().Get("to"))
	absDeltaGTERaw := strings.TrimSpace(r.URL.Query().Get("abs_delta_gte"))
	if absDeltaGTERaw != "" {
		v, err := strconv.ParseInt(absDeltaGTERaw, 10, 64)
		if err != nil {
			return reconcile.Filter{}, fmt.Errorf("invalid abs_delta_gte: must be integer")
		}
		if v < 0 {
			return reconcile.Filter{}, fmt.Errorf("invalid abs_delta_gte: must be >= 0")
		}
		filter.AbsDeltaGTE = v
	}
	mismatchOnly, err := parseQueryBool(r, "mismatch_only")
	if err != nil {
		return reconcile.Filter{}, err
	}
	filter.MismatchOnly = mismatchOnly

	if fromStr != "" {
		from, err := parseTime(fromStr)
		if err != nil {
			return reconcile.Filter{}, fmt.Errorf("invalid from: %w", err)
		}
		filter.From = &from
	}
	if toStr != "" {
		to, err := parseTime(toStr)
		if err != nil {
			return reconcile.Filter{}, fmt.Errorf("invalid to: %w", err)
		}
		filter.To = &to
	}
	if filter.From != nil && filter.To != nil && filter.From.After(*filter.To) {
		return reconcile.Filter{}, fmt.Errorf("from must be <= to")
	}
	return filter, nil
}

func parseTime(v string) (time.Time, error) {
	if unixSec, err := strconv.ParseInt(v, 10, 64); err == nil {
		return time.Unix(unixSec, 0).UTC(), nil
	}
	return time.Parse(time.RFC3339, v)
}

func parseOptionalTime(v string) (*time.Time, error) {
	raw := strings.TrimSpace(v)
	if raw == "" {
		return nil, nil
	}
	parsed, err := parseTime(raw)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func parseQueryInt(r *http.Request, name string) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", name)
	}
	return n, nil
}

func parseQueryBool(r *http.Request, name string) (bool, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return false, nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean", name)
	}
	return v, nil
}

func parseOptionalQueryBool(r *http.Request, name string) (*bool, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return nil, nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, fmt.Errorf("%s must be a boolean", name)
	}
	return &v, nil
}

func splitCommaSeparatedValues(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		v := strings.TrimSpace(part)
		if v == "" {
			continue
		}
		out = append(out, v)
	}
	return out
}

func decodeJSON(r *http.Request, target any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeJSONRaw(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func setCompatibilityDeprecationHeaders(w http.ResponseWriter, successor string) {
	w.Header().Set("Deprecation", "true")
	if successor != "" {
		w.Header().Add("Link", fmt.Sprintf("<%s>; rel=\"successor-version\"", successor))
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeErrorCode(w, status, message, defaultErrorCodeForStatus(status))
}

func writeErrorCode(w http.ResponseWriter, status int, message, code string) {
	message, code = translateUserVisibleError(status, code, message)
	body := map[string]string{"error": message}
	if strings.TrimSpace(code) != "" {
		body["error_code"] = strings.TrimSpace(code)
	}
	if requestID := strings.TrimSpace(w.Header().Get(requestIDHeaderKey)); requestID != "" {
		body["request_id"] = requestID
	}
	writeJSON(w, status, body)
}

func writeMethodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

type upstreamErrorEnvelope struct {
	Status       int                  `json:"status"`
	Error        string               `json:"error"`
	Code         string               `json:"code"`
	Provider     *upstreamProviderRef `json:"provider,omitempty"`
	ErrorDetails map[string]any       `json:"error_details,omitempty"`
}

type upstreamProviderRef struct {
	Code string `json:"code"`
}

func writeTranslatedUpstreamError(w http.ResponseWriter, status int, fallback string, body []byte) {
	if status < http.StatusBadRequest || status > 599 {
		status = http.StatusBadGateway
	}
	message, code := translateUpstreamUserVisibleError(status, fallback, body)
	writeErrorCode(w, status, message, code)
}

func writeDomainError(w http.ResponseWriter, err error) {
	if err == nil {
		writeError(w, http.StatusInternalServerError, "unknown error")
		return
	}
	writeErrorCode(w, classifyDomainErrorStatus(err), err.Error(), classifyDomainErrorCode(err))
}

func translateUpstreamUserVisibleError(status int, fallback string, body []byte) (string, string) {
	code := defaultErrorCodeForStatus(status)
	if env, ok := decodeUpstreamErrorEnvelope(body); ok {
		if message, translatedCode, matched := translateStructuredUpstreamError(status, fallback, env); matched {
			return message, translatedCode
		}
		if message := strings.TrimSpace(env.Error); message != "" {
			return translateUserVisibleError(status, coalesceErrorCode(env.Code, code), message)
		}
	}
	return translateUserVisibleError(status, code, fallback)
}

func decodeUpstreamErrorEnvelope(body []byte) (upstreamErrorEnvelope, bool) {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return upstreamErrorEnvelope{}, false
	}
	var env upstreamErrorEnvelope
	if err := json.Unmarshal([]byte(trimmed), &env); err != nil {
		return upstreamErrorEnvelope{}, false
	}
	if strings.TrimSpace(env.Error) == "" && strings.TrimSpace(env.Code) == "" && env.Status == 0 {
		return upstreamErrorEnvelope{}, false
	}
	return env, true
}

func translateStructuredUpstreamError(status int, fallback string, env upstreamErrorEnvelope) (string, string, bool) {
	upstreamCode := strings.TrimSpace(env.Code)
	providerCode := ""
	if env.Provider != nil {
		providerCode = strings.ToLower(strings.TrimSpace(env.Provider.Code))
	}
	detailsMessage := strings.ToLower(strings.TrimSpace(stringMapValue(env.ErrorDetails, "message")))
	detailsCode := strings.ToLower(strings.TrimSpace(stringMapValue(env.ErrorDetails, "code")))
	httpStatus := intMapValue(env.ErrorDetails, "http_status")
	providerName := strings.ToLower(strings.TrimSpace(stringMapValue(env.ErrorDetails, "provider_name")))
	thirdParty := strings.ToLower(strings.TrimSpace(stringMapValue(env.ErrorDetails, "third_party")))

	switch upstreamCode {
	case "provider_error":
		if providerCode == "stripe" {
			if httpStatus == http.StatusUnauthorized ||
				strings.Contains(detailsMessage, "invalid api key") ||
				strings.Contains(detailsMessage, "authentication") ||
				strings.Contains(detailsCode, "invalid_api_key") {
				return "Stripe credentials could not be verified.", "stripe_authentication_failed", true
			}
			return "Stripe rejected the request.", "stripe_request_failed", true
		}
		return "The billing provider rejected the request.", "provider_request_failed", true
	case "too_many_provider_requests":
		if providerName == "stripe" || providerCode == "stripe" {
			return "Stripe is rate limiting requests right now.", "stripe_rate_limited", true
		}
		return "The billing provider is rate limiting requests right now.", "provider_rate_limited", true
	case "third_party_error":
		if thirdParty == "stripe" || providerCode == "stripe" {
			return "Stripe rejected the request.", "stripe_request_failed", true
		}
		return "An external billing provider rejected the request.", "provider_request_failed", true
	case "validation_errors":
		switch {
		case status == http.StatusNotFound && strings.Contains(strings.ToLower(fallback), "invoice detail"):
			return "Invoice could not be found.", "not_found", true
		case status == http.StatusNotFound && strings.Contains(strings.ToLower(fallback), "credit notes"):
			return "Invoice could not be found.", "not_found", true
		}
	}

	switch status {
	case http.StatusUnauthorized:
		return "Billing service authentication failed.", "billing_authentication_failed", true
	case http.StatusForbidden:
		return "This billing action is not available right now.", "billing_access_denied", true
	case http.StatusTooManyRequests:
		return "Billing activity is temporarily rate limited.", "rate_limited", true
	case http.StatusNotFound:
		if strings.Contains(strings.ToLower(fallback), "invoice") {
			return "Invoice could not be found.", "not_found", true
		}
	}

	return "", "", false
}

func translateUserVisibleError(status int, code, message string) (string, string) {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return trimmed, code
	}

	lower := strings.ToLower(trimmed)
	switch {
	case strings.Contains(lower, "lago organization id is required"),
		strings.Contains(lower, "workspace has no billing execution context"),
		strings.Contains(lower, "workspace billing binding exists but is not ready"),
		strings.Contains(lower, "billing setup is incomplete for this workspace"):
		return "Billing setup is incomplete for this workspace or connection.", code
	case strings.Contains(lower, "billing provider connection must be checked before workspace assignment"):
		return "Check this Stripe connection before assigning it to a workspace.", code
	case strings.Contains(lower, "meter sync adapter is required"),
		strings.Contains(lower, "lago meter sync adapter is required"),
		strings.Contains(lower, "lago tax sync adapter is required"),
		strings.Contains(lower, "lago plan sync adapter is required"),
		strings.Contains(lower, "lago subscription sync adapter is required"),
		strings.Contains(lower, "lago usage sync adapter is required"):
		return "Pricing updates are unavailable right now.", code
	case strings.Contains(lower, "metric created but lago sync failed"),
		strings.Contains(lower, "meter created but lago sync failed"),
		strings.Contains(lower, "meter updated but lago sync failed"),
		strings.Contains(lower, "lago meter sync failed"):
		return "Pricing metric changes could not be applied right now.", code
	case strings.Contains(lower, "lago tax sync failed"):
		return "Tax changes could not be applied right now.", code
	case strings.Contains(lower, "lago plan sync failed"),
		strings.Contains(lower, "lago add-on sync failed"),
		strings.Contains(lower, "lago coupon sync failed"),
		strings.Contains(lower, "lago fixed charge sync failed"),
		strings.Contains(lower, "list lago fixed charges failed"),
		strings.Contains(lower, "decode lago fixed charges response"),
		strings.Contains(lower, "lago billable metric lookup failed"),
		strings.Contains(lower, "decode lago billable metric response"):
		return "Pricing changes could not be applied right now.", code
	case strings.Contains(lower, "package pricing with overage is not yet supported for lago plan sync"),
		(strings.Contains(lower, "pricing mode") && strings.Contains(lower, "not supported for lago plan sync")):
		return "This pricing configuration is not supported yet.", code
	case strings.Contains(lower, "lago subscription sync failed"),
		strings.Contains(lower, "lago subscription terminate failed"),
		strings.Contains(lower, "list lago applied coupons failed"),
		strings.Contains(lower, "decode lago applied coupons response"),
		strings.Contains(lower, "apply lago coupon failed"),
		strings.Contains(lower, "delete lago applied coupon failed"):
		return "Subscription changes could not be applied right now.", code
	case strings.Contains(lower, "count aggregation requires quantity=1 for lago usage sync"),
		(strings.Contains(lower, "unsupported aggregation") && strings.Contains(lower, "for lago usage sync")),
		(strings.Contains(lower, "unsupported aggregation") && strings.Contains(lower, "for lago sync")):
		return "This usage configuration is not supported yet.", code
	case strings.Contains(lower, "lago usage sync failed"):
		return "Usage could not be recorded right now.", code
	case strings.Contains(lower, "failed to load payment receipts from lago"):
		return "Payment receipts could not be loaded right now.", code
	case strings.Contains(lower, "failed to load credit notes from lago"):
		return "Credit notes could not be loaded right now.", code
	case strings.Contains(lower, "failed to proxy payment retry to lago"):
		return "Payment retry could not be started right now.", code
	case strings.Contains(lower, "failed to fetch invoice from lago"):
		return "Invoice details could not be loaded right now.", code
	case strings.Contains(lower, "failed to compute explainability from lago invoice"):
		return "Invoice explainability is unavailable right now.", code
	case strings.Contains(lower, "lago webhook service is required"):
		return "Billing activity is unavailable right now.", code
	case strings.Contains(lower, "lago invoice adapter is required"),
		strings.Contains(lower, "invoice billing adapter is required"):
		return "Billing actions are unavailable right now.", code
	case strings.Contains(lower, "lago customer billing adapter is required"):
		return "Customer billing is unavailable right now.", code
	case strings.Contains(lower, "stripe_secret_key or lago_webhook_hmac_key is required"),
		strings.Contains(lower, "stripe_secret_key is required"),
		strings.Contains(lower, "stripe secret key is required"):
		return "Stripe credentials are required.", code
	case strings.Contains(lower, "stripe verification request failed"):
		return "Stripe could not be reached right now.", code
	case strings.Contains(lower, "payment provider code is required"):
		return "A billing provider must be configured before payment setup can begin.", code
	case strings.Contains(lower, "unsupported payment provider code"):
		return "The configured billing provider is not supported.", code
	default:
		return trimmed, code
	}
}

func coalesceErrorCode(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func stringMapValue(values map[string]any, key string) string {
	if len(values) == 0 {
		return ""
	}
	raw, ok := values[key]
	if !ok || raw == nil {
		return ""
	}
	switch typed := raw.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return fmt.Sprint(raw)
	}
}

func intMapValue(values map[string]any, key string) int {
	if len(values) == 0 {
		return 0
	}
	raw, ok := values[key]
	if !ok || raw == nil {
		return 0
	}
	switch typed := raw.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func classifyDomainErrorStatus(err error) int {
	switch {
	case err == nil:
		return http.StatusInternalServerError
	case errors.Is(err, store.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, store.ErrAlreadyExists), errors.Is(err, store.ErrDuplicateKey):
		return http.StatusConflict
	case errors.Is(err, service.ErrValidation):
		return http.StatusBadRequest
	case errors.Is(err, service.ErrDependency):
		return http.StatusBadGateway
	case errors.Is(err, service.ErrWorkspaceLastActiveAdmin):
		return http.StatusConflict
	case errors.Is(err, service.ErrWorkspaceSelfMembershipMutation):
		return http.StatusForbidden
	case errors.Is(err, service.ErrBrowserTenantAccessDenied):
		return http.StatusForbidden
	case errors.Is(err, service.ErrBrowserTenantSelection):
		return http.StatusConflict
	}

	lower := strings.ToLower(err.Error())
	switch {
	case strings.Contains(lower, "not found"):
		return http.StatusNotFound
	case strings.Contains(lower, "validation"), strings.Contains(lower, "required"), strings.Contains(lower, "invalid"):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func classifyDomainErrorCode(err error) string {
	switch {
	case err == nil:
		return "internal_error"
	case errors.Is(err, store.ErrNotFound):
		return "not_found"
	case errors.Is(err, store.ErrAlreadyExists), errors.Is(err, store.ErrDuplicateKey):
		return "already_exists"
	case errors.Is(err, service.ErrValidation):
		return "validation_error"
	case errors.Is(err, service.ErrDependency):
		return "dependency_error"
	case errors.Is(err, service.ErrWorkspaceLastActiveAdmin):
		return "last_active_admin_conflict"
	case errors.Is(err, service.ErrWorkspaceSelfMembershipMutation):
		return "self_membership_mutation_forbidden"
	case errors.Is(err, service.ErrBrowserTenantAccessDenied):
		return "tenant_access_denied"
	case errors.Is(err, service.ErrBrowserTenantSelection):
		return "workspace_selection_required"
	default:
		return defaultErrorCodeForStatus(classifyDomainErrorStatus(err))
	}
}

func defaultErrorCodeForStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusMethodNotAllowed:
		return "method_not_allowed"
	case http.StatusConflict:
		return "conflict"
	case http.StatusGone:
		return "gone"
	case http.StatusTooManyRequests:
		return "rate_limited"
	case http.StatusNotImplemented:
		return "not_implemented"
	case http.StatusBadGateway:
		return "bad_gateway"
	case http.StatusServiceUnavailable:
		return "service_unavailable"
	case http.StatusInternalServerError:
		return "internal_error"
	default:
		if status >= http.StatusInternalServerError {
			return "internal_error"
		}
		if status >= http.StatusBadRequest {
			return "request_error"
		}
		return ""
	}
}

func classifyDomainErrorKind(err error) string {
	switch classifyDomainErrorStatus(err) {
	case http.StatusBadRequest:
		return "validation"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	default:
		return "internal"
	}
}
