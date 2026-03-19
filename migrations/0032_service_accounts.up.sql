CREATE TABLE service_accounts (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    role TEXT NOT NULL,
    purpose TEXT NOT NULL DEFAULT '',
    environment TEXT NOT NULL DEFAULT '',
    created_by_user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    created_by_platform_user BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT service_accounts_role_check CHECK (role IN ('reader', 'writer', 'admin'))
);

CREATE INDEX service_accounts_tenant_created_idx
    ON service_accounts (tenant_id, created_at DESC, id DESC);

CREATE UNIQUE INDEX service_accounts_tenant_name_idx
    ON service_accounts (tenant_id, lower(name));
