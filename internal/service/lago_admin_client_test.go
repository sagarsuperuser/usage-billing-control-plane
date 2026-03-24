package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
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

func TestTenantBackedLagoTransportResolver_UsesTenantAPIKey(t *testing.T) {
	t.Parallel()

	repo := &stubLagoTenantCredentialRepo{
		byTenant: map[string]domain.Tenant{
			"tenant_test": {ID: "tenant_test", LagoAPIKey: "tenant_key"},
		},
		byOrg: map[string]domain.Tenant{},
	}
	resolver, err := NewTenantBackedLagoTransportResolver(repo, LagoClientConfig{BaseURL: "https://lago.example.test", APIKey: "default_key"})
	if err != nil {
		t.Fatalf("new resolver: %v", err)
	}

	transport, err := resolver.Resolve(context.Background(), "tenant_test", "")
	if err != nil {
		t.Fatalf("resolve transport: %v", err)
	}
	if transport.apiKey != "tenant_key" {
		t.Fatalf("expected tenant api key, got %q", transport.apiKey)
	}
}

func TestTenantBackedLagoTransportResolver_FallsBackToDefault(t *testing.T) {
	t.Parallel()

	resolver, err := NewTenantBackedLagoTransportResolver(nil, LagoClientConfig{BaseURL: "https://lago.example.test", APIKey: "default_key"})
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
