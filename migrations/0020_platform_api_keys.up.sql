CREATE TABLE IF NOT EXISTS platform_api_keys (
  id TEXT PRIMARY KEY,
  key_prefix TEXT NOT NULL UNIQUE,
  key_hash TEXT NOT NULL,
  name TEXT NOT NULL,
  role TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  expires_at TIMESTAMPTZ,
  revoked_at TIMESTAMPTZ,
  last_used_at TIMESTAMPTZ,
  CONSTRAINT chk_platform_api_keys_role
    CHECK (role IN ('platform_admin'))
);

CREATE INDEX IF NOT EXISTS idx_platform_api_keys_active_lookup
  ON platform_api_keys (key_prefix, created_at DESC);
