CREATE TABLE IF NOT EXISTS user_federated_identities (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider_key TEXT NOT NULL CHECK (provider_key <> ''),
    provider_type TEXT NOT NULL CHECK (provider_type IN ('oidc')),
    subject TEXT NOT NULL CHECK (subject <> ''),
    email TEXT NOT NULL DEFAULT '',
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider_key, subject)
);

CREATE INDEX IF NOT EXISTS idx_user_federated_identities_user_id
    ON user_federated_identities (user_id);

CREATE INDEX IF NOT EXISTS idx_user_federated_identities_provider_key
    ON user_federated_identities (provider_key);
