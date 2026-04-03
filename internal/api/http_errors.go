package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"usage-billing-control-plane/internal/service"
	"usage-billing-control-plane/internal/store"
)

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
	case strings.Contains(lower, "billing setup incomplete"),
		strings.Contains(lower, "workspace has no billing execution context"),
		strings.Contains(lower, "workspace billing binding exists but is not ready"),
		strings.Contains(lower, "billing setup is incomplete for this workspace"):
		return "Billing setup is incomplete for this workspace or connection.", code
	case strings.Contains(lower, "billing provider connection must be checked before workspace assignment"):
		return "Check this Stripe connection before assigning it to a workspace.", code
	case strings.Contains(lower, "meter sync adapter is required"),
		strings.Contains(lower, "meter sync adapter is required"),
		strings.Contains(lower, "tax sync adapter is required"),
		strings.Contains(lower, "plan sync adapter is required"),
		strings.Contains(lower, "subscription sync adapter is required"),
		strings.Contains(lower, "usage sync adapter is required"):
		return "Pricing updates are unavailable right now.", code
	case strings.Contains(lower, "metric sync failed"),
		strings.Contains(lower, "meter sync failed"),
		strings.Contains(lower, "meter sync failed"),
		strings.Contains(lower, "meter sync failed"):
		return "Pricing metric changes could not be applied right now.", code
	case strings.Contains(lower, "tax sync failed"):
		return "Tax changes could not be applied right now.", code
	case strings.Contains(lower, "plan sync failed"),
		strings.Contains(lower, "add-on sync failed"),
		strings.Contains(lower, "coupon sync failed"),
		strings.Contains(lower, "fixed charge sync failed"),
		strings.Contains(lower, "list fixed charges failed"),
		strings.Contains(lower, "decode fixed charges response"),
		strings.Contains(lower, "billable metric lookup failed"),
		strings.Contains(lower, "decode billable metric response"):
		return "Pricing changes could not be applied right now.", code
	case strings.Contains(lower, "package pricing with overage is not yet supported"),
		(strings.Contains(lower, "pricing mode") && strings.Contains(lower, "not supported")):
		return "This pricing configuration is not supported yet.", code
	case strings.Contains(lower, "subscription sync failed"),
		strings.Contains(lower, "subscription terminate failed"),
		strings.Contains(lower, "list applied coupons failed"),
		strings.Contains(lower, "decode applied coupons response"),
		strings.Contains(lower, "apply coupon failed"),
		strings.Contains(lower, "delete applied coupon failed"):
		return "Subscription changes could not be applied right now.", code
	case strings.Contains(lower, "count aggregation requires quantity=1 for usage sync"),
		(strings.Contains(lower, "unsupported aggregation") && strings.Contains(lower, "for usage sync")),
		(strings.Contains(lower, "unsupported aggregation") && strings.Contains(lower, "for sync")):
		return "This usage configuration is not supported yet.", code
	case strings.Contains(lower, "usage sync failed"):
		return "Usage could not be recorded right now.", code
	case strings.Contains(lower, "failed to load payment receipts from billing provider"):
		return "Payment receipts could not be loaded right now.", code
	case strings.Contains(lower, "failed to load credit notes from billing provider"):
		return "Credit notes could not be loaded right now.", code
	case strings.Contains(lower, "payment retry failed"):
		return "Payment retry could not be started right now.", code
	case strings.Contains(lower, "failed to fetch invoice"):
		return "Invoice details could not be loaded right now.", code
	case strings.Contains(lower, "failed to compute invoice explainability"):
		return "Invoice explainability is unavailable right now.", code
	case strings.Contains(lower, "payment status service is required"):
		return "Billing activity is unavailable right now.", code
	case strings.Contains(lower, "invoice billing adapter is required"),
		strings.Contains(lower, "invoice billing adapter is required"):
		return "Billing actions are unavailable right now.", code
	case strings.Contains(lower, "customer billing adapter is required"):
		return "Customer billing is unavailable right now.", code
	case strings.Contains(lower, "stripe_secret_key is required"),
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
