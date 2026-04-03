package api

import (
	"net/http"
	"time"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
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
