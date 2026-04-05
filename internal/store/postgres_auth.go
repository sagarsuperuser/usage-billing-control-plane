package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
)

func (s *PostgresStore) CreateUser(input domain.User) (domain.User, error) {
	input.ID = strings.TrimSpace(input.ID)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Status = domain.UserStatus(strings.ToLower(strings.TrimSpace(string(input.Status))))
	input.PlatformRole = domain.UserPlatformRole(strings.ToLower(strings.TrimSpace(string(input.PlatformRole))))
	if input.Email == "" || input.DisplayName == "" || input.Status == "" {
		return domain.User{}, fmt.Errorf("validation failed: email, display_name, and status are required")
	}
	if input.ID == "" {
		input.ID = newID("usr")
	}
	now := time.Now().UTC()
	if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = now
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.User{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `INSERT INTO users (id, email, display_name, status, platform_role, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, email, display_name, status, platform_role, created_at, updated_at`,
		input.ID, input.Email, input.DisplayName, string(input.Status), string(input.PlatformRole), input.CreatedAt, input.UpdatedAt)
	out, err := scanUser(row)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.User{}, ErrAlreadyExists
		}
		return domain.User{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.User{}, err
	}
	return out, nil
}

func (s *PostgresStore) GetUser(id string) (domain.User, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.User{}, ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.User{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `SELECT id, email, display_name, status, platform_role, created_at, updated_at FROM users WHERE id = $1`, id)
	out, err := scanUser(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.User{}, ErrNotFound
		}
		return domain.User{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.User{}, err
	}
	return out, nil
}

func (s *PostgresStore) GetUserByEmail(email string) (domain.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return domain.User{}, ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.User{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `SELECT id, email, display_name, status, platform_role, created_at, updated_at FROM users WHERE lower(email) = lower($1)`, email)
	out, err := scanUser(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.User{}, ErrNotFound
		}
		return domain.User{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.User{}, err
	}
	return out, nil
}

func (s *PostgresStore) UpdateUser(input domain.User) (domain.User, error) {
	input.ID = strings.TrimSpace(input.ID)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Status = domain.UserStatus(strings.ToLower(strings.TrimSpace(string(input.Status))))
	input.PlatformRole = domain.UserPlatformRole(strings.ToLower(strings.TrimSpace(string(input.PlatformRole))))
	if input.ID == "" || input.Email == "" || input.DisplayName == "" || input.Status == "" {
		return domain.User{}, fmt.Errorf("validation failed: id, email, display_name, and status are required")
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = time.Now().UTC()
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.User{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `UPDATE users SET email = $1, display_name = $2, status = $3, platform_role = $4, updated_at = $5
		WHERE id = $6
		RETURNING id, email, display_name, status, platform_role, created_at, updated_at`,
		input.Email, input.DisplayName, string(input.Status), string(input.PlatformRole), input.UpdatedAt, input.ID)
	out, err := scanUser(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.User{}, ErrNotFound
		}
		if isUniqueViolation(err) {
			return domain.User{}, ErrAlreadyExists
		}
		return domain.User{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.User{}, err
	}
	return out, nil
}

func (s *PostgresStore) UpsertUserPasswordCredential(input domain.UserPasswordCredential) (domain.UserPasswordCredential, error) {
	input.UserID = strings.TrimSpace(input.UserID)
	input.PasswordHash = strings.TrimSpace(input.PasswordHash)
	if input.UserID == "" || input.PasswordHash == "" {
		return domain.UserPasswordCredential{}, fmt.Errorf("validation failed: user_id and password_hash are required")
	}
	now := time.Now().UTC()
	if input.PasswordUpdatedAt.IsZero() {
		input.PasswordUpdatedAt = now
	}
	if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = now
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.UserPasswordCredential{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `INSERT INTO user_password_credentials (user_id, password_hash, password_updated_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id) DO UPDATE
		SET password_hash = EXCLUDED.password_hash,
		    password_updated_at = EXCLUDED.password_updated_at,
		    updated_at = EXCLUDED.updated_at
		RETURNING user_id, password_hash, password_updated_at, created_at, updated_at`,
		input.UserID, input.PasswordHash, input.PasswordUpdatedAt, input.CreatedAt, input.UpdatedAt)
	out, err := scanUserPasswordCredential(row)
	if err != nil {
		return domain.UserPasswordCredential{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.UserPasswordCredential{}, err
	}
	return out, nil
}

func (s *PostgresStore) GetUserPasswordCredential(userID string) (domain.UserPasswordCredential, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return domain.UserPasswordCredential{}, ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.UserPasswordCredential{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `SELECT user_id, password_hash, password_updated_at, created_at, updated_at FROM user_password_credentials WHERE user_id = $1`, userID)
	out, err := scanUserPasswordCredential(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.UserPasswordCredential{}, ErrNotFound
		}
		return domain.UserPasswordCredential{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.UserPasswordCredential{}, err
	}
	return out, nil
}

func (s *PostgresStore) CreatePasswordResetToken(input domain.PasswordResetToken) (domain.PasswordResetToken, error) {
	input.UserID = strings.TrimSpace(input.UserID)
	input.TokenHash = strings.TrimSpace(input.TokenHash)
	if input.UserID == "" || input.TokenHash == "" || input.ExpiresAt.IsZero() {
		return domain.PasswordResetToken{}, fmt.Errorf("validation failed: user_id, token_hash, and expires_at are required")
	}
	now := time.Now().UTC()
	if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = now
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.PasswordResetToken{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `INSERT INTO password_reset_tokens (user_id, token_hash, expires_at, used_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, user_id, token_hash, expires_at, used_at, created_at, updated_at`,
		input.UserID, input.TokenHash, input.ExpiresAt, input.UsedAt, input.CreatedAt, input.UpdatedAt)
	out, err := scanPasswordResetToken(row)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.PasswordResetToken{}, ErrAlreadyExists
		}
		return domain.PasswordResetToken{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.PasswordResetToken{}, err
	}
	return out, nil
}

func (s *PostgresStore) GetPasswordResetTokenByTokenHash(tokenHash string) (domain.PasswordResetToken, error) {
	tokenHash = strings.TrimSpace(tokenHash)
	if tokenHash == "" {
		return domain.PasswordResetToken{}, ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.PasswordResetToken{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `SELECT id, user_id, token_hash, expires_at, used_at, created_at, updated_at FROM password_reset_tokens WHERE token_hash = $1`, tokenHash)
	out, err := scanPasswordResetToken(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.PasswordResetToken{}, ErrNotFound
		}
		return domain.PasswordResetToken{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.PasswordResetToken{}, err
	}
	return out, nil
}

func (s *PostgresStore) UpdatePasswordResetToken(input domain.PasswordResetToken) (domain.PasswordResetToken, error) {
	input.ID = strings.TrimSpace(input.ID)
	input.UserID = strings.TrimSpace(input.UserID)
	input.TokenHash = strings.TrimSpace(input.TokenHash)
	if input.ID == "" || input.UserID == "" || input.TokenHash == "" || input.ExpiresAt.IsZero() {
		return domain.PasswordResetToken{}, fmt.Errorf("validation failed: id, user_id, token_hash, and expires_at are required")
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = time.Now().UTC()
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.PasswordResetToken{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `UPDATE password_reset_tokens SET user_id = $1, token_hash = $2, expires_at = $3, used_at = $4, updated_at = $5
		WHERE id = $6
		RETURNING id, user_id, token_hash, expires_at, used_at, created_at, updated_at`,
		input.UserID, input.TokenHash, input.ExpiresAt, input.UsedAt, input.UpdatedAt, input.ID)
	out, err := scanPasswordResetToken(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.PasswordResetToken{}, ErrNotFound
		}
		if isUniqueViolation(err) {
			return domain.PasswordResetToken{}, ErrAlreadyExists
		}
		return domain.PasswordResetToken{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.PasswordResetToken{}, err
	}
	return out, nil
}

func (s *PostgresStore) UpsertUserTenantMembership(input domain.UserTenantMembership) (domain.UserTenantMembership, error) {
	input.UserID = strings.TrimSpace(input.UserID)
	input.TenantID = normalizeTenantID(input.TenantID)
	input.Role = strings.ToLower(strings.TrimSpace(input.Role))
	input.Status = domain.UserTenantMembershipStatus(strings.ToLower(strings.TrimSpace(string(input.Status))))
	if input.UserID == "" || input.TenantID == "" || input.Role == "" || input.Status == "" {
		return domain.UserTenantMembership{}, fmt.Errorf("validation failed: user_id, tenant_id, role, and status are required")
	}
	now := time.Now().UTC()
	if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = now
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.UserTenantMembership{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `INSERT INTO user_tenant_memberships (user_id, tenant_id, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, tenant_id) DO UPDATE
		SET role = EXCLUDED.role,
		    status = EXCLUDED.status,
		    updated_at = EXCLUDED.updated_at
		RETURNING user_id, tenant_id, role, status, created_at, updated_at`,
		input.UserID, input.TenantID, input.Role, string(input.Status), input.CreatedAt, input.UpdatedAt)
	out, err := scanUserTenantMembership(row)
	if err != nil {
		return domain.UserTenantMembership{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.UserTenantMembership{}, err
	}
	return out, nil
}

func (s *PostgresStore) GetUserTenantMembership(userID, tenantID string) (domain.UserTenantMembership, error) {
	userID = strings.TrimSpace(userID)
	tenantID = normalizeTenantID(tenantID)
	if userID == "" || tenantID == "" {
		return domain.UserTenantMembership{}, ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.UserTenantMembership{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `SELECT user_id, tenant_id, role, status, created_at, updated_at FROM user_tenant_memberships WHERE user_id = $1 AND tenant_id = $2`, userID, tenantID)
	out, err := scanUserTenantMembership(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.UserTenantMembership{}, ErrNotFound
		}
		return domain.UserTenantMembership{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.UserTenantMembership{}, err
	}
	return out, nil
}

func (s *PostgresStore) ListUserTenantMemberships(userID string) ([]domain.UserTenantMembership, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, nil
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return nil, err
	}
	defer rollbackSilently(tx)

	rows, err := tx.QueryContext(ctx, `SELECT user_id, tenant_id, role, status, created_at, updated_at FROM user_tenant_memberships WHERE user_id = $1 ORDER BY created_at ASC, tenant_id ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.UserTenantMembership, 0)
	for rows.Next() {
		item, scanErr := scanUserTenantMembership(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PostgresStore) ListTenantMemberships(tenantID string) ([]domain.UserTenantMembership, error) {
	tenantID = normalizeTenantID(tenantID)
	if tenantID == "" {
		return nil, nil
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return nil, err
	}
	defer rollbackSilently(tx)

	rows, err := tx.QueryContext(ctx, `SELECT user_id, tenant_id, role, status, created_at, updated_at FROM user_tenant_memberships WHERE tenant_id = $1 ORDER BY created_at ASC, user_id ASC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.UserTenantMembership, 0)
	for rows.Next() {
		item, scanErr := scanUserTenantMembership(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PostgresStore) CreateWorkspaceInvitation(input domain.WorkspaceInvitation) (domain.WorkspaceInvitation, error) {
	input.ID = strings.TrimSpace(input.ID)
	input.WorkspaceID = normalizeTenantID(input.WorkspaceID)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Role = strings.ToLower(strings.TrimSpace(input.Role))
	input.Status = domain.WorkspaceInvitationStatus(strings.ToLower(strings.TrimSpace(string(input.Status))))
	input.TokenHash = strings.TrimSpace(input.TokenHash)
	input.AcceptedByUserID = strings.TrimSpace(input.AcceptedByUserID)
	input.InvitedByUserID = strings.TrimSpace(input.InvitedByUserID)
	if input.WorkspaceID == "" || input.Email == "" || input.Role == "" || input.Status == "" || input.TokenHash == "" || input.ExpiresAt.IsZero() {
		return domain.WorkspaceInvitation{}, fmt.Errorf("validation failed: workspace_id, email, role, status, token_hash, and expires_at are required")
	}
	if input.ID == "" {
		input.ID = newID("wsi")
	}
	now := time.Now().UTC()
	if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = now
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.WorkspaceInvitation{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `INSERT INTO workspace_invitations (
		id, workspace_id, email, role, status, token_hash, expires_at, accepted_at, accepted_by_user_id,
		invited_by_user_id, invited_by_platform_user, revoked_at, created_at, updated_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9,''),NULLIF($10,''),$11,$12,$13,$14)
	RETURNING id, workspace_id, email, role, status, token_hash, expires_at, accepted_at, accepted_by_user_id,
		invited_by_user_id, invited_by_platform_user, revoked_at, created_at, updated_at`,
		input.ID,
		input.WorkspaceID,
		input.Email,
		input.Role,
		string(input.Status),
		input.TokenHash,
		input.ExpiresAt,
		input.AcceptedAt,
		input.AcceptedByUserID,
		input.InvitedByUserID,
		input.InvitedByPlatformUser,
		input.RevokedAt,
		input.CreatedAt,
		input.UpdatedAt,
	)
	out, err := scanWorkspaceInvitation(row)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.WorkspaceInvitation{}, ErrAlreadyExists
		}
		return domain.WorkspaceInvitation{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.WorkspaceInvitation{}, err
	}
	return out, nil
}

func (s *PostgresStore) GetWorkspaceInvitation(id string) (domain.WorkspaceInvitation, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.WorkspaceInvitation{}, ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.WorkspaceInvitation{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `SELECT id, workspace_id, email, role, status, token_hash, expires_at, accepted_at, accepted_by_user_id,
		invited_by_user_id, invited_by_platform_user, revoked_at, created_at, updated_at
		FROM workspace_invitations
		WHERE id = $1`, id)
	out, err := scanWorkspaceInvitation(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.WorkspaceInvitation{}, ErrNotFound
		}
		return domain.WorkspaceInvitation{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.WorkspaceInvitation{}, err
	}
	return out, nil
}

func (s *PostgresStore) GetWorkspaceInvitationByTokenHash(tokenHash string) (domain.WorkspaceInvitation, error) {
	tokenHash = strings.TrimSpace(tokenHash)
	if tokenHash == "" {
		return domain.WorkspaceInvitation{}, ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.WorkspaceInvitation{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `SELECT id, workspace_id, email, role, status, token_hash, expires_at, accepted_at, accepted_by_user_id,
		invited_by_user_id, invited_by_platform_user, revoked_at, created_at, updated_at
		FROM workspace_invitations
		WHERE token_hash = $1`, tokenHash)
	out, err := scanWorkspaceInvitation(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.WorkspaceInvitation{}, ErrNotFound
		}
		return domain.WorkspaceInvitation{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.WorkspaceInvitation{}, err
	}
	return out, nil
}

func (s *PostgresStore) ListWorkspaceInvitations(filter WorkspaceInvitationListFilter) ([]domain.WorkspaceInvitation, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return nil, err
	}
	defer rollbackSilently(tx)

	clauses := []string{"1=1"}
	args := []any{}
	nextArg := 1
	if workspaceID := normalizeTenantID(filter.WorkspaceID); workspaceID != "" {
		clauses = append(clauses, fmt.Sprintf("workspace_id = $%d", nextArg))
		args = append(args, workspaceID)
		nextArg++
	}
	if status := strings.ToLower(strings.TrimSpace(filter.Status)); status != "" {
		clauses = append(clauses, fmt.Sprintf("status = $%d", nextArg))
		args = append(args, status)
		nextArg++
	}
	if email := strings.ToLower(strings.TrimSpace(filter.Email)); email != "" {
		clauses = append(clauses, fmt.Sprintf("lower(email) = lower($%d)", nextArg))
		args = append(args, email)
		nextArg++
	}
	query := fmt.Sprintf(`SELECT id, workspace_id, email, role, status, token_hash, expires_at, accepted_at, accepted_by_user_id,
		invited_by_user_id, invited_by_platform_user, revoked_at, created_at, updated_at
		FROM workspace_invitations
		WHERE %s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d OFFSET $%d`, strings.Join(clauses, " AND "), nextArg, nextArg+1)
	args = append(args, limit, offset)

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.WorkspaceInvitation, 0)
	for rows.Next() {
		item, scanErr := scanWorkspaceInvitation(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PostgresStore) UpdateWorkspaceInvitation(input domain.WorkspaceInvitation) (domain.WorkspaceInvitation, error) {
	input.ID = strings.TrimSpace(input.ID)
	input.WorkspaceID = normalizeTenantID(input.WorkspaceID)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Role = strings.ToLower(strings.TrimSpace(input.Role))
	input.Status = domain.WorkspaceInvitationStatus(strings.ToLower(strings.TrimSpace(string(input.Status))))
	input.TokenHash = strings.TrimSpace(input.TokenHash)
	input.AcceptedByUserID = strings.TrimSpace(input.AcceptedByUserID)
	input.InvitedByUserID = strings.TrimSpace(input.InvitedByUserID)
	if input.ID == "" || input.WorkspaceID == "" || input.Email == "" || input.Role == "" || input.Status == "" || input.TokenHash == "" || input.ExpiresAt.IsZero() {
		return domain.WorkspaceInvitation{}, fmt.Errorf("validation failed: id, workspace_id, email, role, status, token_hash, and expires_at are required")
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = time.Now().UTC()
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.WorkspaceInvitation{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `UPDATE workspace_invitations
		SET workspace_id = $1,
		    email = $2,
		    role = $3,
		    status = $4,
		    token_hash = $5,
		    expires_at = $6,
		    accepted_at = $7,
		    accepted_by_user_id = NULLIF($8,''),
		    invited_by_user_id = NULLIF($9,''),
		    invited_by_platform_user = $10,
		    revoked_at = $11,
		    updated_at = $12
		WHERE id = $13
		RETURNING id, workspace_id, email, role, status, token_hash, expires_at, accepted_at, accepted_by_user_id,
			invited_by_user_id, invited_by_platform_user, revoked_at, created_at, updated_at`,
		input.WorkspaceID,
		input.Email,
		input.Role,
		string(input.Status),
		input.TokenHash,
		input.ExpiresAt,
		input.AcceptedAt,
		input.AcceptedByUserID,
		input.InvitedByUserID,
		input.InvitedByPlatformUser,
		input.RevokedAt,
		input.UpdatedAt,
		input.ID,
	)
	out, err := scanWorkspaceInvitation(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.WorkspaceInvitation{}, ErrNotFound
		}
		if isUniqueViolation(err) {
			return domain.WorkspaceInvitation{}, ErrAlreadyExists
		}
		return domain.WorkspaceInvitation{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.WorkspaceInvitation{}, err
	}
	return out, nil
}

func (s *PostgresStore) GetUserFederatedIdentity(providerKey, subject string) (domain.UserFederatedIdentity, error) {
	providerKey = strings.ToLower(strings.TrimSpace(providerKey))
	subject = strings.TrimSpace(subject)
	if providerKey == "" || subject == "" {
		return domain.UserFederatedIdentity{}, ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.UserFederatedIdentity{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `SELECT id, user_id, provider_key, provider_type, subject, email, email_verified, last_login_at, created_at, updated_at
		FROM user_federated_identities
		WHERE provider_key = $1 AND subject = $2`,
		providerKey, subject)
	out, err := scanUserFederatedIdentity(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.UserFederatedIdentity{}, ErrNotFound
		}
		return domain.UserFederatedIdentity{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.UserFederatedIdentity{}, err
	}
	return out, nil
}

func (s *PostgresStore) UpsertUserFederatedIdentity(input domain.UserFederatedIdentity) (domain.UserFederatedIdentity, error) {
	input.ID = strings.TrimSpace(input.ID)
	input.UserID = strings.TrimSpace(input.UserID)
	input.ProviderKey = strings.ToLower(strings.TrimSpace(input.ProviderKey))
	input.ProviderType = domain.BrowserSSOProviderType(strings.ToLower(strings.TrimSpace(string(input.ProviderType))))
	input.Subject = strings.TrimSpace(input.Subject)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if input.UserID == "" || input.ProviderKey == "" || input.ProviderType == "" || input.Subject == "" {
		return domain.UserFederatedIdentity{}, fmt.Errorf("validation failed: user_id, provider_key, provider_type, and subject are required")
	}
	if input.ID == "" {
		input.ID = newID("ufi")
	}
	now := time.Now().UTC()
	if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = now
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.UserFederatedIdentity{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `INSERT INTO user_federated_identities (
			id, user_id, provider_key, provider_type, subject, email, email_verified, last_login_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (provider_key, subject) DO UPDATE
		SET user_id = EXCLUDED.user_id,
		    provider_type = EXCLUDED.provider_type,
		    email = EXCLUDED.email,
		    email_verified = EXCLUDED.email_verified,
		    last_login_at = EXCLUDED.last_login_at,
		    updated_at = EXCLUDED.updated_at
		RETURNING id, user_id, provider_key, provider_type, subject, email, email_verified, last_login_at, created_at, updated_at`,
		input.ID, input.UserID, input.ProviderKey, string(input.ProviderType), input.Subject, input.Email, input.EmailVerified, input.LastLoginAt, input.CreatedAt, input.UpdatedAt)
	out, err := scanUserFederatedIdentity(row)
	if err != nil {
		return domain.UserFederatedIdentity{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.UserFederatedIdentity{}, err
	}
	return out, nil
}


func scanWorkspaceInvitation(s rowScanner) (domain.WorkspaceInvitation, error) {
	var out domain.WorkspaceInvitation
	var status string
	var acceptedAt sql.NullTime
	var acceptedByUserID sql.NullString
	var invitedByUserID sql.NullString
	var revokedAt sql.NullTime
	if err := s.Scan(
		&out.ID,
		&out.WorkspaceID,
		&out.Email,
		&out.Role,
		&status,
		&out.TokenHash,
		&out.ExpiresAt,
		&acceptedAt,
		&acceptedByUserID,
		&invitedByUserID,
		&out.InvitedByPlatformUser,
		&revokedAt,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		return domain.WorkspaceInvitation{}, err
	}
	out.ID = strings.TrimSpace(out.ID)
	out.WorkspaceID = normalizeTenantID(out.WorkspaceID)
	out.Email = strings.ToLower(strings.TrimSpace(out.Email))
	out.Role = strings.ToLower(strings.TrimSpace(out.Role))
	out.Status = domain.WorkspaceInvitationStatus(strings.ToLower(strings.TrimSpace(status)))
	out.TokenHash = strings.TrimSpace(out.TokenHash)
	if acceptedAt.Valid {
		t := acceptedAt.Time.UTC()
		out.AcceptedAt = &t
	}
	if acceptedByUserID.Valid {
		out.AcceptedByUserID = strings.TrimSpace(acceptedByUserID.String)
	}
	if invitedByUserID.Valid {
		out.InvitedByUserID = strings.TrimSpace(invitedByUserID.String)
	}
	if revokedAt.Valid {
		t := revokedAt.Time.UTC()
		out.RevokedAt = &t
	}
	return out, nil
}


func scanUser(s rowScanner) (domain.User, error) {
	var out domain.User
	var status string
	var platformRole string
	if err := s.Scan(&out.ID, &out.Email, &out.DisplayName, &status, &platformRole, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return domain.User{}, err
	}
	out.ID = strings.TrimSpace(out.ID)
	out.Email = strings.ToLower(strings.TrimSpace(out.Email))
	out.DisplayName = strings.TrimSpace(out.DisplayName)
	out.Status = domain.UserStatus(strings.ToLower(strings.TrimSpace(status)))
	out.PlatformRole = domain.UserPlatformRole(strings.ToLower(strings.TrimSpace(platformRole)))
	return out, nil
}

func scanUserPasswordCredential(s rowScanner) (domain.UserPasswordCredential, error) {
	var out domain.UserPasswordCredential
	if err := s.Scan(&out.UserID, &out.PasswordHash, &out.PasswordUpdatedAt, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return domain.UserPasswordCredential{}, err
	}
	out.UserID = strings.TrimSpace(out.UserID)
	out.PasswordHash = strings.TrimSpace(out.PasswordHash)
	return out, nil
}

func scanPasswordResetToken(s rowScanner) (domain.PasswordResetToken, error) {
	var out domain.PasswordResetToken
	if err := s.Scan(&out.ID, &out.UserID, &out.TokenHash, &out.ExpiresAt, &out.UsedAt, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return domain.PasswordResetToken{}, err
	}
	out.ID = strings.TrimSpace(out.ID)
	out.UserID = strings.TrimSpace(out.UserID)
	out.TokenHash = strings.TrimSpace(out.TokenHash)
	return out, nil
}

func scanUserTenantMembership(s rowScanner) (domain.UserTenantMembership, error) {
	var out domain.UserTenantMembership
	var status string
	if err := s.Scan(&out.UserID, &out.TenantID, &out.Role, &status, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return domain.UserTenantMembership{}, err
	}
	out.UserID = strings.TrimSpace(out.UserID)
	out.TenantID = normalizeTenantID(out.TenantID)
	out.Role = strings.ToLower(strings.TrimSpace(out.Role))
	out.Status = domain.UserTenantMembershipStatus(strings.ToLower(strings.TrimSpace(status)))
	return out, nil
}

func scanUserFederatedIdentity(s rowScanner) (domain.UserFederatedIdentity, error) {
	var out domain.UserFederatedIdentity
	var providerType string
	if err := s.Scan(&out.ID, &out.UserID, &out.ProviderKey, &providerType, &out.Subject, &out.Email, &out.EmailVerified, &out.LastLoginAt, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return domain.UserFederatedIdentity{}, err
	}
	out.ID = strings.TrimSpace(out.ID)
	out.UserID = strings.TrimSpace(out.UserID)
	out.ProviderKey = strings.ToLower(strings.TrimSpace(out.ProviderKey))
	out.ProviderType = domain.BrowserSSOProviderType(strings.ToLower(strings.TrimSpace(providerType)))
	out.Subject = strings.TrimSpace(out.Subject)
	out.Email = strings.ToLower(strings.TrimSpace(out.Email))
	return out, nil
}


// RegisterUserWithWorkspace creates a user, password credential, tenant, and
// membership in a single transaction. If any step fails, everything rolls back.
// No orphaned users possible.
func (s *PostgresStore) RegisterUserWithWorkspace(input RegisterUserInput) (RegisterUserResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.queryTimeout)
	defer cancel()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return RegisterUserResult{}, fmt.Errorf("begin registration tx: %w", err)
	}
	defer rollbackSilently(tx)

	// 1. Create user
	userID := newID("usr")
	var user domain.User
	err = tx.QueryRowContext(ctx,
		`INSERT INTO users (id, email, display_name, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, NOW(), NOW())
		 RETURNING id, email, display_name, status, created_at, updated_at`,
		userID, input.Email, input.DisplayName, domain.UserStatusActive,
	).Scan(&user.ID, &user.Email, &user.DisplayName, &user.Status, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return RegisterUserResult{}, ErrAlreadyExists
		}
		return RegisterUserResult{}, fmt.Errorf("create user: %w", err)
	}

	// 2. Store password credential
	_, err = tx.ExecContext(ctx,
		`INSERT INTO user_password_credentials (user_id, password_hash, password_updated_at, created_at, updated_at)
		 VALUES ($1, $2, NOW(), NOW(), NOW())
		 ON CONFLICT (user_id) DO UPDATE SET password_hash = $2, password_updated_at = NOW(), updated_at = NOW()`,
		user.ID, input.PasswordHash,
	)
	if err != nil {
		return RegisterUserResult{}, fmt.Errorf("store password: %w", err)
	}

	// 3. Create tenant
	var tenant domain.Tenant
	err = tx.QueryRowContext(ctx,
		`INSERT INTO tenants (id, name, status, created_at, updated_at)
		 VALUES ($1, $2, 'active', NOW(), NOW())
		 RETURNING id, name, status, created_at, updated_at`,
		input.TenantID, input.WorkspaceName,
	).Scan(&tenant.ID, &tenant.Name, &tenant.Status, &tenant.CreatedAt, &tenant.UpdatedAt)
	if err != nil {
		return RegisterUserResult{}, fmt.Errorf("create tenant: %w", err)
	}

	// 4. Create admin membership
	_, err = tx.ExecContext(ctx,
		`INSERT INTO user_tenant_memberships (user_id, tenant_id, role, status, created_at, updated_at)
		 VALUES ($1, $2, 'admin', 'active', NOW(), NOW())
		 ON CONFLICT (user_id, tenant_id) DO UPDATE SET role = 'admin', status = 'active', updated_at = NOW()`,
		user.ID, tenant.ID,
	)
	if err != nil {
		return RegisterUserResult{}, fmt.Errorf("create membership: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return RegisterUserResult{}, fmt.Errorf("commit registration: %w", err)
	}

	return RegisterUserResult{User: user, Tenant: tenant}, nil
}

type RegisterUserInput struct {
	Email         string
	DisplayName   string
	PasswordHash  string
	TenantID      string
	WorkspaceName string
}

type RegisterUserResult struct {
	User   domain.User
	Tenant domain.Tenant
}
