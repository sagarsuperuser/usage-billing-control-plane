package service

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

type stubTaxSyncAdapter struct {
	called bool
	tax    domain.Tax
}

func (s *stubTaxSyncAdapter) SyncTax(_ context.Context, tax domain.Tax) error {
	s.called = true
	s.tax = tax
	return nil
}

func TestTaxServiceCreateTax(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for integration tests")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := store.NewPostgresStore(db)
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	tenantID := "tenant_tax_service_" + time.Now().UTC().Format("20060102150405.000000000")
	if _, err := repo.CreateTenant(domain.Tenant{ID: tenantID, Name: "Tenant Tax Service", Status: domain.TenantStatusActive}); err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	adapter := &stubTaxSyncAdapter{}
	svc := NewTaxService(repo).WithSyncAdapter(adapter)

	item, err := svc.CreateTax(context.Background(), domain.Tax{
		TenantID: tenantID,
		Code:     "gst in",
		Name:     "GST India",
		Status:   domain.TaxStatusActive,
		Rate:     18,
	})
	if err != nil {
		t.Fatalf("create tax: %v", err)
	}
	if item.Code != "gst_in" {
		t.Fatalf("expected normalized code gst_in, got %q", item.Code)
	}
	if !adapter.called || adapter.tax.Code != "gst_in" {
		t.Fatalf("expected tax sync adapter to run, got %#v", adapter.tax)
	}
}
