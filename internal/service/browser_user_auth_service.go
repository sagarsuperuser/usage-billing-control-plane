package service

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"usage-billing-control-plane/internal/domain"
)

var (
	ErrInvalidBrowserCredentials   = errors.New("invalid credentials")
	ErrBrowserTenantSelection      = errors.New("tenant selection required")
	ErrBrowserTenantAccessDenied   = errors.New("tenant access denied")
	ErrBrowserUserDisabled         = errors.New("user disabled")
	ErrBrowserPasswordUnavailable  = errors.New("password credential unavailable")
)

type BrowserUserAuthStore interface {
	GetUserByEmail(email string) (domain.User, error)
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
	if email == "" || password == "" {
		return BrowserUserPrincipal{}, ErrInvalidBrowserCredentials
	}

	user, err := s.store.GetUserByEmail(email)
	if err != nil {
		return BrowserUserPrincipal{}, ErrInvalidBrowserCredentials
	}
	if user.Status != domain.UserStatusActive {
		return BrowserUserPrincipal{}, ErrBrowserUserDisabled
	}

	credential, err := s.store.GetUserPasswordCredential(user.ID)
	if err != nil {
		return BrowserUserPrincipal{}, ErrBrowserPasswordUnavailable
	}
	if err := CheckPasswordHash(password, credential.PasswordHash); err != nil {
		return BrowserUserPrincipal{}, ErrInvalidBrowserCredentials
	}

	if user.PlatformRole == domain.UserPlatformRoleAdmin && requestedTenantID == "" {
		return BrowserUserPrincipal{
			User:         user,
			Scope:        "platform",
			PlatformRole: string(domain.UserPlatformRoleAdmin),
		}, nil
	}

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
		return BrowserUserPrincipal{}, ErrBrowserTenantAccessDenied
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
		return BrowserUserPrincipal{}, ErrBrowserTenantSelection
	}

	if user.PlatformRole == domain.UserPlatformRoleAdmin {
		return BrowserUserPrincipal{
			User:         user,
			Scope:        "platform",
			PlatformRole: string(domain.UserPlatformRoleAdmin),
		}, nil
	}

	return BrowserUserPrincipal{}, ErrBrowserTenantAccessDenied
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
