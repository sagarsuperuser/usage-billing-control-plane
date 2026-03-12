ALTER TABLE billed_entries
  DROP CONSTRAINT IF EXISTS chk_billed_entry_amount_non_negative;
