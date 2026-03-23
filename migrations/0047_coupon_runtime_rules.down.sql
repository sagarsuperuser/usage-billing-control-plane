ALTER TABLE coupons
  DROP COLUMN IF EXISTS expiration_at,
  DROP COLUMN IF EXISTS frequency_duration,
  DROP COLUMN IF EXISTS frequency;
