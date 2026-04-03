package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

type BillingSecretStore interface {
	PutConnectionSecrets(ctx context.Context, connectionID string, secrets BillingProviderSecrets) (string, error)
	GetConnectionSecrets(ctx context.Context, secretRef string) (BillingProviderSecrets, error)
	UpdateConnectionSecrets(ctx context.Context, secretRef string, secrets BillingProviderSecrets) (string, error)
	PutStripeSecret(ctx context.Context, connectionID, secret string) (string, error)
	GetStripeSecret(ctx context.Context, secretRef string) (string, error)
	RotateStripeSecret(ctx context.Context, secretRef, secret string) (string, error)
	DeleteSecret(ctx context.Context, secretRef string) error
}

type BillingProviderSecrets struct {
	StripeSecretKey     string `json:"stripe_secret_key,omitempty"`
	StripeWebhookSecret string `json:"stripe_webhook_secret,omitempty"`
	DeprecatedHMACKey  string `json:"lago_webhook_hmac_key,omitempty"`
}

type MemoryBillingSecretStore struct {
	mu      sync.RWMutex
	secrets map[string]string
}

func NewMemoryBillingSecretStore() *MemoryBillingSecretStore {
	return &MemoryBillingSecretStore{secrets: map[string]string{}}
}

func (s *MemoryBillingSecretStore) PutConnectionSecrets(_ context.Context, connectionID string, secrets BillingProviderSecrets) (string, error) {
	connectionID = strings.TrimSpace(connectionID)
	secrets = normalizeBillingProviderSecrets(secrets)
	if connectionID == "" {
		return "", fmt.Errorf("%w: connection id is required", ErrValidation)
	}
	if err := validateBillingProviderSecrets(secrets); err != nil {
		return "", err
	}
	secretRef, err := newMemoryBillingSecretRef(connectionID)
	if err != nil {
		return "", err
	}
	encoded, err := encodeBillingProviderSecrets(secrets)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.secrets[secretRef] = encoded
	return secretRef, nil
}

func (s *MemoryBillingSecretStore) GetConnectionSecrets(_ context.Context, secretRef string) (BillingProviderSecrets, error) {
	secretRef = strings.TrimSpace(secretRef)
	if secretRef == "" {
		return BillingProviderSecrets{}, fmt.Errorf("%w: secret ref is required", ErrValidation)
	}
	s.mu.RLock()
	stored, ok := s.secrets[secretRef]
	s.mu.RUnlock()
	if !ok {
		return BillingProviderSecrets{}, storeErrNotFound("billing secret")
	}
	return decodeBillingProviderSecrets(stored)
}

func (s *MemoryBillingSecretStore) UpdateConnectionSecrets(_ context.Context, secretRef string, secrets BillingProviderSecrets) (string, error) {
	secretRef = strings.TrimSpace(secretRef)
	secrets = normalizeBillingProviderSecrets(secrets)
	if secretRef == "" {
		return "", fmt.Errorf("%w: secret ref is required", ErrValidation)
	}
	if err := validateBillingProviderSecrets(secrets); err != nil {
		return "", err
	}
	encoded, err := encodeBillingProviderSecrets(secrets)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.secrets[secretRef]; !ok {
		return "", storeErrNotFound("billing secret")
	}
	s.secrets[secretRef] = encoded
	return secretRef, nil
}

func (s *MemoryBillingSecretStore) PutStripeSecret(ctx context.Context, connectionID, secret string) (string, error) {
	return s.PutConnectionSecrets(ctx, connectionID, BillingProviderSecrets{StripeSecretKey: secret})
}

func (s *MemoryBillingSecretStore) GetStripeSecret(ctx context.Context, secretRef string) (string, error) {
	secrets, err := s.GetConnectionSecrets(ctx, secretRef)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(secrets.StripeSecretKey) == "" {
		return "", storeErrNotFound("stripe secret")
	}
	return secrets.StripeSecretKey, nil
}

func (s *MemoryBillingSecretStore) RotateStripeSecret(ctx context.Context, secretRef, secret string) (string, error) {
	secrets, err := s.GetConnectionSecrets(ctx, secretRef)
	if err != nil {
		return "", err
	}
	secrets.StripeSecretKey = strings.TrimSpace(secret)
	if err := validateBillingProviderSecrets(secrets); err != nil {
		return "", err
	}
	connectionID := connectionIDFromSecretRef(secretRef)
	newRef, err := s.PutConnectionSecrets(ctx, connectionID, secrets)
	if err != nil {
		return "", err
	}
	_ = s.DeleteSecret(ctx, secretRef)
	return newRef, nil
}

func (s *MemoryBillingSecretStore) DeleteSecret(_ context.Context, secretRef string) error {
	secretRef = strings.TrimSpace(secretRef)
	if secretRef == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.secrets, secretRef)
	return nil
}

func newMemoryBillingSecretRef(connectionID string) (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate billing secret ref: %w", err)
	}
	return fmt.Sprintf("memory://billing-provider-connections/%s/%s", connectionID, hex.EncodeToString(buf)), nil
}

func connectionIDFromSecretRef(secretRef string) string {
	secretRef = strings.TrimSpace(secretRef)
	parts := strings.Split(secretRef, "/")
	if len(parts) >= 4 {
		return strings.TrimSpace(parts[3])
	}
	return "rotated"
}

func storeErrNotFound(label string) error {
	if strings.TrimSpace(label) == "" {
		label = "resource"
	}
	return fmt.Errorf("%w: %s not found", ErrValidation, label)
}

func normalizeBillingProviderSecrets(secrets BillingProviderSecrets) BillingProviderSecrets {
	return BillingProviderSecrets{
		StripeSecretKey:    strings.TrimSpace(secrets.StripeSecretKey),
		DeprecatedHMACKey: strings.TrimSpace(secrets.DeprecatedHMACKey),
	}
}

func validateBillingProviderSecrets(secrets BillingProviderSecrets) error {
	if strings.TrimSpace(secrets.StripeSecretKey) == "" && strings.TrimSpace(secrets.DeprecatedHMACKey) == "" {
		return fmt.Errorf("%w: at least one billing provider secret is required", ErrValidation)
	}
	return nil
}

func encodeBillingProviderSecrets(secrets BillingProviderSecrets) (string, error) {
	secrets = normalizeBillingProviderSecrets(secrets)
	if err := validateBillingProviderSecrets(secrets); err != nil {
		return "", err
	}
	payload, err := json.Marshal(secrets)
	if err != nil {
		return "", fmt.Errorf("encode billing provider secrets: %w", err)
	}
	return string(payload), nil
}

func decodeBillingProviderSecrets(raw string) (BillingProviderSecrets, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return BillingProviderSecrets{}, fmt.Errorf("%w: billing secret is empty", ErrValidation)
	}
	if !strings.HasPrefix(raw, "{") {
		return BillingProviderSecrets{StripeSecretKey: raw}, nil
	}
	var out BillingProviderSecrets
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return BillingProviderSecrets{}, fmt.Errorf("decode billing provider secrets: %w", err)
	}
	out = normalizeBillingProviderSecrets(out)
	if err := validateBillingProviderSecrets(out); err != nil {
		return BillingProviderSecrets{}, err
	}
	return out, nil
}
