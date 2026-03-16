package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
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
	repo                      store.Repository
	ratingService             *service.RatingService
	tenantService             *service.TenantService
	customerService           *service.CustomerService
	customerOnboardingService *service.CustomerOnboardingService
	meterService              *service.MeterService
	usageService              *service.UsageService
	apiKeyService             *service.APIKeyService
	onboardingService         *service.TenantOnboardingService
	auditExportSvc            *service.AuditExportService
	meterSyncAdapter          service.MeterSyncAdapter
	invoiceBillingAdapter     service.InvoiceBillingAdapter
	customerBillingAdapter    service.CustomerBillingAdapter
	lagoWebhookSvc            *service.LagoWebhookService
	replayService             *replay.Service
	recService                *reconcile.Service
	authorizer                APIKeyAuthorizer
	sessionManager            *scs.SessionManager
	metricsFn                 func() map[string]any
	readinessFn               func() error
	requestMetrics            *requestMetricsCollector
	logger                    *slog.Logger
	rateLimiter               RateLimiter
	rateLimitFailOpen         bool
	rateLimitLoginFailOpen    bool
	requireSessionOriginCheck bool
	allowedSessionOrigins     map[string]struct{}
	mux                       *http.ServeMux
}

const (
	sessionRoleKey     = "principal_role"
	sessionTenantIDKey = "principal_tenant_id"
	sessionAPIKeyIDKey = "principal_api_key_id"
	sessionCSRFKey     = "csrf_token"
	csrfHeaderName     = "X-CSRF-Token"
	requestIDHeaderKey = "X-Request-ID"
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
	s.tenantService = service.NewTenantService(repo)
	s.customerService = service.NewCustomerService(repo, s.customerBillingAdapter)
	s.customerOnboardingService = service.NewCustomerOnboardingService(s.customerService)
	s.meterService = service.NewMeterService(repo)
	s.usageService = service.NewUsageService(repo)
	s.apiKeyService = service.NewAPIKeyService(repo)
	s.onboardingService = service.NewTenantOnboardingService(s.tenantService, s.customerService, s.apiKeyService, s.ratingService, s.meterService)
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

	roleRaw := strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionRoleKey))
	tenantID := strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionTenantIDKey))
	apiKeyID := strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionAPIKeyIDKey))
	if roleRaw == "" || tenantID == "" {
		return Principal{}, true, errUnauthorized
	}
	role, err := ParseRole(roleRaw)
	if err != nil {
		return Principal{}, true, errUnauthorized
	}
	return Principal{
		Scope:    ScopeTenant,
		Role:     role,
		TenantID: normalizeTenantID(tenantID),
		APIKeyID: apiKeyID,
	}, true, nil
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
	if path == "/v1/ui/sessions/me" {
		return RoleReader, true
	}
	if path == "/v1/ui/sessions/logout" {
		return RoleReader, true
	}
	if path == "/v1/customer-onboarding" {
		return RoleWriter, true
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
	case path == "/v1/invoices/preview":
		return RoleReader, true
	case strings.HasPrefix(path, "/v1/invoices/"):
		if r.Method == http.MethodPost && strings.HasSuffix(strings.Trim(path, "/"), "/retry-payment") {
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
	case path == "/internal/tenants":
		return "/internal/tenants"
	case strings.HasPrefix(path, "/internal/tenants/"):
		return "/internal/tenants/{id}"
	case path == "/v1/ui/sessions/login":
		return "/v1/ui/sessions/login"
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
	case path == "/v1/invoices/preview":
		return "/v1/invoices/preview"
	case strings.HasPrefix(path, "/v1/invoices/"):
		tail := strings.Trim(strings.TrimPrefix(path, "/v1/invoices/"), "/")
		if strings.HasSuffix(tail, "/retry-payment") {
			return "/v1/invoices/{id}/retry-payment"
		}
		if strings.HasSuffix(tail, "/explainability") {
			return "/v1/invoices/{id}/explainability"
		}
		return "/v1/invoices/{id}"
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
	s.mux.HandleFunc("/internal/tenants/audit", s.handleInternalTenantAudit)
	s.mux.HandleFunc("/internal/tenants", s.handleInternalTenants)
	s.mux.HandleFunc("/internal/tenants/", s.handleInternalTenantByID)
	s.mux.HandleFunc("/v1/ui/sessions/login", s.handleUISessionLogin)
	s.mux.HandleFunc("/v1/ui/sessions/me", s.handleUISessionMe)
	s.mux.HandleFunc("/v1/ui/sessions/logout", s.handleUISessionLogout)
	s.mux.HandleFunc("/v1/customer-onboarding", s.handleCustomerOnboarding)

	s.mux.HandleFunc("/v1/customers", s.handleCustomers)
	s.mux.HandleFunc("/v1/customers/", s.handleCustomerByExternalID)

	s.mux.HandleFunc("/v1/rating-rules", s.handleRatingRules)
	s.mux.HandleFunc("/v1/rating-rules/", s.handleRatingRuleByID)

	s.mux.HandleFunc("/v1/meters", s.handleMeters)
	s.mux.HandleFunc("/v1/meters/", s.handleMeterByID)

	s.mux.HandleFunc("/v1/invoices/preview", s.handleInvoicePreview)
	s.mux.HandleFunc("/v1/invoices/", s.handleInvoiceByID)

	s.mux.HandleFunc("/v1/usage-events", s.handleUsageEvents)
	s.mux.HandleFunc("/v1/billed-entries", s.handleBilledEntries)

	s.mux.HandleFunc("/v1/replay-jobs", s.handleReplayJobs)
	s.mux.HandleFunc("/v1/replay-jobs/", s.handleReplayJobByID)
	s.mux.HandleFunc("/v1/invoice-payment-statuses", s.handleInvoicePaymentStatuses)
	s.mux.HandleFunc("/v1/invoice-payment-statuses/", s.handleInvoicePaymentStatusByID)

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
			"tenant":  tenant,
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
		writeJSON(w, http.StatusOK, tenants)
	default:
		writeMethodNotAllowed(w)
	}
}

func (s *Server) handleInternalTenantByID(w http.ResponseWriter, r *http.Request) {
	if !s.isOperatorRequest(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	id, action := parseInternalTenantPath(r.URL.Path)
	if id == "" {
		writeError(w, http.StatusBadRequest, "tenant id is required")
		return
	}
	if action != "" && action != "bootstrap-admin-key" {
		writeError(w, http.StatusBadRequest, "unsupported tenant subresource")
		return
	}
	if action == "bootstrap-admin-key" {
		s.handleInternalTenantBootstrapAdminKey(w, r, id)
		return
	}

	switch r.Method {
	case http.MethodGet:
		tenant, err := s.tenantService.GetTenant(id)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, tenant)
	case http.MethodPatch:
		var req struct {
			Name                    *string              `json:"name,omitempty"`
			Status                  *domain.TenantStatus `json:"status,omitempty"`
			LagoOrganizationID      *string              `json:"lago_organization_id,omitempty"`
			LagoBillingProviderCode *string              `json:"lago_billing_provider_code,omitempty"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		tenant, err := s.tenantService.UpdateTenant(id, service.UpdateTenantRequest{
			Name:                    req.Name,
			Status:                  req.Status,
			LagoOrganizationID:      req.LagoOrganizationID,
			LagoBillingProviderCode: req.LagoBillingProviderCode,
		}, requestActorAPIKeyID(r))
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, tenant)
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

	created, err := s.apiKeyService.CreateAPIKey(tenantID, requestActorAPIKeyID(r), service.CreateAPIKeyRequest{
		Name:      req.Name,
		Role:      string(RoleAdmin),
		ExpiresAt: req.ExpiresAt,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"api_key":              created.APIKey,
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
	writeJSON(w, http.StatusOK, events)
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
		writeDomainError(w, err)
		return
	}
	status := http.StatusOK
	if result.TenantCreated {
		status = http.StatusCreated
	}
	writeJSON(w, status, result)
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
		"tenant":    tenant,
		"readiness": readiness,
		"tenant_id": normalizeTenantID(id),
	})
}

type uiSessionLoginRequest struct {
	APIKey string `json:"api_key"`
}

func (s *Server) handleUISessionLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if s.rateLimiter != nil {
		if !s.enforceRateLimit(w, r, RateLimitPolicyPreAuthLogin, "ip:"+requestClientIP(r), "", s.rateLimitLoginFailOpen) {
			return
		}
	}
	if s.sessionManager == nil || s.authorizer == nil {
		writeError(w, http.StatusServiceUnavailable, "ui sessions are not configured")
		return
	}

	var req uiSessionLoginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.APIKey = strings.TrimSpace(req.APIKey)
	if req.APIKey == "" {
		writeError(w, http.StatusBadRequest, "api_key is required")
		return
	}

	authReq := r.Clone(r.Context())
	authReq.Header = r.Header.Clone()
	authReq.Header.Set(apiKeyHeader, req.APIKey)

	principal, err := s.authorizer.Authorize(authReq)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	if principal.Scope != ScopeTenant {
		writeError(w, http.StatusForbidden, "ui sessions require tenant api keys")
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

	s.sessionManager.Put(r.Context(), sessionRoleKey, string(principal.Role))
	s.sessionManager.Put(r.Context(), sessionTenantIDKey, normalizeTenantID(principal.TenantID))
	s.sessionManager.Put(r.Context(), sessionAPIKeyIDKey, strings.TrimSpace(principal.APIKeyID))
	s.sessionManager.Put(r.Context(), sessionCSRFKey, csrfToken)

	expiresAt := time.Now().UTC().Add(s.sessionManager.Lifetime)
	writeJSON(w, http.StatusCreated, map[string]any{
		"authenticated": true,
		"role":          principal.Role,
		"tenant_id":     normalizeTenantID(principal.TenantID),
		"api_key_id":    strings.TrimSpace(principal.APIKeyID),
		"csrf_token":    csrfToken,
		"expires_at":    expiresAt,
	})
}

func parseInternalTenantPath(path string) (tenantID string, action string) {
	tail := strings.Trim(strings.TrimPrefix(path, "/internal/tenants/"), "/")
	if tail == "" {
		return "", ""
	}
	parts := strings.Split(tail, "/")
	tenantID = strings.TrimSpace(parts[0])
	if len(parts) > 1 {
		action = strings.TrimSpace(parts[1])
	}
	return tenantID, action
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
	if !ok || principal.Role == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	csrfToken := strings.TrimSpace(s.sessionManager.GetString(r.Context(), sessionCSRFKey))
	if csrfToken == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"role":          principal.Role,
		"tenant_id":     normalizeTenantID(principal.TenantID),
		"api_key_id":    strings.TrimSpace(principal.APIKeyID),
		"csrf_token":    csrfToken,
	})
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
		lifecycle, err := s.lagoWebhookSvc.GetInvoicePaymentLifecycle(requestTenantID(r), invoiceID, eventLimit)
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
		writeError(w, http.StatusServiceUnavailable, "meter sync adapter is required")
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
			writeError(w, http.StatusBadGateway, "meter created but lago sync failed: "+err.Error())
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
		writeError(w, http.StatusServiceUnavailable, "meter sync adapter is required")
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
			writeError(w, http.StatusBadGateway, "meter updated but lago sync failed: "+err.Error())
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
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if s.invoiceBillingAdapter == nil {
		writeError(w, http.StatusServiceUnavailable, "invoice billing adapter is required")
		return
	}

	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	statusCode, body, err := s.invoiceBillingAdapter.PreviewInvoice(r.Context(), rawBody)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to proxy invoice preview to lago: "+err.Error())
		return
	}
	writeJSONRaw(w, statusCode, body)
}

func (s *Server) handleInvoiceByID(w http.ResponseWriter, r *http.Request) {
	if s.invoiceBillingAdapter == nil {
		writeError(w, http.StatusServiceUnavailable, "invoice billing adapter is required")
		return
	}

	tail := strings.TrimPrefix(r.URL.Path, "/v1/invoices/")
	parts := strings.Split(strings.Trim(tail, "/"), "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		writeError(w, http.StatusBadRequest, "invoice id is required")
		return
	}

	invoiceID := strings.TrimSpace(parts[0])
	if len(parts) == 2 && strings.EqualFold(strings.TrimSpace(parts[1]), "retry-payment") {
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

		statusCode, body, err := s.invoiceBillingAdapter.RetryInvoicePayment(r.Context(), invoiceID, rawBody)
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to proxy retry payment to lago: "+err.Error())
			return
		}
		writeJSONRaw(w, statusCode, body)
		return
	}
	if len(parts) == 2 && strings.EqualFold(strings.TrimSpace(parts[1]), "explainability") {
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

		statusCode, body, err := s.invoiceBillingAdapter.GetInvoice(r.Context(), invoiceID)
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to fetch invoice from lago: "+err.Error())
			return
		}
		if statusCode < 200 || statusCode >= 300 {
			writeJSONRaw(w, statusCode, body)
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

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeMethodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func writeDomainError(w http.ResponseWriter, err error) {
	if err == nil {
		writeError(w, http.StatusInternalServerError, "unknown error")
		return
	}
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if strings.Contains(strings.ToLower(err.Error()), "not found") {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if errors.Is(err, store.ErrDuplicateKey) {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if strings.Contains(strings.ToLower(err.Error()), "validation") || strings.Contains(strings.ToLower(err.Error()), "required") || strings.Contains(strings.ToLower(err.Error()), "invalid") {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error())
}
