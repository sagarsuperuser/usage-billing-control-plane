ALTER TABLE subscriptions
  ADD COLUMN IF NOT EXISTS billing_time TEXT;

UPDATE subscriptions
SET billing_time = 'calendar'
WHERE billing_time IS NULL OR billing_time = '';

ALTER TABLE subscriptions
  ALTER COLUMN billing_time SET DEFAULT 'calendar';

ALTER TABLE subscriptions
  ALTER COLUMN billing_time SET NOT NULL;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'subscriptions_billing_time_check'
  ) THEN
    ALTER TABLE subscriptions
      ADD CONSTRAINT subscriptions_billing_time_check
      CHECK (billing_time IN ('calendar', 'anniversary'));
  END IF;
END $$;
