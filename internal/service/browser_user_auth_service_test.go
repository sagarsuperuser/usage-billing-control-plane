package service_test

import (
	"testing"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/service"
	"usage-billing-control-plane/internal/store"
)

type browserAuthStoreStub struct {
	user        domain.User
	credential  domain.UserPasswordCredential
	memberships []domain.UserTenantMembership
}

func (s browserAuthStoreStub) GetUser(id string) (domain.User, error) {
	if s.user.ID == "" || s.user.ID != id {
		return domain.User{}, store.ErrNotFound
	}
	return s.user, nil
}

func (s browserAuthStoreStub) GetTenant(id string) (domain.Tenant, error) {
	return domain.Tenant{
		ID:        id,
		Name:      id,
		Status:    domain.TenantStatusActive,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}, nil
}

func (s browserAuthStoreStub) GetUserByEmail(email string) (domain.User, error) {
	if s.user.Email == "" || s.user.Email != email {
		return domain.User{}, store.ErrNotFound
	}
	return s.user, nil
}

func (s browserAuthStoreStub) GetUserPasswordCredential(userID string) (domain.UserPasswordCredential, error) {
	if s.credential.UserID == "" || s.credential.UserID != userID {
		return domain.UserPasswordCredential{}, store.ErrNotFound
	}
	return s.credential, nil
}

func (s browserAuthStoreStub) ListUserTenantMemberships(userID string) ([]domain.UserTenantMembership, error) {
	if s.user.ID == "" || s.user.ID != userID {
		return nil, store.ErrNotFound
	}
	return s.memberships, nil
}

func TestBrowserUserAuthServiceDeniesplatformUserWithNoWorkspace(t *testing.T) {
	hash, err := service.HashPassword("correct horse battery")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	svc, err := service.NewBrowserUserAuthService(browserAuthStoreStub{
		user: domain.User{
			ID:           "usr_platform",
			Email:        "admin@example.com",
			DisplayName:  "Admin",
			Status:       domain.UserStatusActive,
			PlatformRole: domain.UserPlatformRoleAdmin,
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		},
		credential: domain.UserPasswordCredential{
			UserID:            "usr_platform",
			PasswordHash:      hash,
			PasswordUpdatedAt: time.Now().UTC(),
			CreatedAt:         time.Now().UTC(),
			UpdatedAt:         time.Now().UTC(),
		},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	// Platform admin with no workspace memberships gets access denied
	// (they need to register or be invited to a workspace first).
	// Platform ops use API keys, not browser sessions.
	_, err = svc.Authenticate(service.BrowserUserLoginRequest{
		Email:    "admin@example.com",
		Password: "correct horse battery",
	})
	if err == nil {
		t.Fatal("expected access denied for platform admin with no workspace")
	}
}

func TestBrowserUserAuthServiceAutoSelectsFirstWorkspaceForMultiTenantUser(t *testing.T) {
	hash, err := service.HashPassword("correct horse battery")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	svc, err := service.NewBrowserUserAuthService(browserAuthStoreStub{
		user: domain.User{
			ID:          "usr_tenant",
			Email:       "tenant@example.com",
			DisplayName: "Tenant User",
			Status:      domain.UserStatusActive,
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		},
		credential: domain.UserPasswordCredential{
			UserID:            "usr_tenant",
			PasswordHash:      hash,
			PasswordUpdatedAt: time.Now().UTC(),
			CreatedAt:         time.Now().UTC(),
			UpdatedAt:         time.Now().UTC(),
		},
		memberships: []domain.UserTenantMembership{
			{UserID: "usr_tenant", TenantID: "tenant_b", Role: "admin", Status: domain.UserTenantMembershipStatusActive},
			{UserID: "usr_tenant", TenantID: "tenant_a", Role: "writer", Status: domain.UserTenantMembershipStatusActive},
		},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	principal, err := svc.Authenticate(service.BrowserUserLoginRequest{
		Email:    "tenant@example.com",
		Password: "correct horse battery",
	})
	if err != nil {
		t.Fatalf("expected auto-select, got error: %v", err)
	}
	if principal.TenantID != "tenant_a" {
		t.Fatalf("expected auto-select tenant_a (alphabetically first), got %s", principal.TenantID)
	}
	if principal.Role != "writer" {
		t.Fatalf("expected writer role for tenant_a, got %s", principal.Role)
	}
}

func TestBrowserUserAuthServiceAuthenticatesSpecificTenantMembership(t *testing.T) {
	hash, err := service.HashPassword("correct horse battery")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	svc, err := service.NewBrowserUserAuthService(browserAuthStoreStub{
		user: domain.User{
			ID:          "usr_tenant",
			Email:       "tenant@example.com",
			DisplayName: "Tenant User",
			Status:      domain.UserStatusActive,
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		},
		credential: domain.UserPasswordCredential{
			UserID:            "usr_tenant",
			PasswordHash:      hash,
			PasswordUpdatedAt: time.Now().UTC(),
			CreatedAt:         time.Now().UTC(),
			UpdatedAt:         time.Now().UTC(),
		},
		memberships: []domain.UserTenantMembership{
			{UserID: "usr_tenant", TenantID: "tenant_a", Role: "writer", Status: domain.UserTenantMembershipStatusActive},
			{UserID: "usr_tenant", TenantID: "tenant_b", Role: "admin", Status: domain.UserTenantMembershipStatusActive},
		},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	principal, err := svc.Authenticate(service.BrowserUserLoginRequest{
		Email:    "tenant@example.com",
		Password: "correct horse battery",
		TenantID: "tenant_b",
	})
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if principal.Scope != "tenant" {
		t.Fatalf("expected tenant scope, got %q", principal.Scope)
	}
	if principal.Role != "admin" {
		t.Fatalf("expected admin role, got %q", principal.Role)
	}
	if principal.TenantID != "tenant_b" {
		t.Fatalf("expected tenant_b, got %q", principal.TenantID)
	}
}
