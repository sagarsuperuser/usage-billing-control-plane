CREATE TABLE IF NOT EXISTS customers (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL REFERENCES tenants (id),
  external_id TEXT NOT NULL,
  display_name TEXT NOT NULL,
  email TEXT,
  status TEXT NOT NULL DEFAULT 'active',
  lago_customer_id TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_customers_status CHECK (status IN ('active', 'suspended', 'archived')),
  CONSTRAINT uq_customers_id_tenant UNIQUE (id, tenant_id),
  CONSTRAINT uq_customers_tenant_external UNIQUE (tenant_id, external_id)
);

CREATE INDEX IF NOT EXISTS idx_customers_tenant_status_created_at
  ON customers (tenant_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_customers_tenant_external_id
  ON customers (tenant_id, external_id);

CREATE TABLE IF NOT EXISTS customer_billing_profiles (
  customer_id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  legal_name TEXT,
  email TEXT,
  phone TEXT,
  billing_address_line1 TEXT,
  billing_address_line2 TEXT,
  billing_city TEXT,
  billing_state TEXT,
  billing_postal_code TEXT,
  billing_country TEXT,
  currency TEXT,
  tax_identifier TEXT,
  provider_code TEXT,
  profile_status TEXT NOT NULL DEFAULT 'missing',
  last_synced_at TIMESTAMPTZ,
  last_sync_error TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_customer_billing_profiles_status CHECK (profile_status IN ('missing', 'incomplete', 'ready', 'sync_error')),
  CONSTRAINT fk_customer_billing_profiles_customer
    FOREIGN KEY (customer_id, tenant_id) REFERENCES customers (id, tenant_id)
);

CREATE INDEX IF NOT EXISTS idx_customer_billing_profiles_tenant_status
  ON customer_billing_profiles (tenant_id, profile_status, updated_at DESC);

CREATE TABLE IF NOT EXISTS customer_payment_setup (
  customer_id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  setup_status TEXT NOT NULL DEFAULT 'missing',
  default_payment_method_present BOOLEAN NOT NULL DEFAULT FALSE,
  payment_method_type TEXT,
  provider_customer_reference TEXT,
  provider_payment_method_reference TEXT,
  last_verified_at TIMESTAMPTZ,
  last_verification_result TEXT,
  last_verification_error TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_customer_payment_setup_status CHECK (setup_status IN ('missing', 'pending', 'ready', 'error')),
  CONSTRAINT fk_customer_payment_setup_customer
    FOREIGN KEY (customer_id, tenant_id) REFERENCES customers (id, tenant_id)
);

CREATE INDEX IF NOT EXISTS idx_customer_payment_setup_tenant_status
  ON customer_payment_setup (tenant_id, setup_status, updated_at DESC);

ALTER TABLE customers ENABLE ROW LEVEL SECURITY;
ALTER TABLE customers FORCE ROW LEVEL SECURITY;
ALTER TABLE customer_billing_profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE customer_billing_profiles FORCE ROW LEVEL SECURITY;
ALTER TABLE customer_payment_setup ENABLE ROW LEVEL SECURITY;
ALTER TABLE customer_payment_setup FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS p_customers_tenant ON customers;
CREATE POLICY p_customers_tenant ON customers
  USING (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
         OR tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
              OR tenant_id = current_setting('app.tenant_id', true));

DROP POLICY IF EXISTS p_customer_billing_profiles_tenant ON customer_billing_profiles;
CREATE POLICY p_customer_billing_profiles_tenant ON customer_billing_profiles
  USING (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
         OR tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
              OR tenant_id = current_setting('app.tenant_id', true));

DROP POLICY IF EXISTS p_customer_payment_setup_tenant ON customer_payment_setup;
CREATE POLICY p_customer_payment_setup_tenant ON customer_payment_setup
  USING (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
         OR tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
              OR tenant_id = current_setting('app.tenant_id', true));
