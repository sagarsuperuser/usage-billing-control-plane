CREATE TABLE IF NOT EXISTS plans (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL REFERENCES tenants(id),
  plan_code TEXT NOT NULL,
  name TEXT NOT NULL,
  description TEXT,
  currency TEXT NOT NULL,
  billing_interval TEXT NOT NULL CHECK (billing_interval IN ('monthly', 'yearly')),
  status TEXT NOT NULL CHECK (status IN ('draft', 'active', 'archived')),
  base_amount_cents BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, plan_code)
);

CREATE TABLE IF NOT EXISTS plan_metrics (
  tenant_id TEXT NOT NULL REFERENCES tenants(id),
  plan_id TEXT NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
  meter_id TEXT NOT NULL REFERENCES meters(id),
  position INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (plan_id, meter_id)
);

CREATE INDEX IF NOT EXISTS idx_plans_tenant_created_at
  ON plans (tenant_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_plan_metrics_tenant_plan_position
  ON plan_metrics (tenant_id, plan_id, position ASC, meter_id ASC);

ALTER TABLE plans ENABLE ROW LEVEL SECURITY;
ALTER TABLE plan_metrics ENABLE ROW LEVEL SECURITY;

ALTER TABLE plans FORCE ROW LEVEL SECURITY;
ALTER TABLE plan_metrics FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS p_plans_tenant ON plans;
CREATE POLICY p_plans_tenant ON plans
  USING (current_setting('app.tenant_id', true) IS NULL OR tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (current_setting('app.tenant_id', true) IS NULL OR tenant_id = current_setting('app.tenant_id', true));

DROP POLICY IF EXISTS p_plan_metrics_tenant ON plan_metrics;
CREATE POLICY p_plan_metrics_tenant ON plan_metrics
  USING (current_setting('app.tenant_id', true) IS NULL OR tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (current_setting('app.tenant_id', true) IS NULL OR tenant_id = current_setting('app.tenant_id', true));
