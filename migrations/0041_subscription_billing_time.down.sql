ALTER TABLE subscriptions
  DROP CONSTRAINT IF EXISTS subscriptions_billing_time_check;

ALTER TABLE subscriptions
  DROP COLUMN IF EXISTS billing_time;
