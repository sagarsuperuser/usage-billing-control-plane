DROP INDEX IF EXISTS service_accounts_tenant_status_created_idx;

ALTER TABLE service_accounts
    DROP CONSTRAINT IF EXISTS service_accounts_status_check;

ALTER TABLE service_accounts
    DROP COLUMN IF EXISTS disabled_at,
    DROP COLUMN IF EXISTS status;
