package api

import (
	"net/http"
	"testing"
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
			code:        "validation_error",
			message:     "validation error: lago organization id is required",
			wantMessage: "Billing setup is incomplete for this workspace or connection.",
		},
		{
			name:        "payment retry proxy",
			status:      502,
			code:        "bad_gateway",
			message:     "failed to proxy payment retry to lago: boom",
			wantMessage: "Payment retry could not be started right now.",
		},
		{
			name:        "billing activity unavailable",
			status:      503,
			code:        "service_unavailable",
			message:     "lago webhook service is required",
			wantMessage: "Billing activity is unavailable right now.",
		},
		{
			name:        "stripe credentials required",
			status:      400,
			code:        "validation_error",
			message:     "validation error: stripe_secret_key is required",
			wantMessage: "Stripe credentials are required.",
		},
		{
			name:        "unsupported provider code",
			status:      400,
			code:        "validation_error",
			message:     "validation error: unsupported payment provider code \"foo\"",
			wantMessage: "The configured billing provider is not supported.",
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
			name:     "stripe auth failure from provider error",
			status:   http.StatusUnprocessableEntity,
			fallback: "Connection could not be refreshed right now.",
			body: `{
				"status":422,
				"error":"Unprocessable Entity",
				"code":"provider_error",
				"provider":{"code":"stripe"},
				"error_details":{"message":"Invalid API Key provided","http_status":401}
			}`,
			wantMessage: "Stripe credentials could not be verified.",
			wantCode:    "stripe_authentication_failed",
		},
		{
			name:     "provider throttling",
			status:   http.StatusTooManyRequests,
			fallback: "Connection could not be refreshed right now.",
			body: `{
				"status":429,
				"error":"Too Many Provider Requests",
				"code":"too_many_provider_requests",
				"error_details":{"provider_name":"stripe","message":"too many requests"}
			}`,
			wantMessage: "Stripe is rate limiting requests right now.",
			wantCode:    "stripe_rate_limited",
		},
		{
			name:        "invoice not found",
			status:      http.StatusNotFound,
			fallback:    "Invoice details could not be loaded right now.",
			body:        `{"status":404,"error":"Not Found","code":"invoice_not_found"}`,
			wantMessage: "Invoice could not be found.",
			wantCode:    "not_found",
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
