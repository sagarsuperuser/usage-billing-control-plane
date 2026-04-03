package service

import (
	"context"
	"net/url"

	"usage-billing-control-plane/internal/domain"
)

// ---------------------------------------------------------------------------
// Billing adapter interfaces (formerly in lago_client.go).
// These define the contract for external billing operations.
// Implementations: direct_sync_adapters.go, stripe_customer_adapter.go,
// stripe_invoice_adapter.go, stripe_billing_provider_adapter.go.
// ---------------------------------------------------------------------------

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

// CustomerSyncResult holds the provider-neutral outcome of a customer sync
// and payment method verification.
type CustomerSyncResult struct {
	LagoCustomerID              string
	ProviderCustomerID          string
	PaymentMethodType           string
	DefaultPaymentMethodPresent bool
	ProviderPaymentMethodRef    string
}

type CustomerBillingAdapter interface {
	UpsertCustomer(ctx context.Context, payload []byte) (int, []byte, error)
	GetCustomer(ctx context.Context, externalID string) (int, []byte, error)
	ListCustomerPaymentMethods(ctx context.Context, externalID string) (int, []byte, error)
	GenerateCustomerCheckoutURL(ctx context.Context, externalID string) (int, []byte, error)
	SyncCustomer(ctx context.Context, providerCode string, customer domain.Customer, profile domain.CustomerBillingProfile, setup domain.CustomerPaymentSetup, settings domain.WorkspaceBillingSettings) (CustomerSyncResult, error)
	GetCustomerCheckoutURL(ctx context.Context, externalID string) (string, error)
}

type BillingEntitySettingsSyncAdapter interface {
	SyncBillingEntitySettings(ctx context.Context, settings domain.WorkspaceBillingSettings) error
}

type TaxSyncAdapter interface {
	SyncTax(ctx context.Context, tax domain.Tax) error
}
