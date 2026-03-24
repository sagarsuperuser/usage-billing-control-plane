package service

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

type stubBillingProviderAdapter struct {
	result EnsureStripeProviderResult
	err    error
	calls  int
	last   EnsureStripeProviderInput
}

func (s *stubBillingProviderAdapter) EnsureStripeProvider(_ context.Context, input EnsureStripeProviderInput) (EnsureStripeProviderResult, error) {
	s.calls++
	s.last = input
	if s.err != nil {
		return EnsureStripeProviderResult{}, s.err
	}
	return s.result, nil
}

func TestMemoryBillingSecretStoreLifecycle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewMemoryBillingSecretStore()
	secretRef, err := store.PutStripeSecret(ctx, "bpc_test", "sk_test_123")
	if err != nil {
		t.Fatalf("put secret: %v", err)
	}
	secret, err := store.GetStripeSecret(ctx, secretRef)
	if err != nil {
		t.Fatalf("get secret: %v", err)
	}
	if secret != "sk_test_123" {
		t.Fatalf("expected original secret, got %q", secret)
	}
	rotatedRef, err := store.RotateStripeSecret(ctx, secretRef, "sk_test_456")
	if err != nil {
		t.Fatalf("rotate secret: %v", err)
	}
	if rotatedRef == secretRef {
		t.Fatalf("expected rotated secret ref to change")
	}
	rotatedSecret, err := store.GetStripeSecret(ctx, rotatedRef)
	if err != nil {
		t.Fatalf("get rotated secret: %v", err)
	}
	if rotatedSecret != "sk_test_456" {
		t.Fatalf("expected rotated secret, got %q", rotatedSecret)
	}
	if _, err := store.GetStripeSecret(ctx, secretRef); err == nil {
		t.Fatalf("expected old ref lookup to fail")
	}
}

func TestBillingProviderConnectionService_CreateAndRotate(t *testing.T) {
	repo := newTestBillingProviderRepo(t)
	secretStore := NewMemoryBillingSecretStore()
	svc := NewBillingProviderConnectionService(repo, secretStore, nil)

	created, err := svc.CreateBillingProviderConnection(context.Background(), CreateBillingProviderConnectionRequest{
		ProviderType:    "stripe",
		Environment:     "test",
		DisplayName:     "Stripe Test",
		Scope:           "platform",
		StripeSecretKey: "sk_test_create",
	}, "platform_api_key", "pkey_1")
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}
	if created.Status != domain.BillingProviderConnectionStatusPending {
		t.Fatalf("expected pending status, got %q", created.Status)
	}
	if created.SecretRef == "" {
		t.Fatalf("expected secret ref")
	}
	if strings.Contains(created.SecretRef, "sk_test_create") {
		t.Fatalf("secret ref leaked secret material")
	}
	secret, err := secretStore.GetStripeSecret(context.Background(), created.SecretRef)
	if err != nil {
		t.Fatalf("get stored secret: %v", err)
	}
	if secret != "sk_test_create" {
		t.Fatalf("expected stored secret, got %q", secret)
	}

	rotated, err := svc.RotateBillingProviderConnectionSecret(context.Background(), created.ID, "sk_test_rotated")
	if err != nil {
		t.Fatalf("rotate secret: %v", err)
	}
	if rotated.SecretRef == created.SecretRef {
		t.Fatalf("expected secret ref to change on rotate")
	}
	if rotated.Status != domain.BillingProviderConnectionStatusPending {
		t.Fatalf("expected status pending after rotate, got %q", rotated.Status)
	}
	rotatedSecret, err := secretStore.GetStripeSecret(context.Background(), rotated.SecretRef)
	if err != nil {
		t.Fatalf("get rotated secret: %v", err)
	}
	if rotatedSecret != "sk_test_rotated" {
		t.Fatalf("expected rotated secret, got %q", rotatedSecret)
	}
}

func TestBillingProviderConnectionService_SyncSuccess(t *testing.T) {
	repo := newTestBillingProviderRepo(t)
	secretStore := NewMemoryBillingSecretStore()
	adapter := &stubBillingProviderAdapter{result: EnsureStripeProviderResult{
		LagoOrganizationID: "org_sync",
		LagoProviderCode:   "stripe_sync",
		ConnectedAt:        time.Now().UTC(),
		LastSyncedAt:       time.Now().UTC(),
	}}
	svc := NewBillingProviderConnectionService(repo, secretStore, adapter)

	created, err := svc.CreateBillingProviderConnection(context.Background(), CreateBillingProviderConnectionRequest{
		ProviderType:       "stripe",
		Environment:        "test",
		DisplayName:        "Stripe Sync",
		Scope:              "platform",
		StripeSecretKey:    "sk_test_sync",
		LagoOrganizationID: "org_seed",
		LagoProviderCode:   "stripe_seed",
	}, "platform_api_key", "pkey_sync")
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}

	synced, err := svc.SyncBillingProviderConnection(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("sync connection: %v", err)
	}
	if adapter.calls != 1 {
		t.Fatalf("expected adapter call, got %d", adapter.calls)
	}
	if adapter.last.SecretKey != "sk_test_sync" {
		t.Fatalf("expected adapter to receive secret, got %q", adapter.last.SecretKey)
	}
	if synced.Status != domain.BillingProviderConnectionStatusConnected {
		t.Fatalf("expected connected status, got %q", synced.Status)
	}
	if synced.LagoOrganizationID != "org_sync" || synced.LagoProviderCode != "stripe_sync" {
		t.Fatalf("expected synced lago mapping, got org=%q provider=%q", synced.LagoOrganizationID, synced.LagoProviderCode)
	}
	if synced.LastSyncedAt == nil {
		t.Fatalf("expected last_synced_at to be set")
	}
}

func TestBillingProviderConnectionService_SyncMissingLagoOrganizationID(t *testing.T) {
	repo := newTestBillingProviderRepo(t)
	secretStore := NewMemoryBillingSecretStore()
	adapter := &stubBillingProviderAdapter{}
	svc := NewBillingProviderConnectionService(repo, secretStore, adapter)

	created, err := svc.CreateBillingProviderConnection(context.Background(), CreateBillingProviderConnectionRequest{
		ProviderType:    "stripe",
		Environment:     "test",
		DisplayName:     "Stripe Missing Org",
		Scope:           "platform",
		StripeSecretKey: "sk_test_missing_org",
	}, "platform_api_key", "pkey_missing_org")
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}

	_, err = svc.SyncBillingProviderConnection(context.Background(), created.ID)
	if err == nil || !strings.Contains(err.Error(), "lago organization id is required") {
		t.Fatalf("expected missing lago organization validation error, got %v", err)
	}
	if adapter.calls != 0 {
		t.Fatalf("expected adapter not to be called, got %d", adapter.calls)
	}
}

func TestBillingProviderConnectionService_SyncUsesDefaultLagoOrganizationID(t *testing.T) {
	repo := newTestBillingProviderRepo(t)
	secretStore := NewMemoryBillingSecretStore()
	adapter := &stubBillingProviderAdapter{result: EnsureStripeProviderResult{
		LagoProviderCode: "stripe_default_org",
		ConnectedAt:      time.Now().UTC(),
		LastSyncedAt:     time.Now().UTC(),
	}}
	svc := NewBillingProviderConnectionService(repo, secretStore, adapter).WithDefaultLagoOrganizationID("org_default")

	created, err := svc.CreateBillingProviderConnection(context.Background(), CreateBillingProviderConnectionRequest{
		ProviderType:    "stripe",
		Environment:     "test",
		DisplayName:     "Stripe Default Org",
		Scope:           "platform",
		StripeSecretKey: "sk_test_default_org",
	}, "platform_api_key", "pkey_default_org")
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}

	synced, err := svc.SyncBillingProviderConnection(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("sync connection with default org: %v", err)
	}
	if adapter.last.LagoOrganizationID != "org_default" {
		t.Fatalf("expected default org to be sent to adapter, got %q", adapter.last.LagoOrganizationID)
	}
	if synced.LagoOrganizationID != "org_default" {
		t.Fatalf("expected synced connection to persist default org, got %q", synced.LagoOrganizationID)
	}
}

func TestBillingProviderConnectionService_SyncFailurePersistsState(t *testing.T) {
	repo := newTestBillingProviderRepo(t)
	secretStore := NewMemoryBillingSecretStore()
	adapter := &stubBillingProviderAdapter{err: errors.New("lago sync failed")}
	svc := NewBillingProviderConnectionService(repo, secretStore, adapter)

	created, err := svc.CreateBillingProviderConnection(context.Background(), CreateBillingProviderConnectionRequest{
		ProviderType:    "stripe",
		Environment:     "test",
		DisplayName:     "Stripe Fail",
		Scope:           "platform",
		StripeSecretKey: "sk_test_fail",
	}, "platform_api_key", "pkey_fail")
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}

	updated, err := svc.SyncBillingProviderConnection(context.Background(), created.ID)
	if err == nil {
		t.Fatalf("expected sync error")
	}
	if updated.Status != domain.BillingProviderConnectionStatusSyncError {
		t.Fatalf("expected sync_error status, got %q", updated.Status)
	}
	if updated.LastSyncError != "lago sync failed" {
		t.Fatalf("expected last sync error to persist, got %q", updated.LastSyncError)
	}
}

func TestBillingProviderConnectionService_Disable(t *testing.T) {
	repo := newTestBillingProviderRepo(t)
	secretStore := NewMemoryBillingSecretStore()
	svc := NewBillingProviderConnectionService(repo, secretStore, nil)

	created, err := svc.CreateBillingProviderConnection(context.Background(), CreateBillingProviderConnectionRequest{
		ProviderType:    "stripe",
		Environment:     "test",
		DisplayName:     "Stripe Disable",
		Scope:           "platform",
		StripeSecretKey: "sk_test_disable",
	}, "platform_api_key", "pkey_disable")
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}

	disabled, err := svc.DisableBillingProviderConnection(created.ID)
	if err != nil {
		t.Fatalf("disable connection: %v", err)
	}
	if disabled.Status != domain.BillingProviderConnectionStatusDisabled {
		t.Fatalf("expected disabled status, got %q", disabled.Status)
	}
	if disabled.DisabledAt == nil {
		t.Fatalf("expected disabled_at to be set")
	}
}

func newTestBillingProviderRepo(t *testing.T) store.Repository {
	t.Helper()
	databaseURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for integration tests")
	}
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repo := store.NewPostgresStore(db)
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	return repo
}

func TestBillingProviderConnectionService_SyncUsesOwnerTenantOrganizationID(t *testing.T) {
	repo := newTestBillingProviderRepo(t)
	secretStore := NewMemoryBillingSecretStore()
	adapter := &stubBillingProviderAdapter{result: EnsureStripeProviderResult{
		LagoProviderCode: "stripe_owner_org",
		ConnectedAt:      time.Now().UTC(),
		LastSyncedAt:     time.Now().UTC(),
	}}
	svc := NewBillingProviderConnectionService(repo, secretStore, adapter)

	if _, err := repo.CreateTenant(domain.Tenant{
		ID:                 "tenant_owner_org",
		Name:               "Owner Tenant",
		Status:             domain.TenantStatusActive,
		LagoOrganizationID: "org_owner",
	}); err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	created, err := svc.CreateBillingProviderConnection(context.Background(), CreateBillingProviderConnectionRequest{
		ProviderType:    "stripe",
		Environment:     "test",
		DisplayName:     "Stripe Owner Org",
		Scope:           "tenant",
		OwnerTenantID:   "tenant_owner_org",
		StripeSecretKey: "sk_test_owner_org",
	}, "platform_api_key", "pkey_owner_org")
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}

	synced, err := svc.SyncBillingProviderConnection(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("sync connection with owner tenant org: %v", err)
	}
	if adapter.last.LagoOrganizationID != "org_owner" {
		t.Fatalf("expected owner tenant org to be sent to adapter, got %q", adapter.last.LagoOrganizationID)
	}
	if synced.LagoOrganizationID != "org_owner" {
		t.Fatalf("expected synced connection to persist owner tenant org, got %q", synced.LagoOrganizationID)
	}
}
