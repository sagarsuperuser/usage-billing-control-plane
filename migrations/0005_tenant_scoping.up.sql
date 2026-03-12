ALTER TABLE rating_rule_versions
  ADD COLUMN IF NOT EXISTS tenant_id TEXT DEFAULT 'default';
UPDATE rating_rule_versions SET tenant_id = 'default' WHERE tenant_id IS NULL OR tenant_id = '';
ALTER TABLE rating_rule_versions
  ALTER COLUMN tenant_id SET DEFAULT 'default',
  ALTER COLUMN tenant_id SET NOT NULL;

ALTER TABLE meters
  ADD COLUMN IF NOT EXISTS tenant_id TEXT DEFAULT 'default';
UPDATE meters SET tenant_id = 'default' WHERE tenant_id IS NULL OR tenant_id = '';
ALTER TABLE meters
  ALTER COLUMN tenant_id SET DEFAULT 'default',
  ALTER COLUMN tenant_id SET NOT NULL;

ALTER TABLE usage_events
  ADD COLUMN IF NOT EXISTS tenant_id TEXT DEFAULT 'default';
UPDATE usage_events SET tenant_id = 'default' WHERE tenant_id IS NULL OR tenant_id = '';
ALTER TABLE usage_events
  ALTER COLUMN tenant_id SET DEFAULT 'default',
  ALTER COLUMN tenant_id SET NOT NULL;

ALTER TABLE billed_entries
  ADD COLUMN IF NOT EXISTS tenant_id TEXT DEFAULT 'default';
UPDATE billed_entries SET tenant_id = 'default' WHERE tenant_id IS NULL OR tenant_id = '';
ALTER TABLE billed_entries
  ALTER COLUMN tenant_id SET DEFAULT 'default',
  ALTER COLUMN tenant_id SET NOT NULL;

ALTER TABLE replay_jobs
  ADD COLUMN IF NOT EXISTS tenant_id TEXT DEFAULT 'default';
UPDATE replay_jobs SET tenant_id = 'default' WHERE tenant_id IS NULL OR tenant_id = '';
ALTER TABLE replay_jobs
  ALTER COLUMN tenant_id SET DEFAULT 'default',
  ALTER COLUMN tenant_id SET NOT NULL;

ALTER TABLE api_keys
  ADD COLUMN IF NOT EXISTS tenant_id TEXT DEFAULT 'default';
UPDATE api_keys SET tenant_id = 'default' WHERE tenant_id IS NULL OR tenant_id = '';
ALTER TABLE api_keys
  ALTER COLUMN tenant_id SET DEFAULT 'default',
  ALTER COLUMN tenant_id SET NOT NULL;

ALTER TABLE meters
  DROP CONSTRAINT IF EXISTS meters_meter_key_key;
CREATE UNIQUE INDEX IF NOT EXISTS uq_meters_tenant_meter_key
  ON meters (tenant_id, meter_key);

ALTER TABLE replay_jobs
  DROP CONSTRAINT IF EXISTS replay_jobs_idempotency_key_key;
CREATE UNIQUE INDEX IF NOT EXISTS uq_replay_jobs_tenant_idempotency
  ON replay_jobs (tenant_id, idempotency_key);

CREATE INDEX IF NOT EXISTS idx_rating_rule_versions_tenant_created
  ON rating_rule_versions (tenant_id, created_at);
CREATE INDEX IF NOT EXISTS idx_meters_tenant_created
  ON meters (tenant_id, created_at);
CREATE INDEX IF NOT EXISTS idx_usage_events_tenant_lookup
  ON usage_events (tenant_id, customer_id, meter_id, occurred_at);
CREATE INDEX IF NOT EXISTS idx_billed_entries_tenant_lookup
  ON billed_entries (tenant_id, customer_id, meter_id, occurred_at);
CREATE INDEX IF NOT EXISTS idx_replay_jobs_tenant_status_created
  ON replay_jobs (tenant_id, status, created_at);
CREATE INDEX IF NOT EXISTS idx_api_keys_tenant_prefix
  ON api_keys (tenant_id, key_prefix)
  WHERE revoked_at IS NULL;

