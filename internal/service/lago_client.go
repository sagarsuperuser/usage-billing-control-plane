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

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
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

type PlanSyncComponent struct {
	Meter             domain.Meter
	RatingRuleVersion domain.RatingRuleVersion
}

type PlanSyncAdapter interface {
	SyncPlan(ctx context.Context, plan domain.Plan, components []PlanSyncComponent) error
}

type SubscriptionSyncAdapter interface {
	SyncSubscription(ctx context.Context, subscription domain.Subscription, customer domain.Customer, plan domain.Plan) error
}

type UsageSyncAdapter interface {
	SyncUsageEvent(ctx context.Context, event domain.UsageEvent, meter domain.Meter, subscription domain.Subscription) error
}

type InvoiceBillingAdapter interface {
	ListInvoices(ctx context.Context, query url.Values) (int, []byte, error)
	ListPaymentReceipts(ctx context.Context, query url.Values) (int, []byte, error)
	ListCreditNotes(ctx context.Context, query url.Values) (int, []byte, error)
	PreviewInvoice(ctx context.Context, payload []byte) (int, []byte, error)
	RetryInvoicePayment(ctx context.Context, invoiceID string, payload []byte) (int, []byte, error)
	GetInvoice(ctx context.Context, invoiceID string) (int, []byte, error)
	ResendInvoiceEmail(ctx context.Context, invoiceID string, input BillingDocumentEmail) error
	ResendPaymentReceiptEmail(ctx context.Context, paymentReceiptID string, input BillingDocumentEmail) error
	ResendCreditNoteEmail(ctx context.Context, creditNoteID string, input BillingDocumentEmail) error
}

type CustomerBillingAdapter interface {
	UpsertCustomer(ctx context.Context, payload []byte) (int, []byte, error)
	GetCustomer(ctx context.Context, externalID string) (int, []byte, error)
	ListCustomerPaymentMethods(ctx context.Context, externalID string) (int, []byte, error)
	GenerateCustomerCheckoutURL(ctx context.Context, externalID string) (int, []byte, error)
}

type BillingEntitySettingsSyncAdapter interface {
	SyncBillingEntitySettings(ctx context.Context, settings domain.WorkspaceBillingSettings) error
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
	return a.listCollection(ctx, "/api/v1/invoices", query, `{"invoices":[]}`)
}

func (a *LagoInvoiceAdapter) ListPaymentReceipts(ctx context.Context, query url.Values) (int, []byte, error) {
	return a.listCollection(ctx, "/api/v1/payment_receipts", query, `{"payment_receipts":[]}`)
}

func (a *LagoInvoiceAdapter) ListCreditNotes(ctx context.Context, query url.Values) (int, []byte, error) {
	return a.listCollection(ctx, "/api/v1/credit_notes", query, `{"credit_notes":[]}`)
}

func (a *LagoInvoiceAdapter) listCollection(ctx context.Context, basePath string, query url.Values, emptyResponse string) (int, []byte, error) {
	if a == nil || a.transport == nil {
		return 0, nil, fmt.Errorf("%w: lago invoice adapter is required", ErrValidation)
	}

	path := basePath
	if encoded := strings.TrimSpace(query.Encode()); encoded != "" {
		path += "?" + encoded
	}

	status, body, err := a.transport.doRawRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return 0, nil, err
	}
	if len(body) == 0 {
		body = []byte(emptyResponse)
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

func (a *LagoInvoiceAdapter) ResendPaymentReceiptEmail(ctx context.Context, paymentReceiptID string, input BillingDocumentEmail) error {
	return a.resendBillingDocumentEmail(ctx, "/api/v1/payment_receipts/", paymentReceiptID, input)
}

func (a *LagoInvoiceAdapter) ResendCreditNoteEmail(ctx context.Context, creditNoteID string, input BillingDocumentEmail) error {
	return a.resendBillingDocumentEmail(ctx, "/api/v1/credit_notes/", creditNoteID, input)
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

func (a *LagoCustomerBillingAdapter) SyncBillingEntitySettings(ctx context.Context, settings domain.WorkspaceBillingSettings) error {
	if a == nil || a.transport == nil {
		return fmt.Errorf("%w: lago customer billing adapter is required", ErrValidation)
	}
	code := strings.TrimSpace(settings.BillingEntityCode)
	if code == "" {
		return fmt.Errorf("%w: billing entity code is required", ErrValidation)
	}

	billingEntity := map[string]any{}
	if settings.NetPaymentTermDays != nil {
		billingEntity["net_payment_term"] = *settings.NetPaymentTermDays
	}
	billingEntity["billing_configuration"] = map[string]any{
		"invoice_footer": settings.InvoiceFooter,
	}

	payload := map[string]any{"billing_entity": billingEntity}
	path := "/api/v1/billing_entities/" + url.PathEscape(code)
	status, body, err := a.transport.doJSONRequest(ctx, http.MethodPut, path, payload)
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: lago billing entity sync failed (status=%d body=%s)", ErrDependency, status, abbrevForLog(body))
}

type LagoPlanSyncAdapter struct {
	transport *LagoHTTPTransport
	store     store.Repository
}

func NewLagoPlanSyncAdapter(transport *LagoHTTPTransport, repo store.Repository) *LagoPlanSyncAdapter {
	return &LagoPlanSyncAdapter{transport: transport, store: repo}
}

func (a *LagoPlanSyncAdapter) SyncPlan(ctx context.Context, plan domain.Plan, components []PlanSyncComponent) error {
	if a == nil || a.transport == nil {
		return fmt.Errorf("%w: lago plan sync adapter is required", ErrValidation)
	}

	charges := make([]map[string]any, 0, len(components))
	for _, component := range components {
		billableMetricID, err := a.lookupBillableMetricID(ctx, component.Meter.Key)
		if err != nil {
			return err
		}
		chargeModel, properties, err := mapPricingRuleToLago(component.RatingRuleVersion)
		if err != nil {
			return err
		}
		if !strings.EqualFold(strings.TrimSpace(component.RatingRuleVersion.Currency), strings.TrimSpace(plan.Currency)) {
			return fmt.Errorf("%w: plan %s currency %s does not match meter rule currency %s", ErrDependency, plan.Code, plan.Currency, component.RatingRuleVersion.Currency)
		}
		charges = append(charges, map[string]any{
			"code":                 lagoChargeCode(plan.Code, component.Meter.Key),
			"invoice_display_name": component.Meter.Name,
			"billable_metric_id":   billableMetricID,
			"charge_model":         chargeModel,
			"pay_in_advance":       false,
			"properties":           properties,
		})
	}

	payload := map[string]any{
		"plan": map[string]any{
			"name":                 plan.Name,
			"invoice_display_name": plan.Name,
			"code":                 plan.Code,
			"interval":             string(plan.BillingInterval),
			"description":          plan.Description,
			"pay_in_advance":       false,
			"amount_cents":         plan.BaseAmountCents,
			"amount_currency":      plan.Currency,
			"charges":              charges,
		},
	}

	createStatus, createBody, err := a.transport.doJSONRequest(ctx, http.MethodPost, "/api/v1/plans", payload)
	if err == nil {
		return a.syncPlanCommercials(ctx, plan)
	}

	updatePath := "/api/v1/plans/" + url.PathEscape(strings.TrimSpace(plan.Code))
	updateStatus, updateBody, updateErr := a.transport.doJSONRequest(ctx, http.MethodPut, updatePath, payload)
	if updateErr == nil {
		return a.syncPlanCommercials(ctx, plan)
	}
	return fmt.Errorf("%w: lago plan sync failed (create_status=%d create_body=%s update_status=%d update_body=%s)",
		ErrDependency,
		createStatus,
		abbrevForLog(createBody),
		updateStatus,
		abbrevForLog(updateBody),
	)
}

func (a *LagoPlanSyncAdapter) syncPlanCommercials(ctx context.Context, plan domain.Plan) error {
	if a == nil || a.transport == nil || a.store == nil {
		return nil
	}

	addOns, err := a.loadPlanAddOns(plan)
	if err != nil {
		return err
	}
	for _, addOn := range addOns {
		if err := a.upsertAddOn(ctx, addOn); err != nil {
			return err
		}
	}
	if err := a.reconcilePlanFixedCharges(ctx, plan, addOns); err != nil {
		return err
	}

	coupons, err := a.loadPlanCoupons(plan)
	if err != nil {
		return err
	}
	for _, coupon := range coupons {
		if err := a.upsertCoupon(ctx, plan.TenantID, coupon); err != nil {
			return err
		}
	}
	return nil
}

func (a *LagoPlanSyncAdapter) lookupBillableMetricID(ctx context.Context, code string) (string, error) {
	path := "/api/v1/billable_metrics/" + url.PathEscape(strings.TrimSpace(code))
	status, body, err := a.transport.doRawRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return "", err
	}
	var payload struct {
		BillableMetric struct {
			LagoID string `json:"lago_id"`
		} `json:"billable_metric"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("%w: decode lago billable metric response", ErrDependency)
	}
	if id := strings.TrimSpace(payload.BillableMetric.LagoID); id != "" {
		return id, nil
	}
	return "", fmt.Errorf("%w: lago billable metric lookup failed for %s (status=%d body=%s)", ErrDependency, code, status, abbrevForLog(body))
}

type LagoSubscriptionSyncAdapter struct {
	transport *LagoHTTPTransport
	store     store.Repository
}

func NewLagoSubscriptionSyncAdapter(transport *LagoHTTPTransport, repo store.Repository) *LagoSubscriptionSyncAdapter {
	return &LagoSubscriptionSyncAdapter{transport: transport, store: repo}
}

func (a *LagoSubscriptionSyncAdapter) SyncSubscription(ctx context.Context, subscription domain.Subscription, customer domain.Customer, plan domain.Plan) error {
	if a == nil || a.transport == nil {
		return fmt.Errorf("%w: lago subscription sync adapter is required", ErrValidation)
	}

	externalID := strings.TrimSpace(subscription.Code)
	if externalID == "" {
		return fmt.Errorf("%w: subscription code is required", ErrValidation)
	}

	if subscription.Status == domain.SubscriptionStatusArchived {
		terminatePath := "/api/v1/subscriptions/" + url.PathEscape(externalID)
		terminateStatus, terminateBody, terminateErr := a.transport.doRawRequest(ctx, http.MethodDelete, terminatePath, nil)
		if terminateErr == nil || terminateStatus == http.StatusNotFound {
			return a.syncAppliedCoupons(ctx, subscription, customer)
		}
		return fmt.Errorf("%w: lago subscription terminate failed (status=%d body=%s)", ErrDependency, terminateStatus, abbrevForLog(terminateBody))
	}

	createPayload := map[string]any{
		"subscription": map[string]any{
			"external_customer_id": customer.ExternalID,
			"plan_code":            plan.Code,
			"name":                 subscription.DisplayName,
			"external_id":          externalID,
			"billing_time":         subscription.BillingTime,
		},
	}
	if subscription.StartedAt != nil && !subscription.StartedAt.IsZero() {
		createPayload["subscription"].(map[string]any)["subscription_at"] = subscription.StartedAt.UTC().Format(time.RFC3339)
	}
	createStatus, createBody, createErr := a.transport.doJSONRequest(ctx, http.MethodPost, "/api/v1/subscriptions", createPayload)
	if createErr == nil {
		return a.syncAppliedCoupons(ctx, subscription, customer)
	}

	updatePayload := map[string]any{
		"subscription": map[string]any{
			"name": subscription.DisplayName,
		},
	}
	if subscription.StartedAt != nil && !subscription.StartedAt.IsZero() {
		updatePayload["subscription"].(map[string]any)["subscription_at"] = subscription.StartedAt.UTC().Format(time.RFC3339)
	}
	updatePath := "/api/v1/subscriptions/" + url.PathEscape(externalID)
	updateStatus, updateBody, updateErr := a.transport.doJSONRequest(ctx, http.MethodPut, updatePath, updatePayload)
	if updateErr == nil {
		return a.syncAppliedCoupons(ctx, subscription, customer)
	}

	return fmt.Errorf("%w: lago subscription sync failed (create_status=%d create_body=%s update_status=%d update_body=%s)",
		ErrDependency,
		createStatus,
		abbrevForLog(createBody),
		updateStatus,
		abbrevForLog(updateBody),
	)
}

type lagoFixedChargesResponse struct {
	FixedCharges []lagoFixedCharge `json:"fixed_charges"`
}

type lagoFixedCharge struct {
	LagoID    string `json:"lago_id"`
	Code      string `json:"code"`
	AddOnCode string `json:"add_on_code"`
}

type lagoAppliedCouponsResponse struct {
	AppliedCoupons []lagoAppliedCoupon `json:"applied_coupons"`
}

type lagoAppliedCoupon struct {
	LagoID             string `json:"lago_id"`
	CouponCode         string `json:"coupon_code"`
	ExternalCustomerID string `json:"external_customer_id"`
	Status             string `json:"status"`
}

func (a *LagoPlanSyncAdapter) loadPlanAddOns(plan domain.Plan) ([]domain.AddOn, error) {
	out := make([]domain.AddOn, 0, len(plan.AddOnIDs))
	for _, addOnID := range plan.AddOnIDs {
		addOn, err := a.store.GetAddOn(plan.TenantID, addOnID)
		if err != nil {
			return nil, err
		}
		if addOn.Status != domain.AddOnStatusActive {
			continue
		}
		out = append(out, addOn)
	}
	return out, nil
}

func (a *LagoPlanSyncAdapter) loadPlanCoupons(plan domain.Plan) ([]domain.Coupon, error) {
	out := make([]domain.Coupon, 0, len(plan.CouponIDs))
	for _, couponID := range plan.CouponIDs {
		coupon, err := a.store.GetCoupon(plan.TenantID, couponID)
		if err != nil {
			return nil, err
		}
		if coupon.Status != domain.CouponStatusActive {
			continue
		}
		out = append(out, coupon)
	}
	return out, nil
}

func (a *LagoPlanSyncAdapter) upsertAddOn(ctx context.Context, addOn domain.AddOn) error {
	payload := map[string]any{
		"add_on": map[string]any{
			"name":                 addOn.Name,
			"invoice_display_name": addOn.Name,
			"code":                 addOn.Code,
			"amount_cents":         addOn.AmountCents,
			"amount_currency":      addOn.Currency,
			"description":          addOn.Description,
		},
	}
	createStatus, createBody, err := a.transport.doJSONRequest(ctx, http.MethodPost, "/api/v1/add_ons", payload)
	if err == nil {
		return nil
	}
	updatePath := "/api/v1/add_ons/" + url.PathEscape(strings.TrimSpace(addOn.Code))
	updateStatus, updateBody, updateErr := a.transport.doJSONRequest(ctx, http.MethodPut, updatePath, payload)
	if updateErr == nil {
		return nil
	}
	return fmt.Errorf("%w: lago add-on sync failed (create_status=%d create_body=%s update_status=%d update_body=%s)", ErrDependency, createStatus, abbrevForLog(createBody), updateStatus, abbrevForLog(updateBody))
}

func (a *LagoPlanSyncAdapter) upsertCoupon(ctx context.Context, tenantID string, coupon domain.Coupon) error {
	planCodes, err := a.couponPlanCodes(tenantID, coupon.ID)
	if err != nil {
		return err
	}
	payload := map[string]any{
		"coupon": map[string]any{
			"name":        coupon.Name,
			"code":        coupon.Code,
			"description": coupon.Description,
			"frequency":   "forever",
			"reusable":    true,
			"applies_to": map[string]any{
				"plan_codes": planCodes,
			},
		},
	}
	if coupon.DiscountType == domain.CouponDiscountTypePercentOff {
		payload["coupon"].(map[string]any)["coupon_type"] = "percentage"
		payload["coupon"].(map[string]any)["percentage_rate"] = coupon.PercentOff
	} else {
		payload["coupon"].(map[string]any)["coupon_type"] = "fixed_amount"
		payload["coupon"].(map[string]any)["amount_cents"] = coupon.AmountOffCents
		payload["coupon"].(map[string]any)["amount_currency"] = coupon.Currency
	}
	createStatus, createBody, err := a.transport.doJSONRequest(ctx, http.MethodPost, "/api/v1/coupons", payload)
	if err == nil {
		return nil
	}
	updatePath := "/api/v1/coupons/" + url.PathEscape(strings.TrimSpace(coupon.Code))
	updateStatus, updateBody, updateErr := a.transport.doJSONRequest(ctx, http.MethodPut, updatePath, payload)
	if updateErr == nil {
		return nil
	}
	return fmt.Errorf("%w: lago coupon sync failed (create_status=%d create_body=%s update_status=%d update_body=%s)", ErrDependency, createStatus, abbrevForLog(createBody), updateStatus, abbrevForLog(updateBody))
}

func (a *LagoPlanSyncAdapter) couponPlanCodes(tenantID, couponID string) ([]string, error) {
	plans, err := a.store.ListPlans(tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, 4)
	for _, plan := range plans {
		for _, attachedCouponID := range plan.CouponIDs {
			if attachedCouponID == couponID {
				out = append(out, plan.Code)
				break
			}
		}
	}
	return out, nil
}

func (a *LagoPlanSyncAdapter) reconcilePlanFixedCharges(ctx context.Context, plan domain.Plan, addOns []domain.AddOn) error {
	status, body, err := a.transport.doRawRequest(ctx, http.MethodGet, "/api/v1/plans/"+url.PathEscape(strings.TrimSpace(plan.Code))+"/fixed_charges", nil)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("%w: list lago fixed charges failed (status=%d body=%s)", ErrDependency, status, abbrevForLog(body))
	}
	var response lagoFixedChargesResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("%w: decode lago fixed charges response", ErrDependency)
	}

	desiredCodes := make(map[string]domain.AddOn, len(addOns))
	for _, addOn := range addOns {
		code := lagoFixedChargeCode(plan.Code, addOn.Code)
		desiredCodes[code] = addOn
		payload := map[string]any{
			"fixed_charge": map[string]any{
				"add_on_code":          addOn.Code,
				"code":                 code,
				"invoice_display_name": addOn.Name,
				"charge_model":         "standard",
				"pay_in_advance":       false,
				"prorated":             false,
				"units":                "1",
				"properties": map[string]any{
					"amount": formatLagoDecimal(addOn.AmountCents),
				},
			},
		}
		createPath := "/api/v1/plans/" + url.PathEscape(strings.TrimSpace(plan.Code)) + "/fixed_charges"
		createStatus, createBody, createErr := a.transport.doJSONRequest(ctx, http.MethodPost, createPath, payload)
		if createErr == nil {
			continue
		}
		updatePath := createPath + "/" + url.PathEscape(code)
		updateStatus, updateBody, updateErr := a.transport.doJSONRequest(ctx, http.MethodPut, updatePath, payload)
		if updateErr != nil {
			return fmt.Errorf("%w: lago fixed charge sync failed (create_status=%d create_body=%s update_status=%d update_body=%s)", ErrDependency, createStatus, abbrevForLog(createBody), updateStatus, abbrevForLog(updateBody))
		}
	}

	for _, existing := range response.FixedCharges {
		if !strings.HasPrefix(existing.Code, lagoFixedChargeCodePrefix(plan.Code)) {
			continue
		}
		if _, ok := desiredCodes[existing.Code]; ok {
			continue
		}
		deletePath := "/api/v1/plans/" + url.PathEscape(strings.TrimSpace(plan.Code)) + "/fixed_charges/" + url.PathEscape(existing.Code)
		deleteStatus, deleteBody, deleteErr := a.transport.doRawRequest(ctx, http.MethodDelete, deletePath, nil)
		if deleteErr != nil {
			return deleteErr
		}
		if deleteStatus < 200 || deleteStatus >= 300 {
			return fmt.Errorf("%w: delete lago fixed charge failed (status=%d body=%s)", ErrDependency, deleteStatus, abbrevForLog(deleteBody))
		}
	}
	return nil
}

func (a *LagoSubscriptionSyncAdapter) syncAppliedCoupons(ctx context.Context, subscription domain.Subscription, customer domain.Customer) error {
	if a == nil || a.transport == nil || a.store == nil {
		return nil
	}
	desiredCoupons, err := a.desiredCustomerCoupons(subscription)
	if err != nil {
		return err
	}
	for _, coupon := range desiredCoupons {
		if err := (&LagoPlanSyncAdapter{transport: a.transport, store: a.store}).upsertCoupon(ctx, subscription.TenantID, coupon); err != nil {
			return err
		}
	}

	appliedCoupons, err := a.listAppliedCoupons(ctx, customer.ExternalID)
	if err != nil {
		return err
	}
	desiredByCode := make(map[string]domain.Coupon, len(desiredCoupons))
	for _, coupon := range desiredCoupons {
		desiredByCode[coupon.Code] = coupon
	}

	alphaCouponCodes, err := a.allAlphaCouponCodes(subscription.TenantID)
	if err != nil {
		return err
	}

	existingActive := map[string]lagoAppliedCoupon{}
	for _, item := range appliedCoupons {
		if strings.EqualFold(strings.TrimSpace(item.Status), "active") {
			existingActive[item.CouponCode] = item
		}
	}
	for code := range desiredByCode {
		if _, ok := existingActive[code]; ok {
			continue
		}
		payload := map[string]any{
			"applied_coupon": map[string]any{
				"external_customer_id": customer.ExternalID,
				"coupon_code":          code,
			},
		}
		status, body, reqErr := a.transport.doJSONRequest(ctx, http.MethodPost, "/api/v1/applied_coupons", payload)
		if reqErr != nil {
			return fmt.Errorf("%w: apply lago coupon failed (status=%d body=%s)", ErrDependency, status, abbrevForLog(body))
		}
	}

	for code, item := range existingActive {
		if _, isAlpha := alphaCouponCodes[code]; !isAlpha {
			continue
		}
		if _, keep := desiredByCode[code]; keep {
			continue
		}
		deletePath := "/api/v1/customers/" + url.PathEscape(customer.ExternalID) + "/applied_coupons/" + url.PathEscape(item.LagoID)
		deleteStatus, deleteBody, deleteErr := a.transport.doRawRequest(ctx, http.MethodDelete, deletePath, nil)
		if deleteErr != nil {
			return deleteErr
		}
		if deleteStatus < 200 || deleteStatus >= 300 {
			return fmt.Errorf("%w: delete lago applied coupon failed (status=%d body=%s)", ErrDependency, deleteStatus, abbrevForLog(deleteBody))
		}
	}
	return nil
}

func (a *LagoSubscriptionSyncAdapter) desiredCustomerCoupons(subscription domain.Subscription) ([]domain.Coupon, error) {
	items, err := a.store.ListSubscriptions(subscription.TenantID)
	if err != nil {
		return nil, err
	}
	seen := map[string]domain.Coupon{}
	for _, item := range items {
		if item.CustomerID != subscription.CustomerID || item.Status == domain.SubscriptionStatusArchived {
			continue
		}
		plan, err := a.store.GetPlan(subscription.TenantID, item.PlanID)
		if err != nil {
			return nil, err
		}
		for _, couponID := range plan.CouponIDs {
			coupon, err := a.store.GetCoupon(subscription.TenantID, couponID)
			if err != nil {
				return nil, err
			}
			if coupon.Status != domain.CouponStatusActive {
				continue
			}
			seen[coupon.Code] = coupon
		}
	}
	out := make([]domain.Coupon, 0, len(seen))
	for _, coupon := range seen {
		out = append(out, coupon)
	}
	return out, nil
}

func (a *LagoSubscriptionSyncAdapter) listAppliedCoupons(ctx context.Context, externalCustomerID string) ([]lagoAppliedCoupon, error) {
	query := url.Values{}
	query.Set("external_customer_id", externalCustomerID)
	path := "/api/v1/applied_coupons?" + query.Encode()
	status, body, err := a.transport.doRawRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("%w: list lago applied coupons failed (status=%d body=%s)", ErrDependency, status, abbrevForLog(body))
	}
	var response lagoAppliedCouponsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("%w: decode lago applied coupons response", ErrDependency)
	}
	return response.AppliedCoupons, nil
}

func (a *LagoSubscriptionSyncAdapter) allAlphaCouponCodes(tenantID string) (map[string]struct{}, error) {
	items, err := a.store.ListCoupons(tenantID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(items))
	for _, item := range items {
		out[item.Code] = struct{}{}
	}
	return out, nil
}

type LagoUsageSyncAdapter struct {
	transport *LagoHTTPTransport
}

func NewLagoUsageSyncAdapter(transport *LagoHTTPTransport) *LagoUsageSyncAdapter {
	return &LagoUsageSyncAdapter{transport: transport}
}

func (a *LagoUsageSyncAdapter) SyncUsageEvent(ctx context.Context, event domain.UsageEvent, meter domain.Meter, subscription domain.Subscription) error {
	if a == nil || a.transport == nil {
		return fmt.Errorf("%w: lago usage sync adapter is required", ErrValidation)
	}
	aggregationType, err := mapAggregationToLago(meter.Aggregation)
	if err != nil {
		return err
	}
	transactionID := strings.TrimSpace(event.IdempotencyKey)
	if transactionID == "" {
		transactionID = strings.TrimSpace(event.ID)
	}
	payload := map[string]any{
		"event": map[string]any{
			"code":                     meter.Key,
			"transaction_id":           transactionID,
			"external_subscription_id": subscription.Code,
			"timestamp":                event.Timestamp.Unix(),
			"properties":               map[string]any{},
		},
	}
	switch aggregationType {
	case "sum_agg", "max_agg":
		payload["event"].(map[string]any)["properties"] = map[string]any{
			"value": fmt.Sprintf("%d", event.Quantity),
		}
	case "count_agg":
		if event.Quantity != 1 {
			return fmt.Errorf("%w: count aggregation requires quantity=1 for lago usage sync", ErrDependency)
		}
	default:
		return fmt.Errorf("%w: unsupported aggregation %q for lago usage sync", ErrDependency, aggregationType)
	}
	status, body, err := a.transport.doJSONRequest(ctx, http.MethodPost, "/api/v1/events", payload)
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: lago usage sync failed (status=%d body=%s)", ErrDependency, status, abbrevForLog(body))
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

func (a *LagoInvoiceAdapter) resendBillingDocumentEmail(ctx context.Context, basePath, resourceID string, input BillingDocumentEmail) error {
	if a == nil || a.transport == nil {
		return fmt.Errorf("%w: lago invoice adapter is required", ErrValidation)
	}
	resourceID = strings.TrimSpace(resourceID)
	if resourceID == "" {
		return fmt.Errorf("%w: resource id is required", ErrValidation)
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

	path := strings.TrimRight(basePath, "/") + "/" + url.PathEscape(resourceID) + "/resend_email"
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

func lagoFixedChargeCode(planCode, addOnCode string) string {
	return lagoFixedChargeCodePrefix(planCode) + strings.TrimSpace(addOnCode)
}

func lagoFixedChargeCodePrefix(planCode string) string {
	return "alpha_fc_" + strings.TrimSpace(planCode) + "_"
}

func mapPricingRuleToLago(rule domain.RatingRuleVersion) (string, map[string]any, error) {
	switch rule.Mode {
	case domain.PricingModeFlat:
		return "standard", map[string]any{
			"amount": formatLagoDecimal(rule.FlatAmountCents),
		}, nil
	case domain.PricingModeGraduated:
		ranges := make([]map[string]any, 0, len(rule.GraduatedTiers))
		fromValue := int64(0)
		for _, tier := range rule.GraduatedTiers {
			value := map[string]any{
				"from_value":      fromValue,
				"per_unit_amount": formatLagoDecimal(tier.UnitAmountCents),
				"flat_amount":     "0",
			}
			if tier.UpTo > 0 {
				value["to_value"] = tier.UpTo
				fromValue = tier.UpTo + 1
			} else {
				value["to_value"] = nil
			}
			ranges = append(ranges, value)
		}
		return "graduated", map[string]any{
			"graduated_ranges": ranges,
		}, nil
	case domain.PricingModePackage:
		if rule.OverageUnitAmountCents > 0 {
			return "", nil, fmt.Errorf("%w: package pricing with overage is not yet supported for lago plan sync", ErrDependency)
		}
		return "package", map[string]any{
			"package_size": rule.PackageSize,
			"amount":       formatLagoDecimal(rule.PackageAmountCents),
			"free_units":   0,
		}, nil
	default:
		return "", nil, fmt.Errorf("%w: pricing mode %q is not supported for lago plan sync", ErrDependency, rule.Mode)
	}
}

func formatLagoDecimal(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	major := cents / 100
	minor := cents % 100
	if minor == 0 {
		return fmt.Sprintf("%s%d", sign, major)
	}
	if minor%10 == 0 {
		return fmt.Sprintf("%s%d.%d", sign, major, minor/10)
	}
	return fmt.Sprintf("%s%d.%02d", sign, major, minor)
}

func lagoChargeCode(planCode, meterKey string) string {
	return strings.Trim(strings.ToLower(strings.TrimSpace(planCode))+"_"+strings.ToLower(strings.TrimSpace(meterKey)), "_")
}

func abbrevForLog(body []byte) string {
	v := strings.TrimSpace(string(body))
	if len(v) <= 300 {
		return v
	}
	return v[:300] + "...(truncated)"
}
