ALTER TABLE service_accounts
    ADD COLUMN status TEXT NOT NULL DEFAULT 'active',
    ADD COLUMN disabled_at TIMESTAMPTZ NULL;

ALTER TABLE service_accounts
    ADD CONSTRAINT service_accounts_status_check CHECK (status IN ('active', 'disabled'));

CREATE INDEX service_accounts_tenant_status_created_idx
    ON service_accounts (tenant_id, status, created_at DESC, id DESC);
