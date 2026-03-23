ALTER TABLE customer_billing_profiles
  DROP COLUMN IF EXISTS tax_codes;

ALTER TABLE workspace_billing_settings
  DROP COLUMN IF EXISTS tax_codes;

DROP TABLE IF EXISTS taxes;
