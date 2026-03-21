package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type LagoBillingProviderAdapter struct {
	transport                *LagoHTTPTransport
	stripeSuccessRedirectURL string
}

func NewLagoBillingProviderAdapter(transport *LagoHTTPTransport, stripeSuccessRedirectURL string) *LagoBillingProviderAdapter {
	return &LagoBillingProviderAdapter{
		transport:                transport,
		stripeSuccessRedirectURL: strings.TrimSpace(stripeSuccessRedirectURL),
	}
}

type lagoStripePaymentProvider struct {
	LagoID             string `json:"lago_id"`
	LagoOrganizationID string `json:"lago_organization_id"`
	Code               string `json:"code"`
	Name               string `json:"name"`
	ProviderType       string `json:"provider_type"`
}

type lagoStripePaymentProviderResponse struct {
	PaymentProvider lagoStripePaymentProvider `json:"payment_provider"`
}

type lagoCurrentOrganization struct {
	ID      string `json:"id"`
	HMACKey string `json:"hmacKey"`
}

func (a *LagoBillingProviderAdapter) EnsureStripeProvider(ctx context.Context, input EnsureStripeProviderInput) (EnsureStripeProviderResult, error) {
	if a == nil || a.transport == nil {
		return EnsureStripeProviderResult{}, fmt.Errorf("%w: lago billing provider adapter is required", ErrValidation)
	}
	input.ConnectionID = strings.TrimSpace(input.ConnectionID)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Environment = strings.TrimSpace(input.Environment)
	input.SecretKey = strings.TrimSpace(input.SecretKey)
	input.LagoOrganizationID = strings.TrimSpace(input.LagoOrganizationID)
	input.LagoProviderCode = strings.TrimSpace(input.LagoProviderCode)
	if input.ConnectionID == "" {
		return EnsureStripeProviderResult{}, fmt.Errorf("%w: connection id is required", ErrValidation)
	}
	if input.DisplayName == "" {
		return EnsureStripeProviderResult{}, fmt.Errorf("%w: display name is required", ErrValidation)
	}
	if input.SecretKey == "" {
		return EnsureStripeProviderResult{}, fmt.Errorf("%w: stripe secret is required", ErrValidation)
	}
	if input.LagoOrganizationID == "" {
		return EnsureStripeProviderResult{}, fmt.Errorf("%w: lago organization id is required", ErrValidation)
	}

	providerCode := input.LagoProviderCode
	if providerCode == "" {
		providerCode = defaultLagoStripeProviderCode(input.ConnectionID, input.Environment)
	}

	currentOrg, err := a.getCurrentOrganization(ctx)
	if err != nil {
		return EnsureStripeProviderResult{}, err
	}
	if currentOrg.ID == "" {
		return EnsureStripeProviderResult{}, fmt.Errorf("%w: current lago organization id is empty", ErrDependency)
	}
	if !strings.EqualFold(currentOrg.ID, input.LagoOrganizationID) {
		return EnsureStripeProviderResult{}, fmt.Errorf("%w: lago api key organization %q does not match requested organization %q", ErrDependency, currentOrg.ID, input.LagoOrganizationID)
	}
	if strings.TrimSpace(currentOrg.HMACKey) == "" {
		return EnsureStripeProviderResult{}, fmt.Errorf("%w: current lago organization hmac key is empty", ErrDependency)
	}

	existing, err := a.getPaymentProviderByCode(ctx, providerCode)
	if err != nil {
		return EnsureStripeProviderResult{}, err
	}
	if existing != nil && existing.ProviderType != "stripe" {
		return EnsureStripeProviderResult{}, fmt.Errorf("lago provider code %q already exists as %s", providerCode, existing.ProviderType)
	}

	upserted, err := a.upsertStripeProvider(ctx, providerCode, input.DisplayName, input.SecretKey)
	if err != nil {
		return EnsureStripeProviderResult{}, err
	}

	now := time.Now().UTC()
	return EnsureStripeProviderResult{
		LagoOrganizationID: currentOrg.ID,
		LagoProviderCode:   upserted.Code,
		LagoWebhookHMACKey: currentOrg.HMACKey,
		ConnectedAt:        now,
		LastSyncedAt:       now,
	}, nil
}

func (a *LagoBillingProviderAdapter) getCurrentOrganization(ctx context.Context) (lagoCurrentOrganization, error) {
	status, body, err := a.transport.doJSONRequest(ctx, http.MethodPost, "/graphql", map[string]any{
		"query": `query AlphaCurrentOrganization { organization { id hmacKey } }`,
	})
	if err != nil {
		return lagoCurrentOrganization{}, fmt.Errorf("fetch current lago organization: %w", err)
	}
	var payload struct {
		Data struct {
			Organization lagoCurrentOrganization `json:"organization"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return lagoCurrentOrganization{}, fmt.Errorf("decode current lago organization response status=%d: %w", status, err)
	}
	if len(payload.Errors) > 0 {
		messages := make([]string, 0, len(payload.Errors))
		for _, item := range payload.Errors {
			if msg := strings.TrimSpace(item.Message); msg != "" {
				messages = append(messages, msg)
			}
		}
		return lagoCurrentOrganization{}, fmt.Errorf("fetch current lago organization failed: %s", strings.Join(messages, "; "))
	}
	return payload.Data.Organization, nil
}

func (a *LagoBillingProviderAdapter) getPaymentProviderByCode(ctx context.Context, code string) (*lagoStripePaymentProvider, error) {
	path := "/api/v1/payment_providers/stripe/" + url.PathEscape(strings.TrimSpace(code))
	status, body, err := a.transport.doRawRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("lookup lago payment provider by code %q: %w", code, err)
	}
	if status == http.StatusNotFound {
		return nil, nil
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("lookup lago payment provider by code %q returned status=%d body=%s", code, status, abbrevForLog(body))
	}

	var resp lagoStripePaymentProviderResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode lago payment provider by code %q: %w", code, err)
	}
	if strings.TrimSpace(resp.PaymentProvider.LagoID) == "" {
		return nil, nil
	}
	return &resp.PaymentProvider, nil
}

func (a *LagoBillingProviderAdapter) upsertStripeProvider(ctx context.Context, code, displayName, secretKey string) (lagoStripePaymentProvider, error) {
	payload := map[string]any{"payment_provider": map[string]any{
		"code":                 code,
		"name":                 displayName,
		"secret_key":           secretKey,
		"success_redirect_url": a.stripeSuccessRedirectURL,
	}}
	status, body, err := a.transport.doJSONRequest(ctx, http.MethodPost, "/api/v1/payment_providers/stripe", payload)
	if err != nil {
		return lagoStripePaymentProvider{}, fmt.Errorf("upsert lago stripe provider %q: %w", code, err)
	}
	var resp lagoStripePaymentProviderResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return lagoStripePaymentProvider{}, fmt.Errorf("decode lago stripe provider %q response status=%d: %w", code, status, err)
	}
	if strings.TrimSpace(resp.PaymentProvider.LagoID) == "" {
		return lagoStripePaymentProvider{}, fmt.Errorf("upsert lago stripe provider %q returned empty id", code)
	}
	return resp.PaymentProvider, nil
}

var lagoProviderCodeSanitizer = regexp.MustCompile(`[^a-z0-9_]+`)

func defaultLagoStripeProviderCode(connectionID, environment string) string {
	base := strings.ToLower(strings.TrimSpace(connectionID))
	if base == "" {
		base = "connection"
	}
	env := strings.ToLower(strings.TrimSpace(environment))
	if env == "" {
		env = "test"
	}
	value := fmt.Sprintf("alpha_stripe_%s_%s", env, base)
	value = lagoProviderCodeSanitizer.ReplaceAllString(value, "_")
	value = strings.Trim(value, "_")
	if len(value) > 48 {
		value = strings.Trim(value[:48], "_")
	}
	if value == "" {
		return "alpha_stripe_test_connection"
	}
	return value
}
