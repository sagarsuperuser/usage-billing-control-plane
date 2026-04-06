-- Enable Row Level Security on 7 tenant-scoped tables that were missing RLS.
-- Uses the modern bypass pattern consistent with migration 0049.
--
-- Skipped: users, user_password_credentials (platform-scoped, no tenant_id).

-- add_ons
ALTER TABLE add_ons ENABLE ROW LEVEL SECURITY;
ALTER TABLE add_ons FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS p_add_ons_tenant ON add_ons;
CREATE POLICY p_add_ons_tenant ON add_ons
  USING (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
         OR tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
              OR tenant_id = current_setting('app.tenant_id', true));

-- coupons
ALTER TABLE coupons ENABLE ROW LEVEL SECURITY;
ALTER TABLE coupons FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS p_coupons_tenant ON coupons;
CREATE POLICY p_coupons_tenant ON coupons
  USING (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
         OR tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
              OR tenant_id = current_setting('app.tenant_id', true));

-- taxes
ALTER TABLE taxes ENABLE ROW LEVEL SECURITY;
ALTER TABLE taxes FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS p_taxes_tenant ON taxes;
CREATE POLICY p_taxes_tenant ON taxes
  USING (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
         OR tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
              OR tenant_id = current_setting('app.tenant_id', true));

-- plan_add_ons
ALTER TABLE plan_add_ons ENABLE ROW LEVEL SECURITY;
ALTER TABLE plan_add_ons FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS p_plan_add_ons_tenant ON plan_add_ons;
CREATE POLICY p_plan_add_ons_tenant ON plan_add_ons
  USING (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
         OR tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
              OR tenant_id = current_setting('app.tenant_id', true));

-- plan_coupons
ALTER TABLE plan_coupons ENABLE ROW LEVEL SECURITY;
ALTER TABLE plan_coupons FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS p_plan_coupons_tenant ON plan_coupons;
CREATE POLICY p_plan_coupons_tenant ON plan_coupons
  USING (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
         OR tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
              OR tenant_id = current_setting('app.tenant_id', true));

-- service_accounts
ALTER TABLE service_accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE service_accounts FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS p_service_accounts_tenant ON service_accounts;
CREATE POLICY p_service_accounts_tenant ON service_accounts
  USING (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
         OR tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
              OR tenant_id = current_setting('app.tenant_id', true));

-- user_tenant_memberships
ALTER TABLE user_tenant_memberships ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_tenant_memberships FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS p_user_tenant_memberships_tenant ON user_tenant_memberships;
CREATE POLICY p_user_tenant_memberships_tenant ON user_tenant_memberships
  USING (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
         OR tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (COALESCE(current_setting('app.bypass_rls', true), '') = 'on'
              OR tenant_id = current_setting('app.tenant_id', true));
