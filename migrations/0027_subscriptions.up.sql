CREATE TABLE IF NOT EXISTS subscriptions (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL REFERENCES tenants(id),
  subscription_code TEXT NOT NULL,
  display_name TEXT NOT NULL,
  customer_id TEXT NOT NULL REFERENCES customers(id),
  plan_id TEXT NOT NULL REFERENCES plans(id),
  status TEXT NOT NULL CHECK (status IN ('draft', 'pending_payment_setup', 'active', 'action_required', 'archived')),
  billing_time TEXT NOT NULL DEFAULT 'calendar' CHECK (billing_time IN ('calendar', 'anniversary')),
  payment_setup_requested_at TIMESTAMPTZ,
  activated_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, subscription_code)
);

CREATE INDEX IF NOT EXISTS idx_subscriptions_tenant_created_at
  ON subscriptions (tenant_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_subscriptions_tenant_customer
  ON subscriptions (tenant_id, customer_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_subscriptions_tenant_plan
  ON subscriptions (tenant_id, plan_id, created_at DESC);

ALTER TABLE subscriptions ENABLE ROW LEVEL SECURITY;
ALTER TABLE subscriptions FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS p_subscriptions_tenant ON subscriptions;
CREATE POLICY p_subscriptions_tenant ON subscriptions
  USING (current_setting('app.tenant_id', true) IS NULL OR tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (current_setting('app.tenant_id', true) IS NULL OR tenant_id = current_setting('app.tenant_id', true));
