package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/stripe/stripe-go/v82"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

// StripeWebhookService handles inbound Stripe webhook events.
// It handles Stripe events by:
// - Verifying the Stripe signature
// - Ingesting events into stripe_webhook_events (idempotent)
// - Updating invoice_payment_status_views (backward-compatible UI projection)
// - Triggering dunning side effects on payment failures
// - Triggering customer payment setup refresh on checkout completion
type StripeWebhookService struct {
	repo         store.Repository
	stripeClient *StripeClient
	customerSvc  *CustomerService
	dunningSvc   *DunningService
}

func NewStripeWebhookService(repo store.Repository, stripeClient *StripeClient, customerSvc *CustomerService) *StripeWebhookService {
	return &StripeWebhookService{
		repo:         repo,
		stripeClient: stripeClient,
		customerSvc:  customerSvc,
	}
}

func (s *StripeWebhookService) WithDunningService(dunningSvc *DunningService) *StripeWebhookService {
	if s == nil {
		return nil
	}
	s.dunningSvc = dunningSvc
	return s
}

type IngestStripeWebhookResult struct {
	Event      domain.StripeWebhookEvent `json:"event"`
	Idempotent bool                      `json:"idempotent"`
}

// Ingest verifies, stores, and processes a Stripe webhook event.
func (s *StripeWebhookService) Ingest(ctx context.Context, payload []byte, sigHeader string, webhookSecret string, tenantID string) (IngestStripeWebhookResult, error) {
	if s == nil || s.repo == nil {
		return IngestStripeWebhookResult{}, fmt.Errorf("%w: stripe webhook service is not configured", ErrValidation)
	}

	// Verify the Stripe webhook signature.
	stripeEvent, err := s.stripeClient.ConstructWebhookEvent(payload, sigHeader, webhookSecret)
	if err != nil {
		return IngestStripeWebhookResult{}, err
	}

	// Parse the event into our domain model.
	event := buildStripeWebhookEvent(stripeEvent, tenantID)

	// Ingest (idempotent by stripe_event_id).
	stored, idempotent, err := s.repo.IngestStripeWebhookEvent(event)
	if err != nil {
		return IngestStripeWebhookResult{}, err
	}

	if !idempotent {
		// Apply side effects only on first ingestion.
		if err := s.applyInvoicePaymentEffects(stored); err != nil {
			// Log but don't fail — the event is already stored.
			_ = err
		}
		if err := s.applyCustomerEffects(stored); err != nil {
			_ = err
		}
		if err := s.applyDunningEffects(stored); err != nil {
			_ = err
		}
	}

	return IngestStripeWebhookResult{
		Event:      stored,
		Idempotent: idempotent,
	}, nil
}

// applyInvoicePaymentEffects updates the invoice and payment status views
// when a payment intent event is received.
func (s *StripeWebhookService) applyInvoicePaymentEffects(event domain.StripeWebhookEvent) error {
	if event.InvoiceID == "" {
		return nil
	}

	now := time.Now().UTC()
	var paymentStatus string
	var paymentOverdue bool

	switch event.EventType {
	case "payment_intent.succeeded":
		paymentStatus = "succeeded"
		paidAt := now
		_, err := s.repo.UpdateInvoicePayment(
			event.TenantID, event.InvoiceID,
			domain.InvoicePaymentSucceeded,
			event.PaymentIntentID, "", &paidAt, now,
		)
		if err != nil {
			// May fail for legacy Lago invoices — that's OK.
			_ = err
		}
		// Also mark invoice as paid.
		_, _ = s.repo.UpdateInvoiceStatus(event.TenantID, event.InvoiceID, domain.InvoiceStatusPaid, now)

	case "payment_intent.payment_failed":
		paymentStatus = "failed"
		_, err := s.repo.UpdateInvoicePayment(
			event.TenantID, event.InvoiceID,
			domain.InvoicePaymentFailed,
			event.PaymentIntentID, event.FailureMessage, nil, now,
		)
		if err != nil {
			_ = err
		}

	default:
		return nil
	}

	// Update the invoice_payment_status_views table for backward-compatible UI reads.
	var totalAmountCents *int64
	if event.AmountCents != nil {
		totalAmountCents = event.AmountCents
	}

	_, err := s.repo.UpsertInvoicePaymentStatusView(domain.InvoicePaymentStatusView{
		TenantID:             event.TenantID,
		InvoiceID:            event.InvoiceID,
		CustomerExternalID:   event.CustomerExternalID,
		Currency:             event.Currency,
		PaymentStatus:        paymentStatus,
		PaymentOverdue:       &paymentOverdue,
		TotalAmountCents:     totalAmountCents,
		LastPaymentError:     event.FailureMessage,
		LastEventType:        event.EventType,
		LastEventAt:          event.OccurredAt,
		LastWebhookKey:       event.StripeEventID,
	})
	if err != nil {
		return fmt.Errorf("upsert invoice payment status view: %w", err)
	}
	return nil
}

// applyCustomerEffects handles checkout session and payment method events.
func (s *StripeWebhookService) applyCustomerEffects(event domain.StripeWebhookEvent) error {
	if s.customerSvc == nil || event.CustomerExternalID == "" || event.TenantID == "" {
		return nil
	}

	switch event.EventType {
	case "checkout.session.completed":
		_, err := s.customerSvc.RefreshCustomerPaymentSetup(event.TenantID, event.CustomerExternalID)
		if err != nil {
			return fmt.Errorf("refresh customer payment setup (checkout completed): %w", err)
		}
		return nil

	case "setup_intent.setup_failed":
		msg := event.FailureMessage
		if msg == "" {
			msg = "payment method setup failed"
		}
		_, err := s.customerSvc.RecordCustomerPaymentProviderError(event.TenantID, event.CustomerExternalID, msg)
		if err != nil {
			return fmt.Errorf("record customer payment provider error: %w", err)
		}
		return nil

	case "payment_method.attached":
		_, err := s.customerSvc.RefreshCustomerPaymentSetup(event.TenantID, event.CustomerExternalID)
		if err != nil {
			return fmt.Errorf("refresh customer payment setup (payment method attached): %w", err)
		}
		return nil

	default:
		return nil
	}
}

// applyDunningEffects triggers or resolves dunning runs based on payment events.
func (s *StripeWebhookService) applyDunningEffects(event domain.StripeWebhookEvent) error {
	if s.dunningSvc == nil || event.TenantID == "" || event.InvoiceID == "" {
		return nil
	}
	switch event.EventType {
	case "payment_intent.payment_failed":
		_, err := s.dunningSvc.EnsureRunForInvoice(event.TenantID, event.InvoiceID)
		if err != nil {
			return fmt.Errorf("ensure dunning run for invoice: %w", err)
		}
		return nil
	case "payment_intent.succeeded":
		if err := s.dunningSvc.ResolveRunByInvoiceID(event.TenantID, event.InvoiceID, "payment_succeeded"); err != nil {
			return fmt.Errorf("resolve dunning run by invoice: %w", err)
		}
		return nil
	default:
		return nil
	}
}

// buildStripeWebhookEvent converts a Stripe SDK event into our domain model.
func buildStripeWebhookEvent(stripeEvent stripe.Event, tenantID string) domain.StripeWebhookEvent {
	event := domain.StripeWebhookEvent{
		TenantID:      tenantID,
		StripeEventID: stripeEvent.ID,
		EventType:     string(stripeEvent.Type),
		ObjectType:    stripeEvent.Data.Object["object"].(string),
		OccurredAt:    time.Unix(stripeEvent.Created, 0).UTC(),
	}

	// Parse the raw payload for storage.
	var payload map[string]any
	if err := json.Unmarshal(stripeEvent.Data.Raw, &payload); err == nil {
		event.Payload = payload
	}

	// Extract common fields from the event data object.
	data := stripeEvent.Data.Object

	if id, ok := data["id"].(string); ok {
		event.PaymentIntentID = id
	}
	if customer, ok := data["customer"].(string); ok {
		event.CustomerExternalID = customer
	}
	if amount, ok := data["amount"].(float64); ok {
		cents := int64(amount)
		event.AmountCents = &cents
	}
	if currency, ok := data["currency"].(string); ok {
		event.Currency = strings.ToUpper(currency)
	}

	// Extract failure message from last_payment_error.
	if lastErr, ok := data["last_payment_error"].(map[string]any); ok {
		if msg, ok := lastErr["message"].(string); ok {
			event.FailureMessage = msg
		}
	}

	// Extract invoice ID and tenant ID from metadata (set during PaymentIntent creation).
	if metadata, ok := data["metadata"].(map[string]any); ok {
		if invID, ok := metadata["invoice_id"].(string); ok {
			event.InvoiceID = invID
		}
		// Use tenant_id from metadata as fallback when not provided by the handler.
		if event.TenantID == "" {
			if tid, ok := metadata["tenant_id"].(string); ok {
				event.TenantID = tid
			}
		}
	}

	return event
}
