CREATE TABLE workspace_invitations (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    role TEXT NOT NULL,
    status TEXT NOT NULL,
    token_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    accepted_by_user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    invited_by_user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    invited_by_platform_user BOOLEAN NOT NULL DEFAULT false,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT workspace_invitations_role_check CHECK (role IN ('reader', 'writer', 'admin')),
    CONSTRAINT workspace_invitations_status_check CHECK (status IN ('pending', 'accepted', 'expired', 'revoked'))
);

CREATE INDEX workspace_invitations_workspace_status_idx
    ON workspace_invitations (workspace_id, status);

CREATE INDEX workspace_invitations_email_status_idx
    ON workspace_invitations (email, status);

CREATE INDEX workspace_invitations_token_hash_idx
    ON workspace_invitations (token_hash);

CREATE UNIQUE INDEX workspace_invitations_workspace_email_pending_idx
    ON workspace_invitations (workspace_id, lower(email))
    WHERE status = 'pending';
