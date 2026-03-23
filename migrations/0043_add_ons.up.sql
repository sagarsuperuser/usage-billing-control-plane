CREATE TABLE IF NOT EXISTS add_ons (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  add_on_code TEXT NOT NULL,
  name TEXT NOT NULL,
  description TEXT,
  currency TEXT NOT NULL,
  billing_interval TEXT NOT NULL,
  status TEXT NOT NULL,
  amount_cents BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT add_ons_tenant_code_unique UNIQUE (tenant_id, add_on_code)
);

CREATE TABLE IF NOT EXISTS plan_add_ons (
  tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  plan_id TEXT NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
  add_on_id TEXT NOT NULL REFERENCES add_ons(id) ON DELETE CASCADE,
  position INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (tenant_id, plan_id, add_on_id)
);
