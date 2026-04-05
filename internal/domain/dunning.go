package domain

import "time"

type DunningFinalAction string

const (
	DunningFinalActionManualReview DunningFinalAction = "manual_review"
	DunningFinalActionPause        DunningFinalAction = "pause"
	DunningFinalActionWriteOff     DunningFinalAction = "write_off_later"
)

type DunningRunState string

const (
	DunningRunStateScheduled            DunningRunState = "scheduled"
	DunningRunStateRetryDue             DunningRunState = "retry_due"
	DunningRunStateAwaitingPaymentSetup DunningRunState = "awaiting_payment_setup"
	DunningRunStateAwaitingRetryResult  DunningRunState = "awaiting_retry_result"
	DunningRunStateResolved             DunningRunState = "resolved"
	DunningRunStatePaused               DunningRunState = "paused"
	DunningRunStateEscalated            DunningRunState = "escalated"
	DunningRunStateExhausted            DunningRunState = "exhausted"
)

type DunningActionType string

const (
	DunningActionTypeRetryPayment           DunningActionType = "retry_payment"
	DunningActionTypeCollectPaymentReminder DunningActionType = "collect_payment_reminder"
)

type DunningResolution string

const (
	DunningResolutionPaymentSucceeded      DunningResolution = "payment_succeeded"
	DunningResolutionInvoiceNotCollectible DunningResolution = "invoice_not_collectible"
	DunningResolutionOperatorResolved      DunningResolution = "operator_resolved"
	DunningResolutionEscalated             DunningResolution = "escalated"
)

type DunningEventType string

const (
	DunningEventTypeStarted             DunningEventType = "dunning_started"
	DunningEventTypeRetryScheduled      DunningEventType = "retry_scheduled"
	DunningEventTypeRetryAttempted      DunningEventType = "retry_attempted"
	DunningEventTypeRetrySucceeded      DunningEventType = "retry_succeeded"
	DunningEventTypeRetryFailed         DunningEventType = "retry_failed"
	DunningEventTypePaymentSetupPending DunningEventType = "payment_setup_pending"
	DunningEventTypePaymentSetupReady   DunningEventType = "payment_setup_ready"
	DunningEventTypeNotificationSent    DunningEventType = "notification_sent"
	DunningEventTypeNotificationFailed  DunningEventType = "notification_failed"
	DunningEventTypePaused              DunningEventType = "paused"
	DunningEventTypeResumed             DunningEventType = "resumed"
	DunningEventTypeEscalated           DunningEventType = "escalated"
	DunningEventTypeResolved            DunningEventType = "resolved"
)

type DunningNotificationIntentType string

const (
	DunningNotificationIntentTypePaymentFailed         DunningNotificationIntentType = "dunning.payment_failed"
	DunningNotificationIntentTypePaymentMethodRequired DunningNotificationIntentType = "dunning.payment_method_required"
	DunningNotificationIntentTypeRetryScheduled        DunningNotificationIntentType = "dunning.retry_scheduled"
	DunningNotificationIntentTypeFinalAttempt          DunningNotificationIntentType = "dunning.final_attempt"
	DunningNotificationIntentTypeEscalated             DunningNotificationIntentType = "dunning.escalated"
)

type DunningNotificationIntentStatus string

const (
	DunningNotificationIntentStatusQueued     DunningNotificationIntentStatus = "queued"
	DunningNotificationIntentStatusDispatched DunningNotificationIntentStatus = "dispatched"
	DunningNotificationIntentStatusFailed     DunningNotificationIntentStatus = "failed"
)

type DunningPolicy struct {
	ID                             string             `json:"id"`
	TenantID                       string             `json:"tenant_id,omitempty"`
	Name                           string             `json:"name"`
	Enabled                        bool               `json:"enabled"`
	RetrySchedule                  []string           `json:"retry_schedule"`
	MaxRetryAttempts               int                `json:"max_retry_attempts"`
	CollectPaymentReminderSchedule []string           `json:"collect_payment_reminder_schedule"`
	FinalAction                    DunningFinalAction `json:"final_action"`
	GracePeriodDays                int                `json:"grace_period_days"`
	CreatedAt                      time.Time          `json:"created_at"`
	UpdatedAt                      time.Time          `json:"updated_at"`
}

type InvoiceDunningRun struct {
	ID                 string            `json:"id"`
	TenantID           string            `json:"tenant_id,omitempty"`
	InvoiceID          string            `json:"invoice_id"`
	CustomerExternalID string            `json:"customer_external_id,omitempty"`
	PolicyID           string            `json:"policy_id"`
	State              DunningRunState   `json:"state"`
	Reason             string            `json:"reason,omitempty"`
	AttemptCount       int               `json:"attempt_count"`
	LastAttemptAt      *time.Time        `json:"last_attempt_at,omitempty"`
	NextActionAt       *time.Time        `json:"next_action_at,omitempty"`
	NextActionType     DunningActionType `json:"next_action_type,omitempty"`
	Paused             bool              `json:"paused"`
	ResolvedAt         *time.Time        `json:"resolved_at,omitempty"`
	Resolution         DunningResolution `json:"resolution,omitempty"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
}

type InvoiceDunningEvent struct {
	ID                 string            `json:"id"`
	RunID              string            `json:"run_id"`
	TenantID           string            `json:"tenant_id,omitempty"`
	InvoiceID          string            `json:"invoice_id"`
	CustomerExternalID string            `json:"customer_external_id,omitempty"`
	EventType          DunningEventType  `json:"event_type"`
	State              DunningRunState   `json:"state"`
	ActionType         DunningActionType `json:"action_type,omitempty"`
	Reason             string            `json:"reason,omitempty"`
	AttemptCount       int               `json:"attempt_count"`
	Metadata           map[string]any    `json:"metadata,omitempty"`
	CreatedAt          time.Time         `json:"created_at"`
}

type DunningNotificationIntent struct {
	ID                 string                          `json:"id"`
	RunID              string                          `json:"run_id"`
	TenantID           string                          `json:"tenant_id,omitempty"`
	InvoiceID          string                          `json:"invoice_id"`
	CustomerExternalID string                          `json:"customer_external_id,omitempty"`
	IntentType         DunningNotificationIntentType   `json:"intent_type"`
	ActionType         DunningActionType               `json:"action_type,omitempty"`
	Status             DunningNotificationIntentStatus `json:"status"`
	DeliveryBackend    string                          `json:"delivery_backend,omitempty"`
	RecipientEmail     string                          `json:"recipient_email,omitempty"`
	Payload            map[string]any                  `json:"payload,omitempty"`
	LastError          string                          `json:"last_error,omitempty"`
	CreatedAt          time.Time                       `json:"created_at"`
	DispatchedAt       *time.Time                      `json:"dispatched_at,omitempty"`
}

type DunningSummary struct {
	RunID                      string                          `json:"run_id"`
	State                      DunningRunState                 `json:"state"`
	Reason                     string                          `json:"reason,omitempty"`
	AttemptCount               int                             `json:"attempt_count"`
	NextActionType             DunningActionType               `json:"next_action_type,omitempty"`
	NextActionAt               *time.Time                      `json:"next_action_at,omitempty"`
	Paused                     bool                            `json:"paused"`
	Resolution                 DunningResolution               `json:"resolution,omitempty"`
	LastEventType              DunningEventType                `json:"last_event_type,omitempty"`
	LastEventAt                *time.Time                      `json:"last_event_at,omitempty"`
	LastNotificationIntentType DunningNotificationIntentType   `json:"last_notification_intent_type,omitempty"`
	LastNotificationStatus     DunningNotificationIntentStatus `json:"last_notification_status,omitempty"`
	LastNotificationAt         *time.Time                      `json:"last_notification_at,omitempty"`
	LastNotificationError      string                          `json:"last_notification_error,omitempty"`
}
