-- Remove Lago-specific columns from core tables.
-- These columns are no longer used after the Lago removal (direct Stripe integration).

ALTER TABLE tenants DROP COLUMN IF EXISTS lago_organization_id;
ALTER TABLE tenants DROP COLUMN IF EXISTS lago_billing_provider_code;
ALTER TABLE tenants DROP COLUMN IF EXISTS lago_api_key;

ALTER TABLE billing_provider_connections DROP COLUMN IF EXISTS lago_organization_id;
ALTER TABLE billing_provider_connections DROP COLUMN IF EXISTS lago_provider_code;

ALTER TABLE customers DROP COLUMN IF EXISTS lago_customer_id;
