package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

type workspaceAccessStore interface {
	GetTenant(id string) (domain.Tenant, error)
	GetUser(id string) (domain.User, error)
	GetUserByEmail(email string) (domain.User, error)
	GetUserTenantMembership(userID, tenantID string) (domain.UserTenantMembership, error)
	ListTenantMemberships(tenantID string) ([]domain.UserTenantMembership, error)
	UpsertUserTenantMembership(input domain.UserTenantMembership) (domain.UserTenantMembership, error)
	CreateWorkspaceInvitation(input domain.WorkspaceInvitation) (domain.WorkspaceInvitation, error)
	GetWorkspaceInvitation(id string) (domain.WorkspaceInvitation, error)
	ListWorkspaceInvitations(filter store.WorkspaceInvitationListFilter) ([]domain.WorkspaceInvitation, error)
	UpdateWorkspaceInvitation(input domain.WorkspaceInvitation) (domain.WorkspaceInvitation, error)
}

type WorkspaceAccessService struct {
	store workspaceAccessStore
}

type WorkspaceMember struct {
	UserID       string    `json:"user_id"`
	Email        string    `json:"email"`
	DisplayName  string    `json:"display_name"`
	Role         string    `json:"role"`
	Status       string    `json:"status"`
	PlatformRole string    `json:"platform_role,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type CreateWorkspaceInvitationRequest struct {
	WorkspaceID           string `json:"workspace_id"`
	Email                 string `json:"email"`
	Role                  string `json:"role"`
	InvitedByUserID       string `json:"invited_by_user_id,omitempty"`
	InvitedByPlatformUser bool   `json:"invited_by_platform_user"`
}

func NewWorkspaceAccessService(repo workspaceAccessStore) *WorkspaceAccessService {
	return &WorkspaceAccessService{store: repo}
}

func (s *WorkspaceAccessService) ListWorkspaceMembers(workspaceID string) ([]WorkspaceMember, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("%w: workspace access repository is required", ErrValidation)
	}
	workspaceID = normalizeTenantID(workspaceID)
	if workspaceID == "" {
		return nil, fmt.Errorf("%w: workspace_id is required", ErrValidation)
	}
	if _, err := s.store.GetTenant(workspaceID); err != nil {
		return nil, err
	}
	memberships, err := s.store.ListTenantMemberships(workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]WorkspaceMember, 0, len(memberships))
	for _, membership := range memberships {
		user, userErr := s.store.GetUser(membership.UserID)
		if userErr != nil {
			if errors.Is(userErr, store.ErrNotFound) {
				continue
			}
			return nil, userErr
		}
		out = append(out, WorkspaceMember{
			UserID:       user.ID,
			Email:        user.Email,
			DisplayName:  user.DisplayName,
			Role:         strings.ToLower(strings.TrimSpace(membership.Role)),
			Status:       strings.ToLower(strings.TrimSpace(string(membership.Status))),
			PlatformRole: strings.ToLower(strings.TrimSpace(string(user.PlatformRole))),
			CreatedAt:    membership.CreatedAt,
			UpdatedAt:    membership.UpdatedAt,
		})
	}
	return out, nil
}

func (s *WorkspaceAccessService) UpdateWorkspaceMemberRole(workspaceID, userID, role string) (WorkspaceMember, error) {
	if s == nil || s.store == nil {
		return WorkspaceMember{}, fmt.Errorf("%w: workspace access repository is required", ErrValidation)
	}
	workspaceID = normalizeTenantID(workspaceID)
	userID = strings.TrimSpace(userID)
	role, err := normalizeWorkspaceAccessRole(role)
	if err != nil {
		return WorkspaceMember{}, err
	}
	membership, err := s.store.GetUserTenantMembership(userID, workspaceID)
	if err != nil {
		return WorkspaceMember{}, err
	}
	membership.Role = role
	membership.Status = domain.UserTenantMembershipStatusActive
	membership.UpdatedAt = time.Now().UTC()
	updated, err := s.store.UpsertUserTenantMembership(membership)
	if err != nil {
		return WorkspaceMember{}, err
	}
	user, err := s.store.GetUser(updated.UserID)
	if err != nil {
		return WorkspaceMember{}, err
	}
	return WorkspaceMember{
		UserID:       user.ID,
		Email:        user.Email,
		DisplayName:  user.DisplayName,
		Role:         updated.Role,
		Status:       string(updated.Status),
		PlatformRole: string(user.PlatformRole),
		CreatedAt:    updated.CreatedAt,
		UpdatedAt:    updated.UpdatedAt,
	}, nil
}

func (s *WorkspaceAccessService) RemoveWorkspaceMember(workspaceID, userID string) error {
	if s == nil || s.store == nil {
		return fmt.Errorf("%w: workspace access repository is required", ErrValidation)
	}
	workspaceID = normalizeTenantID(workspaceID)
	userID = strings.TrimSpace(userID)
	if workspaceID == "" || userID == "" {
		return fmt.Errorf("%w: workspace_id and user_id are required", ErrValidation)
	}
	membership, err := s.store.GetUserTenantMembership(userID, workspaceID)
	if err != nil {
		return err
	}
	membership.Status = domain.UserTenantMembershipStatusDisabled
	membership.UpdatedAt = time.Now().UTC()
	_, err = s.store.UpsertUserTenantMembership(membership)
	return err
}

func (s *WorkspaceAccessService) CreateWorkspaceInvitation(req CreateWorkspaceInvitationRequest) (domain.WorkspaceInvitation, error) {
	if s == nil || s.store == nil {
		return domain.WorkspaceInvitation{}, fmt.Errorf("%w: workspace access repository is required", ErrValidation)
	}
	workspaceID := normalizeTenantID(req.WorkspaceID)
	email := strings.ToLower(strings.TrimSpace(req.Email))
	role, err := normalizeWorkspaceAccessRole(req.Role)
	if err != nil {
		return domain.WorkspaceInvitation{}, err
	}
	if workspaceID == "" || email == "" {
		return domain.WorkspaceInvitation{}, fmt.Errorf("%w: workspace_id and email are required", ErrValidation)
	}
	if _, err := s.store.GetTenant(workspaceID); err != nil {
		return domain.WorkspaceInvitation{}, err
	}
	if user, userErr := s.store.GetUserByEmail(email); userErr == nil {
		if membership, membershipErr := s.store.GetUserTenantMembership(user.ID, workspaceID); membershipErr == nil && membership.Status == domain.UserTenantMembershipStatusActive {
			return domain.WorkspaceInvitation{}, fmt.Errorf("%w: user already has workspace access", ErrValidation)
		} else if membershipErr != nil && !errors.Is(membershipErr, store.ErrNotFound) {
			return domain.WorkspaceInvitation{}, membershipErr
		}
	} else if !errors.Is(userErr, store.ErrNotFound) {
		return domain.WorkspaceInvitation{}, userErr
	}
	invites, err := s.store.ListWorkspaceInvitations(store.WorkspaceInvitationListFilter{
		WorkspaceID: workspaceID,
		Status:      string(domain.WorkspaceInvitationStatusPending),
		Email:       email,
		Limit:       1,
	})
	if err != nil {
		return domain.WorkspaceInvitation{}, err
	}
	if len(invites) > 0 {
		return domain.WorkspaceInvitation{}, fmt.Errorf("%w: pending invite already exists", ErrValidation)
	}
	tokenHash, err := newWorkspaceInvitationTokenHash()
	if err != nil {
		return domain.WorkspaceInvitation{}, err
	}
	now := time.Now().UTC()
	return s.store.CreateWorkspaceInvitation(domain.WorkspaceInvitation{
		WorkspaceID:           workspaceID,
		Email:                 email,
		Role:                  role,
		Status:                domain.WorkspaceInvitationStatusPending,
		TokenHash:             tokenHash,
		ExpiresAt:             now.Add(7 * 24 * time.Hour),
		InvitedByUserID:       strings.TrimSpace(req.InvitedByUserID),
		InvitedByPlatformUser: req.InvitedByPlatformUser,
		CreatedAt:             now,
		UpdatedAt:             now,
	})
}

func (s *WorkspaceAccessService) ListWorkspaceInvitations(workspaceID, status string) ([]domain.WorkspaceInvitation, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("%w: workspace access repository is required", ErrValidation)
	}
	workspaceID = normalizeTenantID(workspaceID)
	if workspaceID == "" {
		return nil, fmt.Errorf("%w: workspace_id is required", ErrValidation)
	}
	if _, err := s.store.GetTenant(workspaceID); err != nil {
		return nil, err
	}
	status = strings.ToLower(strings.TrimSpace(status))
	if status != "" {
		if _, err := normalizeWorkspaceInvitationStatus(status); err != nil {
			return nil, err
		}
	}
	return s.store.ListWorkspaceInvitations(store.WorkspaceInvitationListFilter{
		WorkspaceID: workspaceID,
		Status:      status,
		Limit:       100,
	})
}

func (s *WorkspaceAccessService) RevokeWorkspaceInvitation(workspaceID, invitationID string) (domain.WorkspaceInvitation, error) {
	if s == nil || s.store == nil {
		return domain.WorkspaceInvitation{}, fmt.Errorf("%w: workspace access repository is required", ErrValidation)
	}
	workspaceID = normalizeTenantID(workspaceID)
	invitationID = strings.TrimSpace(invitationID)
	if workspaceID == "" || invitationID == "" {
		return domain.WorkspaceInvitation{}, fmt.Errorf("%w: workspace_id and invitation_id are required", ErrValidation)
	}
	invite, err := s.store.GetWorkspaceInvitation(invitationID)
	if err != nil {
		return domain.WorkspaceInvitation{}, err
	}
	if invite.WorkspaceID != workspaceID {
		return domain.WorkspaceInvitation{}, fmt.Errorf("%w: invitation does not belong to workspace", ErrValidation)
	}
	if invite.Status != domain.WorkspaceInvitationStatusPending {
		return domain.WorkspaceInvitation{}, fmt.Errorf("%w: only pending invitations can be revoked", ErrValidation)
	}
	now := time.Now().UTC()
	invite.Status = domain.WorkspaceInvitationStatusRevoked
	invite.RevokedAt = &now
	invite.UpdatedAt = now
	return s.store.UpdateWorkspaceInvitation(invite)
}

func normalizeWorkspaceAccessRole(value string) (string, error) {
	role := strings.ToLower(strings.TrimSpace(value))
	switch role {
	case "reader", "writer", "admin":
		return role, nil
	default:
		return "", fmt.Errorf("%w: invalid workspace role", ErrValidation)
	}
}

func normalizeWorkspaceInvitationStatus(value string) (domain.WorkspaceInvitationStatus, error) {
	status := domain.WorkspaceInvitationStatus(strings.ToLower(strings.TrimSpace(value)))
	switch status {
	case domain.WorkspaceInvitationStatusPending, domain.WorkspaceInvitationStatusAccepted, domain.WorkspaceInvitationStatusExpired, domain.WorkspaceInvitationStatusRevoked:
		return status, nil
	default:
		return "", fmt.Errorf("%w: invalid invitation status", ErrValidation)
	}
}

func newWorkspaceInvitationTokenHash() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate invitation token: %w", err)
	}
	sum := sha256.Sum256([]byte(base64.RawURLEncoding.EncodeToString(raw[:])))
	return base64.RawURLEncoding.EncodeToString(sum[:]), nil
}
