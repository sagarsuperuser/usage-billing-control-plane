package service

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
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

type MeterSyncAdapter interface {
	SyncMeter(ctx context.Context, meter domain.Meter) error
}

type InvoiceBillingAdapter interface {
	ListInvoices(ctx context.Context, query url.Values) (int, []byte, error)
	PreviewInvoice(ctx context.Context, payload []byte) (int, []byte, error)
	RetryInvoicePayment(ctx context.Context, invoiceID string, payload []byte) (int, []byte, error)
	GetInvoice(ctx context.Context, invoiceID string) (int, []byte, error)
	ResendInvoiceEmail(ctx context.Context, invoiceID string, input BillingDocumentEmail) error
}

type CustomerBillingAdapter interface {
	UpsertCustomer(ctx context.Context, payload []byte) (int, []byte, error)
	GetCustomer(ctx context.Context, externalID string) (int, []byte, error)
	ListCustomerPaymentMethods(ctx context.Context, externalID string) (int, []byte, error)
	GenerateCustomerCheckoutURL(ctx context.Context, externalID string) (int, []byte, error)
}

type WebhookPublicKeyProvider interface {
	FetchWebhookPublicKey(ctx context.Context) (*rsa.PublicKey, error)
	ExpectedIssuer() string
}

type LagoHTTPTransport struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewLagoHTTPTransport(cfg LagoClientConfig) (*LagoHTTPTransport, error) {
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

	return &LagoHTTPTransport{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: timeout},
	}, nil
}

type LagoMeterSyncAdapter struct {
	transport *LagoHTTPTransport
}

func NewLagoMeterSyncAdapter(transport *LagoHTTPTransport) *LagoMeterSyncAdapter {
	return &LagoMeterSyncAdapter{transport: transport}
}

func (a *LagoMeterSyncAdapter) SyncMeter(ctx context.Context, meter domain.Meter) error {
	if a == nil || a.transport == nil {
		return fmt.Errorf("%w: lago meter sync adapter is required", ErrValidation)
	}

	aggregationType, err := mapAggregationToLago(meter.Aggregation)
	if err != nil {
		return err
	}

	billableMetric := map[string]any{
		"name":             meter.Name,
		"code":             meter.Key,
		"aggregation_type": aggregationType,
		"description":      fmt.Sprintf("synced by usage-billing-control-plane meter_id=%s rule_version_id=%s", meter.ID, meter.RatingRuleVersionID),
	}
	if aggregationType != "count_agg" {
		billableMetric["field_name"] = "value"
	}

	payload := map[string]any{"billable_metric": billableMetric}

	createStatus, createBody, err := a.transport.doJSONRequest(ctx, http.MethodPost, "/api/v1/billable_metrics", payload)
	if err == nil {
		return nil
	}

	updatePath := "/api/v1/billable_metrics/" + url.PathEscape(strings.TrimSpace(meter.Key))
	updateStatus, updateBody, updateErr := a.transport.doJSONRequest(ctx, http.MethodPut, updatePath, payload)
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

type LagoInvoiceAdapter struct {
	transport *LagoHTTPTransport
}

func NewLagoInvoiceAdapter(transport *LagoHTTPTransport) *LagoInvoiceAdapter {
	return &LagoInvoiceAdapter{transport: transport}
}

func (a *LagoInvoiceAdapter) ListInvoices(ctx context.Context, query url.Values) (int, []byte, error) {
	if a == nil || a.transport == nil {
		return 0, nil, fmt.Errorf("%w: lago invoice adapter is required", ErrValidation)
	}

	path := "/api/v1/invoices"
	if encoded := strings.TrimSpace(query.Encode()); encoded != "" {
		path += "?" + encoded
	}

	status, body, err := a.transport.doRawRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return 0, nil, err
	}
	if len(body) == 0 {
		body = []byte(`{"invoices":[]}`)
	}
	if !json.Valid(body) {
		return 0, nil, fmt.Errorf("invalid non-json response from lago: %s", abbrevForLog(body))
	}
	return status, body, nil
}

func (a *LagoInvoiceAdapter) PreviewInvoice(ctx context.Context, payload []byte) (int, []byte, error) {
	if a == nil || a.transport == nil {
		return 0, nil, fmt.Errorf("%w: lago invoice adapter is required", ErrValidation)
	}
	if !json.Valid(payload) {
		return 0, nil, fmt.Errorf("%w: request body must be valid json", ErrValidation)
	}

	status, body, err := a.transport.doRawRequest(ctx, http.MethodPost, "/api/v1/invoices/preview", payload)
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

func (a *LagoInvoiceAdapter) RetryInvoicePayment(ctx context.Context, invoiceID string, payload []byte) (int, []byte, error) {
	if a == nil || a.transport == nil {
		return 0, nil, fmt.Errorf("%w: lago invoice adapter is required", ErrValidation)
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
	status, body, err := a.transport.doRawRequest(ctx, http.MethodPost, path, payload)
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

func (a *LagoInvoiceAdapter) GetInvoice(ctx context.Context, invoiceID string) (int, []byte, error) {
	if a == nil || a.transport == nil {
		return 0, nil, fmt.Errorf("%w: lago invoice adapter is required", ErrValidation)
	}
	invoiceID = strings.TrimSpace(invoiceID)
	if invoiceID == "" {
		return 0, nil, fmt.Errorf("%w: invoice id is required", ErrValidation)
	}

	path := "/api/v1/invoices/" + url.PathEscape(invoiceID)
	status, body, err := a.transport.doRawRequest(ctx, http.MethodGet, path, nil)
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

func (a *LagoInvoiceAdapter) ResendInvoiceEmail(ctx context.Context, invoiceID string, input BillingDocumentEmail) error {
	if a == nil || a.transport == nil {
		return fmt.Errorf("%w: lago invoice adapter is required", ErrValidation)
	}
	invoiceID = strings.TrimSpace(invoiceID)
	if invoiceID == "" {
		return fmt.Errorf("%w: invoice id is required", ErrValidation)
	}

	payload := map[string]any{}
	if items := normalizeEmailRecipientList(input.To); len(items) > 0 {
		payload["to"] = items
	}
	if items := normalizeEmailRecipientList(input.Cc); len(items) > 0 {
		payload["cc"] = items
	}
	if items := normalizeEmailRecipientList(input.Bcc); len(items) > 0 {
		payload["bcc"] = items
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	path := "/api/v1/invoices/" + url.PathEscape(invoiceID) + "/resend_email"
	status, respBody, err := a.transport.doRawRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return err
	}
	if status >= 200 && status < 300 {
		return nil
	}
	return &NotificationDispatchError{
		StatusCode: status,
		Backend:    "lago",
		Message:    lagoNotificationErrorMessage(respBody),
	}
}

type LagoCustomerBillingAdapter struct {
	transport *LagoHTTPTransport
}

func NewLagoCustomerBillingAdapter(transport *LagoHTTPTransport) *LagoCustomerBillingAdapter {
	return &LagoCustomerBillingAdapter{transport: transport}
}

func (a *LagoCustomerBillingAdapter) UpsertCustomer(ctx context.Context, payload []byte) (int, []byte, error) {
	if a == nil || a.transport == nil {
		return 0, nil, fmt.Errorf("%w: lago customer billing adapter is required", ErrValidation)
	}
	if !json.Valid(payload) {
		return 0, nil, fmt.Errorf("%w: request body must be valid json", ErrValidation)
	}
	status, body, err := a.transport.doRawRequest(ctx, http.MethodPost, "/api/v1/customers", payload)
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

func (a *LagoCustomerBillingAdapter) GetCustomer(ctx context.Context, externalID string) (int, []byte, error) {
	if a == nil || a.transport == nil {
		return 0, nil, fmt.Errorf("%w: lago customer billing adapter is required", ErrValidation)
	}
	externalID = strings.TrimSpace(externalID)
	if externalID == "" {
		return 0, nil, fmt.Errorf("%w: external customer id is required", ErrValidation)
	}
	path := "/api/v1/customers/" + url.PathEscape(externalID)
	status, body, err := a.transport.doRawRequest(ctx, http.MethodGet, path, nil)
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

func (a *LagoCustomerBillingAdapter) ListCustomerPaymentMethods(ctx context.Context, externalID string) (int, []byte, error) {
	if a == nil || a.transport == nil {
		return 0, nil, fmt.Errorf("%w: lago customer billing adapter is required", ErrValidation)
	}
	externalID = strings.TrimSpace(externalID)
	if externalID == "" {
		return 0, nil, fmt.Errorf("%w: external customer id is required", ErrValidation)
	}
	path := "/api/v1/customers/" + url.PathEscape(externalID) + "/payment_methods"
	status, body, err := a.transport.doRawRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return 0, nil, err
	}
	if len(body) == 0 {
		body = []byte(`{"payment_methods":[]}`)
	}
	if !json.Valid(body) {
		return 0, nil, fmt.Errorf("invalid non-json response from lago: %s", abbrevForLog(body))
	}
	return status, body, nil
}

func (a *LagoCustomerBillingAdapter) GenerateCustomerCheckoutURL(ctx context.Context, externalID string) (int, []byte, error) {
	if a == nil || a.transport == nil {
		return 0, nil, fmt.Errorf("%w: lago customer billing adapter is required", ErrValidation)
	}
	externalID = strings.TrimSpace(externalID)
	if externalID == "" {
		return 0, nil, fmt.Errorf("%w: external customer id is required", ErrValidation)
	}
	path := "/api/v1/customers/" + url.PathEscape(externalID) + "/checkout_url"
	status, body, err := a.transport.doRawRequest(ctx, http.MethodPost, path, []byte("{}"))
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

type LagoWebhookKeyProvider struct {
	transport *LagoHTTPTransport
}

func NewLagoWebhookKeyProvider(transport *LagoHTTPTransport) *LagoWebhookKeyProvider {
	return &LagoWebhookKeyProvider{transport: transport}
}

func (p *LagoWebhookKeyProvider) ExpectedIssuer() string {
	if p == nil || p.transport == nil {
		return ""
	}
	return p.transport.baseURL
}

func (p *LagoWebhookKeyProvider) FetchWebhookPublicKey(ctx context.Context) (*rsa.PublicKey, error) {
	if p == nil || p.transport == nil {
		return nil, fmt.Errorf("%w: lago webhook key provider is required", ErrValidation)
	}
	statusCode, body, err := p.transport.doRawRequest(ctx, http.MethodGet, "/api/v1/webhooks/json_public_key", nil)
	if err != nil {
		return nil, fmt.Errorf("fetch lago webhook public key: %w", err)
	}
	if statusCode < 200 || statusCode >= 300 {
		return nil, fmt.Errorf("fetch lago webhook public key returned status=%d", statusCode)
	}

	var payload struct {
		Webhook struct {
			PublicKey string `json:"public_key"`
		} `json:"webhook"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode lago webhook public key response: %w", err)
	}
	encoded := strings.TrimSpace(payload.Webhook.PublicKey)
	if encoded == "" {
		return nil, fmt.Errorf("lago webhook public key is empty")
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode lago webhook public key: %w", err)
	}

	block, _ := pem.Decode(decoded)
	if block == nil {
		return nil, fmt.Errorf("parse lago webhook public key pem: no pem block")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err == nil {
		if key, ok := pub.(*rsa.PublicKey); ok {
			return key, nil
		}
	}
	if key, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		return key, nil
	}
	return nil, fmt.Errorf("parse lago webhook public key: unsupported key format")
}

func (t *LagoHTTPTransport) doJSONRequest(ctx context.Context, method, path string, payload any) (int, []byte, error) {
	return t.doJSONRequestWithHeaders(ctx, method, path, payload, nil)
}

func (t *LagoHTTPTransport) doJSONRequestWithHeaders(ctx context.Context, method, path string, payload any, headers map[string]string) (int, []byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, err
	}
	status, respBody, reqErr := t.doRawRequestWithHeaders(ctx, method, path, body, headers)
	if reqErr != nil {
		return status, respBody, reqErr
	}
	if status < 200 || status >= 300 {
		return status, respBody, fmt.Errorf("lago api %s %s returned status=%d body=%s", method, path, status, abbrevForLog(respBody))
	}
	return status, respBody, nil
}

func (t *LagoHTTPTransport) doRawRequest(ctx context.Context, method, path string, payload []byte) (int, []byte, error) {
	return t.doRawRequestWithHeaders(ctx, method, path, payload, nil)
}

func (t *LagoHTTPTransport) doRawRequestWithHeaders(ctx context.Context, method, path string, payload []byte, headers map[string]string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, t.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+t.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	for key, value := range headers {
		if strings.TrimSpace(key) == "" {
			continue
		}
		req.Header.Set(key, value)
	}

	resp, err := t.httpClient.Do(req)
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

func normalizeEmailRecipientList(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func lagoNotificationErrorMessage(body []byte) string {
	if len(body) == 0 {
		return "billing notification dispatch failed"
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err == nil {
		for _, key := range []string{"error", "message"} {
			if value := strings.TrimSpace(stringValue(payload[key])); value != "" {
				return value
			}
		}
	}
	return fmt.Sprintf("billing notification dispatch failed: %s", abbrevForLog(body))
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
