ALTER TABLE billed_entries
  ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'api';

ALTER TABLE billed_entries
  ADD COLUMN IF NOT EXISTS replay_job_id TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS uq_replay_jobs_tenant_id_id
  ON replay_jobs (tenant_id, id);

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'chk_billed_entries_source_allowed'
  ) THEN
    ALTER TABLE billed_entries
      ADD CONSTRAINT chk_billed_entries_source_allowed
      CHECK (source IN ('api', 'replay_adjustment'));
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'chk_billed_entries_provenance'
  ) THEN
    ALTER TABLE billed_entries
      ADD CONSTRAINT chk_billed_entries_provenance
      CHECK (
        (source = 'api' AND replay_job_id IS NULL)
        OR (source = 'replay_adjustment' AND replay_job_id IS NOT NULL)
      );
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'fk_billed_entries_replay_job_tenant'
  ) THEN
    ALTER TABLE billed_entries
      ADD CONSTRAINT fk_billed_entries_replay_job_tenant
      FOREIGN KEY (tenant_id, replay_job_id)
      REFERENCES replay_jobs (tenant_id, id);
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_billed_entries_tenant_replay_job
  ON billed_entries (tenant_id, replay_job_id)
  WHERE replay_job_id IS NOT NULL;
