package service

import (
	"fmt"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

type PlatformAPIKeyService struct {
	store store.Repository
}

type CreatePlatformAPIKeyRequest struct {
	Name            string     `json:"name"`
	Role            string     `json:"role"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
	OwnerType       string     `json:"owner_type,omitempty"`
	OwnerID         string     `json:"owner_id,omitempty"`
	Purpose         string     `json:"purpose,omitempty"`
	Environment     string     `json:"environment,omitempty"`
	CreatedByUserID string     `json:"created_by_user_id,omitempty"`
}

type CreatePlatformAPIKeyResult struct {
	APIKey domain.PlatformAPIKey `json:"api_key"`
	Secret string                `json:"secret"`
}

func NewPlatformAPIKeyService(s store.Repository) *PlatformAPIKeyService {
	return &PlatformAPIKeyService{store: s}
}

func (s *PlatformAPIKeyService) CreatePlatformAPIKey(req CreatePlatformAPIKeyRequest) (CreatePlatformAPIKeyResult, error) {
	if s == nil || s.store == nil {
		return CreatePlatformAPIKeyResult{}, fmt.Errorf("%w: api key repository is required", ErrValidation)
	}

	role, err := normalizePlatformAPIKeyRole(req.Role)
	if err != nil {
		return CreatePlatformAPIKeyResult{}, fmt.Errorf("%w: invalid role", ErrValidation)
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return CreatePlatformAPIKeyResult{}, fmt.Errorf("%w: name is required", ErrValidation)
	}
	ownerType, err := normalizePlatformCredentialOwnerType(req.OwnerType)
	if err != nil {
		return CreatePlatformAPIKeyResult{}, err
	}

	secret, err := generateAPIKeySecret()
	if err != nil {
		return CreatePlatformAPIKeyResult{}, err
	}
	hashed := hashAPIKey(secret)
	prefix := keyPrefixFromHash(hashed)

	created, err := s.store.CreatePlatformAPIKey(domain.PlatformAPIKey{
		KeyPrefix:       prefix,
		KeyHash:         hashed,
		Name:            name,
		Role:            role,
		OwnerType:       ownerType,
		OwnerID:         strings.TrimSpace(req.OwnerID),
		Purpose:         strings.TrimSpace(req.Purpose),
		Environment:     strings.TrimSpace(req.Environment),
		CreatedByUserID: strings.TrimSpace(req.CreatedByUserID),
		CreatedAt:       time.Now().UTC(),
		ExpiresAt:       req.ExpiresAt,
	})
	if err != nil {
		if err == store.ErrAlreadyExists || err == store.ErrDuplicateKey {
			return CreatePlatformAPIKeyResult{}, fmt.Errorf("%w: api key collision, retry", ErrValidation)
		}
		return CreatePlatformAPIKeyResult{}, err
	}

	return CreatePlatformAPIKeyResult{
		APIKey: created,
		Secret: secret,
	}, nil
}

func (s *PlatformAPIKeyService) CountActivePlatformAPIKeys() (int, error) {
	if s == nil || s.store == nil {
		return 0, fmt.Errorf("%w: api key repository is required", ErrValidation)
	}
	return s.store.CountActivePlatformAPIKeys(time.Now().UTC())
}

func (s *PlatformAPIKeyService) RevokeActivePlatformAPIKeysByName(name string) (int, error) {
	if s == nil || s.store == nil {
		return 0, fmt.Errorf("%w: api key repository is required", ErrValidation)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, fmt.Errorf("%w: name is required", ErrValidation)
	}
	return s.store.RevokeActivePlatformAPIKeysByName(name, time.Now().UTC())
}

func normalizePlatformCredentialOwnerType(raw string) (string, error) {
	ownerType := strings.ToLower(strings.TrimSpace(raw))
	switch ownerType {
	case "", "platform_credential":
		return "platform_credential", nil
	case "bootstrap", "break_glass", "platform_service_account":
		return ownerType, nil
	default:
		return "", fmt.Errorf("%w: invalid owner_type", ErrValidation)
	}
}

func normalizePlatformAPIKeyRole(raw string) (string, error) {
	role := strings.ToLower(strings.TrimSpace(raw))
	switch role {
	case string(apiPlatformRoleAdmin):
		return role, nil
	default:
		return "", fmt.Errorf("%w: unsupported platform role %q", ErrValidation, raw)
	}
}

const apiPlatformRoleAdmin = "platform_admin"
