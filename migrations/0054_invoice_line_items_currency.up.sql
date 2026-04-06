-- Add currency to invoice_line_items so each line item is self-describing.
-- Inherits from parent invoice at write time. Matches Stripe's pattern
-- where every monetary object carries its own currency.

ALTER TABLE invoice_line_items
  ADD COLUMN IF NOT EXISTS currency TEXT NOT NULL DEFAULT 'USD';
