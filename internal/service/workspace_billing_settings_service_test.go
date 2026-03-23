package service

import (
	"context"
	"testing"

	"usage-billing-control-plane/internal/domain"
)

type testBillingEntitySyncAdapter struct {
	called   bool
	settings domain.WorkspaceBillingSettings
	err      error
}

func (a *testBillingEntitySyncAdapter) SyncBillingEntitySettings(_ context.Context, settings domain.WorkspaceBillingSettings) error {
	a.called = true
	a.settings = settings
	return a.err
}

func TestWorkspaceBillingSettingsServiceSyncWorkspaceBillingSettings(t *testing.T) {
	adapter := &testBillingEntitySyncAdapter{}
	svc := NewWorkspaceBillingSettingsService(nil).WithBillingEntitySyncAdapter(adapter)
	netTerms := 30
	invoiceGracePeriodDays := 7

	err := svc.syncWorkspaceBillingSettings(domain.WorkspaceBillingSettings{
		WorkspaceID:            "tenant_sync",
		BillingEntityCode:      "be_primary",
		NetPaymentTermDays:     &netTerms,
		TaxCodes:               []string{"GST_IN"},
		InvoiceFooter:          "Wire details available on request.",
		DocumentLocale:         "fr",
		InvoiceGracePeriodDays: &invoiceGracePeriodDays,
		DocumentNumbering:      "per_billing_entity",
		DocumentNumberPrefix:   "ALPHA-",
	})
	if err != nil {
		t.Fatalf("sync workspace billing settings: %v", err)
	}
	if !adapter.called {
		t.Fatalf("expected billing entity sync adapter to be called")
	}
	if adapter.settings.BillingEntityCode != "be_primary" {
		t.Fatalf("expected billing entity code be_primary, got %q", adapter.settings.BillingEntityCode)
	}
	if len(adapter.settings.TaxCodes) != 1 || adapter.settings.TaxCodes[0] != "GST_IN" {
		t.Fatalf("expected tax codes to round-trip, got %#v", adapter.settings.TaxCodes)
	}
	if adapter.settings.DocumentLocale != "fr" {
		t.Fatalf("expected document locale fr, got %q", adapter.settings.DocumentLocale)
	}
	if adapter.settings.InvoiceGracePeriodDays == nil || *adapter.settings.InvoiceGracePeriodDays != 7 {
		t.Fatalf("expected invoice grace period 7, got %#v", adapter.settings.InvoiceGracePeriodDays)
	}
}

func TestWorkspaceBillingSettingsServiceSyncWorkspaceBillingSettingsSkipsWithoutBillingEntity(t *testing.T) {
	adapter := &testBillingEntitySyncAdapter{}
	svc := NewWorkspaceBillingSettingsService(nil).WithBillingEntitySyncAdapter(adapter)

	err := svc.syncWorkspaceBillingSettings(domain.WorkspaceBillingSettings{
		WorkspaceID:   "tenant_sync",
		InvoiceFooter: "ignored without billing entity",
	})
	if err != nil {
		t.Fatalf("sync workspace billing settings without billing entity: %v", err)
	}
	if adapter.called {
		t.Fatalf("expected billing entity sync adapter to be skipped")
	}
}
