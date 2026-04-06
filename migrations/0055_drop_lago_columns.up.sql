-- Remove legacy Lago integration columns from tenants.
-- Lago has been fully replaced by the native billing engine.

DROP INDEX IF EXISTS idx_tenants_lago_organization_id;
DROP INDEX IF EXISTS idx_tenants_billing_provider_code;

ALTER TABLE tenants
  DROP COLUMN IF EXISTS lago_organization_id,
  DROP COLUMN IF EXISTS lago_billing_provider_code,
  DROP COLUMN IF EXISTS lago_api_key;
