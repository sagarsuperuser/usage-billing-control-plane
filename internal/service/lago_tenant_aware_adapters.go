package service

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

type TenantAwareLagoMeterSyncAdapter struct {
	resolver LagoTransportResolver
}

func NewTenantAwareLagoMeterSyncAdapter(resolver LagoTransportResolver) *TenantAwareLagoMeterSyncAdapter {
	return &TenantAwareLagoMeterSyncAdapter{resolver: resolver}
}

func (a *TenantAwareLagoMeterSyncAdapter) SyncMeter(ctx context.Context, meter domain.Meter) error {
	transport, err := resolveLagoTransport(ctx, nil, a.resolver, meter.TenantID, "")
	if err != nil {
		return err
	}
	return NewLagoMeterSyncAdapter(transport).SyncMeter(ctx, meter)
}

type TenantAwareLagoTaxSyncAdapter struct {
	resolver LagoTransportResolver
}

func NewTenantAwareLagoTaxSyncAdapter(resolver LagoTransportResolver) *TenantAwareLagoTaxSyncAdapter {
	return &TenantAwareLagoTaxSyncAdapter{resolver: resolver}
}

func (a *TenantAwareLagoTaxSyncAdapter) SyncTax(ctx context.Context, tax domain.Tax) error {
	transport, err := resolveLagoTransport(ctx, nil, a.resolver, tax.TenantID, "")
	if err != nil {
		return err
	}
	return NewLagoTaxSyncAdapter(transport).SyncTax(ctx, tax)
}

type TenantAwareLagoPlanSyncAdapter struct {
	resolver LagoTransportResolver
	repo     store.Repository
}

func NewTenantAwareLagoPlanSyncAdapter(resolver LagoTransportResolver, repo store.Repository) *TenantAwareLagoPlanSyncAdapter {
	return &TenantAwareLagoPlanSyncAdapter{resolver: resolver, repo: repo}
}

func (a *TenantAwareLagoPlanSyncAdapter) SyncPlan(ctx context.Context, plan domain.Plan, components []PlanSyncComponent) error {
	transport, err := resolveLagoTransport(ctx, nil, a.resolver, plan.TenantID, "")
	if err != nil {
		return err
	}
	return NewLagoPlanSyncAdapter(transport, a.repo).SyncPlan(ctx, plan, components)
}

type TenantAwareLagoSubscriptionSyncAdapter struct {
	resolver LagoTransportResolver
	repo     store.Repository
}

func NewTenantAwareLagoSubscriptionSyncAdapter(resolver LagoTransportResolver, repo store.Repository) *TenantAwareLagoSubscriptionSyncAdapter {
	return &TenantAwareLagoSubscriptionSyncAdapter{resolver: resolver, repo: repo}
}

func (a *TenantAwareLagoSubscriptionSyncAdapter) SyncSubscription(ctx context.Context, subscription domain.Subscription, customer domain.Customer, plan domain.Plan) error {
	transport, err := resolveLagoTransport(ctx, nil, a.resolver, subscription.TenantID, "")
	if err != nil {
		return err
	}
	return NewLagoSubscriptionSyncAdapter(transport, a.repo).SyncSubscription(ctx, subscription, customer, plan)
}

type TenantAwareLagoUsageSyncAdapter struct {
	resolver LagoTransportResolver
}

func NewTenantAwareLagoUsageSyncAdapter(resolver LagoTransportResolver) *TenantAwareLagoUsageSyncAdapter {
	return &TenantAwareLagoUsageSyncAdapter{resolver: resolver}
}

func (a *TenantAwareLagoUsageSyncAdapter) SyncUsageEvent(ctx context.Context, event domain.UsageEvent, meter domain.Meter, subscription domain.Subscription) error {
	tenantID := strings.TrimSpace(event.TenantID)
	if tenantID == "" {
		tenantID = strings.TrimSpace(subscription.TenantID)
	}
	transport, err := resolveLagoTransport(ctx, nil, a.resolver, tenantID, "")
	if err != nil {
		return err
	}
	return NewLagoUsageSyncAdapter(transport).SyncUsageEvent(ctx, event, meter, subscription)
}

type TenantAwareLagoCustomerBillingAdapter struct {
	resolver LagoTransportResolver
}

func NewTenantAwareLagoCustomerBillingAdapter(resolver LagoTransportResolver) *TenantAwareLagoCustomerBillingAdapter {
	return &TenantAwareLagoCustomerBillingAdapter{resolver: resolver}
}

func (a *TenantAwareLagoCustomerBillingAdapter) UpsertCustomer(ctx context.Context, payload []byte) (int, []byte, error) {
	scope := lagoScopeFromContext(ctx)
	transport, err := resolveLagoTransport(ctx, nil, a.resolver, scope.TenantID, scope.OrganizationID)
	if err != nil {
		return 0, nil, err
	}
	return NewLagoCustomerBillingAdapter(transport).UpsertCustomer(ctx, payload)
}

func (a *TenantAwareLagoCustomerBillingAdapter) GetCustomer(ctx context.Context, externalID string) (int, []byte, error) {
	scope := lagoScopeFromContext(ctx)
	transport, err := resolveLagoTransport(ctx, nil, a.resolver, scope.TenantID, scope.OrganizationID)
	if err != nil {
		return 0, nil, err
	}
	return NewLagoCustomerBillingAdapter(transport).GetCustomer(ctx, externalID)
}

func (a *TenantAwareLagoCustomerBillingAdapter) ListCustomerPaymentMethods(ctx context.Context, externalID string) (int, []byte, error) {
	scope := lagoScopeFromContext(ctx)
	transport, err := resolveLagoTransport(ctx, nil, a.resolver, scope.TenantID, scope.OrganizationID)
	if err != nil {
		return 0, nil, err
	}
	return NewLagoCustomerBillingAdapter(transport).ListCustomerPaymentMethods(ctx, externalID)
}

func (a *TenantAwareLagoCustomerBillingAdapter) GenerateCustomerCheckoutURL(ctx context.Context, externalID string) (int, []byte, error) {
	scope := lagoScopeFromContext(ctx)
	transport, err := resolveLagoTransport(ctx, nil, a.resolver, scope.TenantID, scope.OrganizationID)
	if err != nil {
		return 0, nil, err
	}
	return NewLagoCustomerBillingAdapter(transport).GenerateCustomerCheckoutURL(ctx, externalID)
}

func (a *TenantAwareLagoCustomerBillingAdapter) SyncBillingEntitySettings(ctx context.Context, settings domain.WorkspaceBillingSettings) error {
	transport, err := resolveLagoTransport(ctx, nil, a.resolver, settings.WorkspaceID, "")
	if err != nil {
		return err
	}
	return NewLagoCustomerBillingAdapter(transport).SyncBillingEntitySettings(ctx, settings)
}

type TenantAwareLagoInvoiceAdapter struct {
	resolver LagoTransportResolver
}

func NewTenantAwareLagoInvoiceAdapter(resolver LagoTransportResolver) *TenantAwareLagoInvoiceAdapter {
	return &TenantAwareLagoInvoiceAdapter{resolver: resolver}
}

func (a *TenantAwareLagoInvoiceAdapter) ListInvoices(ctx context.Context, query url.Values) (int, []byte, error) {
	scope := lagoScopeFromContext(ctx)
	transport, err := resolveLagoTransport(ctx, nil, a.resolver, scope.TenantID, scope.OrganizationID)
	if err != nil {
		return 0, nil, err
	}
	return NewLagoInvoiceAdapter(transport).ListInvoices(ctx, query)
}

func (a *TenantAwareLagoInvoiceAdapter) ListPaymentReceipts(ctx context.Context, query url.Values) (int, []byte, error) {
	scope := lagoScopeFromContext(ctx)
	transport, err := resolveLagoTransport(ctx, nil, a.resolver, scope.TenantID, scope.OrganizationID)
	if err != nil {
		return 0, nil, err
	}
	return NewLagoInvoiceAdapter(transport).ListPaymentReceipts(ctx, query)
}

func (a *TenantAwareLagoInvoiceAdapter) ListCreditNotes(ctx context.Context, query url.Values) (int, []byte, error) {
	scope := lagoScopeFromContext(ctx)
	transport, err := resolveLagoTransport(ctx, nil, a.resolver, scope.TenantID, scope.OrganizationID)
	if err != nil {
		return 0, nil, err
	}
	return NewLagoInvoiceAdapter(transport).ListCreditNotes(ctx, query)
}

func (a *TenantAwareLagoInvoiceAdapter) PreviewInvoice(ctx context.Context, payload []byte) (int, []byte, error) {
	scope := lagoScopeFromContext(ctx)
	transport, err := resolveLagoTransport(ctx, nil, a.resolver, scope.TenantID, scope.OrganizationID)
	if err != nil {
		return 0, nil, err
	}
	return NewLagoInvoiceAdapter(transport).PreviewInvoice(ctx, payload)
}

func (a *TenantAwareLagoInvoiceAdapter) RetryInvoicePayment(ctx context.Context, invoiceID string, payload []byte) (int, []byte, error) {
	scope := lagoScopeFromContext(ctx)
	transport, err := resolveLagoTransport(ctx, nil, a.resolver, scope.TenantID, scope.OrganizationID)
	if err != nil {
		return 0, nil, err
	}
	return NewLagoInvoiceAdapter(transport).RetryInvoicePayment(ctx, invoiceID, payload)
}

func (a *TenantAwareLagoInvoiceAdapter) GetInvoice(ctx context.Context, invoiceID string) (int, []byte, error) {
	scope := lagoScopeFromContext(ctx)
	transport, err := resolveLagoTransport(ctx, nil, a.resolver, scope.TenantID, scope.OrganizationID)
	if err != nil {
		return 0, nil, err
	}
	return NewLagoInvoiceAdapter(transport).GetInvoice(ctx, invoiceID)
}

func (a *TenantAwareLagoInvoiceAdapter) ResendInvoiceEmail(ctx context.Context, invoiceID string, input BillingDocumentEmail) error {
	scope := lagoScopeFromContext(ctx)
	transport, err := resolveLagoTransport(ctx, nil, a.resolver, scope.TenantID, scope.OrganizationID)
	if err != nil {
		return err
	}
	return NewLagoInvoiceAdapter(transport).ResendInvoiceEmail(ctx, invoiceID, input)
}

func (a *TenantAwareLagoInvoiceAdapter) ResendPaymentReceiptEmail(ctx context.Context, paymentReceiptID string, input BillingDocumentEmail) error {
	scope := lagoScopeFromContext(ctx)
	transport, err := resolveLagoTransport(ctx, nil, a.resolver, scope.TenantID, scope.OrganizationID)
	if err != nil {
		return err
	}
	return NewLagoInvoiceAdapter(transport).ResendPaymentReceiptEmail(ctx, paymentReceiptID, input)
}

func (a *TenantAwareLagoInvoiceAdapter) ResendCreditNoteEmail(ctx context.Context, creditNoteID string, input BillingDocumentEmail) error {
	scope := lagoScopeFromContext(ctx)
	transport, err := resolveLagoTransport(ctx, nil, a.resolver, scope.TenantID, scope.OrganizationID)
	if err != nil {
		return err
	}
	return NewLagoInvoiceAdapter(transport).ResendCreditNoteEmail(ctx, creditNoteID, input)
}

type TenantAwareLagoBillingProviderAdapter struct {
	resolver                 LagoTransportResolver
	stripeSuccessRedirectURL string
}

func NewTenantAwareLagoBillingProviderAdapter(resolver LagoTransportResolver, stripeSuccessRedirectURL string) *TenantAwareLagoBillingProviderAdapter {
	return &TenantAwareLagoBillingProviderAdapter{resolver: resolver, stripeSuccessRedirectURL: strings.TrimSpace(stripeSuccessRedirectURL)}
}

func (a *TenantAwareLagoBillingProviderAdapter) EnsureStripeProvider(ctx context.Context, input EnsureStripeProviderInput) (EnsureStripeProviderResult, error) {
	transport, err := resolveLagoTransport(ctx, nil, a.resolver, input.OwnerTenantID, input.LagoOrganizationID)
	if err != nil {
		return EnsureStripeProviderResult{}, err
	}
	return NewLagoBillingProviderAdapter(transport, a.stripeSuccessRedirectURL).EnsureStripeProvider(ctx, input)
}

var (
	_ MeterSyncAdapter                 = (*TenantAwareLagoMeterSyncAdapter)(nil)
	_ TaxSyncAdapter                   = (*TenantAwareLagoTaxSyncAdapter)(nil)
	_ PlanSyncAdapter                  = (*TenantAwareLagoPlanSyncAdapter)(nil)
	_ SubscriptionSyncAdapter          = (*TenantAwareLagoSubscriptionSyncAdapter)(nil)
	_ UsageSyncAdapter                 = (*TenantAwareLagoUsageSyncAdapter)(nil)
	_ CustomerBillingAdapter           = (*TenantAwareLagoCustomerBillingAdapter)(nil)
	_ BillingEntitySettingsSyncAdapter = (*TenantAwareLagoCustomerBillingAdapter)(nil)
	_ InvoiceBillingAdapter            = (*TenantAwareLagoInvoiceAdapter)(nil)
	_ BillingProviderAdapter           = (*TenantAwareLagoBillingProviderAdapter)(nil)
)

func requireTenantAwareResolver(resolver LagoTransportResolver) error {
	if resolver == nil {
		return fmt.Errorf("%w: lago transport resolver is required", ErrValidation)
	}
	return nil
}
