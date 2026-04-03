ALTER TABLE tenants ADD COLUMN IF NOT EXISTS lago_organization_id TEXT;
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS lago_billing_provider_code TEXT;
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS lago_api_key TEXT DEFAULT '';

ALTER TABLE billing_provider_connections ADD COLUMN IF NOT EXISTS lago_organization_id TEXT;
ALTER TABLE billing_provider_connections ADD COLUMN IF NOT EXISTS lago_provider_code TEXT;

ALTER TABLE customers ADD COLUMN IF NOT EXISTS lago_customer_id TEXT;
