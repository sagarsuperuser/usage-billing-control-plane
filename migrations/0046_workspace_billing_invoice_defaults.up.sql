ALTER TABLE workspace_billing_settings
  ADD COLUMN IF NOT EXISTS document_locale TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS invoice_grace_period_days INTEGER,
  ADD COLUMN IF NOT EXISTS document_numbering TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS document_number_prefix TEXT NOT NULL DEFAULT '';
