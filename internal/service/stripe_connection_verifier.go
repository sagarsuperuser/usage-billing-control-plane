package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const defaultStripeAPIBaseURL = "https://api.stripe.com"

type HTTPStripeConnectionVerifier struct {
	baseURL    string
	httpClient *http.Client
}

type stripeAccountResponse struct {
	ID       string `json:"id"`
	Livemode bool   `json:"livemode"`
}

type stripeErrorEnvelope struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

func NewHTTPStripeConnectionVerifier(baseURL string, timeout time.Duration) (*HTTPStripeConnectionVerifier, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = defaultStripeAPIBaseURL
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &HTTPStripeConnectionVerifier{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: timeout},
	}, nil
}

func (v *HTTPStripeConnectionVerifier) VerifyStripeSecret(ctx context.Context, secretKey string) (StripeConnectionVerificationResult, error) {
	if v == nil || v.httpClient == nil {
		return StripeConnectionVerificationResult{}, fmt.Errorf("%w: stripe connection verifier is required", ErrValidation)
	}
	secretKey = strings.TrimSpace(secretKey)
	if secretKey == "" {
		return StripeConnectionVerificationResult{}, fmt.Errorf("%w: stripe secret key is required", ErrValidation)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.baseURL+"/v1/account", nil)
	if err != nil {
		return StripeConnectionVerificationResult{}, fmt.Errorf("build stripe verification request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+secretKey)
	req.Header.Set("Accept", "application/json")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return StripeConnectionVerificationResult{}, fmt.Errorf("%w: stripe verification request failed: %v", ErrDependency, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var envelope stripeErrorEnvelope
		_ = json.NewDecoder(resp.Body).Decode(&envelope)
		message := strings.TrimSpace(envelope.Error.Message)
		if message == "" {
			message = "Stripe rejected the connection details."
		}
		return StripeConnectionVerificationResult{}, fmt.Errorf("%w: %s", ErrDependency, message)
	}

	var payload stripeAccountResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return StripeConnectionVerificationResult{}, fmt.Errorf("decode stripe verification response: %w", err)
	}
	if strings.TrimSpace(payload.ID) == "" {
		return StripeConnectionVerificationResult{}, fmt.Errorf("%w: Stripe verification returned an empty account id", ErrDependency)
	}

	return StripeConnectionVerificationResult{
		AccountID:  strings.TrimSpace(payload.ID),
		Livemode:   payload.Livemode,
		VerifiedAt: time.Now().UTC(),
	}, nil
}
