DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'chk_rating_rule_version_positive'
  ) THEN
    ALTER TABLE rating_rule_versions
      ADD CONSTRAINT chk_rating_rule_version_positive
      CHECK (version > 0);
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'chk_rating_rule_mode_allowed'
  ) THEN
    ALTER TABLE rating_rule_versions
      ADD CONSTRAINT chk_rating_rule_mode_allowed
      CHECK (mode IN ('flat', 'graduated', 'package'));
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'chk_meter_aggregation_allowed'
  ) THEN
    ALTER TABLE meters
      ADD CONSTRAINT chk_meter_aggregation_allowed
      CHECK (aggregation IN ('sum', 'count', 'max'));
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'chk_usage_event_quantity_non_negative'
  ) THEN
    ALTER TABLE usage_events
      ADD CONSTRAINT chk_usage_event_quantity_non_negative
      CHECK (quantity >= 0);
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'chk_billed_entry_amount_non_negative'
  ) THEN
    ALTER TABLE billed_entries
      ADD CONSTRAINT chk_billed_entry_amount_non_negative
      CHECK (amount_cents >= 0);
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'chk_replay_job_status_allowed'
  ) THEN
    ALTER TABLE replay_jobs
      ADD CONSTRAINT chk_replay_job_status_allowed
      CHECK (status IN ('queued', 'running', 'done', 'failed'));
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'chk_replay_job_processed_records_non_negative'
  ) THEN
    ALTER TABLE replay_jobs
      ADD CONSTRAINT chk_replay_job_processed_records_non_negative
      CHECK (processed_records >= 0);
  END IF;
END $$;
