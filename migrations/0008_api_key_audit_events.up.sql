CREATE TABLE IF NOT EXISTS api_key_audit_events (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  api_key_id TEXT NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
  actor_api_key_id TEXT REFERENCES api_keys(id) ON DELETE SET NULL,
  action TEXT NOT NULL,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'chk_api_key_audit_events_action_allowed'
  ) THEN
    ALTER TABLE api_key_audit_events
      ADD CONSTRAINT chk_api_key_audit_events_action_allowed
      CHECK (action IN ('created', 'revoked', 'rotated'));
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_api_key_audit_events_tenant_created
  ON api_key_audit_events (tenant_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_api_key_audit_events_tenant_key_created
  ON api_key_audit_events (tenant_id, api_key_id, created_at DESC, id DESC);

ALTER TABLE api_key_audit_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE api_key_audit_events FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS p_api_key_audit_events_tenant ON api_key_audit_events;
CREATE POLICY p_api_key_audit_events_tenant ON api_key_audit_events
  USING (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
         OR tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
              OR tenant_id = current_setting('app.tenant_id', true));
