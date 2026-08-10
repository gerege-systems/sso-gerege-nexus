-- +goose Up
DELETE FROM oauth2_clients WHERE tenant_id IS NULL;
ALTER TABLE oauth2_clients ALTER COLUMN tenant_id SET NOT NULL;
CREATE INDEX IF NOT EXISTS idx_oauth2_clients_tenant ON oauth2_clients(tenant_id, created_at DESC);

CREATE TABLE IF NOT EXISTS oauth2_access_tokens (
    token_hash CHAR(64) PRIMARY KEY,
    client_id VARCHAR(128) NOT NULL REFERENCES oauth2_clients(client_id) ON DELETE CASCADE,
    subject VARCHAR(255) NOT NULL,
    scope TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_oauth2_tokens_expires ON oauth2_access_tokens(expires_at);

-- +goose Down
DROP TABLE IF EXISTS oauth2_access_tokens;
DROP INDEX IF EXISTS idx_oauth2_clients_tenant;
ALTER TABLE oauth2_clients ALTER COLUMN tenant_id DROP NOT NULL;
