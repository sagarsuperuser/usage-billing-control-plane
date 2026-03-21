package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"usage-billing-control-plane/internal/store"
)

type LagoWebhookVerifier interface {
	Verify(ctx context.Context, headers http.Header, body []byte) error
}

type NoopLagoWebhookVerifier struct{}

func (NoopLagoWebhookVerifier) Verify(context.Context, http.Header, []byte) error { return nil }

type LagoWebhookHMACKeyProvider interface {
	HMACKeyForOrganization(ctx context.Context, organizationID string) (string, error)
}

type TenantBackedLagoWebhookHMACKeyProvider struct {
	repo        store.Repository
	secretStore BillingSecretStore
}

type StaticLagoWebhookHMACKeyProvider struct {
	hmacKey string
}

type FallbackLagoWebhookHMACKeyProvider struct {
	primary   LagoWebhookHMACKeyProvider
	secondary LagoWebhookHMACKeyProvider
}

type LagoHMACWebhookVerifier struct {
	keyProvider LagoWebhookHMACKeyProvider
}

func NewTenantBackedLagoWebhookHMACKeyProvider(repo store.Repository, secretStore BillingSecretStore) *TenantBackedLagoWebhookHMACKeyProvider {
	return &TenantBackedLagoWebhookHMACKeyProvider{repo: repo, secretStore: secretStore}
}

func NewStaticLagoWebhookHMACKeyProvider(hmacKey string) (*StaticLagoWebhookHMACKeyProvider, error) {
	hmacKey = strings.TrimSpace(hmacKey)
	if hmacKey == "" {
		return nil, fmt.Errorf("%w: lago webhook hmac key is required", ErrValidation)
	}
	return &StaticLagoWebhookHMACKeyProvider{hmacKey: hmacKey}, nil
}

func NewFallbackLagoWebhookHMACKeyProvider(primary, secondary LagoWebhookHMACKeyProvider) (*FallbackLagoWebhookHMACKeyProvider, error) {
	if primary == nil && secondary == nil {
		return nil, fmt.Errorf("%w: at least one webhook hmac key provider is required", ErrValidation)
	}
	return &FallbackLagoWebhookHMACKeyProvider{primary: primary, secondary: secondary}, nil
}

func (p *TenantBackedLagoWebhookHMACKeyProvider) HMACKeyForOrganization(ctx context.Context, organizationID string) (string, error) {
	if p == nil || p.repo == nil {
		return "", fmt.Errorf("%w: tenant-backed webhook hmac provider is not configured", ErrValidation)
	}
	if p.secretStore == nil {
		return "", fmt.Errorf("%w: billing secret store is required for webhook hmac verification", ErrValidation)
	}
	organizationID = strings.TrimSpace(organizationID)
	if organizationID == "" {
		return "", fmt.Errorf("%w: organization_id is required for webhook hmac verification", ErrValidation)
	}
	tenant, err := p.repo.GetTenantByLagoOrganizationID(organizationID)
	if err != nil {
		if err == store.ErrNotFound {
			return "", fmt.Errorf("%w: organization_id %q is not mapped to a tenant", ErrValidation, organizationID)
		}
		return "", err
	}
	connectionID := strings.TrimSpace(tenant.BillingProviderConnectionID)
	if connectionID == "" {
		return "", fmt.Errorf("%w: tenant %q has no billing provider connection for webhook hmac verification", ErrValidation, strings.TrimSpace(tenant.ID))
	}
	connection, err := p.repo.GetBillingProviderConnection(connectionID)
	if err != nil {
		if err == store.ErrNotFound {
			return "", fmt.Errorf("%w: billing provider connection %q not found for webhook hmac verification", ErrValidation, connectionID)
		}
		return "", err
	}
	if strings.TrimSpace(connection.SecretRef) == "" {
		return "", fmt.Errorf("%w: billing provider connection %q has no secret_ref", ErrValidation, connectionID)
	}
	secrets, err := p.secretStore.GetConnectionSecrets(ctx, connection.SecretRef)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(secrets.LagoWebhookHMACKey) == "" {
		return "", fmt.Errorf("%w: billing provider connection %q is missing lago webhook hmac key", ErrValidation, connectionID)
	}
	return secrets.LagoWebhookHMACKey, nil
}

func (p *StaticLagoWebhookHMACKeyProvider) HMACKeyForOrganization(context.Context, string) (string, error) {
	if p == nil || strings.TrimSpace(p.hmacKey) == "" {
		return "", fmt.Errorf("%w: static lago webhook hmac key is not configured", ErrValidation)
	}
	return p.hmacKey, nil
}

func (p *FallbackLagoWebhookHMACKeyProvider) HMACKeyForOrganization(ctx context.Context, organizationID string) (string, error) {
	if p == nil {
		return "", fmt.Errorf("%w: webhook hmac provider is not configured", ErrValidation)
	}
	if p.primary != nil {
		key, err := p.primary.HMACKeyForOrganization(ctx, organizationID)
		if err == nil && strings.TrimSpace(key) != "" {
			return key, nil
		}
	}
	if p.secondary != nil {
		return p.secondary.HMACKeyForOrganization(ctx, organizationID)
	}
	return "", fmt.Errorf("%w: no webhook hmac key resolved", ErrValidation)
}

func NewLagoHMACWebhookVerifier(keyProvider LagoWebhookHMACKeyProvider) (*LagoHMACWebhookVerifier, error) {
	if keyProvider == nil {
		return nil, fmt.Errorf("%w: webhook hmac key provider is required", ErrValidation)
	}
	return &LagoHMACWebhookVerifier{keyProvider: keyProvider}, nil
}

func (v *LagoHMACWebhookVerifier) Verify(ctx context.Context, headers http.Header, body []byte) error {
	if v == nil || v.keyProvider == nil {
		return fmt.Errorf("%w: webhook verifier is not configured", ErrValidation)
	}
	if !json.Valid(body) {
		return fmt.Errorf("%w: webhook body must be valid json", ErrValidation)
	}
	envelope, err := parseLagoWebhookEnvelope(body)
	if err != nil {
		return err
	}
	signatureAlgo := strings.ToLower(strings.TrimSpace(headers.Get("X-Lago-Signature-Algorithm")))
	if signatureAlgo != "hmac" {
		return fmt.Errorf("%w: unsupported webhook signature algorithm %q", ErrValidation, signatureAlgo)
	}
	signature := strings.TrimSpace(headers.Get("X-Lago-Signature"))
	if signature == "" {
		return fmt.Errorf("%w: missing X-Lago-Signature header", ErrValidation)
	}
	providedSig, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("%w: invalid webhook hmac signature encoding", ErrValidation)
	}
	key, err := v.keyProvider.HMACKeyForOrganization(ctx, envelope.OrganizationID)
	if err != nil {
		return err
	}
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write(body)
	if !hmac.Equal(providedSig, mac.Sum(nil)) {
		return fmt.Errorf("%w: invalid webhook hmac signature", ErrValidation)
	}
	return nil
}
