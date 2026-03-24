package service

import (
	"context"
	"strings"
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

	stored, err := repo.GetTenant(created.ID)
	if err != nil {
		t.Fatalf("get tenant: %v", err)
	}
	if stored.LagoAPIKeySecretRef == "" {
		t.Fatalf("expected stored tenant to include a lago api key secret ref")
	}

	apiKey, err := secretStore.GetTenantLagoAPIKey(context.Background(), stored.LagoAPIKeySecretRef)
	if err != nil {
		t.Fatalf("get tenant lago api key from secret store: %v", err)
	}
	if apiKey != "lago_key_secret_store_test" {
		t.Fatalf("expected stored lago api key to match bootstrap result, got %q", apiKey)
	}
}

func TestTenantServiceCreateTenantRequiresTenantLagoAPIKeySecretStore(t *testing.T) {
	repo := newTestBillingProviderRepo(t)
	bootstrapper := &stubTenantLagoOrganizationBootstrapper{
		result: LagoOrganizationBootstrapResult{
			OrganizationID: "org_secret_store_required_test",
			APIKey:         "lago_key_secret_store_required_test",
		},
	}

	svc := NewTenantService(repo).
		WithLagoOrganizationBootstrapper(bootstrapper)

	_, err := svc.CreateTenant(EnsureTenantRequest{
		ID:   "tenant_secret_store_required_test",
		Name: "Tenant Secret Store Required Test",
	}, "pkey_platform_secret_store_required_test")
	if err == nil {
		t.Fatal("expected missing tenant lago api key secret store error")
	}
	if !strings.Contains(err.Error(), "lago tenant api key secret store is required") {
		t.Fatalf("expected missing secret store error, got %q", err)
	}
}
