package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

type lagoGraphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type lagoGraphQLError struct {
	Message string `json:"message"`
}

func (t *LagoHTTPTransport) doGraphQLRequest(ctx context.Context, query string, variables map[string]any, out any) error {
	if t == nil {
		return fmt.Errorf("%w: lago transport is required", ErrValidation)
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return fmt.Errorf("%w: graphql query is required", ErrValidation)
	}
	payload := lagoGraphQLRequest{Query: query, Variables: variables}
	status, body, err := t.doJSONRequest(ctx, http.MethodPost, "/graphql", payload)
	if err != nil {
		return err
	}
	var resp struct {
		Data   json.RawMessage    `json:"data"`
		Errors []lagoGraphQLError `json:"errors"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("decode lago graphql response status=%d: %w", status, err)
	}
	if len(resp.Errors) > 0 {
		parts := make([]string, 0, len(resp.Errors))
		for _, item := range resp.Errors {
			if msg := strings.TrimSpace(item.Message); msg != "" {
				parts = append(parts, msg)
			}
		}
		if len(parts) == 0 {
			parts = append(parts, "unknown graphql error")
		}
		return fmt.Errorf("lago graphql error: %s", strings.Join(parts, "; "))
	}
	if out != nil && len(resp.Data) > 0 && string(resp.Data) != "null" {
		if err := json.Unmarshal(resp.Data, out); err != nil {
			return fmt.Errorf("decode lago graphql data: %w", err)
		}
	}
	return nil
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

	providerCode := input.LagoProviderCode
	if providerCode == "" {
		providerCode = defaultLagoStripeProviderCode(input.ConnectionID, input.Environment)
	}

	existing, err := a.getPaymentProviderByCode(ctx, providerCode)
	if err != nil {
		return EnsureStripeProviderResult{}, err
	}

	now := time.Now().UTC()
	if existing == nil {
		created, err := a.addStripeProvider(ctx, providerCode, input.DisplayName, input.SecretKey)
		if err != nil {
			return EnsureStripeProviderResult{}, err
		}
		return EnsureStripeProviderResult{
			LagoOrganizationID: input.LagoOrganizationID,
			LagoProviderCode:   created.Code,
			ConnectedAt:        now,
			LastSyncedAt:       now,
		}, nil
	}
	if existing.TypeName != "StripeProvider" {
		return EnsureStripeProviderResult{}, fmt.Errorf("lago provider code %q already exists as %s", providerCode, existing.TypeName)
	}
	updated, err := a.updateStripeProvider(ctx, existing.ID, providerCode, input.DisplayName)
	if err != nil {
		return EnsureStripeProviderResult{}, err
	}
	return EnsureStripeProviderResult{
		LagoOrganizationID: input.LagoOrganizationID,
		LagoProviderCode:   updated.Code,
		ConnectedAt:        now,
		LastSyncedAt:       now,
	}, nil
}

type lagoPaymentProviderLookup struct {
	PaymentProvider *struct {
		TypeName string `json:"__typename"`
		ID       string `json:"id"`
		Code     string `json:"code"`
		Name     string `json:"name"`
	} `json:"paymentProvider"`
}

func (a *LagoBillingProviderAdapter) getPaymentProviderByCode(ctx context.Context, code string) (*struct {
	TypeName string `json:"__typename"`
	ID       string `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
}, error) {
	var resp lagoPaymentProviderLookup
	err := a.transport.doGraphQLRequest(ctx, `query alphaPaymentProviderByCode($code: String!) {
  paymentProvider(code: $code) {
    __typename
    ... on StripeProvider {
      id
      code
      name
    }
  }
}`, map[string]any{"code": code}, &resp)
	if err != nil {
		return nil, fmt.Errorf("lookup lago payment provider by code %q: %w", code, err)
	}
	return resp.PaymentProvider, nil
}

type lagoStripeProviderMutation struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

func (a *LagoBillingProviderAdapter) addStripeProvider(ctx context.Context, code, displayName, secretKey string) (lagoStripeProviderMutation, error) {
	var resp struct {
		AddStripePaymentProvider lagoStripeProviderMutation `json:"addStripePaymentProvider"`
	}
	err := a.transport.doGraphQLRequest(ctx, `mutation alphaAddStripePaymentProvider($input: AddStripePaymentProviderInput!) {
  addStripePaymentProvider(input: $input) {
    id
    code
    name
  }
}`, map[string]any{"input": map[string]any{
		"code":               code,
		"name":               displayName,
		"secretKey":          secretKey,
		"successRedirectUrl": a.stripeSuccessRedirectURL,
	}}, &resp)
	if err != nil {
		return lagoStripeProviderMutation{}, fmt.Errorf("create lago stripe provider %q: %w", code, err)
	}
	if strings.TrimSpace(resp.AddStripePaymentProvider.ID) == "" {
		return lagoStripeProviderMutation{}, fmt.Errorf("create lago stripe provider %q returned empty id", code)
	}
	return resp.AddStripePaymentProvider, nil
}

func (a *LagoBillingProviderAdapter) updateStripeProvider(ctx context.Context, id, code, displayName string) (lagoStripeProviderMutation, error) {
	var resp struct {
		UpdateStripePaymentProvider lagoStripeProviderMutation `json:"updateStripePaymentProvider"`
	}
	err := a.transport.doGraphQLRequest(ctx, `mutation alphaUpdateStripePaymentProvider($input: UpdateStripePaymentProviderInput!) {
  updateStripePaymentProvider(input: $input) {
    id
    code
    name
  }
}`, map[string]any{"input": map[string]any{
		"id":                 id,
		"code":               code,
		"name":               displayName,
		"successRedirectUrl": a.stripeSuccessRedirectURL,
	}}, &resp)
	if err != nil {
		return lagoStripeProviderMutation{}, fmt.Errorf("update lago stripe provider %q: %w", code, err)
	}
	if strings.TrimSpace(resp.UpdateStripePaymentProvider.ID) == "" {
		return lagoStripeProviderMutation{}, fmt.Errorf("update lago stripe provider %q returned empty id", code)
	}
	return resp.UpdateStripePaymentProvider, nil
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
