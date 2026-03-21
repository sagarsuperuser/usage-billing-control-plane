ALTER TABLE usage_events
  ADD COLUMN IF NOT EXISTS subscription_id text;

CREATE INDEX IF NOT EXISTS idx_usage_events_subscription_lookup
  ON usage_events (tenant_id, subscription_id, occurred_at)
  WHERE subscription_id IS NOT NULL AND subscription_id <> '';
