ALTER TABLE tenants
  ADD COLUMN IF NOT EXISTS lago_organization_id TEXT,
  ADD COLUMN IF NOT EXISTS lago_billing_provider_code TEXT;

WITH inferred AS (
  SELECT tenant_id, MIN(organization_id) AS organization_id, COUNT(DISTINCT organization_id) AS org_count
  FROM (
    SELECT tenant_id, organization_id FROM lago_webhook_events
    UNION ALL
    SELECT tenant_id, organization_id FROM invoice_payment_status_views
  ) src
  WHERE NULLIF(BTRIM(organization_id), '') IS NOT NULL
  GROUP BY tenant_id
)
UPDATE tenants t
SET lago_organization_id = inferred.organization_id,
    updated_at = NOW()
FROM inferred
WHERE t.id = inferred.tenant_id
  AND inferred.org_count = 1
  AND NULLIF(BTRIM(COALESCE(t.lago_organization_id, '')), '') IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_tenants_lago_organization_id
  ON tenants (lago_organization_id)
  WHERE NULLIF(BTRIM(lago_organization_id), '') IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_tenants_lago_billing_provider_code
  ON tenants (lago_billing_provider_code)
  WHERE NULLIF(BTRIM(lago_billing_provider_code), '') IS NOT NULL;
