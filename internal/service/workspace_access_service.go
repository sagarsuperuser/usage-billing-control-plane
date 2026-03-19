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
	CreateUser(input domain.User) (domain.User, error)
	UpsertUserPasswordCredential(input domain.UserPasswordCredential) (domain.UserPasswordCredential, error)
	GetUserTenantMembership(userID, tenantID string) (domain.UserTenantMembership, error)
	ListTenantMemberships(tenantID string) ([]domain.UserTenantMembership, error)
	UpsertUserTenantMembership(input domain.UserTenantMembership) (domain.UserTenantMembership, error)
	CreateTenantAuditEvent(input domain.TenantAuditEvent) (domain.TenantAuditEvent, error)
	CreateWorkspaceInvitation(input domain.WorkspaceInvitation) (domain.WorkspaceInvitation, error)
	GetWorkspaceInvitation(id string) (domain.WorkspaceInvitation, error)
	GetWorkspaceInvitationByTokenHash(tokenHash string) (domain.WorkspaceInvitation, error)
	ListWorkspaceInvitations(filter store.WorkspaceInvitationListFilter) ([]domain.WorkspaceInvitation, error)
	UpdateWorkspaceInvitation(input domain.WorkspaceInvitation) (domain.WorkspaceInvitation, error)
}

var (
	ErrWorkspaceInvitationExpired       = errors.New("workspace invitation expired")
	ErrWorkspaceInvitationRevoked       = errors.New("workspace invitation revoked")
	ErrWorkspaceInvitationAccepted      = errors.New("workspace invitation accepted")
	ErrWorkspaceInvitationEmailMismatch = errors.New("workspace invitation email mismatch")
	ErrWorkspaceInvitationAccountExists = errors.New("workspace invitation account already exists")
)

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

type IssuedWorkspaceInvitation struct {
	Invitation domain.WorkspaceInvitation `json:"invitation"`
	Token      string                     `json:"token"`
}

type WorkspaceInvitationPreview struct {
	Invitation          domain.WorkspaceInvitation `json:"invitation"`
	WorkspaceName       string                     `json:"workspace_name"`
	RequiresLogin       bool                       `json:"requires_login"`
	Authenticated       bool                       `json:"authenticated"`
	CurrentUserEmail    string                     `json:"current_user_email,omitempty"`
	EmailMatchesSession bool                       `json:"email_matches_session"`
	CanAccept           bool                       `json:"can_accept"`
	AccountExists       bool                       `json:"account_exists"`
}

type WorkspaceAccessAuditActor struct {
	SubjectType  string `json:"subject_type,omitempty"`
	SubjectID    string `json:"subject_id,omitempty"`
	UserEmail    string `json:"user_email,omitempty"`
	Scope        string `json:"scope,omitempty"`
	PlatformRole string `json:"platform_role,omitempty"`
	APIKeyID     string `json:"api_key_id,omitempty"`
	Reason       string `json:"reason,omitempty"`
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
	return s.UpdateWorkspaceMemberRoleWithAudit(workspaceID, userID, role, WorkspaceAccessAuditActor{})
}

func (s *WorkspaceAccessService) UpdateWorkspaceMemberRoleWithAudit(workspaceID, userID, role string, actor WorkspaceAccessAuditActor) (WorkspaceMember, error) {
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
	previousRole := membership.Role
	previousStatus := membership.Status
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
	member := WorkspaceMember{
		UserID:       user.ID,
		Email:        user.Email,
		DisplayName:  user.DisplayName,
		Role:         updated.Role,
		Status:       string(updated.Status),
		PlatformRole: string(user.PlatformRole),
		CreatedAt:    updated.CreatedAt,
		UpdatedAt:    updated.UpdatedAt,
	}
	if auditErr := s.writeWorkspaceAccessAuditEvent(workspaceID, actor, workspaceAccessAuditEventInput{
		Action: func() string {
			if previousStatus == domain.UserTenantMembershipStatusDisabled {
				return "workspace_member_reactivated"
			}
			return "workspace_member_role_changed"
		}(),
		Metadata: map[string]any{
			"target_user_id":  user.ID,
			"target_email":    user.Email,
			"previous_role":   previousRole,
			"new_role":        updated.Role,
			"previous_status": string(previousStatus),
			"new_status":      string(updated.Status),
		},
	}); auditErr != nil {
		return WorkspaceMember{}, auditErr
	}
	return member, nil
}

func (s *WorkspaceAccessService) RemoveWorkspaceMember(workspaceID, userID string) error {
	return s.RemoveWorkspaceMemberWithAudit(workspaceID, userID, WorkspaceAccessAuditActor{})
}

func (s *WorkspaceAccessService) RemoveWorkspaceMemberWithAudit(workspaceID, userID string, actor WorkspaceAccessAuditActor) error {
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
	user, err := s.store.GetUser(membership.UserID)
	if err != nil {
		return err
	}
	previousRole := membership.Role
	previousStatus := membership.Status
	membership.Status = domain.UserTenantMembershipStatusDisabled
	membership.UpdatedAt = time.Now().UTC()
	updated, err := s.store.UpsertUserTenantMembership(membership)
	if err != nil {
		return err
	}
	return s.writeWorkspaceAccessAuditEvent(workspaceID, actor, workspaceAccessAuditEventInput{
		Action: "workspace_member_disabled",
		Metadata: map[string]any{
			"target_user_id":  user.ID,
			"target_email":    user.Email,
			"previous_role":   previousRole,
			"new_role":        updated.Role,
			"previous_status": string(previousStatus),
			"new_status":      string(updated.Status),
		},
	})
}

func (s *WorkspaceAccessService) CreateWorkspaceInvitation(req CreateWorkspaceInvitationRequest) (domain.WorkspaceInvitation, error) {
	issued, err := s.IssueWorkspaceInvitation(req)
	if err != nil {
		return domain.WorkspaceInvitation{}, err
	}
	return issued.Invitation, nil
}

func (s *WorkspaceAccessService) IssueWorkspaceInvitation(req CreateWorkspaceInvitationRequest) (IssuedWorkspaceInvitation, error) {
	if s == nil || s.store == nil {
		return IssuedWorkspaceInvitation{}, fmt.Errorf("%w: workspace access repository is required", ErrValidation)
	}
	workspaceID := normalizeTenantID(req.WorkspaceID)
	email := strings.ToLower(strings.TrimSpace(req.Email))
	role, err := normalizeWorkspaceAccessRole(req.Role)
	if err != nil {
		return IssuedWorkspaceInvitation{}, err
	}
	if workspaceID == "" || email == "" {
		return IssuedWorkspaceInvitation{}, fmt.Errorf("%w: workspace_id and email are required", ErrValidation)
	}
	if _, err := s.store.GetTenant(workspaceID); err != nil {
		return IssuedWorkspaceInvitation{}, err
	}
	if user, userErr := s.store.GetUserByEmail(email); userErr == nil {
		if membership, membershipErr := s.store.GetUserTenantMembership(user.ID, workspaceID); membershipErr == nil && membership.Status == domain.UserTenantMembershipStatusActive {
			return IssuedWorkspaceInvitation{}, fmt.Errorf("%w: user already has workspace access", ErrValidation)
		} else if membershipErr != nil && !errors.Is(membershipErr, store.ErrNotFound) {
			return IssuedWorkspaceInvitation{}, membershipErr
		}
	} else if !errors.Is(userErr, store.ErrNotFound) {
		return IssuedWorkspaceInvitation{}, userErr
	}
	invites, err := s.store.ListWorkspaceInvitations(store.WorkspaceInvitationListFilter{
		WorkspaceID: workspaceID,
		Status:      string(domain.WorkspaceInvitationStatusPending),
		Email:       email,
		Limit:       1,
	})
	if err != nil {
		return IssuedWorkspaceInvitation{}, err
	}
	if len(invites) > 0 {
		return IssuedWorkspaceInvitation{}, fmt.Errorf("%w: pending invite already exists", ErrValidation)
	}
	token, tokenHash, err := newWorkspaceInvitationToken()
	if err != nil {
		return IssuedWorkspaceInvitation{}, err
	}
	now := time.Now().UTC()
	invite, err := s.store.CreateWorkspaceInvitation(domain.WorkspaceInvitation{
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
	if err != nil {
		return IssuedWorkspaceInvitation{}, err
	}
	return IssuedWorkspaceInvitation{
		Invitation: invite,
		Token:      token,
	}, nil
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
	return s.RevokeWorkspaceInvitationWithAudit(workspaceID, invitationID, WorkspaceAccessAuditActor{})
}

func (s *WorkspaceAccessService) RevokeWorkspaceInvitationWithAudit(workspaceID, invitationID string, actor WorkspaceAccessAuditActor) (domain.WorkspaceInvitation, error) {
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
	updated, err := s.store.UpdateWorkspaceInvitation(invite)
	if err != nil {
		return domain.WorkspaceInvitation{}, err
	}
	if auditErr := s.writeWorkspaceAccessAuditEvent(workspaceID, actor, workspaceAccessAuditEventInput{
		Action: "workspace_invitation_revoked",
		Metadata: map[string]any{
			"invitation_id": updated.ID,
			"target_email":  updated.Email,
			"role":          updated.Role,
			"status":        string(updated.Status),
		},
	}); auditErr != nil {
		return domain.WorkspaceInvitation{}, auditErr
	}
	return updated, nil
}

type workspaceAccessAuditEventInput struct {
	Action   string
	Metadata map[string]any
}

func workspaceAccessTenantAuditActorAPIKeyID(actorAPIKeyID string) string {
	actorAPIKeyID = strings.TrimSpace(actorAPIKeyID)
	if strings.HasPrefix(actorAPIKeyID, "pkey_") {
		return ""
	}
	return actorAPIKeyID
}

func workspaceAccessTenantAuditMetadata(actorAPIKeyID string, metadata map[string]any) map[string]any {
	if metadata == nil {
		metadata = map[string]any{}
	}
	actorAPIKeyID = strings.TrimSpace(actorAPIKeyID)
	if strings.HasPrefix(actorAPIKeyID, "pkey_") {
		metadata["actor_platform_api_key_id"] = actorAPIKeyID
	}
	return metadata
}

func (s *WorkspaceAccessService) writeWorkspaceAccessAuditEvent(workspaceID string, actor WorkspaceAccessAuditActor, input workspaceAccessAuditEventInput) error {
	if s == nil || s.store == nil || strings.TrimSpace(input.Action) == "" {
		return nil
	}
	metadata := workspaceAccessTenantAuditMetadata(actor.APIKeyID, input.Metadata)
	if strings.TrimSpace(actor.SubjectType) != "" {
		metadata["actor_subject_type"] = strings.TrimSpace(actor.SubjectType)
	}
	if strings.TrimSpace(actor.SubjectID) != "" {
		metadata["actor_subject_id"] = strings.TrimSpace(actor.SubjectID)
	}
	if strings.TrimSpace(actor.UserEmail) != "" {
		metadata["actor_user_email"] = strings.TrimSpace(actor.UserEmail)
	}
	if strings.TrimSpace(actor.Scope) != "" {
		metadata["actor_scope"] = strings.TrimSpace(actor.Scope)
	}
	if strings.TrimSpace(actor.PlatformRole) != "" {
		metadata["actor_platform_role"] = strings.TrimSpace(actor.PlatformRole)
	}
	if strings.TrimSpace(actor.Reason) != "" {
		metadata["reason"] = strings.TrimSpace(actor.Reason)
	}
	_, err := s.store.CreateTenantAuditEvent(domain.TenantAuditEvent{
		TenantID:      normalizeTenantID(workspaceID),
		ActorAPIKeyID: workspaceAccessTenantAuditActorAPIKeyID(actor.APIKeyID),
		Action:        strings.TrimSpace(input.Action),
		Metadata:      metadata,
		CreatedAt:     time.Now().UTC(),
	})
	return err
}

func (s *WorkspaceAccessService) PreviewWorkspaceInvitation(token, currentUserEmail string) (WorkspaceInvitationPreview, error) {
	invite, workspace, err := s.lookupWorkspaceInvitation(token)
	if err != nil {
		return WorkspaceInvitationPreview{}, err
	}
	currentUserEmail = strings.ToLower(strings.TrimSpace(currentUserEmail))
	emailMatches := currentUserEmail != "" && currentUserEmail == invite.Email
	accountExists := false
	if _, err := s.store.GetUserByEmail(invite.Email); err == nil {
		accountExists = true
	} else if !errors.Is(err, store.ErrNotFound) {
		return WorkspaceInvitationPreview{}, err
	}
	return WorkspaceInvitationPreview{
		Invitation:          invite,
		WorkspaceName:       workspace.Name,
		RequiresLogin:       currentUserEmail == "",
		Authenticated:       currentUserEmail != "",
		CurrentUserEmail:    currentUserEmail,
		EmailMatchesSession: emailMatches,
		CanAccept:           invite.Status == domain.WorkspaceInvitationStatusPending && !invite.ExpiresAt.Before(time.Now().UTC()) && emailMatches,
		AccountExists:       accountExists,
	}, nil
}

func (s *WorkspaceAccessService) RegisterInvitedUser(token, displayName, password string) (domain.User, domain.WorkspaceInvitation, domain.UserTenantMembership, error) {
	if s == nil || s.store == nil {
		return domain.User{}, domain.WorkspaceInvitation{}, domain.UserTenantMembership{}, fmt.Errorf("%w: workspace access repository is required", ErrValidation)
	}
	invite, _, err := s.lookupWorkspaceInvitation(token)
	if err != nil {
		return domain.User{}, domain.WorkspaceInvitation{}, domain.UserTenantMembership{}, err
	}
	if _, err := s.store.GetUserByEmail(invite.Email); err == nil {
		return domain.User{}, domain.WorkspaceInvitation{}, domain.UserTenantMembership{}, ErrWorkspaceInvitationAccountExists
	} else if !errors.Is(err, store.ErrNotFound) {
		return domain.User{}, domain.WorkspaceInvitation{}, domain.UserTenantMembership{}, err
	}

	passwordHash, err := HashPassword(password)
	if err != nil {
		return domain.User{}, domain.WorkspaceInvitation{}, domain.UserTenantMembership{}, err
	}
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		displayName = defaultDisplayNameForEmail(invite.Email)
	}
	now := time.Now().UTC()
	user, err := s.store.CreateUser(domain.User{
		Email:       invite.Email,
		DisplayName: displayName,
		Status:      domain.UserStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		return domain.User{}, domain.WorkspaceInvitation{}, domain.UserTenantMembership{}, err
	}
	if _, err := s.store.UpsertUserPasswordCredential(domain.UserPasswordCredential{
		UserID:            user.ID,
		PasswordHash:      passwordHash,
		PasswordUpdatedAt: now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}); err != nil {
		return domain.User{}, domain.WorkspaceInvitation{}, domain.UserTenantMembership{}, err
	}
	acceptedInvite, membership, err := s.AcceptWorkspaceInvitation(token, user.ID)
	if err != nil {
		return domain.User{}, domain.WorkspaceInvitation{}, domain.UserTenantMembership{}, err
	}
	return user, acceptedInvite, membership, nil
}

func (s *WorkspaceAccessService) AcceptWorkspaceInvitation(token, userID string) (domain.WorkspaceInvitation, domain.UserTenantMembership, error) {
	if s == nil || s.store == nil {
		return domain.WorkspaceInvitation{}, domain.UserTenantMembership{}, fmt.Errorf("%w: workspace access repository is required", ErrValidation)
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return domain.WorkspaceInvitation{}, domain.UserTenantMembership{}, fmt.Errorf("%w: user id is required", ErrValidation)
	}
	user, err := s.store.GetUser(userID)
	if err != nil {
		return domain.WorkspaceInvitation{}, domain.UserTenantMembership{}, err
	}
	if user.Status != domain.UserStatusActive {
		return domain.WorkspaceInvitation{}, domain.UserTenantMembership{}, ErrBrowserUserDisabled
	}
	invite, _, err := s.lookupWorkspaceInvitation(token)
	if err != nil {
		return domain.WorkspaceInvitation{}, domain.UserTenantMembership{}, err
	}
	if user.Email != invite.Email {
		return domain.WorkspaceInvitation{}, domain.UserTenantMembership{}, ErrWorkspaceInvitationEmailMismatch
	}
	now := time.Now().UTC()
	membership, membershipErr := s.store.GetUserTenantMembership(user.ID, invite.WorkspaceID)
	if membershipErr != nil && !errors.Is(membershipErr, store.ErrNotFound) {
		return domain.WorkspaceInvitation{}, domain.UserTenantMembership{}, membershipErr
	}
	if errors.Is(membershipErr, store.ErrNotFound) {
		membership = domain.UserTenantMembership{
			UserID:    user.ID,
			TenantID:  invite.WorkspaceID,
			CreatedAt: now,
		}
	}
	membership.Role = invite.Role
	membership.Status = domain.UserTenantMembershipStatusActive
	membership.UpdatedAt = now
	updatedMembership, err := s.store.UpsertUserTenantMembership(membership)
	if err != nil {
		return domain.WorkspaceInvitation{}, domain.UserTenantMembership{}, err
	}
	invite.Status = domain.WorkspaceInvitationStatusAccepted
	invite.AcceptedAt = &now
	invite.AcceptedByUserID = user.ID
	invite.UpdatedAt = now
	updatedInvite, err := s.store.UpdateWorkspaceInvitation(invite)
	if err != nil {
		return domain.WorkspaceInvitation{}, domain.UserTenantMembership{}, err
	}
	return updatedInvite, updatedMembership, nil
}

func (s *WorkspaceAccessService) lookupWorkspaceInvitation(token string) (domain.WorkspaceInvitation, domain.Tenant, error) {
	if s == nil || s.store == nil {
		return domain.WorkspaceInvitation{}, domain.Tenant{}, fmt.Errorf("%w: workspace access repository is required", ErrValidation)
	}
	tokenHash := hashWorkspaceInvitationToken(token)
	if tokenHash == "" {
		return domain.WorkspaceInvitation{}, domain.Tenant{}, store.ErrNotFound
	}
	invite, err := s.store.GetWorkspaceInvitationByTokenHash(tokenHash)
	if err != nil {
		return domain.WorkspaceInvitation{}, domain.Tenant{}, err
	}
	workspace, err := s.store.GetTenant(invite.WorkspaceID)
	if err != nil {
		return domain.WorkspaceInvitation{}, domain.Tenant{}, err
	}
	now := time.Now().UTC()
	if invite.Status == domain.WorkspaceInvitationStatusPending && invite.ExpiresAt.Before(now) {
		invite.Status = domain.WorkspaceInvitationStatusExpired
		invite.UpdatedAt = now
		updated, updateErr := s.store.UpdateWorkspaceInvitation(invite)
		if updateErr == nil {
			invite = updated
		}
	}
	switch invite.Status {
	case domain.WorkspaceInvitationStatusAccepted:
		return domain.WorkspaceInvitation{}, domain.Tenant{}, ErrWorkspaceInvitationAccepted
	case domain.WorkspaceInvitationStatusRevoked:
		return domain.WorkspaceInvitation{}, domain.Tenant{}, ErrWorkspaceInvitationRevoked
	case domain.WorkspaceInvitationStatusExpired:
		return domain.WorkspaceInvitation{}, domain.Tenant{}, ErrWorkspaceInvitationExpired
	}
	return invite, workspace, nil
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

func newWorkspaceInvitationToken() (string, string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", "", fmt.Errorf("generate invitation token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	return token, hashWorkspaceInvitationToken(token), nil
}

func hashWorkspaceInvitationToken(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
