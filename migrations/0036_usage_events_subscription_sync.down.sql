DROP INDEX IF EXISTS idx_usage_events_subscription_lookup;

ALTER TABLE usage_events
  DROP COLUMN IF EXISTS subscription_id;
