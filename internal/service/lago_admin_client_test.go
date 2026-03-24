package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

func TestLagoAdminOrganizationBootstrapper(t *testing.T) {
	t.Parallel()

	var receivedAdminKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAdminKey = r.Header.Get("X-Admin-API-Key")
		if r.Method != http.MethodPost || r.URL.Path != "/admin/organizations" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"organization":{"id":"org_test"},"organization_api_key":"lago_key_test"}`)
	}))
	defer server.Close()

	client, err := NewLagoAdminOrganizationBootstrapper(LagoClientConfig{BaseURL: server.URL}, "admin_key_test")
	if err != nil {
		t.Fatalf("new bootstrapper: %v", err)
	}

	result, err := client.BootstrapOrganization(context.Background(), "Tenant Test")
	if err != nil {
		t.Fatalf("bootstrap organization: %v", err)
	}
	if receivedAdminKey != "admin_key_test" {
		t.Fatalf("expected admin api key header, got %q", receivedAdminKey)
	}
	if result.OrganizationID != "org_test" || result.APIKey != "lago_key_test" {
		t.Fatalf("unexpected bootstrap result: %+v", result)
	}
}

type stubLagoTenantCredentialRepo struct {
	byTenant map[string]domain.Tenant
	byOrg    map[string]domain.Tenant
}

func (r *stubLagoTenantCredentialRepo) GetTenant(id string) (domain.Tenant, error) {
	if tenant, ok := r.byTenant[id]; ok {
		return tenant, nil
	}
	return domain.Tenant{}, store.ErrNotFound
}

func (r *stubLagoTenantCredentialRepo) GetTenantByLagoOrganizationID(organizationID string) (domain.Tenant, error) {
	if tenant, ok := r.byOrg[organizationID]; ok {
		return tenant, nil
	}
	return domain.Tenant{}, store.ErrNotFound
}

func TestTenantBackedLagoTransportResolver_RejectsTenantWithoutSecretRef(t *testing.T) {
	t.Parallel()

	repo := &stubLagoTenantCredentialRepo{
		byTenant: map[string]domain.Tenant{
			"tenant_test": {ID: "tenant_test"},
		},
		byOrg: map[string]domain.Tenant{},
	}
	resolver, err := NewTenantBackedLagoTransportResolver(repo, nil, LagoClientConfig{BaseURL: "https://lago.example.test", APIKey: "default_key"})
	if err != nil {
		t.Fatalf("new resolver: %v", err)
	}

	_, err = resolver.Resolve(context.Background(), "tenant_test", "")
	if err == nil {
		t.Fatal("expected missing secret ref error")
	}
	want := "no tenant lago api key secret ref configured"
	if got := err.Error(); !strings.Contains(got, want) {
		t.Fatalf("expected error starting with %q, got %q", want, got)
	}
}

func TestTenantBackedLagoTransportResolver_UsesTenantAPIKeySecretRef(t *testing.T) {
	t.Parallel()

	secretStore := NewMemoryBillingSecretStore()
	secretRef, err := secretStore.PutTenantLagoAPIKey(context.Background(), "tenant_secret_ref", "tenant_secret_key")
	if err != nil {
		t.Fatalf("put tenant lago api key: %v", err)
	}

	repo := &stubLagoTenantCredentialRepo{
		byTenant: map[string]domain.Tenant{
			"tenant_secret_ref": {ID: "tenant_secret_ref", LagoAPIKeySecretRef: secretRef},
		},
		byOrg: map[string]domain.Tenant{},
	}
	resolver, err := NewTenantBackedLagoTransportResolver(repo, secretStore, LagoClientConfig{BaseURL: "https://lago.example.test", APIKey: "default_key"})
	if err != nil {
		t.Fatalf("new resolver: %v", err)
	}

	transport, err := resolver.Resolve(context.Background(), "tenant_secret_ref", "")
	if err != nil {
		t.Fatalf("resolve transport: %v", err)
	}
	if transport.apiKey != "tenant_secret_key" {
		t.Fatalf("expected tenant secret-store api key, got %q", transport.apiKey)
	}
}

func TestTenantBackedLagoTransportResolver_FallsBackToDefault(t *testing.T) {
	t.Parallel()

	resolver, err := NewTenantBackedLagoTransportResolver(nil, nil, LagoClientConfig{BaseURL: "https://lago.example.test", APIKey: "default_key"})
	if err != nil {
		t.Fatalf("new resolver: %v", err)
	}

	transport, err := resolver.Resolve(context.Background(), "", "")
	if err != nil {
		t.Fatalf("resolve transport: %v", err)
	}
	if transport.apiKey != "default_key" {
		t.Fatalf("expected default api key, got %q", transport.apiKey)
	}
}
