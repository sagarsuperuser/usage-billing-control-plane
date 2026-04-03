package service

import (
	"context"
	"fmt"
	"time"
)

// StripeBillingProviderAdapter implements BillingProviderAdapter using
// direct Stripe verification only (no Lago organization provisioning).
// It replaces LagoBillingProviderAdapter.
type StripeBillingProviderAdapter struct {
	secretStore BillingSecretStore
	verifier    StripeConnectionVerifier
}

func NewStripeBillingProviderAdapter(secretStore BillingSecretStore, verifier StripeConnectionVerifier) *StripeBillingProviderAdapter {
	return &StripeBillingProviderAdapter{
		secretStore: secretStore,
		verifier:    verifier,
	}
}

// EnsureStripeProvider verifies that the Stripe secret key is valid
// and returns a result indicating the connection is ready.
// Unlike the Lago adapter, this does NOT provision a Lago organization
// or register a Stripe provider in Lago.
func (a *StripeBillingProviderAdapter) EnsureStripeProvider(ctx context.Context, input EnsureStripeProviderInput) (EnsureStripeProviderResult, error) {
	if input.SecretKey == "" {
		return EnsureStripeProviderResult{}, fmt.Errorf("stripe secret key is required")
	}

	// Verify the Stripe key by calling Stripe's account API.
	verification, err := a.verifier.VerifyStripeSecret(ctx, input.SecretKey)
	if err != nil {
		return EnsureStripeProviderResult{}, fmt.Errorf("stripe verification failed: %w", err)
	}

	now := time.Now().UTC()
	return EnsureStripeProviderResult{
		// Lago-specific fields are populated with Stripe equivalents
		// for backward compatibility during the migration period.
		LagoOrganizationID: verification.AccountID, // Stripe account ID reused as org ID
		LagoProviderCode:   fmt.Sprintf("stripe_%s_%s", input.Environment, input.ConnectionID),
		ConnectedAt:        now,
		LastSyncedAt:       now,
	}, nil
}

// Compile-time interface check.
var _ BillingProviderAdapter = (*StripeBillingProviderAdapter)(nil)
