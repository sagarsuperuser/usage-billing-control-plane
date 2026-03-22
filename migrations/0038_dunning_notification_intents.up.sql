CREATE TABLE IF NOT EXISTS dunning_notification_intents (
  id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES invoice_dunning_runs (id) ON DELETE CASCADE,
  tenant_id TEXT NOT NULL REFERENCES tenants (id),
  invoice_id TEXT NOT NULL,
  customer_external_id TEXT,
  intent_type TEXT NOT NULL,
  action_type TEXT,
  status TEXT NOT NULL DEFAULT 'queued',
  delivery_backend TEXT,
  recipient_email TEXT,
  payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  last_error TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  dispatched_at TIMESTAMPTZ,
  CONSTRAINT chk_dunning_notification_intents_intent_type
    CHECK (intent_type IN ('dunning.payment_failed', 'dunning.payment_method_required', 'dunning.retry_scheduled', 'dunning.final_attempt', 'dunning.escalated')),
  CONSTRAINT chk_dunning_notification_intents_action_type
    CHECK (action_type IS NULL OR action_type IN ('retry_payment', 'collect_payment_reminder')),
  CONSTRAINT chk_dunning_notification_intents_status
    CHECK (status IN ('queued', 'dispatched', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_dunning_notification_intents_tenant_run_created_at
  ON dunning_notification_intents (tenant_id, run_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_dunning_notification_intents_tenant_status_created_at
  ON dunning_notification_intents (tenant_id, status, created_at DESC);

ALTER TABLE dunning_notification_intents ENABLE ROW LEVEL SECURITY;
ALTER TABLE dunning_notification_intents FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS p_dunning_notification_intents_tenant ON dunning_notification_intents;
CREATE POLICY p_dunning_notification_intents_tenant ON dunning_notification_intents
  USING (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
         OR tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
              OR tenant_id = current_setting('app.tenant_id', true));
