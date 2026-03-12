package service

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"lago-usage-billing-alpha/internal/domain"
	"lago-usage-billing-alpha/internal/store"
)

const (
	defaultLagoWebhookPublicKeyTTL = 5 * time.Minute
	maxWebhookListLimit            = 500
)

type LagoWebhookVerifier interface {
	Verify(ctx context.Context, headers http.Header, body []byte) error
}

type NoopLagoWebhookVerifier struct{}

func (NoopLagoWebhookVerifier) Verify(context.Context, http.Header, []byte) error { return nil }

type LagoJWTWebhookVerifier struct {
	lagoClient *LagoClient
	keyTTL     time.Duration

	mu        sync.RWMutex
	cachedKey *rsa.PublicKey
	cachedAt  time.Time
}

func NewLagoJWTWebhookVerifier(lagoClient *LagoClient, keyTTL time.Duration) (*LagoJWTWebhookVerifier, error) {
	if lagoClient == nil {
		return nil, fmt.Errorf("%w: lago client is required", ErrValidation)
	}
	if keyTTL <= 0 {
		keyTTL = defaultLagoWebhookPublicKeyTTL
	}
	return &LagoJWTWebhookVerifier{
		lagoClient: lagoClient,
		keyTTL:     keyTTL,
	}, nil
}

func (v *LagoJWTWebhookVerifier) Verify(ctx context.Context, headers http.Header, body []byte) error {
	if v == nil || v.lagoClient == nil {
		return fmt.Errorf("%w: webhook verifier is not configured", ErrValidation)
	}
	if !json.Valid(body) {
		return fmt.Errorf("%w: webhook body must be valid json", ErrValidation)
	}

	signatureAlgo := strings.ToLower(strings.TrimSpace(headers.Get("X-Lago-Signature-Algorithm")))
	if signatureAlgo != "jwt" {
		return fmt.Errorf("%w: unsupported webhook signature algorithm %q", ErrValidation, signatureAlgo)
	}
	signature := strings.TrimSpace(headers.Get("X-Lago-Signature"))
	if signature == "" {
		return fmt.Errorf("%w: missing X-Lago-Signature header", ErrValidation)
	}

	key, err := v.publicKey(ctx)
	if err != nil {
		return err
	}

	token, err := jwt.Parse(
		signature,
		func(token *jwt.Token) (any, error) {
			if token.Method.Alg() != jwt.SigningMethodRS256.Alg() {
				return nil, fmt.Errorf("unexpected signing algorithm: %s", token.Method.Alg())
			}
			return key, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
	)
	if err != nil {
		return fmt.Errorf("%w: invalid webhook jwt signature", ErrValidation)
	}
	if !token.Valid {
		return fmt.Errorf("%w: invalid webhook jwt token", ErrValidation)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return fmt.Errorf("%w: invalid webhook jwt claims", ErrValidation)
	}

	if issuer, _ := claims["iss"].(string); strings.TrimSpace(issuer) != "" {
		if !sameNormalizedURL(issuer, v.lagoClient.baseURL) {
			return fmt.Errorf("%w: unexpected webhook issuer", ErrValidation)
		}
	}

	signedData, ok := claims["data"].(string)
	if !ok || strings.TrimSpace(signedData) == "" {
		return fmt.Errorf("%w: webhook jwt payload is missing data claim", ErrValidation)
	}
	var signedPayload any
	if err := json.Unmarshal([]byte(signedData), &signedPayload); err != nil {
		return fmt.Errorf("%w: invalid webhook jwt data claim", ErrValidation)
	}
	var requestPayload any
	if err := json.Unmarshal(body, &requestPayload); err != nil {
		return fmt.Errorf("%w: invalid webhook request payload", ErrValidation)
	}

	if !reflect.DeepEqual(signedPayload, requestPayload) {
		return fmt.Errorf("%w: webhook payload does not match signed data", ErrValidation)
	}
	return nil
}

func (v *LagoJWTWebhookVerifier) publicKey(ctx context.Context) (*rsa.PublicKey, error) {
	v.mu.RLock()
	if v.cachedKey != nil && time.Since(v.cachedAt) < v.keyTTL {
		key := v.cachedKey
		v.mu.RUnlock()
		return key, nil
	}
	v.mu.RUnlock()

	v.mu.Lock()
	defer v.mu.Unlock()
	if v.cachedKey != nil && time.Since(v.cachedAt) < v.keyTTL {
		return v.cachedKey, nil
	}

	key, err := v.fetchPublicKey(ctx)
	if err != nil {
		return nil, err
	}
	v.cachedKey = key
	v.cachedAt = time.Now().UTC()
	return key, nil
}

func (v *LagoJWTWebhookVerifier) fetchPublicKey(ctx context.Context) (*rsa.PublicKey, error) {
	statusCode, body, err := v.lagoClient.doRawRequest(ctx, http.MethodGet, "/api/v1/webhooks/json_public_key", nil)
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

func sameNormalizedURL(a, b string) bool {
	normalize := func(v string) string {
		v = strings.TrimSpace(strings.ToLower(v))
		return strings.TrimRight(v, "/")
	}
	return normalize(a) == normalize(b)
}

type LagoOrganizationTenantMapper interface {
	TenantIDForOrganization(organizationID string) string
}

type StaticLagoOrganizationTenantMapper struct {
	defaultTenantID string
	byOrganization  map[string]string
}

func NewStaticLagoOrganizationTenantMapper(defaultTenantID string, byOrganization map[string]string) *StaticLagoOrganizationTenantMapper {
	cleanDefault := normalizeTenantID(defaultTenantID)
	clean := make(map[string]string, len(byOrganization))
	for orgID, tenantID := range byOrganization {
		orgID = strings.TrimSpace(orgID)
		if orgID == "" {
			continue
		}
		clean[orgID] = normalizeTenantID(tenantID)
	}
	return &StaticLagoOrganizationTenantMapper{
		defaultTenantID: cleanDefault,
		byOrganization:  clean,
	}
}

func (m *StaticLagoOrganizationTenantMapper) TenantIDForOrganization(organizationID string) string {
	if m == nil {
		return defaultTenantID
	}
	orgID := strings.TrimSpace(organizationID)
	if orgID == "" {
		return m.defaultTenantID
	}
	if tenantID, ok := m.byOrganization[orgID]; ok {
		return normalizeTenantID(tenantID)
	}
	return m.defaultTenantID
}

func ParseLagoOrganizationTenantMap(raw string) (map[string]string, error) {
	out := make(map[string]string)
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		pair := strings.SplitN(item, ":", 2)
		if len(pair) != 2 {
			return nil, fmt.Errorf("invalid LAGO_ORG_TENANT_MAP entry %q: expected organization_id:tenant_id", item)
		}
		orgID := strings.TrimSpace(pair[0])
		tenantID := strings.TrimSpace(pair[1])
		if orgID == "" || tenantID == "" {
			return nil, fmt.Errorf("invalid LAGO_ORG_TENANT_MAP entry %q: organization_id and tenant_id are required", item)
		}
		out[orgID] = tenantID
	}
	return out, nil
}

type LagoWebhookService struct {
	repo         store.Repository
	verifier     LagoWebhookVerifier
	tenantMapper LagoOrganizationTenantMapper
}

func NewLagoWebhookService(repo store.Repository, verifier LagoWebhookVerifier, tenantMapper LagoOrganizationTenantMapper) *LagoWebhookService {
	if verifier == nil {
		verifier = NoopLagoWebhookVerifier{}
	}
	if tenantMapper == nil {
		tenantMapper = NewStaticLagoOrganizationTenantMapper(defaultTenantID, nil)
	}
	return &LagoWebhookService{
		repo:         repo,
		verifier:     verifier,
		tenantMapper: tenantMapper,
	}
}

type IngestLagoWebhookResult struct {
	Event      domain.LagoWebhookEvent `json:"event"`
	Idempotent bool                    `json:"idempotent"`
}

func (s *LagoWebhookService) Ingest(ctx context.Context, headers http.Header, body []byte) (IngestLagoWebhookResult, error) {
	if s == nil || s.repo == nil {
		return IngestLagoWebhookResult{}, fmt.Errorf("%w: lago webhook service is not configured", ErrValidation)
	}
	if err := s.verifier.Verify(ctx, headers, body); err != nil {
		return IngestLagoWebhookResult{}, err
	}

	event, err := parseLagoWebhookEnvelope(body)
	if err != nil {
		return IngestLagoWebhookResult{}, err
	}
	event.WebhookKey = strings.TrimSpace(headers.Get("X-Lago-Unique-Key"))
	event.TenantID = s.tenantMapper.TenantIDForOrganization(event.OrganizationID)
	if event.WebhookKey == "" {
		event.WebhookKey = buildWebhookKey(event)
	}

	stored, created, err := s.repo.IngestLagoWebhookEvent(event)
	if err != nil {
		return IngestLagoWebhookResult{}, err
	}
	return IngestLagoWebhookResult{
		Event:      stored,
		Idempotent: !created,
	}, nil
}

type ListInvoicePaymentStatusViewsRequest struct {
	OrganizationID string
	PaymentStatus  string
	InvoiceStatus  string
	PaymentOverdue *bool
	SortBy         string
	Order          string
	Limit          int
	Offset         int
}

func (s *LagoWebhookService) ListInvoicePaymentStatusViews(tenantID string, req ListInvoicePaymentStatusViewsRequest) ([]domain.InvoicePaymentStatusView, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("%w: lago webhook service is not configured", ErrValidation)
	}
	limit, offset, err := validateWebhookListWindow(req.Limit, req.Offset)
	if err != nil {
		return nil, err
	}
	sortBy, err := normalizeInvoicePaymentStatusSortBy(req.SortBy)
	if err != nil {
		return nil, err
	}
	sortDesc, err := normalizeWebhookListOrder(req.Order)
	if err != nil {
		return nil, err
	}
	return s.repo.ListInvoicePaymentStatusViews(store.InvoicePaymentStatusListFilter{
		TenantID:       normalizeTenantID(tenantID),
		OrganizationID: strings.TrimSpace(req.OrganizationID),
		PaymentStatus:  strings.TrimSpace(req.PaymentStatus),
		InvoiceStatus:  strings.TrimSpace(req.InvoiceStatus),
		PaymentOverdue: req.PaymentOverdue,
		SortBy:         sortBy,
		SortDesc:       sortDesc,
		Limit:          limit,
		Offset:         offset,
	})
}

func (s *LagoWebhookService) GetInvoicePaymentStatusView(tenantID, invoiceID string) (domain.InvoicePaymentStatusView, error) {
	if s == nil || s.repo == nil {
		return domain.InvoicePaymentStatusView{}, fmt.Errorf("%w: lago webhook service is not configured", ErrValidation)
	}
	return s.repo.GetInvoicePaymentStatusView(normalizeTenantID(tenantID), strings.TrimSpace(invoiceID))
}

type ListLagoWebhookEventsRequest struct {
	OrganizationID string
	InvoiceID      string
	WebhookType    string
	SortBy         string
	Order          string
	Limit          int
	Offset         int
}

func (s *LagoWebhookService) ListLagoWebhookEvents(tenantID string, req ListLagoWebhookEventsRequest) ([]domain.LagoWebhookEvent, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("%w: lago webhook service is not configured", ErrValidation)
	}
	limit, offset, err := validateWebhookListWindow(req.Limit, req.Offset)
	if err != nil {
		return nil, err
	}
	sortBy, err := normalizeLagoWebhookEventSortBy(req.SortBy)
	if err != nil {
		return nil, err
	}
	sortDesc, err := normalizeWebhookListOrder(req.Order)
	if err != nil {
		return nil, err
	}
	return s.repo.ListLagoWebhookEvents(store.LagoWebhookEventListFilter{
		TenantID:       normalizeTenantID(tenantID),
		OrganizationID: strings.TrimSpace(req.OrganizationID),
		InvoiceID:      strings.TrimSpace(req.InvoiceID),
		WebhookType:    strings.TrimSpace(req.WebhookType),
		SortBy:         sortBy,
		SortDesc:       sortDesc,
		Limit:          limit,
		Offset:         offset,
	})
}

func validateWebhookListWindow(limit, offset int) (int, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > maxWebhookListLimit {
		return 0, 0, fmt.Errorf("%w: limit must be between 1 and %d", ErrValidation, maxWebhookListLimit)
	}
	if offset < 0 {
		return 0, 0, fmt.Errorf("%w: offset must be >= 0", ErrValidation)
	}
	return limit, offset, nil
}

func normalizeWebhookListOrder(raw string) (bool, error) {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" || v == "desc" {
		return true, nil
	}
	if v == "asc" {
		return false, nil
	}
	return false, fmt.Errorf("%w: order must be asc or desc", ErrValidation)
}

func normalizeInvoicePaymentStatusSortBy(raw string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" {
		return "last_event_at", nil
	}
	switch v {
	case "last_event_at", "updated_at", "total_due_amount_cents", "total_amount_cents":
		return v, nil
	default:
		return "", fmt.Errorf("%w: sort_by must be one of last_event_at, updated_at, total_due_amount_cents, total_amount_cents", ErrValidation)
	}
}

func normalizeLagoWebhookEventSortBy(raw string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" {
		return "received_at", nil
	}
	switch v {
	case "received_at", "occurred_at":
		return v, nil
	default:
		return "", fmt.Errorf("%w: sort_by must be one of received_at, occurred_at", ErrValidation)
	}
}

func buildWebhookKey(event domain.LagoWebhookEvent) string {
	base := strings.Join([]string{
		strings.TrimSpace(event.OrganizationID),
		strings.TrimSpace(event.WebhookType),
		strings.TrimSpace(event.ObjectType),
		strings.TrimSpace(event.InvoiceID),
		strings.TrimSpace(event.PaymentRequestID),
		strings.TrimSpace(event.DunningCampaignCode),
		strconv.FormatInt(event.OccurredAt.UnixNano(), 10),
	}, ":")
	if strings.Trim(base, ":") == "" {
		return fmt.Sprintf("generated:%d", time.Now().UTC().UnixNano())
	}
	return base
}

func parseLagoWebhookEnvelope(body []byte) (domain.LagoWebhookEvent, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return domain.LagoWebhookEvent{}, fmt.Errorf("%w: invalid webhook payload", ErrValidation)
	}
	var envelope struct {
		WebhookType    string `json:"webhook_type"`
		ObjectType     string `json:"object_type"`
		OrganizationID string `json:"organization_id"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return domain.LagoWebhookEvent{}, fmt.Errorf("%w: invalid webhook envelope", ErrValidation)
	}
	envelope.WebhookType = strings.TrimSpace(envelope.WebhookType)
	envelope.ObjectType = strings.TrimSpace(envelope.ObjectType)
	envelope.OrganizationID = strings.TrimSpace(envelope.OrganizationID)
	if envelope.WebhookType == "" || envelope.ObjectType == "" || envelope.OrganizationID == "" {
		return domain.LagoWebhookEvent{}, fmt.Errorf("%w: webhook_type, object_type, and organization_id are required", ErrValidation)
	}

	payload := map[string]any{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return domain.LagoWebhookEvent{}, fmt.Errorf("%w: invalid webhook payload", ErrValidation)
	}
	objectPayload := map[string]any{}
	if objectRaw, ok := raw[envelope.ObjectType]; ok && len(objectRaw) > 0 {
		_ = json.Unmarshal(objectRaw, &objectPayload)
	}

	event := domain.LagoWebhookEvent{
		OrganizationID: envelope.OrganizationID,
		WebhookType:    envelope.WebhookType,
		ObjectType:     envelope.ObjectType,
		Payload:        payload,
		ReceivedAt:     time.Now().UTC(),
		OccurredAt:     time.Now().UTC(),
	}
	populateLagoWebhookDerivedFields(&event, objectPayload)
	return event, nil
}

func populateLagoWebhookDerivedFields(event *domain.LagoWebhookEvent, objectPayload map[string]any) {
	if event == nil {
		return
	}

	switch event.WebhookType {
	case "invoice.payment_status_updated", "invoice.payment_overdue":
		event.InvoiceID = stringValue(objectPayload["lago_id"])
		event.InvoiceStatus = stringValue(objectPayload["status"])
		event.PaymentStatus = stringValue(objectPayload["payment_status"])
		event.PaymentOverdue = boolPtr(objectPayload["payment_overdue"])
		event.InvoiceNumber = stringValue(objectPayload["number"])
		event.Currency = stringValue(objectPayload["currency"])
		event.TotalAmountCents = int64Ptr(objectPayload["total_amount_cents"])
		event.TotalDueAmountCents = int64Ptr(objectPayload["total_due_amount_cents"])
		event.TotalPaidAmountCents = int64Ptr(objectPayload["total_paid_amount_cents"])
		if customer, ok := objectPayload["customer"].(map[string]any); ok {
			event.CustomerExternalID = stringValue(customer["external_id"])
		}
		event.OccurredAt = firstTimestamp(objectPayload["updated_at"], objectPayload["created_at"], event.ReceivedAt)

	case "invoice.payment_failure":
		event.InvoiceID = stringValue(objectPayload["lago_invoice_id"])
		event.CustomerExternalID = stringValue(objectPayload["external_customer_id"])
		event.PaymentStatus = "failed"
		event.LastPaymentError = stringValue(objectPayload["provider_error"])
		event.OccurredAt = event.ReceivedAt

	case "payment_request.payment_status_updated":
		event.PaymentRequestID = stringValue(objectPayload["lago_id"])
		event.PaymentStatus = stringValue(objectPayload["payment_status"])
		if invoices, ok := objectPayload["invoices"].([]any); ok && len(invoices) > 0 {
			if inv, ok := invoices[0].(map[string]any); ok {
				event.InvoiceID = stringValue(inv["lago_id"])
				event.InvoiceStatus = stringValue(inv["status"])
				if event.Currency == "" {
					event.Currency = stringValue(inv["currency"])
				}
				if event.PaymentStatus == "" {
					event.PaymentStatus = stringValue(inv["payment_status"])
				}
			}
		}
		if customer, ok := objectPayload["customer"].(map[string]any); ok {
			event.CustomerExternalID = stringValue(customer["external_id"])
		}
		event.OccurredAt = firstTimestamp(objectPayload["created_at"], nil, event.ReceivedAt)

	case "dunning_campaign.finished":
		event.DunningCampaignCode = stringValue(objectPayload["dunning_campaign_code"])
		event.CustomerExternalID = stringValue(objectPayload["external_customer_id"])
		if event.CustomerExternalID == "" {
			event.CustomerExternalID = stringValue(objectPayload["customer_external_id"])
		}
		event.OccurredAt = event.ReceivedAt

	default:
		event.OccurredAt = event.ReceivedAt
	}
}

func stringValue(v any) string {
	switch typed := v.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return ""
	}
}

func boolPtr(v any) *bool {
	switch typed := v.(type) {
	case bool:
		out := typed
		return &out
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(typed))
		if err != nil {
			return nil
		}
		out := parsed
		return &out
	default:
		return nil
	}
}

func int64Ptr(v any) *int64 {
	switch typed := v.(type) {
	case int64:
		out := typed
		return &out
	case int:
		out := int64(typed)
		return &out
	case float64:
		out := int64(typed)
		return &out
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return nil
		}
		out := parsed
		return &out
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err != nil {
			return nil
		}
		out := parsed
		return &out
	default:
		return nil
	}
}

func firstTimestamp(values ...any) time.Time {
	for _, raw := range values {
		switch typed := raw.(type) {
		case string:
			typed = strings.TrimSpace(typed)
			if typed == "" {
				continue
			}
			if ts, err := time.Parse(time.RFC3339, typed); err == nil {
				return ts.UTC()
			}
			if ts, err := time.Parse(time.RFC3339Nano, typed); err == nil {
				return ts.UTC()
			}
		case time.Time:
			return typed.UTC()
		}
	}
	return time.Now().UTC()
}
