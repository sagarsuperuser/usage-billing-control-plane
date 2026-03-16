package service

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const defaultLagoWebhookPublicKeyTTL = 5 * time.Minute

type LagoWebhookVerifier interface {
	Verify(ctx context.Context, headers http.Header, body []byte) error
}

type NoopLagoWebhookVerifier struct{}

func (NoopLagoWebhookVerifier) Verify(context.Context, http.Header, []byte) error { return nil }

type LagoJWTWebhookVerifier struct {
	keyProvider WebhookPublicKeyProvider
	keyTTL      time.Duration

	mu        sync.RWMutex
	cachedKey *rsa.PublicKey
	cachedAt  time.Time
}

func NewLagoJWTWebhookVerifier(keyProvider WebhookPublicKeyProvider, keyTTL time.Duration) (*LagoJWTWebhookVerifier, error) {
	if keyProvider == nil {
		return nil, fmt.Errorf("%w: webhook public key provider is required", ErrValidation)
	}
	if keyTTL <= 0 {
		keyTTL = defaultLagoWebhookPublicKeyTTL
	}
	return &LagoJWTWebhookVerifier{
		keyProvider: keyProvider,
		keyTTL:      keyTTL,
	}, nil
}

func (v *LagoJWTWebhookVerifier) Verify(ctx context.Context, headers http.Header, body []byte) error {
	if v == nil || v.keyProvider == nil {
		return fmt.Errorf("%w: webhook verifier is not configured", ErrValidation)
	}
	if !json.Valid(body) {
		return fmt.Errorf("%w: webhook body must be valid json", ErrValidation)
	}

	signatureAlgo := strings.ToLower(strings.TrimSpace(headers.Get("X-Lago-Signature-Algorithm")))
	if signatureAlgo != "jwt" {
		return fmt.Errorf("%w: unsupported webhook signature algorithm %q", ErrValidation, signatureAlgo)
	}
	signature := strings.TrimSpace(headers.Get("X-Lago-Signature"))
	if signature == "" {
		return fmt.Errorf("%w: missing X-Lago-Signature header", ErrValidation)
	}

	key, err := v.publicKey(ctx)
	if err != nil {
		return err
	}

	token, err := jwt.Parse(
		signature,
		func(token *jwt.Token) (any, error) {
			if token.Method.Alg() != jwt.SigningMethodRS256.Alg() {
				return nil, fmt.Errorf("unexpected signing algorithm: %s", token.Method.Alg())
			}
			return key, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
	)
	if err != nil {
		return fmt.Errorf("%w: invalid webhook jwt signature", ErrValidation)
	}
	if !token.Valid {
		return fmt.Errorf("%w: invalid webhook jwt token", ErrValidation)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return fmt.Errorf("%w: invalid webhook jwt claims", ErrValidation)
	}

	if issuer, _ := claims["iss"].(string); strings.TrimSpace(issuer) != "" {
		if !sameNormalizedURL(issuer, v.keyProvider.ExpectedIssuer()) {
			return fmt.Errorf("%w: unexpected webhook issuer", ErrValidation)
		}
	}

	signedData, ok := claims["data"].(string)
	if !ok || strings.TrimSpace(signedData) == "" {
		return fmt.Errorf("%w: webhook jwt payload is missing data claim", ErrValidation)
	}
	var signedPayload any
	if err := json.Unmarshal([]byte(signedData), &signedPayload); err != nil {
		return fmt.Errorf("%w: invalid webhook jwt data claim", ErrValidation)
	}
	var requestPayload any
	if err := json.Unmarshal(body, &requestPayload); err != nil {
		return fmt.Errorf("%w: invalid webhook request payload", ErrValidation)
	}

	if !reflect.DeepEqual(signedPayload, requestPayload) {
		return fmt.Errorf("%w: webhook payload does not match signed data", ErrValidation)
	}
	return nil
}

func (v *LagoJWTWebhookVerifier) publicKey(ctx context.Context) (*rsa.PublicKey, error) {
	v.mu.RLock()
	if v.cachedKey != nil && time.Since(v.cachedAt) < v.keyTTL {
		key := v.cachedKey
		v.mu.RUnlock()
		return key, nil
	}
	v.mu.RUnlock()

	v.mu.Lock()
	defer v.mu.Unlock()
	if v.cachedKey != nil && time.Since(v.cachedAt) < v.keyTTL {
		return v.cachedKey, nil
	}

	key, err := v.keyProvider.FetchWebhookPublicKey(ctx)
	if err != nil {
		return nil, err
	}
	v.cachedKey = key
	v.cachedAt = time.Now().UTC()
	return key, nil
}

func sameNormalizedURL(a, b string) bool {
	normalize := func(v string) string {
		v = strings.TrimSpace(strings.ToLower(v))
		return strings.TrimRight(v, "/")
	}
	return normalize(a) == normalize(b)
}
