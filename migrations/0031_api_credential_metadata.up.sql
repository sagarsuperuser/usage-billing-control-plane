ALTER TABLE api_keys
  ADD COLUMN IF NOT EXISTS owner_type TEXT NOT NULL DEFAULT 'workspace_credential',
  ADD COLUMN IF NOT EXISTS owner_id TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS purpose TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS environment TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS created_by_user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS created_by_platform_user BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS last_rotated_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS rotation_required_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS revocation_reason TEXT NOT NULL DEFAULT '';

ALTER TABLE platform_api_keys
  ADD COLUMN IF NOT EXISTS owner_type TEXT NOT NULL DEFAULT 'platform_credential',
  ADD COLUMN IF NOT EXISTS owner_id TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS purpose TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS environment TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS created_by_user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS last_rotated_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS rotation_required_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS revocation_reason TEXT NOT NULL DEFAULT '';

UPDATE api_keys
SET owner_type = 'workspace_credential'
WHERE owner_type = '';

UPDATE platform_api_keys
SET owner_type = 'platform_credential'
WHERE owner_type = '';
