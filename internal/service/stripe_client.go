package service

import (
	"context"
	"fmt"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/client"
	"github.com/stripe/stripe-go/v82/webhook"
)

// StripeClient wraps stripe-go for multi-tenant use.
// Each call resolves a per-tenant Stripe API key via BillingSecretStore.
// Never set stripe.Key globally — always create a client.API per-key.
type StripeClient struct{}

func NewStripeClient() *StripeClient {
	return &StripeClient{}
}

// newAPI creates a stripe client.API bound to the given secret key.
func (c *StripeClient) newAPI(secretKey string) *client.API {
	sc := &client.API{}
	sc.Init(secretKey, nil)
	return sc
}

// ---------------------------------------------------------------------------
// Payment Intents
// ---------------------------------------------------------------------------

type CreatePaymentIntentInput struct {
	AmountCents    int64
	Currency       string
	CustomerID     string // Stripe customer ID (cus_xxx)
	Description    string
	IdempotencyKey string
}

func (c *StripeClient) CreatePaymentIntent(_ context.Context, secretKey string, input CreatePaymentIntentInput) (*stripe.PaymentIntent, error) {
	sc := c.newAPI(secretKey)
	params := &stripe.PaymentIntentParams{
		Amount:      stripe.Int64(input.AmountCents),
		Currency:    stripe.String(input.Currency),
		Customer:    stripe.String(input.CustomerID),
		Description: stripe.String(input.Description),
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled: stripe.Bool(true),
		},
	}
	if input.IdempotencyKey != "" {
		params.IdempotencyKey = stripe.String(input.IdempotencyKey)
	}
	return sc.PaymentIntents.New(params)
}

func (c *StripeClient) ConfirmPaymentIntent(_ context.Context, secretKey string, paymentIntentID string) (*stripe.PaymentIntent, error) {
	sc := c.newAPI(secretKey)
	params := &stripe.PaymentIntentConfirmParams{}
	return sc.PaymentIntents.Confirm(paymentIntentID, params)
}

func (c *StripeClient) GetPaymentIntent(_ context.Context, secretKey string, paymentIntentID string) (*stripe.PaymentIntent, error) {
	sc := c.newAPI(secretKey)
	params := &stripe.PaymentIntentParams{}
	return sc.PaymentIntents.Get(paymentIntentID, params)
}

// ---------------------------------------------------------------------------
// Customers
// ---------------------------------------------------------------------------

type CreateStripeCustomerInput struct {
	Email       string
	Name        string
	Description string
	Metadata    map[string]string
}

func (c *StripeClient) CreateCustomer(_ context.Context, secretKey string, input CreateStripeCustomerInput) (*stripe.Customer, error) {
	sc := c.newAPI(secretKey)
	params := &stripe.CustomerParams{
		Email:       stripe.String(input.Email),
		Name:        stripe.String(input.Name),
		Description: stripe.String(input.Description),
	}
	for k, v := range input.Metadata {
		params.AddMetadata(k, v)
	}
	return sc.Customers.New(params)
}

func (c *StripeClient) UpdateCustomer(_ context.Context, secretKey string, customerID string, input CreateStripeCustomerInput) (*stripe.Customer, error) {
	sc := c.newAPI(secretKey)
	params := &stripe.CustomerParams{
		Email:       stripe.String(input.Email),
		Name:        stripe.String(input.Name),
		Description: stripe.String(input.Description),
	}
	for k, v := range input.Metadata {
		params.AddMetadata(k, v)
	}
	return sc.Customers.Update(customerID, params)
}

// ---------------------------------------------------------------------------
// Payment Methods
// ---------------------------------------------------------------------------

func (c *StripeClient) ListPaymentMethods(_ context.Context, secretKey string, customerID string) ([]*stripe.PaymentMethod, error) {
	sc := c.newAPI(secretKey)
	params := &stripe.PaymentMethodListParams{
		Customer: stripe.String(customerID),
	}
	iter := sc.PaymentMethods.List(params)
	var methods []*stripe.PaymentMethod
	for iter.Next() {
		methods = append(methods, iter.PaymentMethod())
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	return methods, nil
}

// ---------------------------------------------------------------------------
// Checkout Sessions (payment method setup)
// ---------------------------------------------------------------------------

type CreateCheckoutSessionInput struct {
	CustomerID string // Stripe customer ID
	SuccessURL string
	CancelURL  string
}

func (c *StripeClient) CreateCheckoutSession(_ context.Context, secretKey string, input CreateCheckoutSessionInput) (*stripe.CheckoutSession, error) {
	sc := c.newAPI(secretKey)
	params := &stripe.CheckoutSessionParams{
		Mode:       stripe.String(string(stripe.CheckoutSessionModeSetup)),
		Customer:   stripe.String(input.CustomerID),
		SuccessURL: stripe.String(input.SuccessURL),
		CancelURL:  stripe.String(input.CancelURL),
	}
	return sc.CheckoutSessions.New(params)
}

// ---------------------------------------------------------------------------
// Refunds
// ---------------------------------------------------------------------------

func (c *StripeClient) CreateRefund(_ context.Context, secretKey string, paymentIntentID string, amountCents int64) (*stripe.Refund, error) {
	sc := c.newAPI(secretKey)
	params := &stripe.RefundParams{
		PaymentIntent: stripe.String(paymentIntentID),
	}
	if amountCents > 0 {
		params.Amount = stripe.Int64(amountCents)
	}
	return sc.Refunds.New(params)
}

// ---------------------------------------------------------------------------
// Webhook verification (stateless — does not need a key-bound client)
// ---------------------------------------------------------------------------

func (c *StripeClient) ConstructWebhookEvent(payload []byte, sigHeader string, webhookSecret string) (stripe.Event, error) {
	event, err := webhook.ConstructEvent(payload, sigHeader, webhookSecret)
	if err != nil {
		return stripe.Event{}, fmt.Errorf("stripe webhook signature verification failed: %w", err)
	}
	return event, nil
}
