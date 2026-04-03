-- Stripe webhook events: append-only event log for payment lifecycle.
-- Replaces lago_webhook_events. All payment status views are projected from this log.

CREATE TABLE IF NOT EXISTS stripe_webhook_events (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  stripe_event_id TEXT NOT NULL,
  event_type TEXT NOT NULL,
  object_type TEXT NOT NULL,
  invoice_id TEXT,
  customer_external_id TEXT,
  payment_intent_id TEXT,
  payment_status TEXT,
  amount_cents BIGINT,
  currency TEXT,
  failure_message TEXT,
  payload JSONB NOT NULL,
  received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Idempotent ingestion: reject duplicate Stripe events.
CREATE UNIQUE INDEX IF NOT EXISTS uq_stripe_webhook_events_stripe_id
  ON stripe_webhook_events (stripe_event_id);

CREATE INDEX IF NOT EXISTS idx_stripe_webhook_events_tenant_invoice
  ON stripe_webhook_events (tenant_id, invoice_id, received_at DESC)
  WHERE invoice_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_stripe_webhook_events_tenant_type
  ON stripe_webhook_events (tenant_id, event_type, received_at DESC);

-- Row Level Security
ALTER TABLE stripe_webhook_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE stripe_webhook_events FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS p_stripe_webhook_events_tenant ON stripe_webhook_events;
CREATE POLICY p_stripe_webhook_events_tenant ON stripe_webhook_events
  USING (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
         OR tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
              OR tenant_id = current_setting('app.tenant_id', true));
