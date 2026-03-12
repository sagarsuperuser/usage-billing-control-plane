CREATE TABLE IF NOT EXISTS invoice_payment_status_views (
  tenant_id TEXT NOT NULL,
  organization_id TEXT NOT NULL,
  invoice_id TEXT NOT NULL,
  customer_external_id TEXT,
  invoice_number TEXT,
  currency TEXT,
  invoice_status TEXT,
  payment_status TEXT,
  payment_overdue BOOLEAN,
  total_amount_cents BIGINT,
  total_due_amount_cents BIGINT,
  total_paid_amount_cents BIGINT,
  last_payment_error TEXT,
  last_event_type TEXT NOT NULL,
  last_event_at TIMESTAMPTZ NOT NULL,
  last_webhook_key TEXT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (tenant_id, invoice_id)
);

CREATE INDEX IF NOT EXISTS idx_invoice_payment_status_views_tenant_payment
  ON invoice_payment_status_views (tenant_id, payment_status, payment_overdue, last_event_at DESC);

CREATE TABLE IF NOT EXISTS lago_webhook_events (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  organization_id TEXT NOT NULL,
  webhook_key TEXT NOT NULL,
  webhook_type TEXT NOT NULL,
  object_type TEXT NOT NULL,
  invoice_id TEXT,
  payment_request_id TEXT,
  dunning_campaign_code TEXT,
  customer_external_id TEXT,
  invoice_number TEXT,
  currency TEXT,
  invoice_status TEXT,
  payment_status TEXT,
  payment_overdue BOOLEAN,
  total_amount_cents BIGINT,
  total_due_amount_cents BIGINT,
  total_paid_amount_cents BIGINT,
  last_payment_error TEXT,
  payload JSONB NOT NULL,
  received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_lago_webhook_events_webhook_key
  ON lago_webhook_events (webhook_key);

CREATE INDEX IF NOT EXISTS idx_lago_webhook_events_tenant_invoice_received
  ON lago_webhook_events (tenant_id, invoice_id, received_at DESC);

ALTER TABLE invoice_payment_status_views ENABLE ROW LEVEL SECURITY;
ALTER TABLE lago_webhook_events ENABLE ROW LEVEL SECURITY;

ALTER TABLE invoice_payment_status_views FORCE ROW LEVEL SECURITY;
ALTER TABLE lago_webhook_events FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS p_invoice_payment_status_views_tenant ON invoice_payment_status_views;
CREATE POLICY p_invoice_payment_status_views_tenant ON invoice_payment_status_views
  USING (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
         OR tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
              OR tenant_id = current_setting('app.tenant_id', true));

DROP POLICY IF EXISTS p_lago_webhook_events_tenant ON lago_webhook_events;
CREATE POLICY p_lago_webhook_events_tenant ON lago_webhook_events
  USING (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
         OR tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
              OR tenant_id = current_setting('app.tenant_id', true));
