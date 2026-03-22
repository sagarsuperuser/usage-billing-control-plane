DROP POLICY IF EXISTS p_invoice_dunning_events_tenant ON invoice_dunning_events;
DROP POLICY IF EXISTS p_invoice_dunning_runs_tenant ON invoice_dunning_runs;
DROP POLICY IF EXISTS p_dunning_policies_tenant ON dunning_policies;

DROP TABLE IF EXISTS invoice_dunning_events;
DROP TABLE IF EXISTS invoice_dunning_runs;
DROP TABLE IF EXISTS dunning_policies;
