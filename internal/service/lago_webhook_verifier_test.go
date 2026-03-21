package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"testing"
)

func TestLagoHMACWebhookVerifierAcceptsValidSignature(t *testing.T) {
	t.Parallel()

	provider, err := NewStaticLagoWebhookHMACKeyProvider("hmac_test_key")
	if err != nil {
		t.Fatalf("new static provider: %v", err)
	}
	verifier, err := NewLagoHMACWebhookVerifier(provider)
	if err != nil {
		t.Fatalf("new verifier: %v", err)
	}

	body := []byte(`{"webhook_type":"invoice.payment_status_updated","object_type":"invoice","organization_id":"org_test","invoice":{"lago_id":"inv_123"}}`)
	mac := hmac.New(sha256.New, []byte("hmac_test_key"))
	_, _ = mac.Write(body)
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	headers := http.Header{}
	headers.Set("X-Lago-Signature-Algorithm", "hmac")
	headers.Set("X-Lago-Signature", signature)

	if err := verifier.Verify(context.Background(), headers, body); err != nil {
		t.Fatalf("verify valid hmac webhook: %v", err)
	}
}

func TestLagoHMACWebhookVerifierRejectsInvalidSignature(t *testing.T) {
	t.Parallel()

	provider, err := NewStaticLagoWebhookHMACKeyProvider("hmac_test_key")
	if err != nil {
		t.Fatalf("new static provider: %v", err)
	}
	verifier, err := NewLagoHMACWebhookVerifier(provider)
	if err != nil {
		t.Fatalf("new verifier: %v", err)
	}

	body := []byte(`{"webhook_type":"invoice.payment_status_updated","object_type":"invoice","organization_id":"org_test","invoice":{"lago_id":"inv_123"}}`)
	headers := http.Header{}
	headers.Set("X-Lago-Signature-Algorithm", "hmac")
	headers.Set("X-Lago-Signature", base64.StdEncoding.EncodeToString([]byte("bad")))

	if err := verifier.Verify(context.Background(), headers, body); err == nil {
		t.Fatalf("expected invalid hmac signature error")
	}
}
