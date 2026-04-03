package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
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
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

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
	meterSyncAdapter                   service.MeterSyncAdapter
	taxSyncAdapter                     service.TaxSyncAdapter
	planSyncAdapter                    service.PlanSyncAdapter
	subscriptionSyncAdapter            service.SubscriptionSyncAdapter
	usageSyncAdapter                   service.UsageSyncAdapter
	invoiceBillingAdapter              service.InvoiceBillingAdapter
	customerBillingAdapter             service.CustomerBillingAdapter
	paymentStatusSvc                   *service.PaymentStatusService
	stripeWebhookSvc                   *service.StripeWebhookService
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
	router                             chi.Router
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

func WithPaymentStatusService(svc *service.PaymentStatusService) ServerOption {
	return func(s *Server) {
		s.paymentStatusSvc = svc
	}
}

func WithStripeWebhookService(stripeWebhookSvc *service.StripeWebhookService) ServerOption {
	return func(s *Server) {
		s.stripeWebhookSvc = stripeWebhookSvc
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
		router:                 chi.NewRouter(),
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
		WithBillingProviderConnectionService(s.billingProviderConnectionService)
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
	return s.router
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

// requireAuth returns a chi middleware that authenticates the request, enforces
// the given minimum role, performs CSRF validation for session-based callers on
// unsafe methods, and applies post-auth rate limiting.  The scope parameter
// controls whether the route requires platform scope, tenant scope, or either
// (session-self routes).
func (s *Server) requireAuth(minRole Role, scope authScope) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if s.authorizer == nil && s.sessionManager == nil {
			return next
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Pre-auth rate limiting for protected routes.
			if policy, identifier, failOpen, ok := s.preAuthRateLimitTarget(r, true); ok {
				if !s.enforceRateLimit(w, r, policy, identifier, "", failOpen) {
					return
				}
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

			switch scope {
			case authScopePlatform:
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
			case authScopeSessionSelf:
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
					if !roleAllows(principal.Role, minRole) {
						s.logAuthFailure(r, http.StatusForbidden, "insufficient_role", nil)
						writeError(w, http.StatusForbidden, "forbidden")
						return
					}
				default:
					s.logAuthFailure(r, http.StatusForbidden, "session_scope_required", nil)
					writeError(w, http.StatusForbidden, "forbidden")
					return
				}
			default: // authScopeTenant
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
				if !roleAllows(principal.Role, minRole) {
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
				if expectedCSRF == "" || providedCSRF == "" || !subtleConstantTimeMatch(expectedCSRF, providedCSRF) {
					s.logAuthFailure(r, http.StatusForbidden, "csrf_mismatch", nil)
					writeError(w, http.StatusForbidden, "forbidden")
					return
				}
			}

			if policy, identifier, failOpen, ok := s.authRateLimitTarget(r, principal, minRole, usingSession); ok {
				if !s.enforceRateLimit(w, r, policy, identifier, principal.TenantID, failOpen) {
					return
				}
			}

			next.ServeHTTP(w, r.WithContext(withPrincipal(r.Context(), principal)))
		})
	}
}

type authScope int

const (
	authScopeTenant     authScope = iota
	authScopePlatform   authScope = iota
	authScopeSessionSelf authScope = iota
)

// preAuthRateLimitMiddleware applies rate limiting to public (unauthenticated)
// routes that still need protection, such as webhooks and login endpoints.
func (s *Server) preAuthRateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if policy, identifier, failOpen, ok := s.preAuthRateLimitTarget(r, false); ok {
			if !s.enforceRateLimit(w, r, policy, identifier, "", failOpen) {
				return
			}
		}
		next.ServeHTTP(w, r)
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
	case path == "/internal/stripe/webhooks":
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
	case path == "/internal/stripe/webhooks":
		return "/internal/stripe/webhooks"
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

func (s *Server) registerRoutes() {
	r := s.router

	// Global middleware (applied to every route).
	r.Use(s.requestIDMiddleware)
	r.Use(s.instrumentMiddleware)
	r.Use(s.corsMiddleware)
	r.Use(s.accessLogMiddleware)
	r.Use(chimiddleware.Recoverer)

	// ── Public routes (no auth) ─────────────────────────────────────────
	r.Get("/health", s.handleHealth)
	r.HandleFunc("/internal/stripe/webhooks", s.handleStripeWebhooks)
	r.HandleFunc("/v1/ui/sessions/login", s.handleUISessionLogin)
	r.Get("/v1/ui/auth/providers", s.handleUIAuthProviders)
	r.Post("/v1/ui/password/forgot", s.handleUIPasswordForgot)
	r.Post("/v1/ui/password/reset", s.handleUIPasswordReset)
	r.HandleFunc("/v1/ui/workspaces/pending", s.handleUIWorkspaceSelectionPending)
	r.HandleFunc("/v1/ui/workspaces/select", s.handleUIWorkspaceSelectionSelect)
	r.HandleFunc("/v1/ui/invitations/*", s.handleUIInvitations)
	r.HandleFunc("/v1/ui/auth/sso/*", s.handleUISSO)
	r.Post("/v1/ui/sessions/rate-limit-probe", s.handleUIPreAuthRateLimitProbe)

	// ── Session-self routes (any authenticated scope — reader+) ─────────
	r.Group(func(r chi.Router) {
		r.Use(s.requireAuth(RoleReader, authScopeSessionSelf))
		r.HandleFunc("/v1/ui/sessions/me", s.handleUISessionMe)
		r.HandleFunc("/v1/ui/sessions/logout", s.handleUISessionLogout)
	})

	// ── Platform admin routes ───────────────────────────────────────────
	r.Group(func(r chi.Router) {
		r.Use(s.requireAuth(RoleAdmin, authScopePlatform))
		r.Get("/internal/metrics", s.handleInternalMetrics)
		r.Get("/internal/ready", s.handleInternalReady)
		r.HandleFunc("/internal/onboarding/tenants", s.handleInternalOnboardingTenants)
		r.HandleFunc("/internal/onboarding/tenants/*", s.handleInternalOnboardingTenantByID)
		r.HandleFunc("/internal/billing-provider-connections", s.handleInternalBillingProviderConnections)
		r.HandleFunc("/internal/billing-provider-connections/*", s.handleInternalBillingProviderConnectionByID)
		r.Get("/internal/tenants/audit", s.handleInternalTenantAudit)
		r.HandleFunc("/internal/tenants", s.handleInternalTenants)
		r.HandleFunc("/internal/tenants/*", s.handleInternalTenantByID)
	})

	// ── Tenant admin routes ─────────────────────────────────────────────
	r.Group(func(r chi.Router) {
		r.Use(s.requireAuth(RoleAdmin, authScopeTenant))
		r.HandleFunc("/v1/workspace/members", s.handleTenantWorkspaceMembers)
		r.HandleFunc("/v1/workspace/members/*", s.handleTenantWorkspaceMembers)
		r.HandleFunc("/v1/workspace/invitations", s.handleTenantWorkspaceInvitations)
		r.HandleFunc("/v1/workspace/invitations/*", s.handleTenantWorkspaceInvitations)
		r.HandleFunc("/v1/workspace/service-accounts", s.handleTenantWorkspaceServiceAccounts)
		r.HandleFunc("/v1/workspace/service-accounts/*", s.handleTenantWorkspaceServiceAccounts)
		r.HandleFunc("/v1/api-keys", s.handleAPIKeys)
		r.HandleFunc("/v1/api-keys/audit", s.handleAPIKeyAuditEvents)
		r.HandleFunc("/v1/api-keys/audit/exports", s.handleAPIKeyAuditExports)
		r.HandleFunc("/v1/api-keys/audit/exports/*", s.handleAPIKeyAuditExportByID)
		r.HandleFunc("/v1/api-keys/*", s.handleAPIKeyByID)
	})

	// ── Tenant writer routes ────────────────────────────────────────────
	r.Group(func(r chi.Router) {
		r.Use(s.requireAuth(RoleWriter, authScopeTenant))
		r.Post("/v1/customer-onboarding", s.handleCustomerOnboarding)
		r.Post("/v1/customers", s.handleCustomers)
		r.Post("/v1/rating-rules", s.handleRatingRules)
		r.Post("/v1/meters", s.handleMeters)
		r.Put("/v1/meters/*", s.handleMeterByID)
		r.Post("/v1/pricing/metrics", s.handlePricingMetrics)
		r.Post("/v1/taxes", s.handleTaxes)
		r.Post("/v1/plans", s.handlePlans)
		r.Patch("/v1/plans/*", s.handlePlanByID)
		r.Post("/v1/subscriptions", s.handleSubscriptions)
		r.Post("/v1/subscriptions/*", s.handleSubscriptionByID)
		r.Patch("/v1/subscriptions/*", s.handleSubscriptionByID)
		r.Patch("/v1/customers/*", s.handleCustomerByExternalID)
		r.Put("/v1/customers/{externalId}/billing-profile", s.handleCustomerByExternalID)
		r.Post("/v1/customers/{externalId}/billing-profile/retry-sync", s.handleCustomerByExternalID)
		r.Post("/v1/customers/{externalId}/payment-setup/checkout-url", s.handleCustomerByExternalID)
		r.Post("/v1/customers/{externalId}/payment-setup/request", s.handleCustomerByExternalID)
		r.Post("/v1/customers/{externalId}/payment-setup/resend", s.handleCustomerByExternalID)
		r.Post("/v1/customers/{externalId}/payment-setup/refresh", s.handleCustomerByExternalID)
		r.Post("/v1/usage-events", s.handleUsageEvents)
		r.Post("/v1/billed-entries", s.handleBilledEntries)
		r.Post("/v1/replay-jobs", s.handleReplayJobs)
		r.Post("/v1/replay-jobs/{id}/retry", s.handleReplayJobByID)
		r.Post("/v1/invoices/{id}/retry-payment", s.handleInvoiceByID)
		r.Post("/v1/invoices/{id}/resend-email", s.handleInvoiceByID)
		r.Post("/v1/payment-receipts/{id}/resend-email", s.handlePaymentReceiptByID)
		r.Post("/v1/credit-notes/{id}/resend-email", s.handleCreditNoteByID)
		r.Post("/v1/payments/{id}/retry", s.handlePaymentByID)
		r.Put("/v1/dunning/policy", s.handleDunningPolicy)
		r.Post("/v1/dunning/runs/{id}/collect-payment-reminder", s.handleDunningRunByID)
		r.Post("/v1/add-ons", s.handleAddOns)
		r.Post("/v1/coupons", s.handleCoupons)
	})

	// ── Tenant reader routes ────────────────────────────────────────────
	r.Group(func(r chi.Router) {
		r.Use(s.requireAuth(RoleReader, authScopeTenant))
		r.Get("/v1/customers", s.handleCustomers)
		r.Get("/v1/customers/*", s.handleCustomerByExternalID)
		r.Get("/v1/rating-rules", s.handleRatingRules)
		r.Get("/v1/rating-rules/*", s.handleRatingRuleByID)
		r.Get("/v1/meters", s.handleMeters)
		r.Get("/v1/meters/*", s.handleMeterByID)
		r.Get("/v1/pricing/metrics", s.handlePricingMetrics)
		r.Get("/v1/pricing/metrics/*", s.handlePricingMetricByID)
		r.Get("/v1/taxes", s.handleTaxes)
		r.Get("/v1/taxes/*", s.handleTaxByID)
		r.Get("/v1/add-ons", s.handleAddOns)
		r.Get("/v1/add-ons/*", s.handleAddOnByID)
		r.Get("/v1/coupons", s.handleCoupons)
		r.Get("/v1/coupons/*", s.handleCouponByID)
		r.Get("/v1/plans", s.handlePlans)
		r.Get("/v1/plans/*", s.handlePlanByID)
		r.Get("/v1/subscriptions", s.handleSubscriptions)
		r.Get("/v1/subscriptions/*", s.handleSubscriptionByID)
		r.Get("/v1/invoices", s.handleInvoices)
		r.Get("/v1/invoices/preview", s.handleInvoicePreview)
		r.Get("/v1/invoices/*", s.handleInvoiceByID)
		r.Get("/v1/payment-receipts/*", s.handlePaymentReceiptByID)
		r.Get("/v1/credit-notes/*", s.handleCreditNoteByID)
		r.Get("/v1/payments", s.handlePayments)
		r.Get("/v1/payments/*", s.handlePaymentByID)
		r.Get("/v1/usage-events", s.handleUsageEvents)
		r.Get("/v1/billed-entries", s.handleBilledEntries)
		r.Get("/v1/replay-jobs", s.handleReplayJobs)
		r.Get("/v1/replay-jobs/*", s.handleReplayJobByID)
		r.Get("/v1/invoice-payment-statuses", s.handleInvoicePaymentStatuses)
		r.Get("/v1/invoice-payment-statuses/summary", s.handleInvoicePaymentStatuses)
		r.Get("/v1/invoice-payment-statuses/*", s.handleInvoicePaymentStatusByID)
		r.Get("/v1/dunning/policy", s.handleDunningPolicy)
		r.Get("/v1/dunning/runs", s.handleDunningRuns)
		r.Get("/v1/dunning/runs/*", s.handleDunningRunByID)
		r.Get("/v1/reconciliation-report", s.handleReconciliationReport)
	})
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
