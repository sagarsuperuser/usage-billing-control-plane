package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
)

type BillingSecretStore interface {
	PutStripeSecret(ctx context.Context, connectionID, secret string) (string, error)
	GetStripeSecret(ctx context.Context, secretRef string) (string, error)
	RotateStripeSecret(ctx context.Context, secretRef, secret string) (string, error)
	DeleteSecret(ctx context.Context, secretRef string) error
}

type MemoryBillingSecretStore struct {
	mu      sync.RWMutex
	secrets map[string]string
}

func NewMemoryBillingSecretStore() *MemoryBillingSecretStore {
	return &MemoryBillingSecretStore{secrets: map[string]string{}}
}

func (s *MemoryBillingSecretStore) PutStripeSecret(_ context.Context, connectionID, secret string) (string, error) {
	connectionID = strings.TrimSpace(connectionID)
	secret = strings.TrimSpace(secret)
	if connectionID == "" {
		return "", fmt.Errorf("%w: connection id is required", ErrValidation)
	}
	if secret == "" {
		return "", fmt.Errorf("%w: stripe secret is required", ErrValidation)
	}
	secretRef, err := newMemoryBillingSecretRef(connectionID)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.secrets[secretRef] = secret
	return secretRef, nil
}

func (s *MemoryBillingSecretStore) GetStripeSecret(_ context.Context, secretRef string) (string, error) {
	secretRef = strings.TrimSpace(secretRef)
	if secretRef == "" {
		return "", fmt.Errorf("%w: secret ref is required", ErrValidation)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	secret, ok := s.secrets[secretRef]
	if !ok {
		return "", storeErrNotFound("stripe secret")
	}
	return secret, nil
}

func (s *MemoryBillingSecretStore) RotateStripeSecret(ctx context.Context, secretRef, secret string) (string, error) {
	secretRef = strings.TrimSpace(secretRef)
	secret = strings.TrimSpace(secret)
	if secretRef == "" {
		return "", fmt.Errorf("%w: secret ref is required", ErrValidation)
	}
	if secret == "" {
		return "", fmt.Errorf("%w: stripe secret is required", ErrValidation)
	}
	s.mu.Lock()
	_, ok := s.secrets[secretRef]
	s.mu.Unlock()
	if !ok {
		return "", storeErrNotFound("stripe secret")
	}
	connectionID := connectionIDFromSecretRef(secretRef)
	newRef, err := s.PutStripeSecret(ctx, connectionID, secret)
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
