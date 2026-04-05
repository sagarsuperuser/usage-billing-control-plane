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
	status := classifyDomainErrorStatus(err)
	code := classifyDomainErrorCode(err)
	message := err.Error()

	// Never leak internal/SQL/infrastructure errors to the client.
	// Only domain errors (4xx) get their original message.
	if status >= 500 {
		message = "an internal error occurred"
	}

	writeErrorCode(w, status, message, code)
}

// writeInternalError logs the full error server-side but returns a safe
// generic message to the client. Use this for infrastructure errors
// (DB, Stripe, S3) that should never be exposed.
func (s *Server) writeInternalError(w http.ResponseWriter, r *http.Request, status int, userMessage string, err error) {
	if s.logger != nil {
		s.logger.Error("internal error",
			"component", "api",
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"error", err.Error(),
			"request_id", w.Header().Get(requestIDHeaderKey),
		)
	}
	writeError(w, status, userMessage)
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

// ---------------------------------------------------------------------------
// Upstream (Stripe/billing provider) error translation
// ---------------------------------------------------------------------------

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
		if status == http.StatusNotFound {
			return "The requested resource could not be found.", "not_found", true
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
		return "The requested resource could not be found.", "not_found", true
	}

	return "", "", false
}

// ---------------------------------------------------------------------------
// Domain error → user-visible message translation (code-based, not string-based)
// ---------------------------------------------------------------------------

// translateUserVisibleError converts internal error messages into
// user-friendly text. It first checks for a DomainError code, then
// falls back to the raw message (no string matching).
func translateUserVisibleError(status int, code, message string) (string, string) {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return trimmed, code
	}

	// Extract DomainError code from the message prefix if present.
	// DomainError.Error() formats as "message: cause", but we match on
	// the code that was set when the error was created.
	// The code is passed through classifyDomainErrorCode → writeErrorCode.
	// Here we use the HTTP-level code parameter which may already be set.
	if translated, ok := translateByErrorCode(code); ok {
		return translated, code
	}

	return trimmed, code
}

// translateByErrorCode maps machine-readable DomainError codes to
// user-visible messages. This replaces the old string-matching approach.
func translateByErrorCode(code string) (string, bool) {
	switch code {
	// Billing setup
	case service.ErrCodeBillingSetupIncomplete:
		return "Billing setup is incomplete for this workspace or connection.", true
	case service.ErrCodeBillingCheckRequired:
		return "Check this Stripe connection before assigning it to a workspace.", true
	case service.ErrCodeBillingUnavailable:
		return "Billing activity is unavailable right now.", true

	// Pricing
	case service.ErrCodeSyncAdapterRequired:
		return "Pricing updates are unavailable right now.", true
	case service.ErrCodeMeterSyncFailed:
		return "Pricing metric changes could not be applied right now.", true
	case service.ErrCodeTaxSyncFailed:
		return "Tax changes could not be applied right now.", true
	case service.ErrCodePlanSyncFailed:
		return "Pricing changes could not be applied right now.", true
	case service.ErrCodePricingUnsupported:
		return "This pricing configuration is not supported yet.", true

	// Subscriptions
	case service.ErrCodeSubscriptionSyncFailed:
		return "Subscription changes could not be applied right now.", true

	// Usage
	case service.ErrCodeUsageUnsupported:
		return "This usage configuration is not supported yet.", true
	case service.ErrCodeUsageSyncFailed:
		return "Usage could not be recorded right now.", true

	// Invoice/Payment
	case service.ErrCodeInvoiceFetchFailed:
		return "Invoice details could not be loaded right now.", true
	case service.ErrCodeExplainabilityFailed:
		return "Invoice explainability is unavailable right now.", true
	case service.ErrCodePaymentRetryFailed:
		return "Payment retry could not be started right now.", true
	case service.ErrCodePaymentReceiptsLoadFailed:
		return "Payment receipts could not be loaded right now.", true
	case service.ErrCodeCreditNotesLoadFailed:
		return "Credit notes could not be loaded right now.", true

	// Customer/Stripe
	case service.ErrCodeStripeCredentialsRequired:
		return "Stripe credentials are required.", true
	case service.ErrCodeStripeVerificationFailed:
		return "Stripe could not be reached right now.", true
	case service.ErrCodePaymentProviderRequired:
		return "A billing provider must be configured before payment setup can begin.", true
	case service.ErrCodePaymentProviderUnsupported:
		return "The configured billing provider is not supported.", true

	// Adapter
	case service.ErrCodeAdapterNotConfigured:
		return "Billing actions are unavailable right now.", true

	default:
		return "", false
	}
}

// ---------------------------------------------------------------------------
// Domain error → HTTP status + code classification
// ---------------------------------------------------------------------------

// classifyDomainErrorStatus maps domain/store errors to HTTP status codes.
// Uses sentinel errors only — no string matching.
func classifyDomainErrorStatus(err error) int {
	switch {
	case err == nil:
		return http.StatusInternalServerError
	case errors.Is(err, store.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, store.ErrAlreadyExists), errors.Is(err, store.ErrDuplicateKey):
		return http.StatusConflict
	case errors.Is(err, store.ErrInvalidState):
		return http.StatusBadRequest
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
	case errors.Is(err, service.ErrInvalidBrowserCredentials):
		return http.StatusUnauthorized
	case errors.Is(err, service.ErrBrowserUserDisabled):
		return http.StatusForbidden
	case errors.Is(err, service.ErrPasswordResetTokenExpired), errors.Is(err, service.ErrPasswordResetTokenUsed):
		return http.StatusGone
	case errors.Is(err, service.ErrWorkspaceInvitationExpired), errors.Is(err, service.ErrWorkspaceInvitationRevoked):
		return http.StatusGone
	case errors.Is(err, service.ErrWorkspaceInvitationAccepted):
		return http.StatusConflict
	case errors.Is(err, service.ErrWorkspaceInvitationEmailMismatch), errors.Is(err, service.ErrBrowserSSOInviteEmailMismatch):
		return http.StatusForbidden
	default:
		// No string matching fallback. Unrecognized errors are 500.
		return http.StatusInternalServerError
	}
}

// classifyDomainErrorCode returns a machine-readable error code.
// If the error carries a DomainError code, use that. Otherwise
// derive from the sentinel error type.
func classifyDomainErrorCode(err error) string {
	// Check for DomainError code first (structured errors).
	if code := service.DomainErrorCode(err); code != "" {
		return code
	}

	// Fall back to sentinel-based classification.
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

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

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
