package domain

import "time"

type PricingMode string

const (
	PricingModeFlat      PricingMode = "flat"
	PricingModeGraduated PricingMode = "graduated"
	PricingModePackage   PricingMode = "package"
)

type TenantStatus string

const (
	TenantStatusActive    TenantStatus = "active"
	TenantStatusSuspended TenantStatus = "suspended"
	TenantStatusDeleted   TenantStatus = "deleted"
)

type Tenant struct {
	ID                      string       `json:"id"`
	Name                    string       `json:"name"`
	Status                  TenantStatus `json:"status"`
	LagoOrganizationID      string       `json:"lago_organization_id,omitempty"`
	LagoBillingProviderCode string       `json:"lago_billing_provider_code,omitempty"`
	CreatedAt               time.Time    `json:"created_at"`
	UpdatedAt               time.Time    `json:"updated_at"`
}

type RatingTier struct {
	UpTo            int64 `json:"up_to"`
	UnitAmountCents int64 `json:"unit_amount_cents"`
}

type RatingRuleLifecycleState string

const (
	RatingRuleLifecycleDraft    RatingRuleLifecycleState = "draft"
	RatingRuleLifecycleActive   RatingRuleLifecycleState = "active"
	RatingRuleLifecycleArchived RatingRuleLifecycleState = "archived"
)

type RatingRuleVersion struct {
	ID                     string                   `json:"id"`
	TenantID               string                   `json:"tenant_id,omitempty"`
	RuleKey                string                   `json:"rule_key"`
	Name                   string                   `json:"name"`
	Version                int                      `json:"version"`
	LifecycleState         RatingRuleLifecycleState `json:"lifecycle_state,omitempty"`
	Mode                   PricingMode              `json:"mode"`
	Currency               string                   `json:"currency"`
	FlatAmountCents        int64                    `json:"flat_amount_cents"`
	GraduatedTiers         []RatingTier             `json:"graduated_tiers"`
	PackageSize            int64                    `json:"package_size"`
	PackageAmountCents     int64                    `json:"package_amount_cents"`
	OverageUnitAmountCents int64                    `json:"overage_unit_amount_cents"`
	CreatedAt              time.Time                `json:"created_at"`
}

type Meter struct {
	ID                  string    `json:"id"`
	TenantID            string    `json:"tenant_id,omitempty"`
	Key                 string    `json:"key"`
	Name                string    `json:"name"`
	Unit                string    `json:"unit"`
	Aggregation         string    `json:"aggregation"`
	RatingRuleVersionID string    `json:"rating_rule_version_id"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type UsageEvent struct {
	ID             string    `json:"id"`
	TenantID       string    `json:"tenant_id,omitempty"`
	CustomerID     string    `json:"customer_id"`
	MeterID        string    `json:"meter_id"`
	Quantity       int64     `json:"quantity"`
	IdempotencyKey string    `json:"idempotency_key,omitempty"`
	Timestamp      time.Time `json:"timestamp"`
}

type BilledEntry struct {
	ID             string            `json:"id"`
	TenantID       string            `json:"tenant_id,omitempty"`
	CustomerID     string            `json:"customer_id"`
	MeterID        string            `json:"meter_id"`
	AmountCents    int64             `json:"amount_cents"`
	IdempotencyKey string            `json:"idempotency_key,omitempty"`
	Source         BilledEntrySource `json:"source,omitempty"`
	ReplayJobID    string            `json:"replay_job_id,omitempty"`
	Timestamp      time.Time         `json:"timestamp"`
}

type BilledEntrySource string

const (
	BilledEntrySourceAPI              BilledEntrySource = "api"
	BilledEntrySourceReplayAdjustment BilledEntrySource = "replay_adjustment"
)

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

type ReplayJobStatus string

const (
	ReplayQueued  ReplayJobStatus = "queued"
	ReplayRunning ReplayJobStatus = "running"
	ReplayDone    ReplayJobStatus = "done"
	ReplayFailed  ReplayJobStatus = "failed"
)

type ReplayJob struct {
	ID               string          `json:"id"`
	TenantID         string          `json:"tenant_id,omitempty"`
	CustomerID       string          `json:"customer_id,omitempty"`
	MeterID          string          `json:"meter_id,omitempty"`
	From             *time.Time      `json:"from,omitempty"`
	To               *time.Time      `json:"to,omitempty"`
	IdempotencyKey   string          `json:"idempotency_key"`
	Status           ReplayJobStatus `json:"status"`
	AttemptCount     int             `json:"attempt_count"`
	LastAttemptAt    *time.Time      `json:"last_attempt_at,omitempty"`
	ProcessedRecords int64           `json:"processed_records"`
	Error            string          `json:"error,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	StartedAt        *time.Time      `json:"started_at,omitempty"`
	CompletedAt      *time.Time      `json:"completed_at,omitempty"`
}

type ReconciliationRow struct {
	CustomerID          string `json:"customer_id"`
	MeterID             string `json:"meter_id"`
	EventQuantity       int64  `json:"event_quantity"`
	ComputedAmountCents int64  `json:"computed_amount_cents"`
	BilledAmountCents   int64  `json:"billed_amount_cents"`
	DeltaCents          int64  `json:"delta_cents"`
	Mismatch            bool   `json:"mismatch"`
}

type ReconciliationReport struct {
	Rows               []ReconciliationRow `json:"rows"`
	TotalComputedCents int64               `json:"total_computed_cents"`
	TotalBilledCents   int64               `json:"total_billed_cents"`
	TotalDeltaCents    int64               `json:"total_delta_cents"`
	MismatchRowCount   int                 `json:"mismatch_row_count"`
	GeneratedAt        time.Time           `json:"generated_at"`
}

type LagoWebhookEvent struct {
	ID                   string         `json:"id"`
	TenantID             string         `json:"tenant_id"`
	OrganizationID       string         `json:"organization_id"`
	WebhookKey           string         `json:"webhook_key"`
	WebhookType          string         `json:"webhook_type"`
	ObjectType           string         `json:"object_type"`
	InvoiceID            string         `json:"invoice_id,omitempty"`
	PaymentRequestID     string         `json:"payment_request_id,omitempty"`
	DunningCampaignCode  string         `json:"dunning_campaign_code,omitempty"`
	CustomerExternalID   string         `json:"customer_external_id,omitempty"`
	InvoiceNumber        string         `json:"invoice_number,omitempty"`
	Currency             string         `json:"currency,omitempty"`
	InvoiceStatus        string         `json:"invoice_status,omitempty"`
	PaymentStatus        string         `json:"payment_status,omitempty"`
	PaymentOverdue       *bool          `json:"payment_overdue,omitempty"`
	TotalAmountCents     *int64         `json:"total_amount_cents,omitempty"`
	TotalDueAmountCents  *int64         `json:"total_due_amount_cents,omitempty"`
	TotalPaidAmountCents *int64         `json:"total_paid_amount_cents,omitempty"`
	LastPaymentError     string         `json:"last_payment_error,omitempty"`
	Payload              map[string]any `json:"payload"`
	ReceivedAt           time.Time      `json:"received_at"`
	OccurredAt           time.Time      `json:"occurred_at"`
}

type InvoicePaymentStatusView struct {
	TenantID             string    `json:"tenant_id"`
	OrganizationID       string    `json:"organization_id"`
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

type APIKey struct {
	ID         string     `json:"id"`
	KeyPrefix  string     `json:"key_prefix"`
	KeyHash    string     `json:"-"`
	Name       string     `json:"name"`
	Role       string     `json:"role"`
	TenantID   string     `json:"tenant_id"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

type APIKeyAuditEvent struct {
	ID            string         `json:"id"`
	TenantID      string         `json:"tenant_id"`
	APIKeyID      string         `json:"api_key_id"`
	ActorAPIKeyID string         `json:"actor_api_key_id,omitempty"`
	Action        string         `json:"action"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
}

type TenantAuditEvent struct {
	ID            string         `json:"id"`
	TenantID      string         `json:"tenant_id"`
	ActorAPIKeyID string         `json:"actor_api_key_id,omitempty"`
	Action        string         `json:"action"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
}

type APIKeyAuditExportStatus string

const (
	APIKeyAuditExportQueued  APIKeyAuditExportStatus = "queued"
	APIKeyAuditExportRunning APIKeyAuditExportStatus = "running"
	APIKeyAuditExportDone    APIKeyAuditExportStatus = "done"
	APIKeyAuditExportFailed  APIKeyAuditExportStatus = "failed"
)

type APIKeyAuditExportFilters struct {
	APIKeyID      string `json:"api_key_id,omitempty"`
	ActorAPIKeyID string `json:"actor_api_key_id,omitempty"`
	Action        string `json:"action,omitempty"`
}

type APIKeyAuditExportJob struct {
	ID                  string                   `json:"id"`
	TenantID            string                   `json:"tenant_id"`
	RequestedByAPIKeyID string                   `json:"requested_by_api_key_id"`
	IdempotencyKey      string                   `json:"idempotency_key"`
	Status              APIKeyAuditExportStatus  `json:"status"`
	Filters             APIKeyAuditExportFilters `json:"filters"`
	ObjectKey           string                   `json:"object_key,omitempty"`
	RowCount            int64                    `json:"row_count"`
	Error               string                   `json:"error,omitempty"`
	AttemptCount        int                      `json:"attempt_count"`
	CreatedAt           time.Time                `json:"created_at"`
	StartedAt           *time.Time               `json:"started_at,omitempty"`
	CompletedAt         *time.Time               `json:"completed_at,omitempty"`
	ExpiresAt           *time.Time               `json:"expires_at,omitempty"`
}
