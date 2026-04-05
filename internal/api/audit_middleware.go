package api

import (
	"net/http"
	"time"
)

// auditLogMiddleware records all mutating API operations (POST, PUT, PATCH, DELETE)
// with the actor, target resource, and outcome. Used for compliance and debugging.
func (s *Server) auditLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only audit mutations — skip reads
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		recorder := &statusCapturingResponseWriter{ResponseWriter: w}
		next.ServeHTTP(recorder, r)

		statusCode := recorder.statusCode
		if statusCode == 0 {
			statusCode = http.StatusOK
		}

		// Extract actor from context
		principal, hasPrincipal := principalFromContext(r.Context())
		actorID := ""
		actorEmail := ""
		tenantID := ""
		if hasPrincipal {
			actorID = principal.SubjectID
			actorEmail = principal.UserEmail
			tenantID = string(principal.TenantID)
			if principal.APIKeyID != "" {
				actorID = "apikey:" + principal.APIKeyID
			}
		}

		route := routePattern(r)

		s.logInfo("audit",
			"component", "audit",
			"method", r.Method,
			"route", route,
			"path", r.URL.Path,
			"status", statusCode,
			"actor_id", actorID,
			"actor_email", actorEmail,
			"tenant_id", tenantID,
			"duration_ms", time.Since(start).Milliseconds(),
			"ip", requestClientIP(r),
			"user_agent", truncate(r.UserAgent(), 200),
		)
	})
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
