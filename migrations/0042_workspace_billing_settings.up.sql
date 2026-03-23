CREATE TABLE IF NOT EXISTS workspace_billing_settings (
  workspace_id TEXT PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
  billing_entity_code TEXT,
  net_payment_term_days INTEGER,
  invoice_memo TEXT,
  invoice_footer TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT workspace_billing_settings_net_payment_term_days_check
    CHECK (net_payment_term_days IS NULL OR net_payment_term_days >= 0)
);
