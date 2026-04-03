package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

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

func (s *Server) isOperatorRequest(r *http.Request) bool {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		return false
	}
	return principal.Scope == ScopePlatform && principal.PlatformRole == PlatformRoleAdmin
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

func writeMethodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
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

func formatOptionalTime(v *time.Time) string {
	if v == nil {
		return ""
	}
	return v.UTC().Format(time.RFC3339Nano)
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

func metricsTenantKey(principal Principal) string {
	if principal.Scope == ScopePlatform {
		return "platform"
	}
	return normalizeTenantID(principal.TenantID)
}
