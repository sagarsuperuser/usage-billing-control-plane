package service

import (
	"strings"
	"testing"
	"time"

	"usage-billing-control-plane/internal/domain"
)

func TestWorkspaceBillingBindingService_EnsureAndResolve(t *testing.T) {
	repo := newTestBillingProviderRepo(t)
	tenant := createWorkspaceBillingBindingTestTenant(t, repo, "tenant_binding_ready")
	connection := createWorkspaceBillingBindingTestConnection(t, repo, "bpc_binding_ready")

	svc := NewWorkspaceBillingBindingService(repo)
	binding, created, err := svc.EnsureWorkspaceBillingBinding(EnsureWorkspaceBillingBindingRequest{
		WorkspaceID:                 tenant.ID,
		BillingProviderConnectionID: connection.ID,
		BackendOrganizationID:       "org_binding_ready",
		BackendProviderCode:         "provider_binding_ready",
		IsolationMode:               "shared",
		CreatedByType:               "platform_user",
		CreatedByID:                 "usr_platform",
	})
	if err != nil {
		t.Fatalf("ensure workspace billing binding: %v", err)
	}
	if !created {
		t.Fatalf("expected binding to be created")
	}
	if binding.Status != domain.WorkspaceBillingBindingStatusConnected {
		t.Fatalf("expected connected status, got %q", binding.Status)
	}

	resolved, err := svc.ResolveEffectiveWorkspaceBillingContext(tenant.ID)
	if err != nil {
		t.Fatalf("resolve effective workspace billing context: %v", err)
	}
	if resolved.Source != "binding" {
		t.Fatalf("expected binding source, got %q", resolved.Source)
	}
	if resolved.BackendOrganizationID != "org_binding_ready" || resolved.BackendProviderCode != "provider_binding_ready" {
		t.Fatalf("expected binding context values, got org=%q provider=%q", resolved.BackendOrganizationID, resolved.BackendProviderCode)
	}
}

func TestWorkspaceBillingBindingService_ResolveBackfillsBindingFromTenantFields(t *testing.T) {
	repo := newTestBillingProviderRepo(t)
	connection := createWorkspaceBillingBindingTestConnection(t, repo, "bpc_binding_legacy")

	now := time.Now().UTC()
	tenant, err := repo.CreateTenant(domain.Tenant{
		ID:                          "tenant_binding_legacy",
		Name:                        "Tenant Binding Legacy",
		Status:                      domain.TenantStatusActive,
		BillingProviderConnectionID: connection.ID,
		LagoOrganizationID:          "org_legacy",
		LagoBillingProviderCode:     "provider_legacy",
		CreatedAt:                   now,
		UpdatedAt:                   now,
	})
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	svc := NewWorkspaceBillingBindingService(repo)
	resolved, err := svc.ResolveEffectiveWorkspaceBillingContext(tenant.ID)
	if err != nil {
		t.Fatalf("resolve legacy workspace billing context: %v", err)
	}
	if resolved.Source != "binding" {
		t.Fatalf("expected binding source after backfill, got %q", resolved.Source)
	}
	if resolved.Backend != domain.WorkspaceBillingBackendLago {
		t.Fatalf("expected lago backend, got %q", resolved.Backend)
	}
	if resolved.BackendOrganizationID != "org_legacy" || resolved.BackendProviderCode != "provider_legacy" {
		t.Fatalf("expected legacy context values, got org=%q provider=%q", resolved.BackendOrganizationID, resolved.BackendProviderCode)
	}
	binding, err := svc.GetWorkspaceBillingBinding(tenant.ID)
	if err != nil {
		t.Fatalf("get backfilled workspace billing binding: %v", err)
	}
	if binding.BillingProviderConnectionID != connection.ID {
		t.Fatalf("expected backfilled connection id %q, got %q", connection.ID, binding.BillingProviderConnectionID)
	}
	if binding.Status != domain.WorkspaceBillingBindingStatusConnected {
		t.Fatalf("expected backfilled binding status connected, got %q", binding.Status)
	}
}

func TestWorkspaceBillingBindingService_BindingPreemptsLegacyBackfillUntilReady(t *testing.T) {
	repo := newTestBillingProviderRepo(t)
	connection := createWorkspaceBillingBindingTestConnection(t, repo, "bpc_binding_pending")

	now := time.Now().UTC()
	tenant, err := repo.CreateTenant(domain.Tenant{
		ID:                          "tenant_binding_pending",
		Name:                        "Tenant Binding Pending",
		Status:                      domain.TenantStatusActive,
		BillingProviderConnectionID: connection.ID,
		LagoOrganizationID:          "org_legacy_pending",
		LagoBillingProviderCode:     "provider_legacy_pending",
		CreatedAt:                   now,
		UpdatedAt:                   now,
	})
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	svc := NewWorkspaceBillingBindingService(repo)
	if _, _, err := svc.EnsureWorkspaceBillingBinding(EnsureWorkspaceBillingBindingRequest{
		WorkspaceID:                 tenant.ID,
		BillingProviderConnectionID: connection.ID,
		CreatedByType:               "platform_user",
	}); err != nil {
		t.Fatalf("ensure pending workspace billing binding: %v", err)
	}

	_, err = svc.ResolveEffectiveWorkspaceBillingContext(tenant.ID)
	if err == nil || !strings.Contains(err.Error(), "not ready") {
		t.Fatalf("expected not ready error, got %v", err)
	}
}

func TestWorkspaceBillingBindingService_DescribeWorkspaceBillingDetectsConnectionDrift(t *testing.T) {
	repo := newTestBillingProviderRepo(t)
	tenant := createWorkspaceBillingBindingTestTenant(t, repo, "tenant_binding_drift")
	connection := createWorkspaceBillingBindingTestConnection(t, repo, "bpc_binding_drift")

	svc := NewWorkspaceBillingBindingService(repo)
	if _, _, err := svc.EnsureWorkspaceBillingBinding(EnsureWorkspaceBillingBindingRequest{
		WorkspaceID:                 tenant.ID,
		BillingProviderConnectionID: connection.ID,
		BackendOrganizationID:       "org_binding_drift",
		BackendProviderCode:         "provider_binding_drift",
		IsolationMode:               "shared",
		CreatedByType:               "platform_user",
		CreatedByID:                 "usr_platform",
	}); err != nil {
		t.Fatalf("ensure workspace billing binding: %v", err)
	}

	connection.Status = domain.BillingProviderConnectionStatusSyncError
	connection.LastSyncError = "stripe verification failed"
	connection.UpdatedAt = time.Now().UTC()
	if _, err := repo.UpdateBillingProviderConnection(connection); err != nil {
		t.Fatalf("update billing provider connection: %v", err)
	}

	diagnosis, err := svc.DescribeWorkspaceBilling(tenant.ID)
	if err != nil {
		t.Fatalf("describe workspace billing: %v", err)
	}
	if diagnosis.Connected {
		t.Fatalf("expected connected=false when connection drifted into sync_error")
	}
	if diagnosis.Status != string(domain.WorkspaceBillingBindingStatusVerificationFailed) {
		t.Fatalf("expected status verification_failed, got %q", diagnosis.Status)
	}
	if diagnosis.DiagnosisCode != "verification_failed" {
		t.Fatalf("expected diagnosis_code verification_failed, got %q", diagnosis.DiagnosisCode)
	}
	if !strings.Contains(diagnosis.DiagnosisSummary, "stripe verification failed") {
		t.Fatalf("expected diagnosis summary to include sync error, got %q", diagnosis.DiagnosisSummary)
	}
}

func createWorkspaceBillingBindingTestTenant(t *testing.T, repo interface {
	CreateTenant(input domain.Tenant) (domain.Tenant, error)
}, id string) domain.Tenant {
	t.Helper()
	now := time.Now().UTC()
	tenant, err := repo.CreateTenant(domain.Tenant{
		ID:        id,
		Name:      strings.ReplaceAll(id, "_", " "),
		Status:    domain.TenantStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("create tenant %s: %v", id, err)
	}
	return tenant
}

func createWorkspaceBillingBindingTestConnection(t *testing.T, repo interface {
	CreateBillingProviderConnection(input domain.BillingProviderConnection) (domain.BillingProviderConnection, error)
}, id string) domain.BillingProviderConnection {
	t.Helper()
	now := time.Now().UTC()
	connectedAt := now
	connection, err := repo.CreateBillingProviderConnection(domain.BillingProviderConnection{
		ID:                 id,
		ProviderType:       domain.BillingProviderTypeStripe,
		Environment:        "test",
		DisplayName:        strings.ReplaceAll(id, "_", " "),
		Scope:              domain.BillingProviderConnectionScopePlatform,
		Status:             domain.BillingProviderConnectionStatusConnected,
		LagoOrganizationID: "org_connection_seed",
		LagoProviderCode:   "provider_connection_seed",
		SecretRef:          "secret/ref/" + id,
		ConnectedAt:        &connectedAt,
		CreatedByType:      "platform_user",
		CreatedByID:        "usr_platform",
		CreatedAt:          now,
		UpdatedAt:          now,
	})
	if err != nil {
		t.Fatalf("create billing provider connection %s: %v", id, err)
	}
	return connection
}
