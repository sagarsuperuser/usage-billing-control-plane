ALTER TABLE platform_api_keys
  DROP COLUMN IF EXISTS revocation_reason,
  DROP COLUMN IF EXISTS rotation_required_at,
  DROP COLUMN IF EXISTS last_rotated_at,
  DROP COLUMN IF EXISTS created_by_user_id,
  DROP COLUMN IF EXISTS environment,
  DROP COLUMN IF EXISTS purpose,
  DROP COLUMN IF EXISTS owner_id,
  DROP COLUMN IF EXISTS owner_type;

ALTER TABLE api_keys
  DROP COLUMN IF EXISTS revocation_reason,
  DROP COLUMN IF EXISTS rotation_required_at,
  DROP COLUMN IF EXISTS last_rotated_at,
  DROP COLUMN IF EXISTS created_by_platform_user,
  DROP COLUMN IF EXISTS created_by_user_id,
  DROP COLUMN IF EXISTS environment,
  DROP COLUMN IF EXISTS purpose,
  DROP COLUMN IF EXISTS owner_id,
  DROP COLUMN IF EXISTS owner_type;
