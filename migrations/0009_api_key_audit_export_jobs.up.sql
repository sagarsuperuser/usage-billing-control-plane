CREATE TABLE IF NOT EXISTS api_key_audit_export_jobs (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  requested_by_api_key_id TEXT NOT NULL REFERENCES api_keys(id) ON DELETE RESTRICT,
  idempotency_key TEXT NOT NULL,
  status TEXT NOT NULL,
  filters JSONB NOT NULL DEFAULT '{}'::jsonb,
  object_key TEXT NOT NULL DEFAULT '',
  row_count BIGINT NOT NULL DEFAULT 0,
  error TEXT NOT NULL DEFAULT '',
  attempt_count INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  expires_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_api_key_audit_export_jobs_tenant_idempotency
  ON api_key_audit_export_jobs (tenant_id, idempotency_key);

CREATE INDEX IF NOT EXISTS idx_api_key_audit_export_jobs_tenant_status_created
  ON api_key_audit_export_jobs (tenant_id, status, created_at);

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'chk_api_key_audit_export_jobs_status_allowed'
  ) THEN
    ALTER TABLE api_key_audit_export_jobs
      ADD CONSTRAINT chk_api_key_audit_export_jobs_status_allowed
      CHECK (status IN ('queued', 'running', 'done', 'failed'));
  END IF;
END $$;

ALTER TABLE api_key_audit_export_jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE api_key_audit_export_jobs FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS p_api_key_audit_export_jobs_tenant ON api_key_audit_export_jobs;
CREATE POLICY p_api_key_audit_export_jobs_tenant ON api_key_audit_export_jobs
  USING (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
         OR tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
              OR tenant_id = current_setting('app.tenant_id', true));
