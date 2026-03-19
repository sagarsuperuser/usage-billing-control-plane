package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

type serviceAccountStore interface {
	GetTenant(id string) (domain.Tenant, error)
	CreateServiceAccount(input domain.ServiceAccount) (domain.ServiceAccount, error)
	GetServiceAccount(tenantID, id string) (domain.ServiceAccount, error)
	GetServiceAccountByName(tenantID, name string) (domain.ServiceAccount, error)
	ListServiceAccounts(filter store.ServiceAccountListFilter) ([]domain.ServiceAccount, error)
	ListAPIKeys(filter store.APIKeyListFilter) (store.APIKeyListResult, error)
	GetAPIKeyByID(tenantID, id string) (domain.APIKey, error)
}

type ServiceAccountService struct {
	store   serviceAccountStore
	apiKeys *APIKeyService
}

type WorkspaceServiceAccount struct {
	ServiceAccount        domain.ServiceAccount `json:"service_account"`
	Credentials           []domain.APIKey       `json:"credentials"`
	ActiveCredentialCount int                   `json:"active_credential_count"`
}

type CreateServiceAccountRequest struct {
	Name                   string     `json:"name"`
	Description            string     `json:"description,omitempty"`
	Role                   string     `json:"role"`
	Purpose                string     `json:"purpose,omitempty"`
	Environment            string     `json:"environment,omitempty"`
	IssueInitialCredential bool       `json:"issue_initial_credential,omitempty"`
	CredentialName         string     `json:"credential_name,omitempty"`
	ExpiresAt              *time.Time `json:"expires_at,omitempty"`
}

type CreateServiceAccountResult struct {
	ServiceAccount domain.ServiceAccount `json:"service_account"`
	Credential     *domain.APIKey        `json:"credential,omitempty"`
	Secret         string                `json:"secret,omitempty"`
}

type IssueServiceAccountCredentialRequest struct {
	Name        string     `json:"name,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	OwnerType   string     `json:"owner_type,omitempty"`
	Purpose     string     `json:"purpose,omitempty"`
	Environment string     `json:"environment,omitempty"`
}

func NewServiceAccountService(repo serviceAccountStore, apiKeys *APIKeyService) *ServiceAccountService {
	return &ServiceAccountService{store: repo, apiKeys: apiKeys}
}

func (s *ServiceAccountService) ListWorkspaceServiceAccounts(workspaceID string) ([]WorkspaceServiceAccount, error) {
	if s == nil || s.store == nil || s.apiKeys == nil {
		return nil, fmt.Errorf("%w: service account service is required", ErrValidation)
	}
	workspaceID = normalizeTenantID(workspaceID)
	if workspaceID == "" {
		return nil, fmt.Errorf("%w: workspace_id is required", ErrValidation)
	}
	if _, err := s.store.GetTenant(workspaceID); err != nil {
		return nil, err
	}
	accounts, err := s.store.ListServiceAccounts(store.ServiceAccountListFilter{TenantID: workspaceID, Limit: 100, Offset: 0})
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	out := make([]WorkspaceServiceAccount, 0, len(accounts))
	for _, account := range accounts {
		keys, err := s.store.ListAPIKeys(store.APIKeyListFilter{
			TenantID:      workspaceID,
			OwnerID:       account.ID,
			Limit:         100,
			Offset:        0,
			ReferenceTime: now,
		})
		if err != nil {
			return nil, err
		}
		active := 0
		filtered := make([]domain.APIKey, 0, len(keys.Items))
		for _, item := range keys.Items {
			if item.OwnerType != "service_account" && item.OwnerType != "bootstrap" && item.OwnerType != "break_glass" {
				continue
			}
			filtered = append(filtered, item)
			if item.RevokedAt == nil && (item.ExpiresAt == nil || item.ExpiresAt.After(now)) {
				active++
			}
		}
		out = append(out, WorkspaceServiceAccount{ServiceAccount: account, Credentials: filtered, ActiveCredentialCount: active})
	}
	return out, nil
}

func (s *ServiceAccountService) CreateWorkspaceServiceAccount(workspaceID string, actor APICredentialActor, req CreateServiceAccountRequest) (CreateServiceAccountResult, error) {
	if s == nil || s.store == nil || s.apiKeys == nil {
		return CreateServiceAccountResult{}, fmt.Errorf("%w: service account service is required", ErrValidation)
	}
	workspaceID = normalizeTenantID(workspaceID)
	role, err := normalizeAPIKeyRole(req.Role)
	if err != nil {
		return CreateServiceAccountResult{}, fmt.Errorf("%w: invalid role", ErrValidation)
	}
	name := strings.TrimSpace(req.Name)
	if workspaceID == "" || name == "" {
		return CreateServiceAccountResult{}, fmt.Errorf("%w: workspace_id and name are required", ErrValidation)
	}
	if _, err := s.store.GetTenant(workspaceID); err != nil {
		return CreateServiceAccountResult{}, err
	}
	created, err := s.store.CreateServiceAccount(domain.ServiceAccount{
		TenantID:              workspaceID,
		Name:                  name,
		Description:           strings.TrimSpace(req.Description),
		Role:                  role,
		Purpose:               strings.TrimSpace(req.Purpose),
		Environment:           strings.TrimSpace(req.Environment),
		CreatedByUserID:       strings.TrimSpace(actor.UserID),
		CreatedByPlatformUser: actor.CreatedByPlatform,
		CreatedAt:             time.Now().UTC(),
		UpdatedAt:             time.Now().UTC(),
	})
	if err != nil {
		return CreateServiceAccountResult{}, err
	}
	result := CreateServiceAccountResult{ServiceAccount: created}
	if req.IssueInitialCredential {
		issued, err := s.IssueWorkspaceServiceAccountCredential(workspaceID, created.ID, actor, IssueServiceAccountCredentialRequest{
			Name:      req.CredentialName,
			ExpiresAt: req.ExpiresAt,
		})
		if err != nil {
			return CreateServiceAccountResult{}, err
		}
		result.Credential = &issued.APIKey
		result.Secret = issued.Secret
	}
	return result, nil
}

func (s *ServiceAccountService) EnsureBootstrapServiceAccount(workspaceID, name string, actor APICredentialActor) (domain.ServiceAccount, error) {
	if s == nil || s.store == nil {
		return domain.ServiceAccount{}, fmt.Errorf("%w: service account service is required", ErrValidation)
	}
	workspaceID = normalizeTenantID(workspaceID)
	name = strings.TrimSpace(name)
	if workspaceID == "" || name == "" {
		return domain.ServiceAccount{}, fmt.Errorf("%w: workspace_id and name are required", ErrValidation)
	}
	account, err := s.store.GetServiceAccountByName(workspaceID, name)
	if err == nil {
		return account, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return domain.ServiceAccount{}, err
	}
	return s.store.CreateServiceAccount(domain.ServiceAccount{
		TenantID:              workspaceID,
		Name:                  name,
		Description:           "Bootstrap admin machine identity",
		Role:                  string(domainTenantAdminRole),
		Purpose:               "Workspace bootstrap admin credential",
		CreatedByUserID:       strings.TrimSpace(actor.UserID),
		CreatedByPlatformUser: true,
		CreatedAt:             time.Now().UTC(),
		UpdatedAt:             time.Now().UTC(),
	})
}

func (s *ServiceAccountService) IssueBootstrapWorkspaceServiceAccountCredential(workspaceID, serviceAccountName string, actor APICredentialActor, expiresAt *time.Time) (CreateServiceAccountResult, error) {
	account, err := s.EnsureBootstrapServiceAccount(workspaceID, serviceAccountName, actor)
	if err != nil {
		return CreateServiceAccountResult{}, err
	}
	issued, err := s.IssueWorkspaceServiceAccountCredential(workspaceID, account.ID, actor, IssueServiceAccountCredentialRequest{
		Name:      serviceAccountName,
		ExpiresAt: expiresAt,
		OwnerType: "bootstrap",
		Purpose:   "Workspace bootstrap admin credential",
	})
	if err != nil {
		return CreateServiceAccountResult{}, err
	}
	return CreateServiceAccountResult{ServiceAccount: account, Credential: &issued.APIKey, Secret: issued.Secret}, nil
}

func (s *ServiceAccountService) IssueWorkspaceServiceAccountCredential(workspaceID, serviceAccountID string, actor APICredentialActor, req IssueServiceAccountCredentialRequest) (CreateAPIKeyResult, error) {
	if s == nil || s.store == nil || s.apiKeys == nil {
		return CreateAPIKeyResult{}, fmt.Errorf("%w: service account service is required", ErrValidation)
	}
	account, err := s.getServiceAccount(workspaceID, serviceAccountID)
	if err != nil {
		return CreateAPIKeyResult{}, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = account.Name + " credential " + time.Now().UTC().Format("20060102-150405")
	}
	ownerType := strings.TrimSpace(req.OwnerType)
	if ownerType == "" {
		ownerType = "service_account"
	}
	purpose := strings.TrimSpace(req.Purpose)
	if purpose == "" {
		purpose = account.Purpose
	}
	environment := strings.TrimSpace(req.Environment)
	if environment == "" {
		environment = account.Environment
	}
	return s.apiKeys.CreateAPIKey(normalizeTenantID(workspaceID), strings.TrimSpace(actor.APIKeyID), CreateAPIKeyRequest{
		Name:                  name,
		Role:                  account.Role,
		ExpiresAt:             req.ExpiresAt,
		OwnerType:             ownerType,
		OwnerID:               account.ID,
		Purpose:               purpose,
		Environment:           environment,
		CreatedByUserID:       strings.TrimSpace(actor.UserID),
		CreatedByPlatformUser: actor.CreatedByPlatform,
		ActorPlatformAPIKeyID: strings.TrimSpace(actor.PlatformAPIKeyID),
	})
}

func (s *ServiceAccountService) RotateWorkspaceServiceAccountCredential(workspaceID, serviceAccountID, credentialID string, actor APICredentialActor) (CreateAPIKeyResult, error) {
	if _, err := s.ensureServiceAccountCredential(workspaceID, serviceAccountID, credentialID); err != nil {
		return CreateAPIKeyResult{}, err
	}
	return s.apiKeys.RotateAPIKey(normalizeTenantID(workspaceID), strings.TrimSpace(actor.APIKeyID), strings.TrimSpace(credentialID))
}

func (s *ServiceAccountService) RevokeWorkspaceServiceAccountCredential(workspaceID, serviceAccountID, credentialID string, actor APICredentialActor) (domain.APIKey, error) {
	if _, err := s.ensureServiceAccountCredential(workspaceID, serviceAccountID, credentialID); err != nil {
		return domain.APIKey{}, err
	}
	return s.apiKeys.RevokeAPIKey(normalizeTenantID(workspaceID), strings.TrimSpace(actor.APIKeyID), strings.TrimSpace(credentialID))
}

func (s *ServiceAccountService) GetWorkspaceServiceAccount(workspaceID, serviceAccountID string) (domain.ServiceAccount, error) {
	return s.getServiceAccount(workspaceID, serviceAccountID)
}

func (s *ServiceAccountService) getServiceAccount(workspaceID, serviceAccountID string) (domain.ServiceAccount, error) {
	workspaceID = normalizeTenantID(workspaceID)
	serviceAccountID = strings.TrimSpace(serviceAccountID)
	if workspaceID == "" || serviceAccountID == "" {
		return domain.ServiceAccount{}, fmt.Errorf("%w: workspace_id and service_account_id are required", ErrValidation)
	}
	return s.store.GetServiceAccount(workspaceID, serviceAccountID)
}

func (s *ServiceAccountService) ensureServiceAccountCredential(workspaceID, serviceAccountID, credentialID string) (domain.APIKey, error) {
	if _, err := s.getServiceAccount(workspaceID, serviceAccountID); err != nil {
		return domain.APIKey{}, err
	}
	key, err := s.store.GetAPIKeyByID(normalizeTenantID(workspaceID), strings.TrimSpace(credentialID))
	if err != nil {
		return domain.APIKey{}, err
	}
	if strings.TrimSpace(key.OwnerID) != strings.TrimSpace(serviceAccountID) {
		return domain.APIKey{}, fmt.Errorf("%w: credential does not belong to service account", ErrValidation)
	}
	if key.OwnerType != "service_account" && key.OwnerType != "bootstrap" && key.OwnerType != "break_glass" {
		return domain.APIKey{}, fmt.Errorf("%w: credential does not belong to service account", ErrValidation)
	}
	return key, nil
}
