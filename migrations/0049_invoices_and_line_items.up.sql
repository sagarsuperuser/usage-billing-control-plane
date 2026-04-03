-- First-class invoice storage (replaces Lago-hosted invoices).
-- Each invoice is keyed by (tenant, subscription, billing_period_start) for idempotent generation.
-- State machine: draft → finalized → paid | voided

CREATE TABLE IF NOT EXISTS invoices (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL REFERENCES tenants (id),
  customer_id TEXT NOT NULL REFERENCES customers (id),
  subscription_id TEXT NOT NULL REFERENCES subscriptions (id),
  invoice_number TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'draft',
  payment_status TEXT NOT NULL DEFAULT 'pending',
  currency TEXT NOT NULL DEFAULT 'USD',
  subtotal_cents BIGINT NOT NULL DEFAULT 0,
  discount_cents BIGINT NOT NULL DEFAULT 0,
  tax_amount_cents BIGINT NOT NULL DEFAULT 0,
  total_amount_cents BIGINT NOT NULL DEFAULT 0,
  amount_due_cents BIGINT NOT NULL DEFAULT 0,
  amount_paid_cents BIGINT NOT NULL DEFAULT 0,
  billing_period_start TIMESTAMPTZ NOT NULL,
  billing_period_end TIMESTAMPTZ NOT NULL,
  issued_at TIMESTAMPTZ,
  due_at TIMESTAMPTZ,
  paid_at TIMESTAMPTZ,
  voided_at TIMESTAMPTZ,
  stripe_payment_intent_id TEXT,
  last_payment_error TEXT,
  payment_overdue BOOLEAN NOT NULL DEFAULT FALSE,
  pdf_object_key TEXT,
  net_payment_term_days INTEGER NOT NULL DEFAULT 0,
  memo TEXT,
  footer TEXT,
  metadata JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_invoices_status
    CHECK (status IN ('draft', 'finalized', 'paid', 'voided')),
  CONSTRAINT chk_invoices_payment_status
    CHECK (payment_status IN ('pending', 'processing', 'succeeded', 'failed')),
  CONSTRAINT chk_invoices_amounts_non_negative
    CHECK (total_amount_cents >= 0 AND amount_due_cents >= 0 AND amount_paid_cents >= 0)
);

-- Idempotent billing: one invoice per subscription per billing period.
CREATE UNIQUE INDEX IF NOT EXISTS uq_invoices_tenant_sub_period
  ON invoices (tenant_id, subscription_id, billing_period_start);

CREATE UNIQUE INDEX IF NOT EXISTS uq_invoices_tenant_number
  ON invoices (tenant_id, invoice_number);

CREATE INDEX IF NOT EXISTS idx_invoices_tenant_customer
  ON invoices (tenant_id, customer_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_invoices_tenant_status
  ON invoices (tenant_id, status, payment_status, payment_overdue, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_invoices_stripe_pi
  ON invoices (stripe_payment_intent_id) WHERE stripe_payment_intent_id IS NOT NULL;

-- Invoice line items: one row per charge (meter-based, base fee, add-on, discount, tax).
CREATE TABLE IF NOT EXISTS invoice_line_items (
  id TEXT PRIMARY KEY,
  invoice_id TEXT NOT NULL REFERENCES invoices (id) ON DELETE CASCADE,
  tenant_id TEXT NOT NULL,
  line_type TEXT NOT NULL,
  meter_id TEXT,
  add_on_id TEXT,
  coupon_id TEXT,
  tax_id TEXT,
  description TEXT NOT NULL,
  quantity BIGINT NOT NULL DEFAULT 0,
  unit_amount_cents BIGINT NOT NULL DEFAULT 0,
  amount_cents BIGINT NOT NULL DEFAULT 0,
  tax_rate NUMERIC(7, 4) NOT NULL DEFAULT 0,
  tax_amount_cents BIGINT NOT NULL DEFAULT 0,
  total_amount_cents BIGINT NOT NULL DEFAULT 0,
  pricing_mode TEXT,
  rating_rule_version_id TEXT,
  billing_period_start TIMESTAMPTZ,
  billing_period_end TIMESTAMPTZ,
  metadata JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_invoice_line_items_line_type
    CHECK (line_type IN ('base_fee', 'usage', 'add_on', 'discount', 'tax'))
);

CREATE INDEX IF NOT EXISTS idx_invoice_line_items_invoice
  ON invoice_line_items (invoice_id);

-- Row Level Security
ALTER TABLE invoices ENABLE ROW LEVEL SECURITY;
ALTER TABLE invoices FORCE ROW LEVEL SECURITY;
ALTER TABLE invoice_line_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE invoice_line_items FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS p_invoices_tenant ON invoices;
CREATE POLICY p_invoices_tenant ON invoices
  USING (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
         OR tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
              OR tenant_id = current_setting('app.tenant_id', true));

DROP POLICY IF EXISTS p_invoice_line_items_tenant ON invoice_line_items;
CREATE POLICY p_invoice_line_items_tenant ON invoice_line_items
  USING (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
         OR tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
              OR tenant_id = current_setting('app.tenant_id', true));
