package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

// StripeCustomerBillingAdapter implements CustomerBillingAdapter using
// direct Stripe API calls instead of Lago. Customer data is stored locally
// in Postgres; Stripe is used for payment method management and checkout.
type StripeCustomerBillingAdapter struct {
	store        store.Repository
	stripeClient *StripeClient
	secretStore  BillingSecretStore
	successURL   string
}

func NewStripeCustomerBillingAdapter(repo store.Repository, stripeClient *StripeClient, secretStore BillingSecretStore, successURL string) *StripeCustomerBillingAdapter {
	return &StripeCustomerBillingAdapter{
		store:        repo,
		stripeClient: stripeClient,
		secretStore:  secretStore,
		successURL:   successURL,
	}
}

// getStripeKey resolves the Stripe secret key for the calling tenant.
func (a *StripeCustomerBillingAdapter) getStripeKey(ctx context.Context, tenantID string) (string, error) {
	tenant, err := a.store.GetTenant(tenantID)
	if err != nil {
		return "", fmt.Errorf("load tenant: %w", err)
	}
	if tenant.BillingProviderConnectionID == "" {
		return "", fmt.Errorf("tenant %s has no billing provider connection", tenantID)
	}
	conn, err := a.store.GetBillingProviderConnection(tenant.BillingProviderConnectionID)
	if err != nil {
		return "", fmt.Errorf("load billing connection: %w", err)
	}
	secrets, err := a.secretStore.GetConnectionSecrets(ctx, conn.SecretRef)
	if err != nil {
		return "", fmt.Errorf("load stripe secret: %w", err)
	}
	return secrets.StripeSecretKey, nil
}

// UpsertCustomer is a pass-through for the raw JSON API contract.
// In the Stripe-direct world, this creates/updates the local customer.
// The actual Stripe customer is created via SyncCustomer.
func (a *StripeCustomerBillingAdapter) UpsertCustomer(_ context.Context, payload []byte) (int, []byte, error) {
	// Return the payload as-is for backward compatibility.
	// The actual customer creation is handled by the service layer.
	return http.StatusOK, payload, nil
}

// GetCustomer returns customer data from the local database as JSON.
func (a *StripeCustomerBillingAdapter) GetCustomer(_ context.Context, externalID string) (int, []byte, error) {
	// This method was used to fetch from Lago. Now returns a minimal response.
	resp := map[string]any{"external_id": externalID}
	data, _ := json.Marshal(resp)
	return http.StatusOK, data, nil
}

// ListCustomerPaymentMethods queries Stripe directly for the customer's payment methods.
func (a *StripeCustomerBillingAdapter) ListCustomerPaymentMethods(ctx context.Context, externalID string) (int, []byte, error) {
	// Resolve customer's Stripe ID from payment setup.
	customers, err := a.store.ListCustomers(store.CustomerListFilter{ExternalID: externalID})
	if err != nil || len(customers) == 0 {
		return http.StatusOK, []byte(`{"payment_methods":[]}`), nil
	}
	customer := customers[0]
	setup, err := a.store.GetCustomerPaymentSetup(customer.TenantID, customer.ID)
	if err != nil || setup.ProviderCustomerReference == "" {
		return http.StatusOK, []byte(`{"payment_methods":[]}`), nil
	}

	key, err := a.getStripeKey(ctx, customer.TenantID)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	methods, err := a.stripeClient.ListPaymentMethods(ctx, key, setup.ProviderCustomerReference)
	if err != nil {
		return http.StatusBadGateway, nil, fmt.Errorf("stripe list payment methods: %w", err)
	}

	// Build response compatible with existing frontend expectations.
	var items []map[string]any
	for _, pm := range methods {
		item := map[string]any{
			"id":   pm.ID,
			"type": string(pm.Type),
		}
		items = append(items, item)
	}
	resp := map[string]any{"payment_methods": items}
	data, _ := json.Marshal(resp)
	return http.StatusOK, data, nil
}

// GenerateCustomerCheckoutURL creates a Stripe Checkout Session for payment method setup.
func (a *StripeCustomerBillingAdapter) GenerateCustomerCheckoutURL(ctx context.Context, externalID string) (int, []byte, error) {
	checkoutURL, err := a.GetCustomerCheckoutURL(ctx, externalID)
	if err != nil {
		return http.StatusBadGateway, nil, err
	}
	resp := map[string]any{"checkout_url": checkoutURL}
	data, _ := json.Marshal(resp)
	return http.StatusOK, data, nil
}

// SyncCustomer creates or updates the customer in Stripe and verifies payment methods.
func (a *StripeCustomerBillingAdapter) SyncCustomer(ctx context.Context, _ string, customer domain.Customer, profile domain.CustomerBillingProfile, setup domain.CustomerPaymentSetup, _ domain.WorkspaceBillingSettings) (CustomerSyncResult, error) {
	key, err := a.getStripeKey(ctx, customer.TenantID)
	if err != nil {
		return CustomerSyncResult{}, err
	}

	input := CreateStripeCustomerInput{
		Email:       profile.Email,
		Name:        profile.LegalName,
		Description: customer.DisplayName,
		Metadata: map[string]string{
			"tenant_id":   customer.TenantID,
			"external_id": customer.ExternalID,
		},
	}

	var stripeCustomerID string
	if setup.ProviderCustomerReference != "" {
		// Update existing Stripe customer.
		updated, err := a.stripeClient.UpdateCustomer(ctx, key, setup.ProviderCustomerReference, input)
		if err != nil {
			return CustomerSyncResult{}, fmt.Errorf("stripe update customer: %w", err)
		}
		stripeCustomerID = updated.ID
	} else {
		// Create new Stripe customer.
		created, err := a.stripeClient.CreateCustomer(ctx, key, input)
		if err != nil {
			return CustomerSyncResult{}, fmt.Errorf("stripe create customer: %w", err)
		}
		stripeCustomerID = created.ID
	}

	// Verify payment methods.
	methods, err := a.stripeClient.ListPaymentMethods(ctx, key, stripeCustomerID)
	if err != nil {
		return CustomerSyncResult{}, fmt.Errorf("stripe list payment methods: %w", err)
	}

	result := CustomerSyncResult{
		ProviderCustomerID:          stripeCustomerID,
		DefaultPaymentMethodPresent: len(methods) > 0,
	}
	if len(methods) > 0 {
		result.PaymentMethodType = string(methods[0].Type)
		result.ProviderPaymentMethodRef = methods[0].ID
	}

	return result, nil
}

// GetCustomerCheckoutURL generates a Stripe Checkout Session URL for payment method setup.
func (a *StripeCustomerBillingAdapter) GetCustomerCheckoutURL(ctx context.Context, externalID string) (string, error) {
	customers, err := a.store.ListCustomers(store.CustomerListFilter{ExternalID: externalID})
	if err != nil || len(customers) == 0 {
		return "", fmt.Errorf("customer not found: %s", externalID)
	}
	customer := customers[0]

	setup, err := a.store.GetCustomerPaymentSetup(customer.TenantID, customer.ID)
	if err != nil || setup.ProviderCustomerReference == "" {
		return "", fmt.Errorf("customer %s has no Stripe customer ID", externalID)
	}

	key, err := a.getStripeKey(ctx, customer.TenantID)
	if err != nil {
		return "", err
	}

	session, err := a.stripeClient.CreateCheckoutSession(ctx, key, CreateCheckoutSessionInput{
		CustomerID: setup.ProviderCustomerReference,
		SuccessURL: a.successURL,
		CancelURL:  a.successURL,
	})
	if err != nil {
		return "", fmt.Errorf("stripe create checkout session: %w", err)
	}

	return session.URL, nil
}
