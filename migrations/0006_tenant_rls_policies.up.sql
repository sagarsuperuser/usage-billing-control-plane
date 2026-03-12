ALTER TABLE rating_rule_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE meters ENABLE ROW LEVEL SECURITY;
ALTER TABLE usage_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE billed_entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE replay_jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE api_keys ENABLE ROW LEVEL SECURITY;

ALTER TABLE rating_rule_versions FORCE ROW LEVEL SECURITY;
ALTER TABLE meters FORCE ROW LEVEL SECURITY;
ALTER TABLE usage_events FORCE ROW LEVEL SECURITY;
ALTER TABLE billed_entries FORCE ROW LEVEL SECURITY;
ALTER TABLE replay_jobs FORCE ROW LEVEL SECURITY;
ALTER TABLE api_keys FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS p_rating_rule_versions_tenant ON rating_rule_versions;
CREATE POLICY p_rating_rule_versions_tenant ON rating_rule_versions
  USING (current_setting('app.tenant_id', true) IS NULL OR tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (current_setting('app.tenant_id', true) IS NULL OR tenant_id = current_setting('app.tenant_id', true));

DROP POLICY IF EXISTS p_meters_tenant ON meters;
CREATE POLICY p_meters_tenant ON meters
  USING (current_setting('app.tenant_id', true) IS NULL OR tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (current_setting('app.tenant_id', true) IS NULL OR tenant_id = current_setting('app.tenant_id', true));

DROP POLICY IF EXISTS p_usage_events_tenant ON usage_events;
CREATE POLICY p_usage_events_tenant ON usage_events
  USING (current_setting('app.tenant_id', true) IS NULL OR tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (current_setting('app.tenant_id', true) IS NULL OR tenant_id = current_setting('app.tenant_id', true));

DROP POLICY IF EXISTS p_billed_entries_tenant ON billed_entries;
CREATE POLICY p_billed_entries_tenant ON billed_entries
  USING (current_setting('app.tenant_id', true) IS NULL OR tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (current_setting('app.tenant_id', true) IS NULL OR tenant_id = current_setting('app.tenant_id', true));

DROP POLICY IF EXISTS p_replay_jobs_tenant ON replay_jobs;
CREATE POLICY p_replay_jobs_tenant ON replay_jobs
  USING (current_setting('app.tenant_id', true) IS NULL OR tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (current_setting('app.tenant_id', true) IS NULL OR tenant_id = current_setting('app.tenant_id', true));

DROP POLICY IF EXISTS p_api_keys_tenant ON api_keys;
CREATE POLICY p_api_keys_tenant ON api_keys
  USING (current_setting('app.tenant_id', true) IS NULL OR tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (current_setting('app.tenant_id', true) IS NULL OR tenant_id = current_setting('app.tenant_id', true));
