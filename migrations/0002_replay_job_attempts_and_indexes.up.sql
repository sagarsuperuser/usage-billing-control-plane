ALTER TABLE replay_jobs
  ADD COLUMN IF NOT EXISTS attempt_count INTEGER NOT NULL DEFAULT 0;

ALTER TABLE replay_jobs
  ADD COLUMN IF NOT EXISTS last_attempt_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_replay_jobs_status_started_at
  ON replay_jobs (status, started_at);

CREATE INDEX IF NOT EXISTS idx_usage_events_meter_occurred_at
  ON usage_events (meter_id, occurred_at);

CREATE INDEX IF NOT EXISTS idx_billed_entries_meter_occurred_at
  ON billed_entries (meter_id, occurred_at);
