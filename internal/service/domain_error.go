package service

import (
	"errors"
	"fmt"
)

// DomainError is a structured error with a machine-readable code and a
// human-readable message. It enables the API layer to translate errors
// into user-visible responses by matching on Code rather than string content.
type DomainError struct {
	Code    string // Machine-readable code (e.g. "meter_sync_failed")
	Message string // Internal message for logs
	Cause   error  // Underlying error (optional)
}

func (e *DomainError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *DomainError) Unwrap() error {
	return e.Cause
}

// NewDomainError creates a DomainError wrapping a sentinel and a code.
func NewDomainError(sentinel error, code, message string) *DomainError {
	return &DomainError{Code: code, Message: message, Cause: sentinel}
}

// DomainErrorCode extracts the Code from a DomainError in an error chain.
// Returns empty string if no DomainError is found.
func DomainErrorCode(err error) string {
	var de *DomainError
	if errors.As(err, &de) {
		return de.Code
	}
	return ""
}

// ---------------------------------------------------------------------------
// Error code constants — one per user-visible error category.
// These are matched by the API error translator.
// ---------------------------------------------------------------------------

const (
	// Billing setup
	ErrCodeBillingSetupIncomplete = "billing_setup_incomplete"
	ErrCodeBillingCheckRequired   = "billing_check_required"
	ErrCodeBillingUnavailable     = "billing_unavailable"

	// Pricing sync
	ErrCodeSyncAdapterRequired = "sync_adapter_required"
	ErrCodeMeterSyncFailed     = "meter_sync_failed"
	ErrCodeTaxSyncFailed       = "tax_sync_failed"
	ErrCodePlanSyncFailed      = "plan_sync_failed"
	ErrCodePricingUnsupported  = "pricing_unsupported"

	// Subscription sync
	ErrCodeSubscriptionSyncFailed = "subscription_sync_failed"

	// Usage sync
	ErrCodeUsageUnsupported = "usage_unsupported"
	ErrCodeUsageSyncFailed  = "usage_sync_failed"

	// Invoice/Payment
	ErrCodeInvoiceFetchFailed       = "invoice_fetch_failed"
	ErrCodeExplainabilityFailed     = "explainability_failed"
	ErrCodePaymentRetryFailed       = "payment_retry_failed"
	ErrCodePaymentReceiptsLoadFailed = "payment_receipts_load_failed"
	ErrCodeCreditNotesLoadFailed    = "credit_notes_load_failed"

	// Customer/Stripe
	ErrCodeStripeCredentialsRequired = "stripe_credentials_required"
	ErrCodeStripeVerificationFailed  = "stripe_verification_failed"
	ErrCodePaymentProviderRequired   = "payment_provider_required"
	ErrCodePaymentProviderUnsupported = "payment_provider_unsupported"

	// Adapter not configured
	ErrCodeAdapterNotConfigured = "adapter_not_configured"
)
