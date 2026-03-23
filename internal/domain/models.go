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

type BillingProviderType string

const (
	BillingProviderTypeStripe BillingProviderType = "stripe"
)

type BillingProviderConnectionStatus string

const (
	BillingProviderConnectionStatusPending   BillingProviderConnectionStatus = "pending"
	BillingProviderConnectionStatusConnected BillingProviderConnectionStatus = "connected"
	BillingProviderConnectionStatusSyncError BillingProviderConnectionStatus = "sync_error"
	BillingProviderConnectionStatusDisabled  BillingProviderConnectionStatus = "disabled"
)

type BillingProviderConnectionScope string

const (
	BillingProviderConnectionScopePlatform BillingProviderConnectionScope = "platform"
	BillingProviderConnectionScopeTenant   BillingProviderConnectionScope = "tenant"
)

type WorkspaceBillingBackend string

const (
	WorkspaceBillingBackendLago WorkspaceBillingBackend = "lago"
)

type WorkspaceBillingIsolationMode string

const (
	WorkspaceBillingIsolationModeShared    WorkspaceBillingIsolationMode = "shared"
	WorkspaceBillingIsolationModeDedicated WorkspaceBillingIsolationMode = "dedicated"
)

type WorkspaceBillingBindingStatus string

const (
	WorkspaceBillingBindingStatusPending            WorkspaceBillingBindingStatus = "pending"
	WorkspaceBillingBindingStatusProvisioning       WorkspaceBillingBindingStatus = "provisioning"
	WorkspaceBillingBindingStatusConnected          WorkspaceBillingBindingStatus = "connected"
	WorkspaceBillingBindingStatusVerificationFailed WorkspaceBillingBindingStatus = "verification_failed"
	WorkspaceBillingBindingStatusDisabled           WorkspaceBillingBindingStatus = "disabled"
)

type UserStatus string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusDisabled UserStatus = "disabled"
)

type UserPlatformRole string

const (
	UserPlatformRoleAdmin UserPlatformRole = "platform_admin"
)

type UserTenantMembershipStatus string

const (
	UserTenantMembershipStatusActive   UserTenantMembershipStatus = "active"
	UserTenantMembershipStatusDisabled UserTenantMembershipStatus = "disabled"
)

type BrowserSSOProviderType string

const (
	BrowserSSOProviderTypeOIDC BrowserSSOProviderType = "oidc"
)

type Tenant struct {
	ID                          string       `json:"id"`
	Name                        string       `json:"name"`
	Status                      TenantStatus `json:"status"`
	BillingProviderConnectionID string       `json:"billing_provider_connection_id,omitempty"`
	LagoOrganizationID          string       `json:"lago_organization_id,omitempty"`
	LagoBillingProviderCode     string       `json:"lago_billing_provider_code,omitempty"`
	CreatedAt                   time.Time    `json:"created_at"`
	UpdatedAt                   time.Time    `json:"updated_at"`
}

type BillingProviderConnection struct {
	ID                 string                          `json:"id"`
	ProviderType       BillingProviderType             `json:"provider_type"`
	Environment        string                          `json:"environment"`
	DisplayName        string                          `json:"display_name"`
	Scope              BillingProviderConnectionScope  `json:"scope"`
	OwnerTenantID      string                          `json:"owner_tenant_id,omitempty"`
	Status             BillingProviderConnectionStatus `json:"status"`
	LagoOrganizationID string                          `json:"lago_organization_id,omitempty"`
	LagoProviderCode   string                          `json:"lago_provider_code,omitempty"`
	SecretRef          string                          `json:"secret_ref,omitempty"`
	LastSyncedAt       *time.Time                      `json:"last_synced_at,omitempty"`
	LastSyncError      string                          `json:"last_sync_error,omitempty"`
	ConnectedAt        *time.Time                      `json:"connected_at,omitempty"`
	DisabledAt         *time.Time                      `json:"disabled_at,omitempty"`
	CreatedByType      string                          `json:"created_by_type"`
	CreatedByID        string                          `json:"created_by_id,omitempty"`
	CreatedAt          time.Time                       `json:"created_at"`
	UpdatedAt          time.Time                       `json:"updated_at"`
}

type WorkspaceBillingBinding struct {
	ID                          string                        `json:"id"`
	WorkspaceID                 string                        `json:"workspace_id"`
	BillingProviderConnectionID string                        `json:"billing_provider_connection_id"`
	Backend                     WorkspaceBillingBackend       `json:"backend"`
	BackendOrganizationID       string                        `json:"backend_organization_id,omitempty"`
	BackendProviderCode         string                        `json:"backend_provider_code,omitempty"`
	IsolationMode               WorkspaceBillingIsolationMode `json:"isolation_mode"`
	Status                      WorkspaceBillingBindingStatus `json:"status"`
	ProvisioningError           string                        `json:"provisioning_error,omitempty"`
	LastVerifiedAt              *time.Time                    `json:"last_verified_at,omitempty"`
	ConnectedAt                 *time.Time                    `json:"connected_at,omitempty"`
	DisabledAt                  *time.Time                    `json:"disabled_at,omitempty"`
	CreatedByType               string                        `json:"created_by_type"`
	CreatedByID                 string                        `json:"created_by_id,omitempty"`
	CreatedAt                   time.Time                     `json:"created_at"`
	UpdatedAt                   time.Time                     `json:"updated_at"`
}

type WorkspaceBillingSettings struct {
	WorkspaceID            string    `json:"workspace_id"`
	BillingEntityCode      string    `json:"billing_entity_code,omitempty"`
	NetPaymentTermDays     *int      `json:"net_payment_term_days,omitempty"`
	TaxCodes               []string  `json:"tax_codes,omitempty"`
	InvoiceMemo            string    `json:"invoice_memo,omitempty"`
	InvoiceFooter          string    `json:"invoice_footer,omitempty"`
	DocumentLocale         string    `json:"document_locale,omitempty"`
	InvoiceGracePeriodDays *int      `json:"invoice_grace_period_days,omitempty"`
	DocumentNumbering      string    `json:"document_numbering,omitempty"`
	DocumentNumberPrefix   string    `json:"document_number_prefix,omitempty"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

type User struct {
	ID           string           `json:"id"`
	Email        string           `json:"email"`
	DisplayName  string           `json:"display_name"`
	Status       UserStatus       `json:"status"`
	PlatformRole UserPlatformRole `json:"platform_role,omitempty"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
}

type UserPasswordCredential struct {
	UserID            string    `json:"user_id"`
	PasswordHash      string    `json:"-"`
	PasswordUpdatedAt time.Time `json:"password_updated_at"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type PasswordResetToken struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	TokenHash string     `json:"-"`
	ExpiresAt time.Time  `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type UserTenantMembership struct {
	UserID    string                     `json:"user_id"`
	TenantID  string                     `json:"tenant_id"`
	Role      string                     `json:"role"`
	Status    UserTenantMembershipStatus `json:"status"`
	CreatedAt time.Time                  `json:"created_at"`
	UpdatedAt time.Time                  `json:"updated_at"`
}

type WorkspaceInvitationStatus string

const (
	WorkspaceInvitationStatusPending  WorkspaceInvitationStatus = "pending"
	WorkspaceInvitationStatusAccepted WorkspaceInvitationStatus = "accepted"
	WorkspaceInvitationStatusExpired  WorkspaceInvitationStatus = "expired"
	WorkspaceInvitationStatusRevoked  WorkspaceInvitationStatus = "revoked"
)

type WorkspaceInvitation struct {
	ID                    string                    `json:"id"`
	WorkspaceID           string                    `json:"workspace_id"`
	Email                 string                    `json:"email"`
	Role                  string                    `json:"role"`
	Status                WorkspaceInvitationStatus `json:"status"`
	TokenHash             string                    `json:"-"`
	ExpiresAt             time.Time                 `json:"expires_at"`
	AcceptedAt            *time.Time                `json:"accepted_at,omitempty"`
	AcceptedByUserID      string                    `json:"accepted_by_user_id,omitempty"`
	InvitedByUserID       string                    `json:"invited_by_user_id,omitempty"`
	InvitedByPlatformUser bool                      `json:"invited_by_platform_user"`
	RevokedAt             *time.Time                `json:"revoked_at,omitempty"`
	CreatedAt             time.Time                 `json:"created_at"`
	UpdatedAt             time.Time                 `json:"updated_at"`
}

type UserFederatedIdentity struct {
	ID            string                 `json:"id"`
	UserID        string                 `json:"user_id"`
	ProviderKey   string                 `json:"provider_key"`
	ProviderType  BrowserSSOProviderType `json:"provider_type"`
	Subject       string                 `json:"subject"`
	Email         string                 `json:"email,omitempty"`
	EmailVerified bool                   `json:"email_verified"`
	LastLoginAt   *time.Time             `json:"last_login_at,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

type CustomerStatus string

const (
	CustomerStatusActive    CustomerStatus = "active"
	CustomerStatusSuspended CustomerStatus = "suspended"
	CustomerStatusArchived  CustomerStatus = "archived"
)

type BillingProfileStatus string

const (
	BillingProfileStatusMissing    BillingProfileStatus = "missing"
	BillingProfileStatusIncomplete BillingProfileStatus = "incomplete"
	BillingProfileStatusReady      BillingProfileStatus = "ready"
	BillingProfileStatusSyncError  BillingProfileStatus = "sync_error"
)

type PaymentSetupStatus string

const (
	PaymentSetupStatusMissing PaymentSetupStatus = "missing"
	PaymentSetupStatusPending PaymentSetupStatus = "pending"
	PaymentSetupStatusReady   PaymentSetupStatus = "ready"
	PaymentSetupStatusError   PaymentSetupStatus = "error"
)

type PaymentSetupRequestStatus string

const (
	PaymentSetupRequestStatusNotRequested PaymentSetupRequestStatus = "not_requested"
	PaymentSetupRequestStatusSent         PaymentSetupRequestStatus = "sent"
	PaymentSetupRequestStatusFailed       PaymentSetupRequestStatus = "failed"
)

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

type Customer struct {
	ID             string         `json:"id"`
	TenantID       string         `json:"tenant_id,omitempty"`
	ExternalID     string         `json:"external_id"`
	DisplayName    string         `json:"display_name"`
	Email          string         `json:"email,omitempty"`
	Status         CustomerStatus `json:"status"`
	LagoCustomerID string         `json:"lago_customer_id,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type CustomerBillingProfile struct {
	CustomerID    string               `json:"customer_id"`
	TenantID      string               `json:"tenant_id,omitempty"`
	LegalName     string               `json:"legal_name,omitempty"`
	Email         string               `json:"email,omitempty"`
	Phone         string               `json:"phone,omitempty"`
	AddressLine1  string               `json:"billing_address_line1,omitempty"`
	AddressLine2  string               `json:"billing_address_line2,omitempty"`
	City          string               `json:"billing_city,omitempty"`
	State         string               `json:"billing_state,omitempty"`
	PostalCode    string               `json:"billing_postal_code,omitempty"`
	Country       string               `json:"billing_country,omitempty"`
	Currency      string               `json:"currency,omitempty"`
	TaxIdentifier string               `json:"tax_identifier,omitempty"`
	TaxCodes      []string             `json:"tax_codes,omitempty"`
	ProviderCode  string               `json:"provider_code,omitempty"`
	ProfileStatus BillingProfileStatus `json:"profile_status"`
	LastSyncedAt  *time.Time           `json:"last_synced_at,omitempty"`
	LastSyncError string               `json:"last_sync_error,omitempty"`
	CreatedAt     time.Time            `json:"created_at"`
	UpdatedAt     time.Time            `json:"updated_at"`
}

type CustomerPaymentSetup struct {
	CustomerID                     string                    `json:"customer_id"`
	TenantID                       string                    `json:"tenant_id,omitempty"`
	SetupStatus                    PaymentSetupStatus        `json:"setup_status"`
	DefaultPaymentMethodPresent    bool                      `json:"default_payment_method_present"`
	PaymentMethodType              string                    `json:"payment_method_type,omitempty"`
	ProviderCustomerReference      string                    `json:"provider_customer_reference,omitempty"`
	ProviderPaymentMethodReference string                    `json:"provider_payment_method_reference,omitempty"`
	LastVerifiedAt                 *time.Time                `json:"last_verified_at,omitempty"`
	LastVerificationResult         string                    `json:"last_verification_result,omitempty"`
	LastVerificationError          string                    `json:"last_verification_error,omitempty"`
	LastRequestStatus              PaymentSetupRequestStatus `json:"last_request_status,omitempty"`
	LastRequestKind                string                    `json:"last_request_kind,omitempty"`
	LastRequestToEmail             string                    `json:"last_request_to_email,omitempty"`
	LastRequestSentAt              *time.Time                `json:"last_request_sent_at,omitempty"`
	LastRequestError               string                    `json:"last_request_error,omitempty"`
	CreatedAt                      time.Time                 `json:"created_at"`
	UpdatedAt                      time.Time                 `json:"updated_at"`
}

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

type BillingInterval string

const (
	BillingIntervalMonthly BillingInterval = "monthly"
	BillingIntervalYearly  BillingInterval = "yearly"
)

type PlanStatus string

const (
	PlanStatusDraft    PlanStatus = "draft"
	PlanStatusActive   PlanStatus = "active"
	PlanStatusArchived PlanStatus = "archived"
)

type AddOnStatus string

const (
	AddOnStatusDraft    AddOnStatus = "draft"
	AddOnStatusActive   AddOnStatus = "active"
	AddOnStatusArchived AddOnStatus = "archived"
)

type AddOn struct {
	ID              string          `json:"id"`
	TenantID        string          `json:"tenant_id,omitempty"`
	Code            string          `json:"code"`
	Name            string          `json:"name"`
	Description     string          `json:"description,omitempty"`
	Currency        string          `json:"currency"`
	BillingInterval BillingInterval `json:"billing_interval"`
	Status          AddOnStatus     `json:"status"`
	AmountCents     int64           `json:"amount_cents"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type CouponStatus string

const (
	CouponStatusDraft    CouponStatus = "draft"
	CouponStatusActive   CouponStatus = "active"
	CouponStatusArchived CouponStatus = "archived"
)

type CouponDiscountType string

const (
	CouponDiscountTypeAmountOff  CouponDiscountType = "amount_off"
	CouponDiscountTypePercentOff CouponDiscountType = "percent_off"
)

type CouponFrequency string

const (
	CouponFrequencyOnce      CouponFrequency = "once"
	CouponFrequencyRecurring CouponFrequency = "recurring"
	CouponFrequencyForever   CouponFrequency = "forever"
)

type Coupon struct {
	ID                string             `json:"id"`
	TenantID          string             `json:"tenant_id,omitempty"`
	Code              string             `json:"code"`
	Name              string             `json:"name"`
	Description       string             `json:"description,omitempty"`
	Status            CouponStatus       `json:"status"`
	DiscountType      CouponDiscountType `json:"discount_type"`
	Currency          string             `json:"currency,omitempty"`
	AmountOffCents    int64              `json:"amount_off_cents"`
	PercentOff        int                `json:"percent_off"`
	Frequency         CouponFrequency    `json:"frequency"`
	FrequencyDuration int                `json:"frequency_duration"`
	ExpirationAt      *time.Time         `json:"expiration_at,omitempty"`
	CreatedAt         time.Time          `json:"created_at"`
	UpdatedAt         time.Time          `json:"updated_at"`
}

type TaxStatus string

const (
	TaxStatusDraft    TaxStatus = "draft"
	TaxStatusActive   TaxStatus = "active"
	TaxStatusArchived TaxStatus = "archived"
)

type Tax struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id,omitempty"`
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Status      TaxStatus `json:"status"`
	Rate        float64   `json:"rate"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Plan struct {
	ID              string          `json:"id"`
	TenantID        string          `json:"tenant_id,omitempty"`
	Code            string          `json:"code"`
	Name            string          `json:"name"`
	Description     string          `json:"description,omitempty"`
	Currency        string          `json:"currency"`
	BillingInterval BillingInterval `json:"billing_interval"`
	Status          PlanStatus      `json:"status"`
	BaseAmountCents int64           `json:"base_amount_cents"`
	MeterIDs        []string        `json:"meter_ids"`
	AddOnIDs        []string        `json:"add_on_ids"`
	CouponIDs       []string        `json:"coupon_ids"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

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
	ID                      string                  `json:"id"`
	TenantID                string                  `json:"tenant_id,omitempty"`
	Code                    string                  `json:"code"`
	DisplayName             string                  `json:"display_name"`
	CustomerID              string                  `json:"customer_id"`
	PlanID                  string                  `json:"plan_id"`
	Status                  SubscriptionStatus      `json:"status"`
	BillingTime             SubscriptionBillingTime `json:"billing_time"`
	StartedAt               *time.Time              `json:"started_at,omitempty"`
	PaymentSetupRequestedAt *time.Time              `json:"payment_setup_requested_at,omitempty"`
	ActivatedAt             *time.Time              `json:"activated_at,omitempty"`
	CreatedAt               time.Time               `json:"created_at"`
	UpdatedAt               time.Time               `json:"updated_at"`
}

type UsageEvent struct {
	ID             string    `json:"id"`
	TenantID       string    `json:"tenant_id,omitempty"`
	CustomerID     string    `json:"customer_id"`
	MeterID        string    `json:"meter_id"`
	SubscriptionID string    `json:"subscription_id,omitempty"`
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

type InvoiceSummary struct {
	InvoiceID            string     `json:"invoice_id"`
	InvoiceNumber        string     `json:"invoice_number,omitempty"`
	CustomerExternalID   string     `json:"customer_external_id,omitempty"`
	CustomerDisplayName  string     `json:"customer_display_name,omitempty"`
	OrganizationID       string     `json:"organization_id,omitempty"`
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
	LagoID            string          `json:"lago_id,omitempty"`
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

type ServiceAccount struct {
	ID                    string     `json:"id"`
	TenantID              string     `json:"tenant_id"`
	Name                  string     `json:"name"`
	Description           string     `json:"description,omitempty"`
	Role                  string     `json:"role"`
	Status                string     `json:"status"`
	Purpose               string     `json:"purpose,omitempty"`
	Environment           string     `json:"environment,omitempty"`
	CreatedByUserID       string     `json:"created_by_user_id,omitempty"`
	CreatedByPlatformUser bool       `json:"created_by_platform_user,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
	DisabledAt            *time.Time `json:"disabled_at,omitempty"`
}

const (
	ServiceAccountStatusActive   = "active"
	ServiceAccountStatusDisabled = "disabled"
)

type APIKey struct {
	ID                    string     `json:"id"`
	KeyPrefix             string     `json:"key_prefix"`
	KeyHash               string     `json:"-"`
	Name                  string     `json:"name"`
	Role                  string     `json:"role"`
	TenantID              string     `json:"tenant_id"`
	OwnerType             string     `json:"owner_type,omitempty"`
	OwnerID               string     `json:"owner_id,omitempty"`
	Purpose               string     `json:"purpose,omitempty"`
	Environment           string     `json:"environment,omitempty"`
	CreatedByUserID       string     `json:"created_by_user_id,omitempty"`
	CreatedByPlatformUser bool       `json:"created_by_platform_user,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	ExpiresAt             *time.Time `json:"expires_at,omitempty"`
	RevokedAt             *time.Time `json:"revoked_at,omitempty"`
	LastUsedAt            *time.Time `json:"last_used_at,omitempty"`
	LastRotatedAt         *time.Time `json:"last_rotated_at,omitempty"`
	RotationRequiredAt    *time.Time `json:"rotation_required_at,omitempty"`
	RevocationReason      string     `json:"revocation_reason,omitempty"`
}

type PlatformAPIKey struct {
	ID                 string     `json:"id"`
	KeyPrefix          string     `json:"key_prefix"`
	KeyHash            string     `json:"-"`
	Name               string     `json:"name"`
	Role               string     `json:"role"`
	OwnerType          string     `json:"owner_type,omitempty"`
	OwnerID            string     `json:"owner_id,omitempty"`
	Purpose            string     `json:"purpose,omitempty"`
	Environment        string     `json:"environment,omitempty"`
	CreatedByUserID    string     `json:"created_by_user_id,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	ExpiresAt          *time.Time `json:"expires_at,omitempty"`
	RevokedAt          *time.Time `json:"revoked_at,omitempty"`
	LastUsedAt         *time.Time `json:"last_used_at,omitempty"`
	LastRotatedAt      *time.Time `json:"last_rotated_at,omitempty"`
	RotationRequiredAt *time.Time `json:"rotation_required_at,omitempty"`
	RevocationReason   string     `json:"revocation_reason,omitempty"`
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
	OwnerType     string `json:"owner_type,omitempty"`
	OwnerID       string `json:"owner_id,omitempty"`
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
