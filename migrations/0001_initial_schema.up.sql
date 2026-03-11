CREATE TABLE IF NOT EXISTS rating_rule_versions (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  version INTEGER NOT NULL,
  mode TEXT NOT NULL,
  currency TEXT NOT NULL,
  flat_amount_cents BIGINT NOT NULL DEFAULT 0,
  graduated_tiers JSONB NOT NULL DEFAULT '[]'::jsonb,
  package_size BIGINT NOT NULL DEFAULT 0,
  package_amount_cents BIGINT NOT NULL DEFAULT 0,
  overage_unit_amount_cents BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS meters (
  id TEXT PRIMARY KEY,
  meter_key TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  unit TEXT NOT NULL,
  aggregation TEXT NOT NULL,
  rating_rule_version_id TEXT NOT NULL REFERENCES rating_rule_versions(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS usage_events (
  id TEXT PRIMARY KEY,
  customer_id TEXT NOT NULL,
  meter_id TEXT NOT NULL REFERENCES meters(id),
  quantity BIGINT NOT NULL,
  occurred_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_usage_events_lookup
  ON usage_events (customer_id, meter_id, occurred_at);

CREATE TABLE IF NOT EXISTS billed_entries (
  id TEXT PRIMARY KEY,
  customer_id TEXT NOT NULL,
  meter_id TEXT NOT NULL REFERENCES meters(id),
  amount_cents BIGINT NOT NULL,
  occurred_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_billed_entries_lookup
  ON billed_entries (customer_id, meter_id, occurred_at);

CREATE TABLE IF NOT EXISTS replay_jobs (
  id TEXT PRIMARY KEY,
  customer_id TEXT,
  meter_id TEXT,
  from_ts TIMESTAMPTZ,
  to_ts TIMESTAMPTZ,
  idempotency_key TEXT NOT NULL UNIQUE,
  status TEXT NOT NULL,
  processed_records BIGINT NOT NULL DEFAULT 0,
  error TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_replay_jobs_status_created
  ON replay_jobs (status, created_at);
