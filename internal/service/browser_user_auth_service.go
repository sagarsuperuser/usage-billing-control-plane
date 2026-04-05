package service

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"usage-billing-control-plane/internal/domain"
)

var (
	ErrInvalidBrowserCredentials  = errors.New("invalid credentials")
	ErrBrowserTenantAccessDenied  = errors.New("tenant access denied")
	ErrBrowserUserDisabled        = errors.New("user disabled")
	ErrBrowserPasswordUnavailable = errors.New("password credential unavailable")
)

type BrowserUserAuthStore interface {
	GetUserByEmail(email string) (domain.User, error)
	GetUser(id string) (domain.User, error)
	GetTenant(id string) (domain.Tenant, error)
	GetUserPasswordCredential(userID string) (domain.UserPasswordCredential, error)
	ListUserTenantMemberships(userID string) ([]domain.UserTenantMembership, error)
}

type BrowserUserLoginRequest struct {
	Email    string
	Password string
	TenantID string
}

type BrowserUserPrincipal struct {
	User         domain.User
	Scope        string
	Role         string
	PlatformRole string
	TenantID     string
}

type BrowserWorkspaceOption struct {
	TenantID string
	Name     string
	Role     string
}

type BrowserTenantAccessDeniedError struct {
	User domain.User
}

func (e BrowserTenantAccessDeniedError) Error() string {
	return ErrBrowserTenantAccessDenied.Error()
}

func (e BrowserTenantAccessDeniedError) Unwrap() error {
	return ErrBrowserTenantAccessDenied
}

type BrowserUserAuthService struct {
	store BrowserUserAuthStore
}

func NewBrowserUserAuthService(store BrowserUserAuthStore) (*BrowserUserAuthService, error) {
	if store == nil {
		return nil, fmt.Errorf("browser user auth store is required")
	}
	return &BrowserUserAuthService{store: store}, nil
}

func (s *BrowserUserAuthService) Authenticate(req BrowserUserLoginRequest) (BrowserUserPrincipal, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	password := strings.TrimSpace(req.Password)
	requestedTenantID := normalizeBrowserTenantID(strings.TrimSpace(req.TenantID))
	user, err := s.AuthenticateIdentity(email, password)
	if err != nil {
		return BrowserUserPrincipal{}, err
	}
	return s.ResolveUserPrincipal(user, requestedTenantID)
}

func (s *BrowserUserAuthService) AuthenticateIdentity(email, password string) (domain.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	password = strings.TrimSpace(password)
	if email == "" || password == "" {
		return domain.User{}, ErrInvalidBrowserCredentials
	}

	user, err := s.store.GetUserByEmail(email)
	if err != nil {
		return domain.User{}, ErrInvalidBrowserCredentials
	}
	if user.Status != domain.UserStatusActive {
		return domain.User{}, ErrBrowserUserDisabled
	}

	credential, err := s.store.GetUserPasswordCredential(user.ID)
	if err != nil {
		return domain.User{}, ErrBrowserPasswordUnavailable
	}
	if err := CheckPasswordHash(password, credential.PasswordHash); err != nil {
		return domain.User{}, ErrInvalidBrowserCredentials
	}
	return user, nil
}

func (s *BrowserUserAuthService) ResolveUserPrincipal(user domain.User, tenantID string) (BrowserUserPrincipal, error) {
	requestedTenantID := normalizeBrowserTenantID(strings.TrimSpace(tenantID))
	if user.Status != domain.UserStatusActive {
		return BrowserUserPrincipal{}, ErrBrowserUserDisabled
	}

	// Platform admin no longer gets a separate scope in browser sessions.
	// They land in a workspace like everyone else. Platform ops use API keys.

	memberships, err := s.store.ListUserTenantMemberships(user.ID)
	if err != nil {
		return BrowserUserPrincipal{}, fmt.Errorf("list memberships: %w", err)
	}

	activeMemberships := make([]domain.UserTenantMembership, 0, len(memberships))
	for _, membership := range memberships {
		if membership.Status == domain.UserTenantMembershipStatusActive {
			activeMemberships = append(activeMemberships, membership)
		}
	}

	if requestedTenantID != "" {
		for _, membership := range activeMemberships {
			if normalizeBrowserTenantID(membership.TenantID) == requestedTenantID {
				return BrowserUserPrincipal{
					User:     user,
					Scope:    "tenant",
					Role:     strings.ToLower(strings.TrimSpace(membership.Role)),
					TenantID: requestedTenantID,
				}, nil
			}
		}
		return BrowserUserPrincipal{}, BrowserTenantAccessDeniedError{User: user}
	}

	if len(activeMemberships) == 1 {
		membership := activeMemberships[0]
		return BrowserUserPrincipal{
			User:     user,
			Scope:    "tenant",
			Role:     strings.ToLower(strings.TrimSpace(membership.Role)),
			TenantID: normalizeBrowserTenantID(membership.TenantID),
		}, nil
	}

	if len(activeMemberships) > 1 {
		// Auto-select the first workspace alphabetically (Stripe/Linear pattern).
		// Users switch workspaces from the sidebar menu, not a full-page selector.
		best := activeMemberships[0]
		for _, m := range activeMemberships[1:] {
			if strings.ToLower(m.TenantID) < strings.ToLower(best.TenantID) {
				best = m
			}
		}
		return BrowserUserPrincipal{
			User:     user,
			Scope:    "tenant",
			Role:     strings.ToLower(strings.TrimSpace(best.Role)),
			TenantID: normalizeBrowserTenantID(best.TenantID),
		}, nil
	}

	return BrowserUserPrincipal{}, BrowserTenantAccessDeniedError{User: user}
}

func (s *BrowserUserAuthService) ListWorkspaceOptions(userID string) ([]BrowserWorkspaceOption, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("%w: user id is required", ErrValidation)
	}
	memberships, err := s.store.ListUserTenantMemberships(userID)
	if err != nil {
		return nil, fmt.Errorf("list memberships: %w", err)
	}
	out := make([]BrowserWorkspaceOption, 0, len(memberships))
	for _, membership := range memberships {
		if membership.Status != domain.UserTenantMembershipStatusActive {
			continue
		}
		tenantID := normalizeBrowserTenantID(membership.TenantID)
		if tenantID == "" {
			continue
		}
		name := tenantID
		if tenant, tenantErr := s.store.GetTenant(tenantID); tenantErr == nil {
			if trimmed := strings.TrimSpace(tenant.Name); trimmed != "" {
				name = trimmed
			}
		}
		out = append(out, BrowserWorkspaceOption{
			TenantID: tenantID,
			Name:     name,
			Role:     strings.ToLower(strings.TrimSpace(membership.Role)),
		})
	}
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if strings.ToLower(out[j].Name) < strings.ToLower(out[i].Name) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out, nil
}

func HashPassword(password string) (string, error) {
	password = strings.TrimSpace(password)
	if len(password) < 12 {
		return "", fmt.Errorf("password must be at least 12 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func CheckPasswordHash(password, hash string) error {
	password = strings.TrimSpace(password)
	hash = strings.TrimSpace(hash)
	if password == "" || hash == "" {
		return ErrInvalidBrowserCredentials
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func normalizeBrowserTenantID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return strings.ToLower(value)
}
