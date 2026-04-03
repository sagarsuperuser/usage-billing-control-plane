package service

// lago_compat.go provides minimal type stubs for backward compatibility
// during the Lago removal migration. These types are used by test files
// that have not yet been migrated to the new Stripe-direct adapters.
//
// TODO: Remove this file after test migration is complete.

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

// LagoClientConfig is a stub for tests that reference it.
type LagoClientConfig struct {
	BaseURL string
	APIKey  string
	Timeout time.Duration
}

// LagoHTTPTransport is a stub for tests that reference it.
type LagoHTTPTransport struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewLagoHTTPTransport(cfg LagoClientConfig) (*LagoHTTPTransport, error) {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	return &LagoHTTPTransport{
		baseURL:    cfg.BaseURL,
		apiKey:     cfg.APIKey,
		httpClient: &http.Client{Timeout: timeout},
	}, nil
}

// NewLagoMeterSyncAdapter is a stub that returns a DirectMeterSyncAdapter.
func NewLagoMeterSyncAdapter(_ *LagoHTTPTransport) MeterSyncAdapter {
	return &DirectMeterSyncAdapter{}
}

// NewLagoWebhookService is a stub that returns a PaymentStatusService.
func NewLagoWebhookService(repo store.Repository, _ interface{}, _ interface{}, customerSvc *CustomerService) *PaymentStatusService {
	return NewPaymentStatusService(repo, customerSvc)
}

// LagoWebhookVerifier stub types.
type LagoWebhookVerifier interface{}
type NoopLagoWebhookVerifier struct{}
type LagoOrganizationTenantMapper interface{}

func NewTenantBackedLagoOrganizationTenantMapper(_ store.Repository) LagoOrganizationTenantMapper {
	return nil
}

// LagoInvoiceAdapter is a stub implementation of InvoiceBillingAdapter for tests.
type LagoInvoiceAdapter struct {
	transport *LagoHTTPTransport
}

func NewLagoInvoiceAdapter(transport *LagoHTTPTransport) *LagoInvoiceAdapter {
	return &LagoInvoiceAdapter{transport: transport}
}

func (a *LagoInvoiceAdapter) ListInvoices(_ context.Context, _ url.Values) (int, []byte, error) {
	return http.StatusOK, []byte(`{"invoices":[]}`), nil
}
func (a *LagoInvoiceAdapter) GetInvoice(_ context.Context, _ string) (int, []byte, error) {
	return http.StatusOK, []byte(`{}`), nil
}
func (a *LagoInvoiceAdapter) ListPaymentReceipts(_ context.Context, _ url.Values) (int, []byte, error) {
	return http.StatusOK, []byte(`{"payment_receipts":[]}`), nil
}
func (a *LagoInvoiceAdapter) ListCreditNotes(_ context.Context, _ url.Values) (int, []byte, error) {
	return http.StatusOK, []byte(`{"credit_notes":[]}`), nil
}
func (a *LagoInvoiceAdapter) PreviewInvoice(_ context.Context, p []byte) (int, []byte, error) {
	return http.StatusOK, p, nil
}
func (a *LagoInvoiceAdapter) RetryInvoicePayment(_ context.Context, _ string, _ []byte) (int, []byte, error) {
	return http.StatusOK, []byte(`{}`), nil
}
func (a *LagoInvoiceAdapter) ResendInvoiceEmail(_ context.Context, _ string, _ BillingDocumentEmail) error {
	return nil
}
func (a *LagoInvoiceAdapter) ResendPaymentReceiptEmail(_ context.Context, _ string, _ BillingDocumentEmail) error {
	return nil
}
func (a *LagoInvoiceAdapter) ResendCreditNoteEmail(_ context.Context, _ string, _ BillingDocumentEmail) error {
	return nil
}

// LagoCustomerBillingAdapter is a stub for tests.
type LagoCustomerBillingAdapter struct {
	transport *LagoHTTPTransport
}

func NewLagoCustomerBillingAdapter(transport *LagoHTTPTransport) *LagoCustomerBillingAdapter {
	return &LagoCustomerBillingAdapter{transport: transport}
}

func (a *LagoCustomerBillingAdapter) UpsertCustomer(_ context.Context, p []byte) (int, []byte, error) {
	return http.StatusOK, p, nil
}
func (a *LagoCustomerBillingAdapter) GetCustomer(_ context.Context, _ string) (int, []byte, error) {
	return http.StatusOK, []byte(`{}`), nil
}
func (a *LagoCustomerBillingAdapter) ListCustomerPaymentMethods(_ context.Context, _ string) (int, []byte, error) {
	return http.StatusOK, []byte(`{"payment_methods":[]}`), nil
}
func (a *LagoCustomerBillingAdapter) GenerateCustomerCheckoutURL(_ context.Context, _ string) (int, []byte, error) {
	return http.StatusOK, []byte(`{"checkout_url":""}`), nil
}
func (a *LagoCustomerBillingAdapter) SyncCustomer(_ context.Context, _ string, _ domain.Customer, _ domain.CustomerBillingProfile, _ domain.CustomerPaymentSetup, _ domain.WorkspaceBillingSettings) (CustomerSyncResult, error) {
	return CustomerSyncResult{}, nil
}
func (a *LagoCustomerBillingAdapter) GetCustomerCheckoutURL(_ context.Context, _ string) (string, error) {
	return "", nil
}
