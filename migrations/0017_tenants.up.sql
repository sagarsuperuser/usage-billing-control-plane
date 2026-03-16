CREATE TABLE IF NOT EXISTS tenants (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'active',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_tenants_status
    CHECK (status IN ('active', 'suspended', 'deleted'))
);

INSERT INTO tenants (id, name, status, created_at, updated_at)
SELECT seeded.id, seeded.name, 'active', NOW(), NOW()
FROM (
  SELECT 'default'::TEXT AS id, 'Default Tenant'::TEXT AS name
  UNION
  SELECT DISTINCT tenant_id, tenant_id FROM rating_rule_versions
  UNION
  SELECT DISTINCT tenant_id, tenant_id FROM meters
  UNION
  SELECT DISTINCT tenant_id, tenant_id FROM usage_events
  UNION
  SELECT DISTINCT tenant_id, tenant_id FROM billed_entries
  UNION
  SELECT DISTINCT tenant_id, tenant_id FROM replay_jobs
  UNION
  SELECT DISTINCT tenant_id, tenant_id FROM api_keys
  UNION
  SELECT DISTINCT tenant_id, tenant_id FROM api_key_audit_events
  UNION
  SELECT DISTINCT tenant_id, tenant_id FROM api_key_audit_export_jobs
  UNION
  SELECT DISTINCT tenant_id, tenant_id FROM invoice_payment_status_views
  UNION
  SELECT DISTINCT tenant_id, tenant_id FROM lago_webhook_events
) AS seeded
WHERE NULLIF(BTRIM(seeded.id), '') IS NOT NULL
ON CONFLICT (id) DO NOTHING;

CREATE INDEX IF NOT EXISTS idx_tenants_status_created_at
  ON tenants (status, created_at DESC);

ALTER TABLE tenants ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation_tenants ON tenants;
CREATE POLICY tenant_isolation_tenants ON tenants
  USING (
    current_setting('app.bypass_rls', true) = 'on'
    OR id = current_setting('app.tenant_id', true)
  )
  WITH CHECK (
    current_setting('app.bypass_rls', true) = 'on'
    OR id = current_setting('app.tenant_id', true)
  );

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_rating_rule_versions_tenant') THEN
    ALTER TABLE rating_rule_versions
      ADD CONSTRAINT fk_rating_rule_versions_tenant
      FOREIGN KEY (tenant_id) REFERENCES tenants (id) NOT VALID;
  END IF;
END $$;
ALTER TABLE rating_rule_versions VALIDATE CONSTRAINT fk_rating_rule_versions_tenant;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_meters_tenant') THEN
    ALTER TABLE meters
      ADD CONSTRAINT fk_meters_tenant
      FOREIGN KEY (tenant_id) REFERENCES tenants (id) NOT VALID;
  END IF;
END $$;
ALTER TABLE meters VALIDATE CONSTRAINT fk_meters_tenant;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_usage_events_tenant') THEN
    ALTER TABLE usage_events
      ADD CONSTRAINT fk_usage_events_tenant
      FOREIGN KEY (tenant_id) REFERENCES tenants (id) NOT VALID;
  END IF;
END $$;
ALTER TABLE usage_events VALIDATE CONSTRAINT fk_usage_events_tenant;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_billed_entries_tenant') THEN
    ALTER TABLE billed_entries
      ADD CONSTRAINT fk_billed_entries_tenant
      FOREIGN KEY (tenant_id) REFERENCES tenants (id) NOT VALID;
  END IF;
END $$;
ALTER TABLE billed_entries VALIDATE CONSTRAINT fk_billed_entries_tenant;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_replay_jobs_tenant') THEN
    ALTER TABLE replay_jobs
      ADD CONSTRAINT fk_replay_jobs_tenant
      FOREIGN KEY (tenant_id) REFERENCES tenants (id) NOT VALID;
  END IF;
END $$;
ALTER TABLE replay_jobs VALIDATE CONSTRAINT fk_replay_jobs_tenant;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_api_keys_tenant') THEN
    ALTER TABLE api_keys
      ADD CONSTRAINT fk_api_keys_tenant
      FOREIGN KEY (tenant_id) REFERENCES tenants (id) NOT VALID;
  END IF;
END $$;
ALTER TABLE api_keys VALIDATE CONSTRAINT fk_api_keys_tenant;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_api_key_audit_events_tenant') THEN
    ALTER TABLE api_key_audit_events
      ADD CONSTRAINT fk_api_key_audit_events_tenant
      FOREIGN KEY (tenant_id) REFERENCES tenants (id) NOT VALID;
  END IF;
END $$;
ALTER TABLE api_key_audit_events VALIDATE CONSTRAINT fk_api_key_audit_events_tenant;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_api_key_audit_export_jobs_tenant') THEN
    ALTER TABLE api_key_audit_export_jobs
      ADD CONSTRAINT fk_api_key_audit_export_jobs_tenant
      FOREIGN KEY (tenant_id) REFERENCES tenants (id) NOT VALID;
  END IF;
END $$;
ALTER TABLE api_key_audit_export_jobs VALIDATE CONSTRAINT fk_api_key_audit_export_jobs_tenant;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_invoice_payment_status_views_tenant') THEN
    ALTER TABLE invoice_payment_status_views
      ADD CONSTRAINT fk_invoice_payment_status_views_tenant
      FOREIGN KEY (tenant_id) REFERENCES tenants (id) NOT VALID;
  END IF;
END $$;
ALTER TABLE invoice_payment_status_views VALIDATE CONSTRAINT fk_invoice_payment_status_views_tenant;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_lago_webhook_events_tenant') THEN
    ALTER TABLE lago_webhook_events
      ADD CONSTRAINT fk_lago_webhook_events_tenant
      FOREIGN KEY (tenant_id) REFERENCES tenants (id) NOT VALID;
  END IF;
END $$;
ALTER TABLE lago_webhook_events VALIDATE CONSTRAINT fk_lago_webhook_events_tenant;
