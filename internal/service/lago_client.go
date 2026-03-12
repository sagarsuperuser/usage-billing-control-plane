package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"lago-usage-billing-alpha/internal/domain"
)

const (
	defaultLagoHTTPTimeout = 10 * time.Second
	maxLagoResponseBytes   = 1 << 20
)

type LagoClientConfig struct {
	BaseURL string
	APIKey  string
	Timeout time.Duration
}

type LagoClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewLagoClient(cfg LagoClientConfig) (*LagoClient, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("%w: lago base url is required", ErrValidation)
	}
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("%w: lago api key is required", ErrValidation)
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultLagoHTTPTimeout
	}

	return &LagoClient{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: timeout},
	}, nil
}

func (c *LagoClient) SyncMeter(ctx context.Context, meter domain.Meter) error {
	if c == nil {
		return fmt.Errorf("%w: lago client is required", ErrValidation)
	}

	aggregationType, err := mapAggregationToLago(meter.Aggregation)
	if err != nil {
		return err
	}

	billableMetric := map[string]any{
		"name":             meter.Name,
		"code":             meter.Key,
		"aggregation_type": aggregationType,
		"description":      fmt.Sprintf("synced by lago-usage-billing-alpha meter_id=%s rule_version_id=%s", meter.ID, meter.RatingRuleVersionID),
	}
	if aggregationType != "count_agg" {
		billableMetric["field_name"] = "value"
	}

	payload := map[string]any{"billable_metric": billableMetric}

	createStatus, createBody, err := c.doJSONRequest(ctx, http.MethodPost, "/api/v1/billable_metrics", payload)
	if err == nil {
		return nil
	}

	updatePath := "/api/v1/billable_metrics/" + url.PathEscape(strings.TrimSpace(meter.Key))
	updateStatus, updateBody, updateErr := c.doJSONRequest(ctx, http.MethodPut, updatePath, payload)
	if updateErr == nil {
		return nil
	}

	return fmt.Errorf("lago meter sync failed (create_status=%d create_body=%s update_status=%d update_body=%s)",
		createStatus,
		abbrevForLog(createBody),
		updateStatus,
		abbrevForLog(updateBody),
	)
}

func (c *LagoClient) ProxyInvoicePreview(ctx context.Context, payload []byte) (int, []byte, error) {
	if c == nil {
		return 0, nil, fmt.Errorf("%w: lago client is required", ErrValidation)
	}
	if !json.Valid(payload) {
		return 0, nil, fmt.Errorf("%w: request body must be valid json", ErrValidation)
	}

	status, body, err := c.doRawRequest(ctx, http.MethodPost, "/api/v1/invoices/preview", payload)
	if err != nil {
		return 0, nil, err
	}
	if len(body) == 0 {
		body = []byte(`{"error":"empty response from lago"}`)
	}
	if !json.Valid(body) {
		return 0, nil, fmt.Errorf("invalid non-json response from lago: %s", abbrevForLog(body))
	}
	return status, body, nil
}

func (c *LagoClient) ProxyInvoiceRetryPayment(ctx context.Context, invoiceID string, payload []byte) (int, []byte, error) {
	if c == nil {
		return 0, nil, fmt.Errorf("%w: lago client is required", ErrValidation)
	}
	invoiceID = strings.TrimSpace(invoiceID)
	if invoiceID == "" {
		return 0, nil, fmt.Errorf("%w: invoice id is required", ErrValidation)
	}
	if len(strings.TrimSpace(string(payload))) == 0 {
		payload = []byte("{}")
	}
	if !json.Valid(payload) {
		return 0, nil, fmt.Errorf("%w: request body must be valid json", ErrValidation)
	}

	path := "/api/v1/invoices/" + url.PathEscape(invoiceID) + "/retry_payment"
	status, body, err := c.doRawRequest(ctx, http.MethodPost, path, payload)
	if err != nil {
		return 0, nil, err
	}
	if len(body) == 0 {
		body = []byte("{}")
	}
	if !json.Valid(body) {
		return 0, nil, fmt.Errorf("invalid non-json response from lago: %s", abbrevForLog(body))
	}
	return status, body, nil
}

func (c *LagoClient) ProxyInvoiceByID(ctx context.Context, invoiceID string) (int, []byte, error) {
	if c == nil {
		return 0, nil, fmt.Errorf("%w: lago client is required", ErrValidation)
	}
	invoiceID = strings.TrimSpace(invoiceID)
	if invoiceID == "" {
		return 0, nil, fmt.Errorf("%w: invoice id is required", ErrValidation)
	}

	path := "/api/v1/invoices/" + url.PathEscape(invoiceID)
	status, body, err := c.doRawRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return 0, nil, err
	}
	if len(body) == 0 {
		body = []byte("{}")
	}
	if !json.Valid(body) {
		return 0, nil, fmt.Errorf("invalid non-json response from lago: %s", abbrevForLog(body))
	}
	return status, body, nil
}

func (c *LagoClient) doJSONRequest(ctx context.Context, method, path string, payload any) (int, []byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, err
	}
	status, respBody, reqErr := c.doRawRequest(ctx, method, path, body)
	if reqErr != nil {
		return status, respBody, reqErr
	}
	if status < 200 || status >= 300 {
		return status, respBody, fmt.Errorf("lago api %s %s returned status=%d body=%s", method, path, status, abbrevForLog(respBody))
	}
	return status, respBody, nil
}

func (c *LagoClient) doRawRequest(ctx context.Context, method, path string, payload []byte) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxLagoResponseBytes))
	if err != nil {
		return resp.StatusCode, nil, err
	}

	return resp.StatusCode, body, nil
}

func mapAggregationToLago(v string) (string, error) {
	switch strings.TrimSpace(strings.ToLower(v)) {
	case "count":
		return "count_agg", nil
	case "sum":
		return "sum_agg", nil
	case "max":
		return "max_agg", nil
	default:
		return "", fmt.Errorf("%w: unsupported aggregation %q for lago sync", ErrValidation, v)
	}
}

func abbrevForLog(body []byte) string {
	v := strings.TrimSpace(string(body))
	if len(v) <= 300 {
		return v
	}
	return v[:300] + "...(truncated)"
}
