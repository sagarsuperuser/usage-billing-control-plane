DROP INDEX IF EXISTS idx_subscriptions_next_billing;
ALTER TABLE subscriptions
  DROP COLUMN IF EXISTS current_billing_period_start,
  DROP COLUMN IF EXISTS current_billing_period_end,
  DROP COLUMN IF EXISTS next_billing_at;
