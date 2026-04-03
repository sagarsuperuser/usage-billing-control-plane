-- Add billing cycle tracking columns to subscriptions.
-- Previously Lago owned billing cycle timing; now we own it directly.

ALTER TABLE subscriptions
  ADD COLUMN IF NOT EXISTS current_billing_period_start TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS current_billing_period_end TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS next_billing_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_subscriptions_next_billing
  ON subscriptions (tenant_id, next_billing_at)
  WHERE status = 'active' AND next_billing_at IS NOT NULL;

-- Backfill active subscriptions with their next billing date.
-- Uses calendar-month alignment for simplicity; the billing cycle worker
-- will compute exact boundaries based on billing_time (calendar/anniversary).
UPDATE subscriptions SET
  current_billing_period_start = date_trunc('month', COALESCE(started_at, activated_at, created_at)),
  current_billing_period_end = date_trunc('month', COALESCE(started_at, activated_at, created_at)) + INTERVAL '1 month',
  next_billing_at = date_trunc('month', NOW()) + INTERVAL '1 month'
WHERE status = 'active' AND next_billing_at IS NULL;
