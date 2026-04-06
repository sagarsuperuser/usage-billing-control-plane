package api

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
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
	stripeWebhookSecret                string
	invoicePDFService                  *service.InvoicePDFService
	invoiceGenerationService           *service.InvoiceGenerationService
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
	sessionDisplayNameKey      = "principal_display_name"
	sessionScopeKey            = "principal_scope"
	sessionRoleKey             = "principal_role"
	sessionTenantIDKey         = "principal_tenant_id"
	sessionTenantNameKey       = "principal_tenant_name"
	sessionAPIKeyIDKey         = "principal_api_key_id"
	sessionCSRFKey             = "csrf_token"
	sessionSSOStateKey         = "ui_sso_state"
	sessionSSOProviderKey      = "ui_sso_provider"
	sessionSSONonceKey         = "ui_sso_nonce"
	sessionSSOPKCEKey          = "ui_sso_pkce_verifier"
	sessionSSONextKey          = "ui_sso_next"
	sessionSSOTenantIDKey      = "ui_sso_tenant_id"
	csrfHeaderName             = "X-CSRF-Token"
	requestIDHeaderKey         = "X-Request-ID"
)


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
		start := time.Now()
		recorder := &statusCapturingResponseWriter{ResponseWriter: w}
		next.ServeHTTP(recorder, r)
		statusCode := recorder.statusCode
		if statusCode == 0 {
			statusCode = http.StatusOK
		}
		route := routePattern(r)
		s.requestMetrics.Inc(r.Method, route, statusCode)
		httpRequestsTotal.WithLabelValues(r.Method, route, strconv.Itoa(statusCode)).Inc()
		httpRequestDuration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
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
			"route", routePattern(r),
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
			s.logError("http request", attrs...)
		case statusCode >= http.StatusBadRequest:
			s.logWarn("http request", attrs...)
		default:
			s.logInfo("http request", attrs...)
		}
	})
}

func (s *Server) logAuthFailure(r *http.Request, statusCode int, reason string, err error) {
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

	attrs := []any{
		"component", "api",
		"event", "auth_denied",
		"request_id", requestIDFromContext(r.Context()),
		"method", r.Method,
		"route", routePattern(r),
		"path", r.URL.Path,
		"status", statusCode,
		"reason", reason,
		"auth_method", inferAuthMethod(r),
	}
	if err != nil {
		attrs = append(attrs, "error", err.Error())
	}
	if tenantID != "" {
		attrs = append(attrs, "tenant_id", tenantID)
	}
	switch {
	case statusCode >= http.StatusInternalServerError:
		s.logError("http auth denied", attrs...)
	default:
		s.logWarn("http auth denied", attrs...)
	}
}

func (s *Server) logOnboardingFailure(r *http.Request, req service.TenantOnboardingRequest, err error) {
	if err == nil {
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
		"route", routePattern(r),
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
	s.logError("tenant onboarding failed", attrs...)
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
				switch {
				case errors.Is(err, errUnauthorized):
					statusCode = http.StatusUnauthorized
					reason = "unauthorized"
				case errors.Is(err, errTenantBlocked):
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
			case authScopeAuthenticated:
				// Any valid session — just need a user identity, no workspace required.
				if principal.SubjectID == "" {
					s.logAuthFailure(r, http.StatusUnauthorized, "identity_required", nil)
					writeError(w, http.StatusUnauthorized, "unauthorized")
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
				if expectedCSRF == "" || providedCSRF == "" || subtle.ConstantTimeCompare([]byte(expectedCSRF), []byte(providedCSRF)) != 1 {
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
	authScopeTenant        authScope = iota
	authScopePlatform      authScope = iota
	authScopeSessionSelf   authScope = iota
	authScopeAuthenticated authScope = iota // any valid session, no workspace required
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
		return RateLimitPolicyPreAuthProtected, "ip:" + requestClientIP(r) + ":route:" + routePattern(r), s.rateLimitFailOpen, true
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
	return "ip:" + requestClientIP(r) + ":route:" + routePattern(r)
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
		attrs := []any{
			"component", "api",
			"event", "rate_limit_error",
			"request_id", requestIDFromContext(r.Context()),
			"policy", policy,
			"fail_open", failOpen,
			"error", err.Error(),
		}
		if failOpen {
			s.logWarn("rate limit check failed (fail-open)", attrs...)
		} else {
			s.logError("rate limit check failed (fail-closed)", attrs...)
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
	s.logWarn("rate limit exceeded",
		"component", "api",
		"event", "rate_limited",
		"request_id", requestIDFromContext(r.Context()),
		"policy", policy,
		"path", r.URL.Path,
		"method", r.Method,
	)
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
	displayName := strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionDisplayNameKey))
	scopeRaw := strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionScopeKey))
	roleRaw := strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionRoleKey))
	tenantID := strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionTenantIDKey))
	tenantName := strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionTenantNameKey))
	apiKeyID := strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionAPIKeyIDKey))
	// Browser sessions are always tenant-scoped. Platform ops use API keys.
	switch Scope(scopeRaw) {
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
			DisplayName: displayName,
			Scope:       ScopeTenant,
			Role:        role,
			TenantID:    normalizeTenantID(tenantID),
			TenantName:  tenantName,
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


func (s *Server) registerRoutes() {
	r := s.router

	// Global middleware (applied to every route).
	r.Use(s.requestIDMiddleware)
	r.Use(s.versionMiddleware)
	r.Use(s.instrumentMiddleware)
	r.Use(s.auditLogMiddleware)
	r.Use(s.corsMiddleware)
	r.Use(s.accessLogMiddleware)
	r.Use(chimiddleware.Recoverer)

	// ── Public routes (no auth) ─────────────────────────────────────────
	r.Get("/health", s.handleHealth)
	r.Handle("/metrics", prometheusHandler())
	r.Post("/internal/stripe/webhooks", s.handleStripeWebhooks)
	r.HandleFunc("/v1/ui/sessions/login", s.handleUISessionLogin)
	r.Post("/v1/ui/register", s.handleUIRegister)
	r.Get("/v1/ui/auth/providers", s.handleUIAuthProviders)
	r.Post("/v1/ui/password/forgot", s.handleUIPasswordForgot)
	r.Post("/v1/ui/password/reset", s.handleUIPasswordReset)
	r.HandleFunc("/v1/ui/invitations/*", s.handleUIInvitations)
	r.HandleFunc("/v1/ui/auth/sso/*", s.handleUISSO)
	r.Post("/v1/ui/sessions/rate-limit-probe", s.handleUIPreAuthRateLimitProbe)

	// ── Session-self routes (any authenticated scope — reader+) ─────────
	r.Group(func(r chi.Router) {
		r.Use(s.requireAuth(RoleReader, authScopeSessionSelf))
		r.HandleFunc("/v1/ui/sessions/me", s.handleUISessionMe)
		r.HandleFunc("/v1/ui/sessions/logout", s.handleUISessionLogout)
		r.HandleFunc("/v1/ui/sessions/workspaces", s.handleUISessionWorkspaces)
		r.HandleFunc("/v1/ui/sessions/switch-workspace", s.handleUISessionSwitchWorkspace)
	})

	// ── Authenticated routes (any valid session, no workspace required) ─
	r.Group(func(r chi.Router) {
		r.Use(s.requireAuth(RoleReader, authScopeAuthenticated))
		r.Post("/v1/ui/workspaces", s.handleUISelfServeCreateWorkspace)
	})

	// ── Platform admin routes ───────────────────────────────────────────
	r.Group(func(r chi.Router) {
		r.Use(s.requireAuth(RoleAdmin, authScopePlatform))
		r.Get("/internal/metrics", s.handleInternalMetrics)
		r.Get("/internal/ready", s.handleInternalReady)

		// Onboarding
		r.Post("/internal/onboarding/tenants", s.handleInternalOnboardingTenants)
		r.Get("/internal/onboarding/tenants/{id}", s.handleInternalOnboardingTenantByID)

		// Billing provider connections
		r.Get("/internal/billing-provider-connections", s.listBillingProviderConnections)
		r.Post("/internal/billing-provider-connections", s.createBillingProviderConnection)
		r.Get("/internal/billing-provider-connections/{id}", s.getBillingProviderConnection)
		r.Patch("/internal/billing-provider-connections/{id}", s.updateBillingProviderConnection)
		r.Post("/internal/billing-provider-connections/{id}/sync", s.syncBillingProviderConnection)
		r.Post("/internal/billing-provider-connections/{id}/disable", s.disableBillingProviderConnection)
		r.Post("/internal/billing-provider-connections/{id}/rotate-secret", s.rotateBillingProviderConnectionSecret)

		// Tenant audit
		r.Get("/internal/tenants/audit", s.handleInternalTenantAudit)

		// Tenants
		r.Get("/internal/tenants", s.listInternalTenants)
		r.Post("/internal/tenants", s.createInternalTenant)
		r.Get("/internal/tenants/{id}", s.getInternalTenant)
		r.Patch("/internal/tenants/{id}", s.updateInternalTenant)
		r.Post("/internal/tenants/{id}/bootstrap-admin-key", s.bootstrapInternalTenantAdminKey)
		r.Get("/internal/tenants/{id}/workspace-billing", s.getInternalTenantWorkspaceBilling)
		r.Patch("/internal/tenants/{id}/workspace-billing", s.updateInternalTenantWorkspaceBilling)
		r.Get("/internal/tenants/{id}/workspace-billing-settings", s.getInternalTenantWorkspaceBillingSettings)
		r.Patch("/internal/tenants/{id}/workspace-billing-settings", s.updateInternalTenantWorkspaceBillingSettings)
		r.Get("/internal/tenants/{id}/members", s.listInternalTenantMembers)
		r.Patch("/internal/tenants/{id}/members/{userId}", s.updateInternalTenantMember)
		r.Delete("/internal/tenants/{id}/members/{userId}", s.removeInternalTenantMember)
		r.Get("/internal/tenants/{id}/invitations", s.listInternalTenantInvitations)
		r.Post("/internal/tenants/{id}/invitations", s.createInternalTenantInvitation)
		r.Post("/internal/tenants/{id}/invitations/{invitationId}/revoke", s.revokeInternalTenantInvitation)
	})

	// ── Tenant admin routes ─────────────────────────────────────────────
	r.Group(func(r chi.Router) {
		r.Use(s.requireAuth(RoleAdmin, authScopeTenant))

		// Workspace settings
		r.Get("/v1/workspace/settings", s.getWorkspaceSettings)
		r.Patch("/v1/workspace/settings", s.updateWorkspaceSettings)

		// Workspace members
		r.Get("/v1/workspace/members", s.listWorkspaceMembers)
		r.Patch("/v1/workspace/members/{id}", s.updateWorkspaceMember)
		r.Delete("/v1/workspace/members/{id}", s.removeWorkspaceMember)

		// Workspace invitations
		r.Get("/v1/workspace/invitations", s.listWorkspaceInvitations)
		r.Post("/v1/workspace/invitations", s.createWorkspaceInvitation)
		r.Post("/v1/workspace/invitations/{id}/revoke", s.revokeWorkspaceInvitation)

		// Workspace service accounts
		r.Get("/v1/workspace/service-accounts", s.listServiceAccounts)
		r.Post("/v1/workspace/service-accounts", s.createServiceAccount)
		r.Patch("/v1/workspace/service-accounts/{id}", s.updateServiceAccountStatus)
		r.Get("/v1/workspace/service-accounts/{id}/audit", s.listServiceAccountAudit)
		r.Get("/v1/workspace/service-accounts/{id}/audit/exports", s.listServiceAccountAuditExports)
		r.Post("/v1/workspace/service-accounts/{id}/audit/exports", s.createServiceAccountAuditExport)
		r.Get("/v1/workspace/service-accounts/{id}/audit/exports/{exportId}", s.getServiceAccountAuditExport)
		r.Post("/v1/workspace/service-accounts/{id}/credentials", s.issueServiceAccountCredential)
		r.Post("/v1/workspace/service-accounts/{id}/credentials/{credentialId}/rotate", s.rotateServiceAccountCredential)
		r.Post("/v1/workspace/service-accounts/{id}/credentials/{credentialId}/revoke", s.revokeServiceAccountCredential)

		// API keys
		r.Get("/v1/api-keys", s.listAPIKeys)
		r.Post("/v1/api-keys", s.createAPIKey)
		r.Get("/v1/api-keys/audit", s.listAPIKeyAuditEvents)
		r.Get("/v1/api-keys/audit/exports", s.listAPIKeyAuditExports)
		r.Post("/v1/api-keys/audit/exports", s.createAPIKeyAuditExport)
		r.Get("/v1/api-keys/audit/exports/{id}", s.getAPIKeyAuditExport)
		r.Post("/v1/api-keys/{id}/revoke", s.revokeAPIKey)
		r.Post("/v1/api-keys/{id}/rotate", s.rotateAPIKey)
	})

	// ── Tenant writer routes ────────────────────────────────────────────
	r.Group(func(r chi.Router) {
		r.Use(s.requireAuth(RoleWriter, authScopeTenant))
		r.Post("/v1/customer-onboarding", s.handleCustomerOnboarding)
		r.Post("/v1/customers", s.createCustomer)
		r.Post("/v1/rating-rules", s.createRatingRule)
		r.Post("/v1/meters", s.createMeter)
		r.Put("/v1/meters/{id}", s.updateMeter)
		r.Post("/v1/pricing/metrics", s.createPricingMetric)
		r.Post("/v1/taxes", s.createTax)
		r.Post("/v1/plans", s.createPlan)
		r.Patch("/v1/plans/{id}", s.updatePlan)
		r.Post("/v1/plans/{id}/activate", s.activatePlan)
		r.Post("/v1/plans/{id}/archive", s.archivePlan)
		r.Post("/v1/subscriptions", s.createSubscription)
		r.Post("/v1/subscriptions/{id}/payment-setup/request", s.requestSubscriptionPaymentSetup)
		r.Post("/v1/subscriptions/{id}/payment-setup/resend", s.resendSubscriptionPaymentSetup)
		r.Patch("/v1/subscriptions/{id}", s.updateSubscription)
		r.Patch("/v1/customers/{externalId}", s.updateCustomer)
		r.Put("/v1/customers/{externalId}/billing-profile", s.upsertCustomerBillingProfile)
		r.Post("/v1/customers/{externalId}/billing-profile/retry-sync", s.retryCustomerBillingProfileSync)
		r.Post("/v1/customers/{externalId}/payment-setup/checkout-url", s.getCustomerCheckoutURL)
		r.Post("/v1/customers/{externalId}/payment-setup/request", s.requestCustomerPaymentSetup)
		r.Post("/v1/customers/{externalId}/payment-setup/resend", s.resendCustomerPaymentSetup)
		r.Post("/v1/customers/{externalId}/payment-setup/refresh", s.refreshCustomerPaymentSetup)
		r.Post("/v1/usage-events", s.createUsageEvent)
		r.Post("/v1/billed-entries", s.createBilledEntry)
		r.Post("/v1/replay-jobs", s.createReplayJob)
		r.Post("/v1/replay-jobs/{id}/retry", s.retryReplayJob)
		r.Post("/v1/invoices/{id}/retry-payment", s.retryInvoicePayment)
		r.Post("/v1/invoices/{id}/resend-email", s.resendInvoiceEmail)
		r.Post("/v1/payment-receipts/{id}/resend-email", s.resendPaymentReceiptEmail)
		r.Post("/v1/credit-notes/{id}/resend-email", s.resendCreditNoteEmail)
		r.Post("/v1/payments/{id}/retry", s.retryPayment)
		r.Put("/v1/dunning/policy", s.putDunningPolicy)
		r.Post("/v1/dunning/runs/{id}/collect-payment-reminder", s.collectPaymentReminder)
		r.Post("/v1/dunning/runs/{id}/retry-now", s.retryDunningPayment)
		r.Post("/v1/dunning/runs/{id}/pause", s.pauseDunningRun)
		r.Post("/v1/dunning/runs/{id}/resume", s.resumeDunningRun)
		r.Post("/v1/dunning/runs/{id}/resolve", s.resolveDunningRun)
		r.Post("/v1/add-ons", s.createAddOn)
		r.Post("/v1/coupons", s.createCoupon)
	})

	// ── Tenant reader routes ────────────────────────────────────────────
	r.Group(func(r chi.Router) {
		r.Use(s.requireAuth(RoleReader, authScopeTenant))
		r.Get("/v1/customers", s.listCustomers)
		r.Get("/v1/customers/{externalId}", s.getCustomer)
		r.Get("/v1/customers/{externalId}/billing-profile", s.getCustomerBillingProfile)
		r.Get("/v1/customers/{externalId}/payment-setup", s.getCustomerPaymentSetup)
		r.Get("/v1/customers/{externalId}/readiness", s.getCustomerReadiness)
		r.Get("/v1/rating-rules", s.listRatingRules)
		r.Get("/v1/rating-rules/{id}", s.getRatingRule)
		r.Get("/v1/meters", s.listMeters)
		r.Get("/v1/meters/{id}", s.getMeter)
		r.Get("/v1/pricing/metrics", s.listPricingMetrics)
		r.Get("/v1/pricing/metrics/{id}", s.getPricingMetric)
		r.Get("/v1/taxes", s.listTaxes)
		r.Get("/v1/taxes/{id}", s.getTax)
		r.Get("/v1/add-ons", s.listAddOns)
		r.Get("/v1/add-ons/{id}", s.getAddOn)
		r.Get("/v1/coupons", s.listCoupons)
		r.Get("/v1/coupons/{id}", s.getCoupon)
		r.Get("/v1/plans", s.listPlans)
		r.Get("/v1/plans/{id}", s.getPlan)
		r.Get("/v1/subscriptions", s.listSubscriptions)
		r.Get("/v1/subscriptions/{id}", s.getSubscription)
		r.Get("/v1/invoices", s.handleInvoices)
		r.Get("/v1/invoices/preview", s.handleInvoicePreview)
		r.Get("/v1/invoices/{id}/pdf", s.handleInvoicePDF)
		r.Get("/v1/invoices/{id}", s.getInvoice)
		r.Get("/v1/invoices/{id}/payment-receipts", s.getInvoicePaymentReceipts)
		r.Get("/v1/invoices/{id}/credit-notes", s.getInvoiceCreditNotes)
		r.Get("/v1/invoices/{id}/explainability", s.getInvoiceExplainability)
		r.Get("/v1/payment-receipts/{id}", s.getPaymentReceipt)
		r.Get("/v1/credit-notes/{id}", s.getCreditNote)
		r.Get("/v1/payments", s.listPayments)
		r.Get("/v1/payments/{id}", s.getPayment)
		r.Get("/v1/payments/{id}/events", s.listPaymentEvents)
		r.Get("/v1/usage-events", s.listUsageEvents)
		r.Get("/v1/billed-entries", s.listBilledEntries)
		r.Get("/v1/replay-jobs", s.listReplayJobs)
		r.Get("/v1/replay-jobs/{id}", s.getReplayJob)
		r.Get("/v1/replay-jobs/{id}/events", s.getReplayJobEvents)
		r.Get("/v1/replay-jobs/{id}/artifacts/{artifactName}", s.getReplayJobArtifact)
		r.Get("/v1/invoice-payment-statuses", s.handleInvoicePaymentStatuses)
		r.Get("/v1/invoice-payment-statuses/summary", s.getInvoicePaymentStatusSummary)
		r.Get("/v1/invoice-payment-statuses/{id}", s.getInvoicePaymentStatus)
		r.Get("/v1/invoice-payment-statuses/{id}/events", s.getInvoicePaymentStatusEvents)
		r.Get("/v1/invoice-payment-statuses/{id}/lifecycle", s.getInvoicePaymentStatusLifecycle)
		r.Get("/v1/dunning/policy", s.getDunningPolicy)
		r.Get("/v1/dunning/runs", s.listDunningRuns)
		r.Get("/v1/dunning/runs/{id}", s.getDunningRun)
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
