package api

import (
	"fmt"
	"net/http"
	"testing"

	"usage-billing-control-plane/internal/service"
)

func TestTranslateUserVisibleError(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		code        string
		message     string
		wantMessage string
	}{
		{
			name:        "billing setup incomplete",
			status:      400,
			code:        service.ErrCodeBillingSetupIncomplete,
			message:     "validation error: billing setup incomplete",
			wantMessage: "Billing setup is incomplete for this workspace or connection.",
		},
		{
			name:        "payment retry failed",
			status:      502,
			code:        service.ErrCodePaymentRetryFailed,
			message:     "payment retry failed: boom",
			wantMessage: "Payment retry could not be started right now.",
		},
		{
			name:        "billing unavailable",
			status:      503,
			code:        service.ErrCodeBillingUnavailable,
			message:     "payment status service is required",
			wantMessage: "Billing activity is unavailable right now.",
		},
		{
			name:        "stripe credentials required",
			status:      400,
			code:        service.ErrCodeStripeCredentialsRequired,
			message:     "validation error: stripe_secret_key is required",
			wantMessage: "Stripe credentials are required.",
		},
		{
			name:        "unsupported provider code",
			status:      400,
			code:        service.ErrCodePaymentProviderUnsupported,
			message:     "validation error: unsupported payment provider code",
			wantMessage: "The configured billing provider is not supported.",
		},
		{
			name:        "pricing sync failure",
			status:      502,
			code:        service.ErrCodePlanSyncFailed,
			message:     "plan sync failed",
			wantMessage: "Pricing changes could not be applied right now.",
		},
		{
			name:        "subscription sync failure",
			status:      502,
			code:        service.ErrCodeSubscriptionSyncFailed,
			message:     "subscription sync failed",
			wantMessage: "Subscription changes could not be applied right now.",
		},
		{
			name:        "unsupported pricing configuration",
			status:      502,
			code:        service.ErrCodePricingUnsupported,
			message:     "pricing mode not supported",
			wantMessage: "This pricing configuration is not supported yet.",
		},
		{
			name:        "usage unsupported aggregation",
			status:      502,
			code:        service.ErrCodeUsageUnsupported,
			message:     "unsupported aggregation for usage sync",
			wantMessage: "This usage configuration is not supported yet.",
		},
		{
			name:        "unknown code passes through raw message",
			status:      500,
			code:        "internal_error",
			message:     "database connection pool exhausted",
			wantMessage: "database connection pool exhausted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := translateUserVisibleError(tt.status, tt.code, tt.message)
			if got != tt.wantMessage {
				t.Fatalf("translateUserVisibleError() = %q, want %q", got, tt.wantMessage)
			}
		})
	}
}

func TestTranslateUpstreamUserVisibleError(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		fallback    string
		body        string
		wantMessage string
		wantCode    string
	}{
		{
			name:        "stripe auth failure",
			status:      http.StatusUnauthorized,
			fallback:    "billing check failed",
			body:        `{"error":"Unauthorized","code":"provider_error","provider":{"code":"stripe"},"error_details":{"message":"Invalid API key","http_status":401}}`,
			wantMessage: "Stripe credentials could not be verified.",
			wantCode:    "stripe_authentication_failed",
		},
		{
			name:        "stripe rate limit",
			status:      http.StatusTooManyRequests,
			fallback:    "too many requests",
			body:        `{"error":"too many requests","code":"too_many_provider_requests","provider":{"code":"stripe"}}`,
			wantMessage: "Stripe is rate limiting requests right now.",
			wantCode:    "stripe_rate_limited",
		},
		{
			name:        "generic 502 with no body",
			status:      http.StatusBadGateway,
			fallback:    "upstream failed",
			body:        "",
			wantMessage: "upstream failed",
			wantCode:    "bad_gateway",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMessage, gotCode := translateUpstreamUserVisibleError(tt.status, tt.fallback, []byte(tt.body))
			if gotMessage != tt.wantMessage {
				t.Fatalf("translateUpstreamUserVisibleError() message = %q, want %q", gotMessage, tt.wantMessage)
			}
			if gotCode != tt.wantCode {
				t.Fatalf("translateUpstreamUserVisibleError() code = %q, want %q", gotCode, tt.wantCode)
			}
		})
	}
}

func TestClassifyDomainErrorStatus_NoStringMatchFallback(t *testing.T) {
	// Errors without sentinel wrapping should return 500, not be classified
	// by string matching on their message content.
	tests := []struct {
		name       string
		message    string
		wantStatus int
	}{
		{
			name:       "message containing 'required' is NOT auto-classified as 400",
			message:    "database connection required",
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "message containing 'not found' is NOT auto-classified as 404",
			message:    "config file not found on disk",
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "message containing 'invalid' is NOT auto-classified as 400",
			message:    "invalid TLS certificate in pool",
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := fmt.Errorf("%s", tt.message)
			got := classifyDomainErrorStatus(err)
			if got != tt.wantStatus {
				t.Fatalf("classifyDomainErrorStatus(%q) = %d, want %d", tt.message, got, tt.wantStatus)
			}
		})
	}
}
