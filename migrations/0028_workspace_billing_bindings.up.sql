CREATE TABLE workspace_billing_bindings (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    billing_provider_connection_id TEXT NOT NULL REFERENCES billing_provider_connections(id) ON DELETE RESTRICT,
    backend TEXT NOT NULL,
    backend_organization_id TEXT,
    backend_provider_code TEXT,
    isolation_mode TEXT NOT NULL,
    status TEXT NOT NULL,
    provisioning_error TEXT NOT NULL DEFAULT '',
    last_verified_at TIMESTAMPTZ,
    connected_at TIMESTAMPTZ,
    disabled_at TIMESTAMPTZ,
    created_by_type TEXT NOT NULL,
    created_by_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT workspace_billing_bindings_workspace_unique UNIQUE (workspace_id),
    CONSTRAINT workspace_billing_bindings_backend_check CHECK (backend IN ('lago')),
    CONSTRAINT workspace_billing_bindings_isolation_mode_check CHECK (isolation_mode IN ('shared', 'dedicated')),
    CONSTRAINT workspace_billing_bindings_status_check CHECK (status IN ('pending', 'provisioning', 'connected', 'verification_failed', 'disabled'))
);

CREATE INDEX workspace_billing_bindings_connection_idx
    ON workspace_billing_bindings (billing_provider_connection_id);

CREATE INDEX workspace_billing_bindings_status_idx
    ON workspace_billing_bindings (status);
