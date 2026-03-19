DELETE FROM tenant_audit_events
WHERE action IN ('payment_setup_requested', 'payment_setup_resent');

ALTER TABLE tenant_audit_events
  DROP CONSTRAINT IF EXISTS chk_tenant_audit_events_action_allowed;

ALTER TABLE tenant_audit_events
  ADD CONSTRAINT chk_tenant_audit_events_action_allowed
  CHECK (action IN ('created', 'status_changed', 'updated'));

ALTER TABLE customer_payment_setup
  DROP CONSTRAINT IF EXISTS chk_customer_payment_setup_last_request_kind_allowed;

ALTER TABLE customer_payment_setup
  DROP CONSTRAINT IF EXISTS chk_customer_payment_setup_last_request_status_allowed;

ALTER TABLE customer_payment_setup
  DROP COLUMN IF EXISTS last_request_error,
  DROP COLUMN IF EXISTS last_request_sent_at,
  DROP COLUMN IF EXISTS last_request_to_email,
  DROP COLUMN IF EXISTS last_request_kind,
  DROP COLUMN IF EXISTS last_request_status;
