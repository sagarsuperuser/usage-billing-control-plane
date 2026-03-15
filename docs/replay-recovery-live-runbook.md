# Replay Recovery Live Runbook

Use this runbook to create a fresh live replay mismatch in staging, verify the replay adjustment closed it, and then inspect the same fixture from the browser UI.

## What This Exercises

The replay smoke touches only alpha-owned replay/rating tables:
- `rating_rule_versions`
- `meters`
- `usage_events`
- `billed_entries`
- `replay_jobs`

The fixture intentionally creates this state:
- one usage event computes to `100` cents
- one API billed entry records only `80` cents
- one replay job closes the `20` cent delta with a `replay_adjustment` billed entry

## 1. Create and Verify a Fresh Replay Fixture

```bash
ALPHA_API_BASE_URL='https://api-staging.sagarwaidande.org' \
ALPHA_WRITER_API_KEY='replace_me_writer_key' \
ALPHA_READER_API_KEY='replace_me_reader_key' \
OUTPUT_FILE='/tmp/replay-smoke.json' \
make verify-replay-smoke-staging
```

What the script does:
1. creates a unique flat pricing rule for the run
2. creates a unique meter linked to that rule
3. creates one usage event for a unique customer
4. creates one intentionally under-billed API entry
5. verifies reconciliation reports a `20` cent mismatch before replay
6. creates a replay job and polls until it finishes
7. verifies replay diagnostics and reconciliation after replay
8. verifies exactly one `replay_adjustment` billed entry exists for the replay job

Expected pass conditions:
- pre-replay `mismatch_row_count = 1`
- pre-replay `total_delta_cents = 20`
- replay job finishes `status = done`
- replay diagnostics show `usage_events_count = 1`
- replay diagnostics show `billed_entries_count = 2`
- replay diagnostics show `billed_amount_cents = 100`
- post-replay `mismatch_row_count = 0`
- post-replay `total_delta_cents = 0`
- one replay adjustment row exists with `amount_cents = 20`

## 2. Feed the Same Fixture into the Browser Smoke

Extract the browser env from the JSON output:

```bash
export PLAYWRIGHT_LIVE_BASE_URL='https://staging.sagarwaidande.org'
export PLAYWRIGHT_LIVE_API_BASE_URL='https://api-staging.sagarwaidande.org'
export PLAYWRIGHT_LIVE_WRITER_API_KEY='replace_me_writer_key'
export PLAYWRIGHT_LIVE_READER_API_KEY='replace_me_reader_key'
export PLAYWRIGHT_LIVE_REPLAY_JOB_ID="$(jq -r '.live_browser_smoke.playwright_live_replay_job_id' /tmp/replay-smoke.json)"
export PLAYWRIGHT_LIVE_REPLAY_CUSTOMER_ID="$(jq -r '.live_browser_smoke.playwright_live_replay_customer_id' /tmp/replay-smoke.json)"
export PLAYWRIGHT_LIVE_REPLAY_METER_ID="$(jq -r '.live_browser_smoke.playwright_live_replay_meter_id' /tmp/replay-smoke.json)"

make web-e2e-live
```

What the replay browser smoke verifies:
- reader session can filter down to the live replay fixture and open diagnostics
- diagnostics drawer shows the known replay job and artifact links
- writer session can queue a fresh replay job from the real staging UI using the same customer and meter

## 3. Read the Result Like a Developer

The replay path is append-only.
It does not mutate the original billed row.

Instead it does this:
1. reads `usage_events` for the replay scope
2. reads `billed_entries` for the same scope
3. computes expected amount from `meters -> rating_rule_versions`
4. calculates delta
5. appends a new `billed_entries` row with:
   - `source = replay_adjustment`
   - `replay_job_id = <job id>`
6. marks the `replay_jobs` row `done`

That is why the replay smoke proves both correction behavior and auditability.

## 4. About a Deliberate Failed Replay Fixture

This runbook does not automate a synthetic failed replay job.

Reason:
- the current public replay API validates `meter_id` up front
- the happy-path processor only fails on real backend/data problems
- there is no clean, deterministic public API action that forces a replay job into `failed` without introducing artificial DB or worker corruption

So the current recommendation is:
- automate the successful live replay smoke
- treat failed replay retry as an operator drill using a naturally failed staging job when one exists
- if failure injection becomes a real requirement, add an explicit non-production fault-injection path instead of relying on hidden breakage
## 5. Latest Proven Staging Fixture

Validated on `2026-03-15`:
- replay job: `rpl_432a72de0e30cac9`
- customer: `cust_replay_smoke_20260315062139-27785`
- meter: `mtr_e03fb302d1808662`
- replay adjustment billed entry: `bil_6fe4f70a0e246513`
- pre-replay delta: `20` cents
- post-replay delta: `0` cents

Browser replay smoke also passed against the live staging UI after deploying image tag `staging-20260315-replay-ui`.

