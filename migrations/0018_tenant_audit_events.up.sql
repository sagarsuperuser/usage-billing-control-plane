CREATE TABLE IF NOT EXISTS tenant_audit_events (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  actor_api_key_id TEXT REFERENCES api_keys(id) ON DELETE SET NULL,
  action TEXT NOT NULL,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'chk_tenant_audit_events_action_allowed'
  ) THEN
    ALTER TABLE tenant_audit_events
      ADD CONSTRAINT chk_tenant_audit_events_action_allowed
      CHECK (action IN ('created', 'status_changed'));
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_tenant_audit_events_tenant_created
  ON tenant_audit_events (tenant_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_tenant_audit_events_actor_created
  ON tenant_audit_events (actor_api_key_id, created_at DESC, id DESC);

ALTER TABLE tenant_audit_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenant_audit_events FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS p_tenant_audit_events_tenant ON tenant_audit_events;
CREATE POLICY p_tenant_audit_events_tenant ON tenant_audit_events
  USING (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
         OR tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
              OR tenant_id = current_setting('app.tenant_id', true));
