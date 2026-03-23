ALTER TABLE workspace_billing_settings
  DROP COLUMN IF EXISTS document_number_prefix,
  DROP COLUMN IF EXISTS document_numbering,
  DROP COLUMN IF EXISTS invoice_grace_period_days,
  DROP COLUMN IF EXISTS document_locale;
