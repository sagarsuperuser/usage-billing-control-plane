package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

var (
	ErrBrowserSSOProviderNotFound   = errors.New("sso provider not found")
	ErrBrowserSSOEmailRequired      = errors.New("sso email is required")
	ErrBrowserSSOEmailNotVerified   = errors.New("sso email is not verified")
	ErrBrowserSSOUserNotProvisioned = errors.New("sso user is not provisioned")
)

type BrowserSSOClaims struct {
	Subject       string
	Email         string
	EmailVerified bool
	DisplayName   string
}

type BrowserSSOProviderDefinition struct {
	Key         string
	DisplayName string
	Type        domain.BrowserSSOProviderType
}

type BrowserSSOProvider interface {
	Definition() BrowserSSOProviderDefinition
	BuildAuthCodeURL(state, nonce, codeChallenge, redirectURI string) (string, error)
	Exchange(ctx context.Context, redirectURI, code, codeVerifier, nonce string) (BrowserSSOClaims, error)
}

type BrowserSSOStore interface {
	GetUser(id string) (domain.User, error)
	GetUserByEmail(email string) (domain.User, error)
	CreateUser(input domain.User) (domain.User, error)
	GetWorkspaceInvitationByTokenHash(tokenHash string) (domain.WorkspaceInvitation, error)
	GetUserFederatedIdentity(providerKey, subject string) (domain.UserFederatedIdentity, error)
	UpsertUserFederatedIdentity(input domain.UserFederatedIdentity) (domain.UserFederatedIdentity, error)
}

type BrowserSSOServiceConfig struct {
	AutoProvisionUsers bool
}

type BrowserSSOService struct {
	store     BrowserSSOStore
	auth      *BrowserUserAuthService
	providers map[string]BrowserSSOProvider
	config    BrowserSSOServiceConfig
}

func NewBrowserSSOService(store BrowserSSOStore, auth *BrowserUserAuthService, providers []BrowserSSOProvider, config BrowserSSOServiceConfig) (*BrowserSSOService, error) {
	if store == nil {
		return nil, fmt.Errorf("browser sso store is required")
	}
	if auth == nil {
		return nil, fmt.Errorf("browser user auth service is required")
	}
	providerMap := make(map[string]BrowserSSOProvider, len(providers))
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		definition := provider.Definition()
		key := normalizeBrowserSSOProviderKey(definition.Key)
		if key == "" {
			return nil, fmt.Errorf("sso provider key is required")
		}
		if _, exists := providerMap[key]; exists {
			return nil, fmt.Errorf("duplicate sso provider key: %s", key)
		}
		providerMap[key] = provider
	}
	return &BrowserSSOService{
		store:     store,
		auth:      auth,
		providers: providerMap,
		config:    config,
	}, nil
}

func (s *BrowserSSOService) ListProviders() []BrowserSSOProviderDefinition {
	out := make([]BrowserSSOProviderDefinition, 0, len(s.providers))
	for _, provider := range s.providers {
		definition := provider.Definition()
		definition.Key = normalizeBrowserSSOProviderKey(definition.Key)
		definition.DisplayName = strings.TrimSpace(definition.DisplayName)
		out = append(out, definition)
	}
	sortBrowserSSOProviderDefinitions(out)
	return out
}

func (s *BrowserSSOService) BuildStartURL(providerKey, state, nonce, codeChallenge, redirectURI string) (string, error) {
	provider, err := s.provider(providerKey)
	if err != nil {
		return "", err
	}
	return provider.BuildAuthCodeURL(state, nonce, codeChallenge, redirectURI)
}

func (s *BrowserSSOService) AuthenticateCallback(ctx context.Context, providerKey, code, codeVerifier, nonce, tenantID, redirectURI, invitationToken string) (BrowserUserPrincipal, error) {
	provider, err := s.provider(providerKey)
	if err != nil {
		return BrowserUserPrincipal{}, err
	}
	claims, err := provider.Exchange(ctx, redirectURI, strings.TrimSpace(code), strings.TrimSpace(codeVerifier), strings.TrimSpace(nonce))
	if err != nil {
		return BrowserUserPrincipal{}, fmt.Errorf("exchange sso callback: %w", err)
	}
	return s.resolveClaims(provider.Definition(), claims, tenantID, invitationToken)
}

func (s *BrowserSSOService) resolveClaims(definition BrowserSSOProviderDefinition, claims BrowserSSOClaims, tenantID, invitationToken string) (BrowserUserPrincipal, error) {
	providerKey := normalizeBrowserSSOProviderKey(definition.Key)
	subject := strings.TrimSpace(claims.Subject)
	email := strings.ToLower(strings.TrimSpace(claims.Email))
	if providerKey == "" {
		return BrowserUserPrincipal{}, ErrBrowserSSOProviderNotFound
	}
	if subject == "" {
		return BrowserUserPrincipal{}, fmt.Errorf("sso subject is required")
	}

	user, identity, err := s.findOrProvisionUser(providerKey, definition.Type, subject, email, invitationToken, claims)
	if err != nil {
		return BrowserUserPrincipal{}, err
	}

	now := time.Now().UTC()
	identity.Email = email
	identity.EmailVerified = claims.EmailVerified
	identity.LastLoginAt = &now
	identity.UpdatedAt = now
	if identity.UserID == "" {
		identity.UserID = user.ID
	}
	if _, err := s.store.UpsertUserFederatedIdentity(identity); err != nil {
		return BrowserUserPrincipal{}, fmt.Errorf("upsert federated identity: %w", err)
	}

	return s.auth.ResolveUserPrincipal(user, tenantID)
}

func (s *BrowserSSOService) findOrProvisionUser(providerKey string, providerType domain.BrowserSSOProviderType, subject, email, invitationToken string, claims BrowserSSOClaims) (domain.User, domain.UserFederatedIdentity, error) {
	identity, err := s.store.GetUserFederatedIdentity(providerKey, subject)
	if err == nil {
		user, userErr := s.store.GetUser(identity.UserID)
		if userErr != nil {
			return domain.User{}, domain.UserFederatedIdentity{}, fmt.Errorf("get linked user: %w", userErr)
		}
		return user, identity, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return domain.User{}, domain.UserFederatedIdentity{}, fmt.Errorf("get federated identity: %w", err)
	}

	if email == "" {
		return domain.User{}, domain.UserFederatedIdentity{}, ErrBrowserSSOEmailRequired
	}
	if !claims.EmailVerified {
		return domain.User{}, domain.UserFederatedIdentity{}, ErrBrowserSSOEmailNotVerified
	}

	user, err := s.store.GetUserByEmail(email)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			return domain.User{}, domain.UserFederatedIdentity{}, fmt.Errorf("get user by email: %w", err)
		}
		allowProvision := s.config.AutoProvisionUsers
		if !allowProvision {
			allowProvision, err = s.canProvisionUserFromInvitation(invitationToken, email)
			if err != nil {
				return domain.User{}, domain.UserFederatedIdentity{}, err
			}
		}
		if !allowProvision {
			return domain.User{}, domain.UserFederatedIdentity{}, ErrBrowserSSOUserNotProvisioned
		}
		displayName := strings.TrimSpace(claims.DisplayName)
		if displayName == "" {
			displayName = defaultDisplayNameForEmail(email)
		}
		user, err = s.store.CreateUser(domain.User{
			Email:       email,
			DisplayName: displayName,
			Status:      domain.UserStatusActive,
		})
		if err != nil {
			return domain.User{}, domain.UserFederatedIdentity{}, fmt.Errorf("create user from sso claims: %w", err)
		}
	}

	now := time.Now().UTC()
	identity = domain.UserFederatedIdentity{
		UserID:        user.ID,
		ProviderKey:   providerKey,
		ProviderType:  providerType,
		Subject:       subject,
		Email:         email,
		EmailVerified: claims.EmailVerified,
		LastLoginAt:   &now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	return user, identity, nil
}

func (s *BrowserSSOService) canProvisionUserFromInvitation(invitationToken, email string) (bool, error) {
	tokenHash := hashWorkspaceInvitationToken(invitationToken)
	if tokenHash == "" {
		return false, nil
	}
	invite, err := s.store.GetWorkspaceInvitationByTokenHash(tokenHash)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("get workspace invitation: %w", err)
	}
	if invite.Status != domain.WorkspaceInvitationStatusPending {
		return false, nil
	}
	if invite.ExpiresAt.Before(time.Now().UTC()) {
		return false, nil
	}
	return strings.EqualFold(strings.TrimSpace(invite.Email), strings.TrimSpace(email)), nil
}

func (s *BrowserSSOService) provider(providerKey string) (BrowserSSOProvider, error) {
	key := normalizeBrowserSSOProviderKey(providerKey)
	provider, ok := s.providers[key]
	if !ok {
		return nil, ErrBrowserSSOProviderNotFound
	}
	return provider, nil
}

func normalizeBrowserSSOProviderKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func defaultDisplayNameForEmail(email string) string {
	email = strings.TrimSpace(email)
	if email == "" {
		return "New User"
	}
	localPart := email
	if idx := strings.Index(localPart, "@"); idx >= 0 {
		localPart = localPart[:idx]
	}
	localPart = strings.ReplaceAll(localPart, ".", " ")
	localPart = strings.ReplaceAll(localPart, "_", " ")
	localPart = strings.TrimSpace(localPart)
	if localPart == "" {
		return "New User"
	}
	words := strings.Fields(localPart)
	for i := range words {
		if len(words[i]) == 0 {
			continue
		}
		words[i] = strings.ToUpper(words[i][:1]) + strings.ToLower(words[i][1:])
	}
	return strings.Join(words, " ")
}

func sortBrowserSSOProviderDefinitions(items []BrowserSSOProviderDefinition) {
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if strings.ToLower(items[j].DisplayName) < strings.ToLower(items[i].DisplayName) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}
