package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/service"
	"usage-billing-control-plane/internal/store"
)

type browserSSOStoreStub struct {
	usersByID        map[string]domain.User
	usersByEmail     map[string]domain.User
	memberships      map[string][]domain.UserTenantMembership
	identitiesByPair map[string]domain.UserFederatedIdentity
}

func newBrowserSSOStoreStub(user domain.User, memberships []domain.UserTenantMembership) *browserSSOStoreStub {
	return &browserSSOStoreStub{
		usersByID: map[string]domain.User{
			user.ID: user,
		},
		usersByEmail: map[string]domain.User{
			user.Email: user,
		},
		memberships: map[string][]domain.UserTenantMembership{
			user.ID: memberships,
		},
		identitiesByPair: map[string]domain.UserFederatedIdentity{},
	}
}

func (s *browserSSOStoreStub) GetUser(id string) (domain.User, error) {
	user, ok := s.usersByID[id]
	if !ok {
		return domain.User{}, store.ErrNotFound
	}
	return user, nil
}

func (s *browserSSOStoreStub) GetUserByEmail(email string) (domain.User, error) {
	user, ok := s.usersByEmail[email]
	if !ok {
		return domain.User{}, store.ErrNotFound
	}
	return user, nil
}

func (s *browserSSOStoreStub) CreateUser(input domain.User) (domain.User, error) {
	if input.ID == "" {
		input.ID = "usr_auto"
	}
	now := time.Now().UTC()
	if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = now
	}
	s.usersByID[input.ID] = input
	s.usersByEmail[input.Email] = input
	return input, nil
}

func (s *browserSSOStoreStub) GetUserPasswordCredential(userID string) (domain.UserPasswordCredential, error) {
	return domain.UserPasswordCredential{}, store.ErrNotFound
}

func (s *browserSSOStoreStub) ListUserTenantMemberships(userID string) ([]domain.UserTenantMembership, error) {
	return s.memberships[userID], nil
}

func (s *browserSSOStoreStub) GetUserFederatedIdentity(providerKey, subject string) (domain.UserFederatedIdentity, error) {
	item, ok := s.identitiesByPair[providerKey+"|"+subject]
	if !ok {
		return domain.UserFederatedIdentity{}, store.ErrNotFound
	}
	return item, nil
}

func (s *browserSSOStoreStub) UpsertUserFederatedIdentity(input domain.UserFederatedIdentity) (domain.UserFederatedIdentity, error) {
	if input.ID == "" {
		input.ID = "ufi_1"
	}
	if input.CreatedAt.IsZero() {
		input.CreatedAt = time.Now().UTC()
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = input.CreatedAt
	}
	s.identitiesByPair[input.ProviderKey+"|"+input.Subject] = input
	return input, nil
}

type fakeBrowserSSOProvider struct {
	definition service.BrowserSSOProviderDefinition
	claims     service.BrowserSSOClaims
}

func (p fakeBrowserSSOProvider) Definition() service.BrowserSSOProviderDefinition {
	return p.definition
}

func (p fakeBrowserSSOProvider) BuildAuthCodeURL(state, nonce, codeChallenge, redirectURI string) (string, error) {
	return "https://idp.example.com/auth?state=" + state, nil
}

func (p fakeBrowserSSOProvider) Exchange(ctx context.Context, redirectURI, code, codeVerifier, nonce string) (service.BrowserSSOClaims, error) {
	return p.claims, nil
}

func TestBrowserSSOServiceLinksExistingUserByVerifiedEmail(t *testing.T) {
	user := domain.User{
		ID:           "usr_platform",
		Email:        "admin@example.com",
		DisplayName:  "Admin",
		Status:       domain.UserStatusActive,
		PlatformRole: domain.UserPlatformRoleAdmin,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	storeStub := newBrowserSSOStoreStub(user, nil)
	authSvc, err := service.NewBrowserUserAuthService(storeStub)
	if err != nil {
		t.Fatalf("new browser user auth service: %v", err)
	}
	ssoSvc, err := service.NewBrowserSSOService(
		storeStub,
		authSvc,
		[]service.BrowserSSOProvider{
			fakeBrowserSSOProvider{
				definition: service.BrowserSSOProviderDefinition{
					Key:         "google",
					DisplayName: "Google Workspace",
					Type:        domain.BrowserSSOProviderTypeOIDC,
				},
				claims: service.BrowserSSOClaims{
					Subject:       "google-subject-1",
					Email:         "admin@example.com",
					EmailVerified: true,
					DisplayName:   "Admin",
				},
			},
		},
		service.BrowserSSOServiceConfig{},
	)
	if err != nil {
		t.Fatalf("new browser sso service: %v", err)
	}

	principal, err := ssoSvc.AuthenticateCallback(context.Background(), "google", "code", "verifier", "nonce", "", "https://api.example.com/v1/ui/auth/sso/google/callback")
	if err != nil {
		t.Fatalf("authenticate callback: %v", err)
	}
	if principal.Scope != "platform" {
		t.Fatalf("expected platform scope, got %q", principal.Scope)
	}
	if _, err := storeStub.GetUserFederatedIdentity("google", "google-subject-1"); err != nil {
		t.Fatalf("expected linked federated identity, got %v", err)
	}
}

func TestBrowserSSOServiceRejectsUnverifiedEmailProvisioning(t *testing.T) {
	user := domain.User{
		ID:          "usr_tenant",
		Email:       "tenant@example.com",
		DisplayName: "Tenant",
		Status:      domain.UserStatusActive,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	storeStub := newBrowserSSOStoreStub(user, []domain.UserTenantMembership{
		{UserID: user.ID, TenantID: "tenant_a", Role: "writer", Status: domain.UserTenantMembershipStatusActive},
	})
	authSvc, err := service.NewBrowserUserAuthService(storeStub)
	if err != nil {
		t.Fatalf("new browser user auth service: %v", err)
	}
	ssoSvc, err := service.NewBrowserSSOService(
		storeStub,
		authSvc,
		[]service.BrowserSSOProvider{
			fakeBrowserSSOProvider{
				definition: service.BrowserSSOProviderDefinition{Key: "google", DisplayName: "Google Workspace", Type: domain.BrowserSSOProviderTypeOIDC},
				claims: service.BrowserSSOClaims{
					Subject:       "google-subject-2",
					Email:         "new-user@example.com",
					EmailVerified: false,
				},
			},
		},
		service.BrowserSSOServiceConfig{},
	)
	if err != nil {
		t.Fatalf("new browser sso service: %v", err)
	}

	_, err = ssoSvc.AuthenticateCallback(context.Background(), "google", "code", "verifier", "nonce", "", "https://api.example.com/v1/ui/auth/sso/google/callback")
	if !errors.Is(err, service.ErrBrowserSSOEmailNotVerified) {
		t.Fatalf("expected email-not-verified error, got %v", err)
	}
}
