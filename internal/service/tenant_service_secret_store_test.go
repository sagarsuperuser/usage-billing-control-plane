package service

import (
	"context"
	"testing"
)

type stubTenantLagoOrganizationBootstrapper struct {
	result LagoOrganizationBootstrapResult
	err    error
}

func (s *stubTenantLagoOrganizationBootstrapper) BootstrapOrganization(context.Context, string) (LagoOrganizationBootstrapResult, error) {
	if s == nil {
		return LagoOrganizationBootstrapResult{}, nil
	}
	return s.result, s.err
}

func TestTenantServiceCreateTenantStoresTenantLagoAPIKeyInSecretStore(t *testing.T) {
	repo := newTestBillingProviderRepo(t)
	secretStore := NewMemoryBillingSecretStore()
	bootstrapper := &stubTenantLagoOrganizationBootstrapper{
		result: LagoOrganizationBootstrapResult{
			OrganizationID: "org_secret_store_test",
			APIKey:         "lago_key_secret_store_test",
		},
	}

	svc := NewTenantService(repo).
		WithLagoOrganizationBootstrapper(bootstrapper).
		WithLagoAPIKeySecretStore(secretStore)

	created, err := svc.CreateTenant(EnsureTenantRequest{
		ID:   "tenant_secret_store_test",
		Name: "Tenant Secret Store Test",
	}, "pkey_platform_secret_store_test")
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	if created.LagoOrganizationID != "org_secret_store_test" {
		t.Fatalf("expected lago organization id to be bootstrapped, got %q", created.LagoOrganizationID)
	}
	if created.LagoAPIKeySecretRef == "" {
		t.Fatalf("expected lago api key secret ref to be persisted")
	}
	if created.LagoAPIKey != "" {
		t.Fatalf("expected raw lago api key to be omitted when secret store is configured")
	}

	stored, err := repo.GetTenant(created.ID)
	if err != nil {
		t.Fatalf("get tenant: %v", err)
	}
	if stored.LagoAPIKeySecretRef == "" {
		t.Fatalf("expected stored tenant to include a lago api key secret ref")
	}
	if stored.LagoAPIKey != "" {
		t.Fatalf("expected stored tenant row not to retain raw lago api key")
	}

	apiKey, err := secretStore.GetTenantLagoAPIKey(context.Background(), stored.LagoAPIKeySecretRef)
	if err != nil {
		t.Fatalf("get tenant lago api key from secret store: %v", err)
	}
	if apiKey != "lago_key_secret_store_test" {
		t.Fatalf("expected stored lago api key to match bootstrap result, got %q", apiKey)
	}
}
