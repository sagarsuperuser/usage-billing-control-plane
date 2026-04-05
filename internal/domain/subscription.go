package domain

import "time"

type SubscriptionStatus string

const (
	SubscriptionStatusDraft               SubscriptionStatus = "draft"
	SubscriptionStatusPendingPaymentSetup SubscriptionStatus = "pending_payment_setup"
	SubscriptionStatusActive              SubscriptionStatus = "active"
	SubscriptionStatusActionRequired      SubscriptionStatus = "action_required"
	SubscriptionStatusArchived            SubscriptionStatus = "archived"
)

type SubscriptionBillingTime string

const (
	SubscriptionBillingTimeCalendar    SubscriptionBillingTime = "calendar"
	SubscriptionBillingTimeAnniversary SubscriptionBillingTime = "anniversary"
)

type Subscription struct {
	ID                         string                  `json:"id"`
	TenantID                   string                  `json:"tenant_id,omitempty"`
	Code                       string                  `json:"code"`
	DisplayName                string                  `json:"display_name"`
	CustomerID                 string                  `json:"customer_id"`
	PlanID                     string                  `json:"plan_id"`
	Status                     SubscriptionStatus      `json:"status"`
	BillingTime                SubscriptionBillingTime `json:"billing_time"`
	StartedAt                  *time.Time              `json:"started_at,omitempty"`
	PaymentSetupRequestedAt    *time.Time              `json:"payment_setup_requested_at,omitempty"`
	ActivatedAt                *time.Time              `json:"activated_at,omitempty"`
	CurrentBillingPeriodStart  *time.Time              `json:"current_billing_period_start,omitempty"`
	CurrentBillingPeriodEnd    *time.Time              `json:"current_billing_period_end,omitempty"`
	NextBillingAt              *time.Time              `json:"next_billing_at,omitempty"`
	CreatedAt                  time.Time               `json:"created_at"`
	UpdatedAt                  time.Time               `json:"updated_at"`
}
