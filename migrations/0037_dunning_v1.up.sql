CREATE TABLE IF NOT EXISTS dunning_policies (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL REFERENCES tenants (id),
  name TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  retry_schedule TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
  max_retry_attempts INTEGER NOT NULL DEFAULT 3,
  collect_payment_reminder_schedule TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
  final_action TEXT NOT NULL DEFAULT 'manual_review',
  grace_period_days INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT uq_dunning_policies_tenant UNIQUE (tenant_id),
  CONSTRAINT chk_dunning_policies_final_action
    CHECK (final_action IN ('manual_review', 'pause', 'write_off_later')),
  CONSTRAINT chk_dunning_policies_max_retry_attempts
    CHECK (max_retry_attempts >= 0),
  CONSTRAINT chk_dunning_policies_grace_period_days
    CHECK (grace_period_days >= 0)
);

CREATE TABLE IF NOT EXISTS invoice_dunning_runs (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL REFERENCES tenants (id),
  invoice_id TEXT NOT NULL,
  customer_external_id TEXT,
  policy_id TEXT NOT NULL REFERENCES dunning_policies (id),
  state TEXT NOT NULL,
  reason TEXT,
  attempt_count INTEGER NOT NULL DEFAULT 0,
  last_attempt_at TIMESTAMPTZ,
  next_action_at TIMESTAMPTZ,
  next_action_type TEXT,
  paused BOOLEAN NOT NULL DEFAULT FALSE,
  resolved_at TIMESTAMPTZ,
  resolution TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_invoice_dunning_runs_state
    CHECK (state IN ('scheduled', 'retry_due', 'awaiting_payment_setup', 'awaiting_retry_result', 'resolved', 'paused', 'escalated', 'exhausted')),
  CONSTRAINT chk_invoice_dunning_runs_next_action_type
    CHECK (next_action_type IS NULL OR next_action_type IN ('retry_payment', 'collect_payment_reminder')),
  CONSTRAINT chk_invoice_dunning_runs_resolution
    CHECK (resolution IS NULL OR resolution IN ('payment_succeeded', 'invoice_not_collectible', 'operator_resolved', 'escalated')),
  CONSTRAINT chk_invoice_dunning_runs_attempt_count
    CHECK (attempt_count >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_invoice_dunning_runs_active_invoice
  ON invoice_dunning_runs (tenant_id, invoice_id)
  WHERE resolved_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_invoice_dunning_runs_tenant_state_next_action
  ON invoice_dunning_runs (tenant_id, state, paused, next_action_at);

CREATE INDEX IF NOT EXISTS idx_invoice_dunning_runs_tenant_customer_created_at
  ON invoice_dunning_runs (tenant_id, customer_external_id, created_at DESC);

CREATE TABLE IF NOT EXISTS invoice_dunning_events (
  id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES invoice_dunning_runs (id) ON DELETE CASCADE,
  tenant_id TEXT NOT NULL REFERENCES tenants (id),
  invoice_id TEXT NOT NULL,
  customer_external_id TEXT,
  event_type TEXT NOT NULL,
  state TEXT NOT NULL,
  action_type TEXT,
  reason TEXT,
  attempt_count INTEGER NOT NULL DEFAULT 0,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_invoice_dunning_events_event_type
    CHECK (event_type IN ('dunning_started', 'retry_scheduled', 'payment_setup_pending', 'payment_setup_ready', 'paused', 'resumed', 'escalated', 'resolved')),
  CONSTRAINT chk_invoice_dunning_events_state
    CHECK (state IN ('scheduled', 'retry_due', 'awaiting_payment_setup', 'awaiting_retry_result', 'resolved', 'paused', 'escalated', 'exhausted')),
  CONSTRAINT chk_invoice_dunning_events_action_type
    CHECK (action_type IS NULL OR action_type IN ('retry_payment', 'collect_payment_reminder')),
  CONSTRAINT chk_invoice_dunning_events_attempt_count
    CHECK (attempt_count >= 0)
);

CREATE INDEX IF NOT EXISTS idx_invoice_dunning_events_tenant_run_created_at
  ON invoice_dunning_events (tenant_id, run_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_invoice_dunning_events_tenant_invoice_created_at
  ON invoice_dunning_events (tenant_id, invoice_id, created_at DESC);

ALTER TABLE dunning_policies ENABLE ROW LEVEL SECURITY;
ALTER TABLE dunning_policies FORCE ROW LEVEL SECURITY;
ALTER TABLE invoice_dunning_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE invoice_dunning_runs FORCE ROW LEVEL SECURITY;
ALTER TABLE invoice_dunning_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE invoice_dunning_events FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS p_dunning_policies_tenant ON dunning_policies;
CREATE POLICY p_dunning_policies_tenant ON dunning_policies
  USING (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
         OR tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
              OR tenant_id = current_setting('app.tenant_id', true));

DROP POLICY IF EXISTS p_invoice_dunning_runs_tenant ON invoice_dunning_runs;
CREATE POLICY p_invoice_dunning_runs_tenant ON invoice_dunning_runs
  USING (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
         OR tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
              OR tenant_id = current_setting('app.tenant_id', true));

DROP POLICY IF EXISTS p_invoice_dunning_events_tenant ON invoice_dunning_events;
CREATE POLICY p_invoice_dunning_events_tenant ON invoice_dunning_events
  USING (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
         OR tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
              OR tenant_id = current_setting('app.tenant_id', true));
