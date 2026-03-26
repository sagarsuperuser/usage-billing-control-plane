package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPStripeConnectionVerifierSuccess(t *testing.T) {
	t.Parallel()

	stripe := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/account" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk_test_ok" {
			t.Fatalf("unexpected authorization header %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"acct_123","livemode":false}`))
	}))
	defer stripe.Close()

	verifier, err := NewHTTPStripeConnectionVerifier(stripe.URL, 2*time.Second)
	if err != nil {
		t.Fatalf("new verifier: %v", err)
	}

	result, err := verifier.VerifyStripeSecret(context.Background(), "sk_test_ok")
	if err != nil {
		t.Fatalf("verify stripe secret: %v", err)
	}
	if result.AccountID != "acct_123" {
		t.Fatalf("expected account id acct_123, got %q", result.AccountID)
	}
	if result.Livemode {
		t.Fatalf("expected livemode false")
	}
	if result.VerifiedAt.IsZero() {
		t.Fatalf("expected verified_at to be set")
	}
}

func TestHTTPStripeConnectionVerifierFailure(t *testing.T) {
	t.Parallel()

	stripe := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"Invalid API Key provided"}}`))
	}))
	defer stripe.Close()

	verifier, err := NewHTTPStripeConnectionVerifier(stripe.URL, 2*time.Second)
	if err != nil {
		t.Fatalf("new verifier: %v", err)
	}

	_, err = verifier.VerifyStripeSecret(context.Background(), "sk_test_bad")
	if err == nil {
		t.Fatalf("expected verification error")
	}
	if got := err.Error(); got != "dependency error: Invalid API Key provided" {
		t.Fatalf("unexpected error %q", got)
	}
}
