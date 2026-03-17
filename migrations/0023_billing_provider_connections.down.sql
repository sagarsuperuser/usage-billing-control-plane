DROP INDEX IF EXISTS idx_tenants_billing_provider_connection_id;
ALTER TABLE tenants
  DROP COLUMN IF EXISTS billing_provider_connection_id;

DROP POLICY IF EXISTS p_billing_provider_connections_bypass ON billing_provider_connections;
ALTER TABLE billing_provider_connections DISABLE ROW LEVEL SECURITY;

DROP INDEX IF EXISTS idx_billing_provider_connections_owner_tenant_id;
DROP INDEX IF EXISTS idx_billing_provider_connections_provider_environment;
DROP INDEX IF EXISTS idx_billing_provider_connections_status_created_at;
DROP TABLE IF EXISTS billing_provider_connections;
