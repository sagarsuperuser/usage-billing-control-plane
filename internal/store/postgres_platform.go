package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
)

func (s *PostgresStore) CreatePlatformAPIKey(input domain.PlatformAPIKey) (domain.PlatformAPIKey, error) {
	input.KeyPrefix = strings.TrimSpace(input.KeyPrefix)
	input.KeyHash = strings.ToLower(strings.TrimSpace(input.KeyHash))
	input.Name = strings.TrimSpace(input.Name)
	input.Role = strings.ToLower(strings.TrimSpace(input.Role))

	if input.KeyPrefix == "" || input.KeyHash == "" || input.Role == "" {
		return domain.PlatformAPIKey{}, fmt.Errorf("validation failed: key prefix, key hash, and role are required")
	}
	if input.Name == "" {
		input.Name = input.KeyPrefix
	}
	if input.ID == "" {
		input.ID = newID("pkey")
	}
	if input.CreatedAt.IsZero() {
		input.CreatedAt = time.Now().UTC()
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.PlatformAPIKey{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`INSERT INTO platform_api_keys (
			id, key_prefix, key_hash, name, role, owner_type, owner_id, purpose, environment, created_by_user_id, created_at, expires_at, revoked_at, last_used_at, last_rotated_at, rotation_required_at, revocation_reason
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NULLIF($10,''),$11,$12,$13,$14,$15,$16,$17)
		RETURNING id, key_prefix, key_hash, name, role, owner_type, owner_id, purpose, environment, created_by_user_id, created_at, expires_at, revoked_at, last_used_at, last_rotated_at, rotation_required_at, revocation_reason`,
		input.ID,
		input.KeyPrefix,
		input.KeyHash,
		input.Name,
		input.Role,
		input.OwnerType,
		input.OwnerID,
		input.Purpose,
		input.Environment,
		input.CreatedByUserID,
		input.CreatedAt,
		input.ExpiresAt,
		input.RevokedAt,
		input.LastUsedAt,
		input.LastRotatedAt,
		input.RotationRequiredAt,
		input.RevocationReason,
	)
	key, err := scanPlatformAPIKey(row)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.PlatformAPIKey{}, ErrAlreadyExists
		}
		return domain.PlatformAPIKey{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.PlatformAPIKey{}, err
	}
	return key, nil
}


func (s *PostgresStore) GetPlatformAPIKeyByPrefix(prefix string) (domain.PlatformAPIKey, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return domain.PlatformAPIKey{}, ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.PlatformAPIKey{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`SELECT id, key_prefix, key_hash, name, role, owner_type, owner_id, purpose, environment, created_by_user_id, created_at, expires_at, revoked_at, last_used_at, last_rotated_at, rotation_required_at, revocation_reason
		FROM platform_api_keys
		WHERE key_prefix = $1`,
		prefix,
	)
	key, err := scanPlatformAPIKey(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.PlatformAPIKey{}, ErrNotFound
		}
		return domain.PlatformAPIKey{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.PlatformAPIKey{}, err
	}
	return key, nil
}


func (s *PostgresStore) GetActivePlatformAPIKeyByPrefix(prefix string, at time.Time) (domain.PlatformAPIKey, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return domain.PlatformAPIKey{}, ErrNotFound
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.PlatformAPIKey{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`SELECT id, key_prefix, key_hash, name, role, owner_type, owner_id, purpose, environment, created_by_user_id, created_at, expires_at, revoked_at, last_used_at, last_rotated_at, rotation_required_at, revocation_reason
		FROM platform_api_keys
		WHERE key_prefix = $1
		  AND revoked_at IS NULL
		  AND (expires_at IS NULL OR expires_at > $2)`,
		prefix,
		at,
	)
	key, err := scanPlatformAPIKey(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.PlatformAPIKey{}, ErrNotFound
		}
		return domain.PlatformAPIKey{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.PlatformAPIKey{}, err
	}
	return key, nil
}


func (s *PostgresStore) TouchPlatformAPIKeyLastUsed(id string, usedAt time.Time) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	if usedAt.IsZero() {
		usedAt = time.Now().UTC()
	}

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer rollbackSilently(tx)

	res, err := tx.ExecContext(ctx, `UPDATE platform_api_keys SET last_used_at = $1 WHERE id = $2`, usedAt, id)
	if err != nil {
		return fmt.Errorf("update platform api key last_used_at: %w", err)
	}
	updated, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if updated == 0 {
		return ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func (s *PostgresStore) CountActivePlatformAPIKeys(at time.Time) (int, error) {
	if at.IsZero() {
		at = time.Now().UTC()
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return 0, err
	}
	defer rollbackSilently(tx)

	var count int
	if err := tx.QueryRowContext(
		ctx,
		`SELECT COUNT(*)
		FROM platform_api_keys
		WHERE revoked_at IS NULL
		  AND (expires_at IS NULL OR expires_at > $1)`,
		at,
	).Scan(&count); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *PostgresStore) RevokeActivePlatformAPIKeysByName(name string, revokedAt time.Time) (int, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, ErrNotFound
	}
	if revokedAt.IsZero() {
		revokedAt = time.Now().UTC()
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return 0, err
	}
	defer rollbackSilently(tx)

	res, err := tx.ExecContext(
		ctx,
		`UPDATE platform_api_keys
		SET revoked_at = COALESCE(revoked_at, $1)
		WHERE name = $2
		  AND revoked_at IS NULL
		  AND (expires_at IS NULL OR expires_at > $1)`,
		revokedAt,
		name,
	)
	if err != nil {
		return 0, err
	}
	updated, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return int(updated), nil
}

// ---------------------------------------------------------------------------
// Invoices (first-class billing engine storage)
// ---------------------------------------------------------------------------


func scanPlatformAPIKey(s rowScanner) (domain.PlatformAPIKey, error) {
	var out domain.PlatformAPIKey
	var ownerType sql.NullString
	var ownerID sql.NullString
	var purpose sql.NullString
	var environment sql.NullString
	var createdByUserID sql.NullString
	var expiresAt sql.NullTime
	var revokedAt sql.NullTime
	var lastUsedAt sql.NullTime
	var lastRotatedAt sql.NullTime
	var rotationRequiredAt sql.NullTime
	var revocationReason sql.NullString

	if err := s.Scan(
		&out.ID,
		&out.KeyPrefix,
		&out.KeyHash,
		&out.Name,
		&out.Role,
		&ownerType,
		&ownerID,
		&purpose,
		&environment,
		&createdByUserID,
		&out.CreatedAt,
		&expiresAt,
		&revokedAt,
		&lastUsedAt,
		&lastRotatedAt,
		&rotationRequiredAt,
		&revocationReason,
	); err != nil {
		return domain.PlatformAPIKey{}, err
	}

	out.OwnerType = strings.TrimSpace(ownerType.String)
	out.OwnerID = strings.TrimSpace(ownerID.String)
	out.Purpose = strings.TrimSpace(purpose.String)
	out.Environment = strings.TrimSpace(environment.String)
	out.CreatedByUserID = strings.TrimSpace(createdByUserID.String)
	if expiresAt.Valid {
		t := expiresAt.Time.UTC()
		out.ExpiresAt = &t
	}
	if revokedAt.Valid {
		t := revokedAt.Time.UTC()
		out.RevokedAt = &t
	}
	if lastUsedAt.Valid {
		t := lastUsedAt.Time.UTC()
		out.LastUsedAt = &t
	}
	if lastRotatedAt.Valid {
		t := lastRotatedAt.Time.UTC()
		out.LastRotatedAt = &t
	}
	if rotationRequiredAt.Valid {
		t := rotationRequiredAt.Time.UTC()
		out.RotationRequiredAt = &t
	}
	out.RevocationReason = strings.TrimSpace(revocationReason.String)
	return out, nil
}

