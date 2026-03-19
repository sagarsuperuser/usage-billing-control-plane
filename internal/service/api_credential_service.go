package service

import (
	"fmt"
	"strings"
	"time"
)

type APICredentialActor struct {
	UserID            string
	APIKeyID          string
	PlatformAPIKeyID  string
	CreatedByPlatform bool
}

type APICredentialService struct {
	workspaceKeys *APIKeyService
	platformKeys  *PlatformAPIKeyService
}

type CreateWorkspaceCredentialRequest struct {
	Name        string     `json:"name"`
	Role        string     `json:"role"`
	Purpose     string     `json:"purpose,omitempty"`
	Environment string     `json:"environment,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

type IssueBootstrapWorkspaceCredentialRequest struct {
	Name      string     `json:"name"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type CreatePlatformCredentialRequest struct {
	Name        string     `json:"name"`
	Role        string     `json:"role"`
	Purpose     string     `json:"purpose,omitempty"`
	Environment string     `json:"environment,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

func NewAPICredentialService(workspaceKeys *APIKeyService, platformKeys *PlatformAPIKeyService) *APICredentialService {
	return &APICredentialService{workspaceKeys: workspaceKeys, platformKeys: platformKeys}
}

func (s *APICredentialService) CreateWorkspaceCredential(workspaceID string, actor APICredentialActor, req CreateWorkspaceCredentialRequest) (CreateAPIKeyResult, error) {
	if s == nil || s.workspaceKeys == nil {
		return CreateAPIKeyResult{}, fmt.Errorf("%w: workspace credential service is required", ErrValidation)
	}
	return s.workspaceKeys.CreateAPIKey(normalizeTenantID(workspaceID), strings.TrimSpace(actor.APIKeyID), CreateAPIKeyRequest{
		Name:                  req.Name,
		Role:                  req.Role,
		ExpiresAt:             req.ExpiresAt,
		OwnerType:             "workspace_credential",
		Purpose:               req.Purpose,
		Environment:           req.Environment,
		CreatedByUserID:       strings.TrimSpace(actor.UserID),
		CreatedByPlatformUser: actor.CreatedByPlatform,
		ActorPlatformAPIKeyID: strings.TrimSpace(actor.PlatformAPIKeyID),
	})
}

func (s *APICredentialService) IssueBootstrapWorkspaceCredential(workspaceID string, actor APICredentialActor, req IssueBootstrapWorkspaceCredentialRequest) (CreateAPIKeyResult, error) {
	if s == nil || s.workspaceKeys == nil {
		return CreateAPIKeyResult{}, fmt.Errorf("%w: workspace credential service is required", ErrValidation)
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "bootstrap-admin-" + normalizeTenantID(workspaceID)
	}
	return s.workspaceKeys.CreateAPIKey(normalizeTenantID(workspaceID), strings.TrimSpace(actor.APIKeyID), CreateAPIKeyRequest{
		Name:                  name,
		Role:                  "admin",
		ExpiresAt:             req.ExpiresAt,
		OwnerType:             "bootstrap",
		Purpose:               "Workspace bootstrap admin credential",
		CreatedByUserID:       strings.TrimSpace(actor.UserID),
		CreatedByPlatformUser: true,
		ActorPlatformAPIKeyID: strings.TrimSpace(actor.PlatformAPIKeyID),
	})
}

func (s *APICredentialService) CreatePlatformCredential(actor APICredentialActor, req CreatePlatformCredentialRequest) (CreatePlatformAPIKeyResult, error) {
	if s == nil || s.platformKeys == nil {
		return CreatePlatformAPIKeyResult{}, fmt.Errorf("%w: platform credential service is required", ErrValidation)
	}
	return s.platformKeys.CreatePlatformAPIKey(CreatePlatformAPIKeyRequest{
		Name:            req.Name,
		Role:            req.Role,
		ExpiresAt:       req.ExpiresAt,
		OwnerType:       "platform_credential",
		Purpose:         req.Purpose,
		Environment:     req.Environment,
		CreatedByUserID: strings.TrimSpace(actor.UserID),
	})
}
