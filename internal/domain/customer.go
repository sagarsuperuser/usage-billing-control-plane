package domain

import "time"

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

type Customer struct {
	ID             string         `json:"id"`
	TenantID       string         `json:"tenant_id,omitempty"`
	ExternalID     string         `json:"external_id"`
	DisplayName    string         `json:"display_name"`
	Email          string         `json:"email,omitempty"`
	Status         CustomerStatus `json:"status"`
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
