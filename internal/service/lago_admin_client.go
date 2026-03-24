package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type LagoOrganizationBootstrapResult struct {
	OrganizationID string
	APIKey         string
}

type LagoOrganizationBootstrapper interface {
	BootstrapOrganization(ctx context.Context, name string) (LagoOrganizationBootstrapResult, error)
}

type LagoAdminOrganizationBootstrapper struct {
	baseURL     string
	adminAPIKey string
	httpClient  *http.Client
}

func NewLagoAdminOrganizationBootstrapper(cfg LagoClientConfig, adminAPIKey string) (*LagoAdminOrganizationBootstrapper, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("%w: lago base url is required", ErrValidation)
	}
	adminAPIKey = strings.TrimSpace(adminAPIKey)
	if adminAPIKey == "" {
		return nil, fmt.Errorf("%w: lago admin api key is required", ErrValidation)
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultLagoHTTPTimeout
	}
	return &LagoAdminOrganizationBootstrapper{
		baseURL:     baseURL,
		adminAPIKey: adminAPIKey,
		httpClient:  &http.Client{Timeout: timeout},
	}, nil
}

func (c *LagoAdminOrganizationBootstrapper) BootstrapOrganization(ctx context.Context, name string) (LagoOrganizationBootstrapResult, error) {
	if c == nil || c.httpClient == nil {
		return LagoOrganizationBootstrapResult{}, fmt.Errorf("%w: lago admin organization bootstrapper is required", ErrValidation)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return LagoOrganizationBootstrapResult{}, fmt.Errorf("%w: organization name is required", ErrValidation)
	}

	payload, err := json.Marshal(map[string]any{
		"name":              name,
		"bootstrap_api_key": true,
	})
	if err != nil {
		return LagoOrganizationBootstrapResult{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/admin/organizations", bytes.NewReader(payload))
	if err != nil {
		return LagoOrganizationBootstrapResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Admin-API-Key", c.adminAPIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return LagoOrganizationBootstrapResult{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxLagoResponseBytes))
	if err != nil {
		return LagoOrganizationBootstrapResult{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return LagoOrganizationBootstrapResult{}, fmt.Errorf("%w: lago admin organization bootstrap failed (status=%d body=%s)", ErrDependency, resp.StatusCode, abbrevForLog(body))
	}

	var decoded struct {
		Organization struct {
			ID string `json:"id"`
		} `json:"organization"`
		OrganizationAPIKey string `json:"organization_api_key"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return LagoOrganizationBootstrapResult{}, fmt.Errorf("decode lago admin organization bootstrap response: %w", err)
	}
	result := LagoOrganizationBootstrapResult{
		OrganizationID: strings.TrimSpace(decoded.Organization.ID),
		APIKey:         strings.TrimSpace(decoded.OrganizationAPIKey),
	}
	if result.OrganizationID == "" || result.APIKey == "" {
		return LagoOrganizationBootstrapResult{}, fmt.Errorf("%w: lago admin organization bootstrap returned incomplete credentials", ErrDependency)
	}
	return result, nil
}
