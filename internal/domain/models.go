package domain

import "time"

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
	WorkspaceBillingBackendStripe WorkspaceBillingBackend = "stripe"
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

type BillingEvent struct {
	ID                   string         `json:"id"`
	TenantID             string         `json:"tenant_id"`
	OrganizationID       string         `json:"-"`
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
