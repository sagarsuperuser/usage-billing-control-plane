package domain

import "time"

type PricingMode string

const (
	PricingModeFlat      PricingMode = "flat"
	PricingModeGraduated PricingMode = "graduated"
	PricingModePackage   PricingMode = "package"
)

type RatingTier struct {
	UpTo            int64 `json:"up_to"`
	UnitAmountCents int64 `json:"unit_amount_cents"`
}

type RatingRuleVersion struct {
	ID                     string       `json:"id"`
	Name                   string       `json:"name"`
	Version                int          `json:"version"`
	Mode                   PricingMode  `json:"mode"`
	Currency               string       `json:"currency"`
	FlatAmountCents        int64        `json:"flat_amount_cents"`
	GraduatedTiers         []RatingTier `json:"graduated_tiers"`
	PackageSize            int64        `json:"package_size"`
	PackageAmountCents     int64        `json:"package_amount_cents"`
	OverageUnitAmountCents int64        `json:"overage_unit_amount_cents"`
	CreatedAt              time.Time    `json:"created_at"`
}

type Meter struct {
	ID                  string    `json:"id"`
	Key                 string    `json:"key"`
	Name                string    `json:"name"`
	Unit                string    `json:"unit"`
	Aggregation         string    `json:"aggregation"`
	RatingRuleVersionID string    `json:"rating_rule_version_id"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type UsageEvent struct {
	ID         string    `json:"id"`
	CustomerID string    `json:"customer_id"`
	MeterID    string    `json:"meter_id"`
	Quantity   int64     `json:"quantity"`
	Timestamp  time.Time `json:"timestamp"`
}

type BilledEntry struct {
	ID          string    `json:"id"`
	CustomerID  string    `json:"customer_id"`
	MeterID     string    `json:"meter_id"`
	AmountCents int64     `json:"amount_cents"`
	Timestamp   time.Time `json:"timestamp"`
}

type InvoicePreviewItem struct {
	MeterID             string `json:"meter_id"`
	Quantity            int64  `json:"quantity"`
	RatingRuleVersionID string `json:"rating_rule_version_id,omitempty"`
}

type InvoicePreviewRequest struct {
	CustomerID string               `json:"customer_id"`
	Currency   string               `json:"currency"`
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

type ReplayJobStatus string

const (
	ReplayQueued  ReplayJobStatus = "queued"
	ReplayRunning ReplayJobStatus = "running"
	ReplayDone    ReplayJobStatus = "done"
	ReplayFailed  ReplayJobStatus = "failed"
)

type ReplayJob struct {
	ID               string          `json:"id"`
	CustomerID       string          `json:"customer_id,omitempty"`
	MeterID          string          `json:"meter_id,omitempty"`
	From             *time.Time      `json:"from,omitempty"`
	To               *time.Time      `json:"to,omitempty"`
	IdempotencyKey   string          `json:"idempotency_key"`
	Status           ReplayJobStatus `json:"status"`
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
