ALTER TABLE customer_payment_setup
  ADD COLUMN IF NOT EXISTS last_request_status TEXT NOT NULL DEFAULT 'not_requested',
  ADD COLUMN IF NOT EXISTS last_request_kind TEXT,
  ADD COLUMN IF NOT EXISTS last_request_to_email TEXT,
  ADD COLUMN IF NOT EXISTS last_request_sent_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS last_request_error TEXT;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'chk_customer_payment_setup_last_request_status_allowed'
  ) THEN
    ALTER TABLE customer_payment_setup
      ADD CONSTRAINT chk_customer_payment_setup_last_request_status_allowed
      CHECK (last_request_status IN ('not_requested', 'sent', 'failed'));
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'chk_customer_payment_setup_last_request_kind_allowed'
  ) THEN
    ALTER TABLE customer_payment_setup
      ADD CONSTRAINT chk_customer_payment_setup_last_request_kind_allowed
      CHECK (last_request_kind IS NULL OR last_request_kind IN ('requested', 'resent'));
  END IF;
END $$;

ALTER TABLE tenant_audit_events
  DROP CONSTRAINT IF EXISTS chk_tenant_audit_events_action_allowed;

ALTER TABLE tenant_audit_events
  ADD CONSTRAINT chk_tenant_audit_events_action_allowed
  CHECK (action IN ('created', 'status_changed', 'updated', 'payment_setup_requested', 'payment_setup_resent'));
