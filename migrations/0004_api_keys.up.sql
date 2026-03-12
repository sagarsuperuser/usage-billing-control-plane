CREATE TABLE IF NOT EXISTS api_keys (
  id TEXT PRIMARY KEY,
  key_prefix TEXT NOT NULL UNIQUE,
  key_hash TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL DEFAULT '',
  role TEXT NOT NULL,
  tenant_id TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  expires_at TIMESTAMPTZ,
  revoked_at TIMESTAMPTZ,
  last_used_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_api_keys_active_lookup
  ON api_keys (key_prefix)
  WHERE revoked_at IS NULL;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'chk_api_keys_role_allowed'
  ) THEN
    ALTER TABLE api_keys
      ADD CONSTRAINT chk_api_keys_role_allowed
      CHECK (role IN ('reader', 'writer', 'admin'));
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'chk_api_keys_expiration_after_creation'
  ) THEN
    ALTER TABLE api_keys
      ADD CONSTRAINT chk_api_keys_expiration_after_creation
      CHECK (expires_at IS NULL OR expires_at > created_at);
  END IF;
END $$;

