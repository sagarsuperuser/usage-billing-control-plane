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

type stubStripeConnectionVerifier struct {
	result StripeConnectionVerificationResult
	err    error
	calls  int
	last   string
}

func (s *stubStripeConnectionVerifier) VerifyStripeSecret(_ context.Context, secretKey string) (StripeConnectionVerificationResult, error) {
	s.calls++
	s.last = secretKey
	if s.err != nil {
		return StripeConnectionVerificationResult{}, s.err
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
	verifier := &stubStripeConnectionVerifier{result: StripeConnectionVerificationResult{
		AccountID:  "acct_123",
		Livemode:   false,
		VerifiedAt: time.Now().UTC(),
	}}
	adapter := &stubBillingProviderAdapter{}
	svc := NewBillingProviderConnectionService(repo, secretStore, adapter).WithStripeConnectionVerifier(verifier)

	created, err := svc.CreateBillingProviderConnection(context.Background(), CreateBillingProviderConnectionRequest{
		ProviderType:    "stripe",
		Environment:     "test",
		DisplayName:     "Stripe Sync",
		Scope:           "platform",
		StripeSecretKey: "sk_test_sync",
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
	if verifier.calls != 1 {
		t.Fatalf("expected verifier call, got %d", verifier.calls)
	}
	if verifier.last != "sk_test_sync" {
		t.Fatalf("expected verifier to receive secret, got %q", verifier.last)
	}
	if adapter.calls != 0 {
		t.Fatalf("expected adapter not to be called during connection check, got %d", adapter.calls)
	}
	if synced.Status != domain.BillingProviderConnectionStatusConnected {
		t.Fatalf("expected connected status, got %q", synced.Status)
	}
	if synced.LastSyncedAt == nil {
		t.Fatalf("expected last_synced_at to be set")
	}
}

func TestBillingProviderConnectionService_SyncFailsWhenStripeVerificationFails(t *testing.T) {
	repo := newTestBillingProviderRepo(t)
	secretStore := NewMemoryBillingSecretStore()
	verifier := &stubStripeConnectionVerifier{err: errors.New("Stripe rejected the connection details.")}
	adapter := &stubBillingProviderAdapter{}
	svc := NewBillingProviderConnectionService(repo, secretStore, adapter).WithStripeConnectionVerifier(verifier)

	created, err := svc.CreateBillingProviderConnection(context.Background(), CreateBillingProviderConnectionRequest{
		ProviderType:       "stripe",
		Environment:        "test",
		DisplayName:        "Stripe Verify Fail",
		Scope:              "platform",
		StripeSecretKey:    "sk_test_verify_fail",
		StripeAccountID: "org_seed",
	}, "platform_api_key", "pkey_verify_fail")
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}

	updated, err := svc.SyncBillingProviderConnection(context.Background(), created.ID)
	if err == nil {
		t.Fatalf("expected stripe verification error")
	}
	if verifier.calls != 1 {
		t.Fatalf("expected verifier call, got %d", verifier.calls)
	}
	if adapter.calls != 0 {
		t.Fatalf("expected adapter not to be called, got %d", adapter.calls)
	}
	if updated.Status != domain.BillingProviderConnectionStatusSyncError {
		t.Fatalf("expected sync_error status, got %q", updated.Status)
	}
	if updated.LastSyncError != "Stripe rejected the connection details." {
		t.Fatalf("expected stripe verification error to persist, got %q", updated.LastSyncError)
	}
}

func TestBillingProviderConnectionService_RecheckBatch(t *testing.T) {
	repo := newTestBillingProviderRepo(t)
	secretStore := NewMemoryBillingSecretStore()
	verifier := &stubStripeConnectionVerifier{result: StripeConnectionVerificationResult{
		AccountID:  "acct_batch",
		Livemode:   false,
		VerifiedAt: time.Now().UTC(),
	}}
	adapter := &stubBillingProviderAdapter{}
	svc := NewBillingProviderConnectionService(repo, secretStore, adapter).WithStripeConnectionVerifier(verifier)

	staleA, err := svc.CreateBillingProviderConnection(context.Background(), CreateBillingProviderConnectionRequest{
		ProviderType:       "stripe",
		Environment:        "test",
		DisplayName:        "Stripe Stale A",
		Scope:              "platform",
		StripeSecretKey:    "sk_test_batch_a",
		StripeAccountID: "org_seed",
	}, "platform_api_key", "pkey_batch_a")
	if err != nil {
		t.Fatalf("create stale A: %v", err)
	}
	staleB, err := svc.CreateBillingProviderConnection(context.Background(), CreateBillingProviderConnectionRequest{
		ProviderType:       "stripe",
		Environment:        "test",
		DisplayName:        "Stripe Stale B",
		Scope:              "platform",
		StripeSecretKey:    "sk_test_batch_b",
		StripeAccountID: "org_seed",
	}, "platform_api_key", "pkey_batch_b")
	if err != nil {
		t.Fatalf("create stale B: %v", err)
	}
	fresh, err := svc.CreateBillingProviderConnection(context.Background(), CreateBillingProviderConnectionRequest{
		ProviderType:       "stripe",
		Environment:        "test",
		DisplayName:        "Stripe Fresh",
		Scope:              "platform",
		StripeSecretKey:    "sk_test_batch_c",
		StripeAccountID: "org_seed",
	}, "platform_api_key", "pkey_batch_c")
	if err != nil {
		t.Fatalf("create fresh: %v", err)
	}
	freshLastSyncedAt := time.Now().UTC()
	fresh.Status = domain.BillingProviderConnectionStatusConnected
	fresh.LastSyncedAt = &freshLastSyncedAt
	fresh.ConnectedAt = &freshLastSyncedAt
	if _, err := repo.UpdateBillingProviderConnection(fresh); err != nil {
		t.Fatalf("persist fresh connection: %v", err)
	}
	disabled, err := svc.CreateBillingProviderConnection(context.Background(), CreateBillingProviderConnectionRequest{
		ProviderType:       "stripe",
		Environment:        "test",
		DisplayName:        "Stripe Disabled",
		Scope:              "platform",
		StripeSecretKey:    "sk_test_batch_d",
		StripeAccountID: "org_seed",
	}, "platform_api_key", "pkey_batch_d")
	if err != nil {
		t.Fatalf("create disabled: %v", err)
	}
	if _, err := svc.DisableBillingProviderConnection(disabled.ID); err != nil {
		t.Fatalf("disable connection: %v", err)
	}

	result, err := svc.RecheckBillingProviderConnectionsBatch(context.Background(), BillingProviderConnectionRecheckBatchRequest{
		Limit:      2,
		StaleAfter: time.Hour,
	})
	if err != nil {
		t.Fatalf("recheck batch: %v", err)
	}
	if result.Checked != 2 {
		t.Fatalf("expected 2 checked connections, got %d", result.Checked)
	}
	if result.Healthy != 2 {
		t.Fatalf("expected 2 healthy checks, got %d", result.Healthy)
	}
	if result.Failed != 0 {
		t.Fatalf("expected 0 failed checks, got %d", result.Failed)
	}
	if verifier.calls != 2 {
		t.Fatalf("expected verifier calls 2, got %d", verifier.calls)
	}
	if adapter.calls != 0 {
		t.Fatalf("expected adapter not to be called during periodic checks, got %d", adapter.calls)
	}

	gotStaleA, err := repo.GetBillingProviderConnection(staleA.ID)
	if err != nil {
		t.Fatalf("get stale A: %v", err)
	}
	if gotStaleA.LastSyncedAt == nil {
		t.Fatalf("expected stale A to be rechecked")
	}
	gotStaleB, err := repo.GetBillingProviderConnection(staleB.ID)
	if err != nil {
		t.Fatalf("get stale B: %v", err)
	}
	if gotStaleB.LastSyncedAt == nil {
		t.Fatalf("expected stale B to be rechecked")
	}
	gotFresh, err := repo.GetBillingProviderConnection(fresh.ID)
	if err != nil {
		t.Fatalf("get fresh: %v", err)
	}
	if gotFresh.LastSyncedAt == nil || !gotFresh.LastSyncedAt.Equal(freshLastSyncedAt) {
		t.Fatalf("expected fresh connection to remain unchanged")
	}
}

func TestBillingProviderConnectionService_ProvisionWorkspaceBillingConnectionRequiresCheckedConnection(t *testing.T) {
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

	_, err = svc.ProvisionWorkspaceBillingConnection(context.Background(), ProvisionWorkspaceBillingConnectionInput{
		ConnectionID:       created.ID,
		OwnerTenantID:      "tenant_test",
		StripeAccountID: "org_test",
	})
	if err == nil || !strings.Contains(err.Error(), "must be checked before workspace assignment") {
		t.Fatalf("expected checked-before-assignment validation error, got %v", err)
	}
	if adapter.calls != 0 {
		t.Fatalf("expected adapter not to be called, got %d", adapter.calls)
	}
}

func TestBillingProviderConnectionService_ProvisionWorkspaceBillingConnection(t *testing.T) {
	repo := newTestBillingProviderRepo(t)
	secretStore := NewMemoryBillingSecretStore()
	verifier := &stubStripeConnectionVerifier{result: StripeConnectionVerificationResult{
		AccountID:  "acct_provision",
		Livemode:   false,
		VerifiedAt: time.Now().UTC(),
	}}
	adapter := &stubBillingProviderAdapter{result: EnsureStripeProviderResult{
		StripeAccountID: "org_default",
		StripeProviderCode:   "stripe_default_org",
		ConnectedAt:        time.Now().UTC(),
		LastSyncedAt:       time.Now().UTC(),
		DeprecatedHMACKey: "hmac_workspace",
	}}
	svc := NewBillingProviderConnectionService(repo, secretStore, adapter).WithStripeConnectionVerifier(verifier)

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
	if _, err := svc.SyncBillingProviderConnection(context.Background(), created.ID); err != nil {
		t.Fatalf("check connection before provisioning: %v", err)
	}

	result, err := svc.ProvisionWorkspaceBillingConnection(context.Background(), ProvisionWorkspaceBillingConnectionInput{
		ConnectionID:       created.ID,
		OwnerTenantID:      "tenant_workspace",
		StripeAccountID: "org_default",
	})
	if err != nil {
		t.Fatalf("provision workspace billing: %v", err)
	}
	if adapter.last.StripeAccountID != "org_default" {
		t.Fatalf("expected default org to be sent to adapter, got %q", adapter.last.StripeAccountID)
	}
	if result.StripeAccountID != "org_default" {
		t.Fatalf("expected provision result to preserve org, got %q", result.StripeAccountID)
	}
	secrets, err := secretStore.GetConnectionSecrets(context.Background(), created.SecretRef)
	if err != nil {
		t.Fatalf("get updated connection secrets: %v", err)
	}
	if secrets.DeprecatedHMACKey != "hmac_workspace" {
		t.Fatalf("expected workspace provisioning to persist hmac key, got %q", secrets.DeprecatedHMACKey)
	}
}

func TestBillingProviderConnectionService_ProvisionFailureReturnsAdapterError(t *testing.T) {
	repo := newTestBillingProviderRepo(t)
	secretStore := NewMemoryBillingSecretStore()
	verifier := &stubStripeConnectionVerifier{result: StripeConnectionVerificationResult{AccountID: "acct_checked"}}
	adapter := &stubBillingProviderAdapter{err: errors.New("lago sync failed")}
	svc := NewBillingProviderConnectionService(repo, secretStore, adapter).WithStripeConnectionVerifier(verifier)

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
	if _, err := svc.SyncBillingProviderConnection(context.Background(), created.ID); err != nil {
		t.Fatalf("check connection before provisioning: %v", err)
	}

	_, err = svc.ProvisionWorkspaceBillingConnection(context.Background(), ProvisionWorkspaceBillingConnectionInput{
		ConnectionID:       created.ID,
		OwnerTenantID:      "tenant_fail",
		StripeAccountID: "org_fail",
	})
	if err == nil || !strings.Contains(err.Error(), "lago sync failed") {
		t.Fatalf("expected adapter error during provisioning, got %v", err)
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
