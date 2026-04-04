package service

import (
	"context"
	"fmt"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

// InvoiceFinalizationService handles the draft → finalized transition.
// It creates a Stripe PaymentIntent for the invoice amount, stores the
// payment intent ID, and populates the invoice_payment_status_views table
// so existing UI screens work immediately.
type InvoiceFinalizationService struct {
	store        store.Repository
	stripeClient *StripeClient
	secretStore  BillingSecretStore
	pdfService   *InvoicePDFService
}

func NewInvoiceFinalizationService(repo store.Repository, stripeClient *StripeClient, secretStore BillingSecretStore) *InvoiceFinalizationService {
	return &InvoiceFinalizationService{
		store:        repo,
		stripeClient: stripeClient,
		secretStore:  secretStore,
	}
}

func (s *InvoiceFinalizationService) WithPDFService(pdfService *InvoicePDFService) *InvoiceFinalizationService {
	s.pdfService = pdfService
	return s
}

type FinalizeInvoiceInput struct {
	TenantID  string
	InvoiceID string
	// StripeCustomerID is the Stripe customer ID (cus_xxx) for payment execution.
	StripeCustomerID string
}

type FinalizeInvoiceResult struct {
	Invoice              domain.Invoice
	StripePaymentIntentID string
}

// Finalize transitions a draft invoice to finalized by:
// 1. Creating a Stripe PaymentIntent for the invoice amount.
// 2. Storing the payment intent ID on the invoice.
// 3. Populating invoice_payment_status_views for backward-compatible UI reads.
// 4. Updating the invoice status to finalized + issued_at.
//
// For zero-amount invoices, no PaymentIntent is created and the invoice
// transitions directly to paid.
func (s *InvoiceFinalizationService) Finalize(ctx context.Context, input FinalizeInvoiceInput) (FinalizeInvoiceResult, error) {
	invoice, err := s.store.GetInvoice(input.TenantID, input.InvoiceID)
	if err != nil {
		return FinalizeInvoiceResult{}, fmt.Errorf("load invoice: %w", err)
	}
	if invoice.Status != domain.InvoiceStatusDraft {
		return FinalizeInvoiceResult{}, fmt.Errorf("invoice %s is %s, not draft", invoice.ID, invoice.Status)
	}

	now := time.Now().UTC()

	// Zero-amount invoices skip payment and go directly to paid.
	if invoice.TotalAmountCents == 0 {
		updated, err := s.store.UpdateInvoiceStatus(input.TenantID, invoice.ID, domain.InvoiceStatusPaid, now)
		if err != nil {
			return FinalizeInvoiceResult{}, fmt.Errorf("mark zero-amount invoice paid: %w", err)
		}
		paidNow := now
		updated, err = s.store.UpdateInvoicePayment(input.TenantID, invoice.ID, domain.InvoicePaymentSucceeded, "", "", &paidNow, now)
		if err != nil {
			return FinalizeInvoiceResult{}, fmt.Errorf("update zero-amount payment: %w", err)
		}
		return FinalizeInvoiceResult{Invoice: updated}, nil
	}

	// Retrieve the Stripe secret key for this tenant's billing connection.
	tenant, err := s.store.GetTenant(input.TenantID)
	if err != nil {
		return FinalizeInvoiceResult{}, fmt.Errorf("load tenant: %w", err)
	}
	if tenant.BillingProviderConnectionID == "" {
		return FinalizeInvoiceResult{}, fmt.Errorf("tenant %s has no billing provider connection", input.TenantID)
	}
	conn, err := s.store.GetBillingProviderConnection(tenant.BillingProviderConnectionID)
	if err != nil {
		return FinalizeInvoiceResult{}, fmt.Errorf("load billing connection: %w", err)
	}
	secrets, err := s.secretStore.GetConnectionSecrets(ctx, conn.SecretRef)
	if err != nil {
		return FinalizeInvoiceResult{}, fmt.Errorf("load stripe secret: %w", err)
	}

	// Create a Stripe PaymentIntent. The idempotency key is the invoice ID
	// to ensure exactly-once payment execution.
	pi, err := s.stripeClient.CreatePaymentIntent(ctx, secrets.StripeSecretKey, CreatePaymentIntentInput{
		AmountCents:    invoice.TotalAmountCents,
		Currency:       invoice.Currency,
		CustomerID:     input.StripeCustomerID,
		Description:    fmt.Sprintf("Invoice %s", invoice.InvoiceNumber),
		IdempotencyKey: fmt.Sprintf("inv_%s", invoice.ID),
		OffSession:     true,
		Confirm:        true,
		Metadata: map[string]string{
			"invoice_id":     invoice.ID,
			"invoice_number": invoice.InvoiceNumber,
			"tenant_id":      invoice.TenantID,
			"customer_id":    invoice.CustomerID,
		},
	})
	if err != nil {
		return FinalizeInvoiceResult{}, fmt.Errorf("create payment intent: %w", err)
	}

	// Update invoice with the PaymentIntent ID and finalized status.
	updated, err := s.store.UpdateInvoicePayment(
		input.TenantID, invoice.ID,
		domain.InvoicePaymentProcessing,
		pi.ID, "", nil, now,
	)
	if err != nil {
		return FinalizeInvoiceResult{}, fmt.Errorf("update invoice payment: %w", err)
	}

	updated, err = s.store.UpdateInvoiceStatus(input.TenantID, invoice.ID, domain.InvoiceStatusFinalized, now)
	if err != nil {
		return FinalizeInvoiceResult{}, fmt.Errorf("finalize invoice: %w", err)
	}

	// Populate invoice_payment_status_views for backward-compatible UI reads.
	_, err = s.store.UpsertInvoicePaymentStatusView(domain.InvoicePaymentStatusView{
		TenantID:             input.TenantID,
		InvoiceID:            invoice.ID,
		CustomerExternalID:   invoice.CustomerID,
		InvoiceNumber:        invoice.InvoiceNumber,
		Currency:             invoice.Currency,
		InvoiceStatus:        string(domain.InvoiceStatusFinalized),
		PaymentStatus:        "processing",
		PaymentOverdue:       ptrBool(false),
		TotalAmountCents:     &invoice.TotalAmountCents,
		TotalDueAmountCents:  &invoice.TotalAmountCents,
		TotalPaidAmountCents: intPtr(0),
		LastEventType:        "invoice.finalized",
		LastEventAt:          now,
		LastWebhookKey:       fmt.Sprintf("finalized_%s", invoice.ID),
	})
	if err != nil {
		// Non-fatal: the view is a convenience projection, not critical path.
		_ = err
	}

	// Generate and store PDF (non-fatal — invoice is already finalized).
	if s.pdfService != nil {
		lineItems, _ := s.store.ListInvoiceLineItems(input.TenantID, invoice.ID)
		pdfKey, pdfErr := s.pdfService.GenerateAndStore(updated, lineItems, input.StripeCustomerID)
		if pdfErr == nil && pdfKey != "" {
			_ = s.store.SetInvoicePDFKey(input.TenantID, invoice.ID, pdfKey)
			updated.PDFObjectKey = pdfKey
		}
	}

	return FinalizeInvoiceResult{
		Invoice:              updated,
		StripePaymentIntentID: pi.ID,
	}, nil
}

func intPtr(v int64) *int64    { return &v }
func ptrBool(v bool) *bool     { return &v }
