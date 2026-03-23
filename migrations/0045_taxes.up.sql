CREATE TABLE IF NOT EXISTS taxes (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  tax_code TEXT NOT NULL,
  name TEXT NOT NULL,
  description TEXT,
  status TEXT NOT NULL,
  rate NUMERIC(9,4) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT taxes_tenant_code_unique UNIQUE (tenant_id, tax_code),
  CONSTRAINT taxes_rate_non_negative CHECK (rate >= 0)
);

ALTER TABLE workspace_billing_settings
  ADD COLUMN IF NOT EXISTS tax_codes TEXT[] NOT NULL DEFAULT '{}';

ALTER TABLE customer_billing_profiles
  ADD COLUMN IF NOT EXISTS tax_codes TEXT[] NOT NULL DEFAULT '{}';
