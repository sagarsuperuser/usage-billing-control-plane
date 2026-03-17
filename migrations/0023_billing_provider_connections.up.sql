CREATE TABLE IF NOT EXISTS billing_provider_connections (
  id TEXT PRIMARY KEY,
  provider_type TEXT NOT NULL,
  environment TEXT NOT NULL,
  display_name TEXT NOT NULL,
  scope TEXT NOT NULL,
  owner_tenant_id TEXT REFERENCES tenants (id),
  status TEXT NOT NULL DEFAULT 'pending',
  lago_organization_id TEXT,
  lago_provider_code TEXT,
  secret_ref TEXT,
  last_synced_at TIMESTAMPTZ,
  last_sync_error TEXT,
  connected_at TIMESTAMPTZ,
  disabled_at TIMESTAMPTZ,
  created_by_type TEXT NOT NULL,
  created_by_id TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_billing_provider_connections_type CHECK (provider_type IN ('stripe')),
  CONSTRAINT chk_billing_provider_connections_environment CHECK (environment IN ('test', 'live')),
  CONSTRAINT chk_billing_provider_connections_scope CHECK (scope IN ('platform', 'tenant')),
  CONSTRAINT chk_billing_provider_connections_status CHECK (status IN ('pending', 'connected', 'sync_error', 'disabled'))
);

CREATE INDEX IF NOT EXISTS idx_billing_provider_connections_status_created_at
  ON billing_provider_connections (status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_billing_provider_connections_provider_environment
  ON billing_provider_connections (provider_type, environment, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_billing_provider_connections_owner_tenant_id
  ON billing_provider_connections (owner_tenant_id)
  WHERE owner_tenant_id IS NOT NULL;

ALTER TABLE billing_provider_connections ENABLE ROW LEVEL SECURITY;
ALTER TABLE billing_provider_connections FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS p_billing_provider_connections_bypass ON billing_provider_connections;
CREATE POLICY p_billing_provider_connections_bypass ON billing_provider_connections
  USING (COALESCE(current_setting('app.bypass_rls', true), '') = 'on')
  WITH CHECK (COALESCE(current_setting('app.bypass_rls', true), '') = 'on');

ALTER TABLE tenants
  ADD COLUMN IF NOT EXISTS billing_provider_connection_id TEXT REFERENCES billing_provider_connections (id);

CREATE INDEX IF NOT EXISTS idx_tenants_billing_provider_connection_id
  ON tenants (billing_provider_connection_id)
  WHERE billing_provider_connection_id IS NOT NULL;
