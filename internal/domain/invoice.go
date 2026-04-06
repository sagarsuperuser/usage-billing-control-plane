package domain

import "time"

// ---------------------------------------------------------------------------
// Invoices (first-class, replaces Lago-hosted invoices)
// ---------------------------------------------------------------------------

type InvoiceStatus string

const (
	InvoiceStatusDraft     InvoiceStatus = "draft"
	InvoiceStatusFinalized InvoiceStatus = "finalized"
	InvoiceStatusPaid      InvoiceStatus = "paid"
	InvoiceStatusVoided    InvoiceStatus = "voided"
)

type InvoicePaymentStatus string

const (
	InvoicePaymentPending    InvoicePaymentStatus = "pending"
	InvoicePaymentProcessing InvoicePaymentStatus = "processing"
	InvoicePaymentSucceeded  InvoicePaymentStatus = "succeeded"
	InvoicePaymentFailed     InvoicePaymentStatus = "failed"
)

type Invoice struct {
	ID                     string               `json:"id"`
	TenantID               string               `json:"tenant_id,omitempty"`
	CustomerID             string               `json:"customer_id"`
	SubscriptionID         string               `json:"subscription_id"`
	InvoiceNumber          string               `json:"invoice_number"`
	Status                 InvoiceStatus         `json:"status"`
	PaymentStatus          InvoicePaymentStatus  `json:"payment_status"`
	Currency               string               `json:"currency"`
	SubtotalCents          int64                `json:"subtotal_cents"`
	DiscountCents          int64                `json:"discount_cents"`
	TaxAmountCents         int64                `json:"tax_amount_cents"`
	TotalAmountCents       int64                `json:"total_amount_cents"`
	AmountDueCents         int64                `json:"amount_due_cents"`
	AmountPaidCents        int64                `json:"amount_paid_cents"`
	BillingPeriodStart     time.Time            `json:"billing_period_start"`
	BillingPeriodEnd       time.Time            `json:"billing_period_end"`
	IssuedAt               *time.Time           `json:"issued_at,omitempty"`
	DueAt                  *time.Time           `json:"due_at,omitempty"`
	PaidAt                 *time.Time           `json:"paid_at,omitempty"`
	VoidedAt               *time.Time           `json:"voided_at,omitempty"`
	StripePaymentIntentID  string               `json:"stripe_payment_intent_id,omitempty"`
	LastPaymentError       string               `json:"last_payment_error,omitempty"`
	PaymentOverdue         bool                 `json:"payment_overdue"`
	PDFObjectKey           string               `json:"-"`
	NetPaymentTermDays     int                  `json:"net_payment_term_days"`
	Memo                   string               `json:"memo,omitempty"`
	Footer                 string               `json:"footer,omitempty"`
	Metadata               map[string]any       `json:"metadata,omitempty"`
	CreatedAt              time.Time            `json:"created_at"`
	UpdatedAt              time.Time            `json:"updated_at"`
}

type InvoiceLineItemType string

const (
	LineTypeBaseFee  InvoiceLineItemType = "base_fee"
	LineTypeUsage    InvoiceLineItemType = "usage"
	LineTypeAddOn    InvoiceLineItemType = "add_on"
	LineTypeDiscount InvoiceLineItemType = "discount"
	LineTypeTax      InvoiceLineItemType = "tax"
)

type InvoiceLineItem struct {
	ID                   string              `json:"id"`
	InvoiceID            string              `json:"invoice_id"`
	TenantID             string              `json:"tenant_id,omitempty"`
	LineType             InvoiceLineItemType `json:"line_type"`
	MeterID              string              `json:"meter_id,omitempty"`
	AddOnID              string              `json:"add_on_id,omitempty"`
	CouponID             string              `json:"coupon_id,omitempty"`
	TaxID                string              `json:"tax_id,omitempty"`
	Description          string              `json:"description"`
	Quantity             int64               `json:"quantity"`
	UnitAmountCents      int64               `json:"unit_amount_cents"`
	AmountCents          int64               `json:"amount_cents"`
	TaxRate              float64             `json:"tax_rate"`
	TaxAmountCents       int64               `json:"tax_amount_cents"`
	TotalAmountCents     int64               `json:"total_amount_cents"`
	Currency             string              `json:"currency"`
	PricingMode          string              `json:"pricing_mode,omitempty"`
	RatingRuleVersionID  string              `json:"rating_rule_version_id,omitempty"`
	BillingPeriodStart   *time.Time          `json:"billing_period_start,omitempty"`
	BillingPeriodEnd     *time.Time          `json:"billing_period_end,omitempty"`
	Metadata             map[string]any      `json:"metadata,omitempty"`
	CreatedAt            time.Time           `json:"created_at"`
}

type InvoiceSummary struct {
	InvoiceID            string     `json:"invoice_id"`
	InvoiceNumber        string     `json:"invoice_number,omitempty"`
	CustomerExternalID   string     `json:"customer_external_id,omitempty"`
	CustomerDisplayName  string     `json:"customer_display_name,omitempty"`
	OrganizationID       string     `json:"-"`
	Currency             string     `json:"currency,omitempty"`
	InvoiceStatus        string     `json:"invoice_status,omitempty"`
	PaymentStatus        string     `json:"payment_status,omitempty"`
	PaymentOverdue       *bool      `json:"payment_overdue,omitempty"`
	TotalAmountCents     *int64     `json:"total_amount_cents,omitempty"`
	TotalDueAmountCents  *int64     `json:"total_due_amount_cents,omitempty"`
	TotalPaidAmountCents *int64     `json:"total_paid_amount_cents,omitempty"`
	LastPaymentError     string     `json:"last_payment_error,omitempty"`
	LastEventType        string     `json:"last_event_type,omitempty"`
	IssuingDate          *time.Time `json:"issuing_date,omitempty"`
	PaymentDueDate       *time.Time `json:"payment_due_date,omitempty"`
	CreatedAt            *time.Time `json:"created_at,omitempty"`
	UpdatedAt            *time.Time `json:"updated_at,omitempty"`
	LastEventAt          *time.Time `json:"last_event_at,omitempty"`
}

type InvoiceSummaryList struct {
	Items   []InvoiceSummary `json:"items"`
	Limit   int              `json:"limit,omitempty"`
	Offset  int              `json:"offset,omitempty"`
	Filters map[string]any   `json:"filters,omitempty"`
}

type InvoiceDetail struct {
	InvoiceSummary
	LagoID            string          `json:"-"`
	BillingEntityCode string          `json:"billing_entity_code,omitempty"`
	SequentialID      any             `json:"sequential_id,omitempty"`
	InvoiceType       string          `json:"invoice_type,omitempty"`
	NetPaymentTerm    any             `json:"net_payment_term,omitempty"`
	FileURL           string          `json:"file_url,omitempty"`
	XMLURL            string          `json:"xml_url,omitempty"`
	VersionNumber     any             `json:"version_number,omitempty"`
	SelfBilled        *bool           `json:"self_billed,omitempty"`
	VoidedAt          *time.Time      `json:"voided_at,omitempty"`
	Customer          map[string]any  `json:"customer,omitempty"`
	Subscriptions     []any           `json:"subscriptions,omitempty"`
	Fees              []any           `json:"fees,omitempty"`
	Metadata          []any           `json:"metadata,omitempty"`
	AppliedTaxes      []any           `json:"applied_taxes,omitempty"`
	Dunning           *DunningSummary `json:"dunning,omitempty"`
	Raw               map[string]any  `json:"raw,omitempty"`
}

type PaymentReceiptSummary struct {
	ID            string     `json:"id"`
	Number        string     `json:"number,omitempty"`
	InvoiceID     string     `json:"invoice_id,omitempty"`
	PaymentID     string     `json:"payment_id,omitempty"`
	PaymentStatus string     `json:"payment_status,omitempty"`
	AmountCents   *int64     `json:"amount_cents,omitempty"`
	Currency      string     `json:"currency,omitempty"`
	FileURL       string     `json:"file_url,omitempty"`
	XMLURL        string     `json:"xml_url,omitempty"`
	CreatedAt     *time.Time `json:"created_at,omitempty"`
}

type CreditNoteSummary struct {
	ID               string     `json:"id"`
	Number           string     `json:"number,omitempty"`
	InvoiceID        string     `json:"invoice_id,omitempty"`
	InvoiceNumber    string     `json:"invoice_number,omitempty"`
	CreditStatus     string     `json:"credit_status,omitempty"`
	RefundStatus     string     `json:"refund_status,omitempty"`
	Currency         string     `json:"currency,omitempty"`
	TotalAmountCents *int64     `json:"total_amount_cents,omitempty"`
	FileURL          string     `json:"file_url,omitempty"`
	XMLURL           string     `json:"xml_url,omitempty"`
	IssuingDate      *time.Time `json:"issuing_date,omitempty"`
	CreatedAt        *time.Time `json:"created_at,omitempty"`
}

type InvoicePreviewItem struct {
	MeterID             string `json:"meter_id"`
	Quantity            int64  `json:"quantity"`
	RatingRuleVersionID string `json:"rating_rule_version_id,omitempty"`
}

type InvoicePreviewRequest struct {
	CustomerID string               `json:"customer_id"`
	TenantID   string               `json:"tenant_id,omitempty"`
	Currency   string               `json:"currency"`
	From       *time.Time           `json:"from,omitempty"`
	To         *time.Time           `json:"to,omitempty"`
	Items      []InvoicePreviewItem `json:"items"`
}

type InvoicePreviewLine struct {
	MeterID       string      `json:"meter_id"`
	Quantity      int64       `json:"quantity"`
	Mode          PricingMode `json:"mode"`
	AmountCents   int64       `json:"amount_cents"`
	RuleVersionID string      `json:"rule_version_id"`
}

type InvoicePreviewResponse struct {
	CustomerID  string               `json:"customer_id"`
	Currency    string               `json:"currency"`
	Lines       []InvoicePreviewLine `json:"lines"`
	TotalCents  int64                `json:"total_cents"`
	GeneratedAt time.Time            `json:"generated_at"`
}

type InvoiceExplainabilityLineItem struct {
	FeeID                   string         `json:"fee_id"`
	FeeType                 string         `json:"fee_type"`
	ItemName                string         `json:"item_name"`
	ItemCode                string         `json:"item_code,omitempty"`
	AmountCents             int64          `json:"amount_cents"`
	TaxesAmountCents        int64          `json:"taxes_amount_cents"`
	TotalAmountCents        int64          `json:"total_amount_cents"`
	Units                   *float64       `json:"units,omitempty"`
	EventsCount             *int64         `json:"events_count,omitempty"`
	ComputationMode         string         `json:"computation_mode"`
	ChargeModel             string         `json:"charge_model,omitempty"`
	RuleReference           string         `json:"rule_reference"`
	FromDatetime            *time.Time     `json:"from_datetime,omitempty"`
	ToDatetime              *time.Time     `json:"to_datetime,omitempty"`
	ChargeFilterDisplayName string         `json:"charge_filter_display_name,omitempty"`
	SubscriptionID          string         `json:"subscription_id,omitempty"`
	ChargeID                string         `json:"charge_id,omitempty"`
	BillableMetricCode      string         `json:"billable_metric_code,omitempty"`
	Properties              map[string]any `json:"properties"`
}

type InvoiceExplainability struct {
	InvoiceID             string                          `json:"invoice_id"`
	InvoiceNumber         string                          `json:"invoice_number"`
	InvoiceStatus         string                          `json:"invoice_status"`
	Currency              string                          `json:"currency,omitempty"`
	GeneratedAt           time.Time                       `json:"generated_at"`
	TotalAmountCents      int64                           `json:"total_amount_cents"`
	ExplainabilityVersion string                          `json:"explainability_version"`
	ExplainabilityDigest  string                          `json:"explainability_digest"`
	LineItemsCount        int                             `json:"line_items_count"`
	LineItems             []InvoiceExplainabilityLineItem `json:"line_items"`
}

type InvoicePaymentStatusView struct {
	TenantID             string    `json:"tenant_id"`
	OrganizationID       string    `json:"-"`
	InvoiceID            string    `json:"invoice_id"`
	CustomerExternalID   string    `json:"customer_external_id,omitempty"`
	InvoiceNumber        string    `json:"invoice_number,omitempty"`
	Currency             string    `json:"currency,omitempty"`
	InvoiceStatus        string    `json:"invoice_status,omitempty"`
	PaymentStatus        string    `json:"payment_status,omitempty"`
	PaymentOverdue       *bool     `json:"payment_overdue,omitempty"`
	TotalAmountCents     *int64    `json:"total_amount_cents,omitempty"`
	TotalDueAmountCents  *int64    `json:"total_due_amount_cents,omitempty"`
	TotalPaidAmountCents *int64    `json:"total_paid_amount_cents,omitempty"`
	LastPaymentError     string    `json:"last_payment_error,omitempty"`
	LastEventType        string    `json:"last_event_type"`
	LastEventAt          time.Time `json:"last_event_at"`
	LastWebhookKey       string    `json:"last_webhook_key"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// ---------------------------------------------------------------------------
// Stripe webhook events (replaces BillingEvent for payment lifecycle)
// ---------------------------------------------------------------------------

type StripeWebhookEvent struct {
	ID                 string         `json:"id"`
	TenantID           string         `json:"tenant_id"`
	StripeEventID      string         `json:"stripe_event_id"`
	EventType          string         `json:"event_type"`
	ObjectType         string         `json:"object_type"`
	InvoiceID          string         `json:"invoice_id,omitempty"`
	CustomerExternalID string         `json:"customer_external_id,omitempty"`
	PaymentIntentID    string         `json:"payment_intent_id,omitempty"`
	PaymentStatus      string         `json:"payment_status,omitempty"`
	AmountCents        *int64         `json:"amount_cents,omitempty"`
	Currency           string         `json:"currency,omitempty"`
	FailureMessage     string         `json:"failure_message,omitempty"`
	Payload            map[string]any `json:"payload"`
	ReceivedAt         time.Time      `json:"received_at"`
	OccurredAt         time.Time      `json:"occurred_at"`
}
