-- +goose Up

-- Durable storage for the OAuth2 / OpenID Connect provider.
--
-- Numbered 00032. This file has been renumbered twice, both times because
-- upstream had already used the number while this fork was writing it: first
-- 00010 against 00010_esign_v2, then 00022 against 00022_integrations. goose
-- refuses to run at all with two files claiming one version — a hard error,
-- not a warning — and upstream's number is the one that cannot move, because
-- renumbering a migration other databases have applied makes it run again
-- everywhere.
--
-- Consequence for any database that applied this file under an earlier number:
-- that row is recorded, so goose treats upstream's migration of the same
-- number as already applied and skips it forever. Delete the rows this fork
-- wrote before the next migrate run —
--   DELETE FROM goose_db_version WHERE version_id IN (10, 22, 23);
-- — dropping only the ones that came from this fork's own files, and 00010
-- through 00033 then apply in order. Re-running this file costs nothing: every
-- statement below is written to be idempotent, and so is 00033.
--
-- Ordering note: upstream's 00025_oauth_persistence now runs before this file.
-- It sets oauth2_clients.tenant_id NOT NULL and creates an oauth2_access_tokens
-- table that this provider does not use — it keeps its tokens in oauth2_tokens
-- below, with the refresh family and revocation columns the flow needs. The two
-- do not collide; the unused table is upstream's to drop.
--
-- Everything below used to live in Go maps on the SSOProvider struct. Since
-- the rollout replaces the container on every push, an integrator who
-- registered a client in the developer portal lost it — client_id, secret and
-- all — at the next deploy. Clients also had no owner, so ListClients handed
-- every tenant every other tenant's registrations.

-- oauth2_clients already exists: 00004 created it for the developer portal and
-- no code has ever read or written it, because the provider kept its clients
-- in a Go map instead. It is therefore empty everywhere, and this migration
-- grows it into the real thing rather than creating it.
--
-- Note the deliberate absence of a "DELETE FROM oauth2_clients WHERE tenant_id
-- IS NULL" before the SET NOT NULL below. If some deployment did manage to
-- store an ownerless client, that ALTER should fail loudly and have a human
-- look, not quietly discard the row.
ALTER TABLE oauth2_clients
    -- NULL for public clients, which authenticate with PKCE instead (RFC 7636)
    -- because a secret shipped in a mobile binary or an SPA bundle is not a
    -- secret. Confidential clients keep a SHA-256 digest, never the secret.
    ALTER COLUMN client_secret_hash DROP NOT NULL,
    ALTER COLUMN tenant_id SET NOT NULL,
    -- 'confidential' (server-side, holds a secret) or 'public' (PKCE only).
    ADD COLUMN IF NOT EXISTS client_type VARCHAR(16) NOT NULL DEFAULT 'confidential',
    ADD COLUMN IF NOT EXISTS client_uri TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS logo_uri TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS disabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN IF NOT EXISTS secret_rotated_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_used_at TIMESTAMPTZ;

ALTER TABLE oauth2_clients
    DROP CONSTRAINT IF EXISTS oauth2_clients_type_check,
    ADD CONSTRAINT oauth2_clients_type_check
        CHECK (client_type IN ('confidential', 'public'));

-- A confidential client with no secret could be impersonated by anyone who
-- knows its client_id; a public client holding one is advertising a lie.
ALTER TABLE oauth2_clients
    DROP CONSTRAINT IF EXISTS oauth2_clients_secret_matches_type,
    ADD CONSTRAINT oauth2_clients_secret_matches_type
        CHECK ((client_type = 'confidential') = (client_secret_hash IS NOT NULL));

CREATE INDEX IF NOT EXISTS idx_oauth2_clients_tenant
    ON oauth2_clients(tenant_id, created_at DESC);

-- Authorization codes are single-use, short-lived, and bound to the PKCE
-- challenge that was presented at /oauth2/auth.
CREATE TABLE IF NOT EXISTS oauth2_authorization_codes (
    code_hash CHAR(64) PRIMARY KEY,
    client_id VARCHAR(64) NOT NULL REFERENCES oauth2_clients(client_id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    redirect_uri TEXT NOT NULL,
    scopes TEXT[] NOT NULL DEFAULT '{}',
    code_challenge VARCHAR(128) NOT NULL,
    code_challenge_method VARCHAR(8) NOT NULL DEFAULT 'S256',
    nonce TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL,
    -- Set on first redemption. A second attempt is a replay: RFC 6749 §4.1.2
    -- requires the code be rejected and every token already minted from it be
    -- revoked, which is what makes a stolen code useless.
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_oauth2_codes_expiry
    ON oauth2_authorization_codes(expires_at);

-- Access and refresh tokens. Stored as SHA-256 digests so a database leak does
-- not hand over live credentials.
CREATE TABLE IF NOT EXISTS oauth2_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash CHAR(64) UNIQUE NOT NULL,
    token_type VARCHAR(16) NOT NULL,
    client_id VARCHAR(64) NOT NULL REFERENCES oauth2_clients(client_id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    -- NULL for client_credentials: the client acts as itself, not for a user.
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    scopes TEXT[] NOT NULL DEFAULT '{}',
    -- Ties a rotated refresh token to the one it replaced, and every access
    -- token to the refresh token that minted it, so one detected replay can
    -- revoke the whole family.
    parent_id UUID REFERENCES oauth2_tokens(id) ON DELETE CASCADE,
    auth_code_hash CHAR(64),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT oauth2_tokens_type_check
        CHECK (token_type IN ('access', 'refresh'))
);
CREATE INDEX IF NOT EXISTS idx_oauth2_tokens_expiry
    ON oauth2_tokens(expires_at) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_oauth2_tokens_client
    ON oauth2_tokens(client_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_oauth2_tokens_auth_code
    ON oauth2_tokens(auth_code_hash) WHERE auth_code_hash IS NOT NULL;

-- Remembered consent, so a returning user is not asked to approve the same
-- scopes on every sign-in. Widening the scope set asks again.
CREATE TABLE IF NOT EXISTS oauth2_consents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    client_id VARCHAR(64) NOT NULL REFERENCES oauth2_clients(client_id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    scopes TEXT[] NOT NULL DEFAULT '{}',
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, client_id)
);

-- RSA keys for signing id_tokens. Persisted rather than generated per process
-- so that a restart does not invalidate every id_token in flight and so both
-- replicas of the API publish the same JWKS.
CREATE TABLE IF NOT EXISTS oauth2_signing_keys (
    kid VARCHAR(64) PRIMARY KEY,
    algorithm VARCHAR(16) NOT NULL DEFAULT 'RS256',
    private_key_pem TEXT NOT NULL,
    public_key_pem TEXT NOT NULL,
    -- Exactly one active key signs; retired keys stay published in the JWKS
    -- until every token they signed has expired.
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    retired_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_oauth2_signing_keys_one_active
    ON oauth2_signing_keys(active) WHERE active;

-- Row-level security for the three new tenant-scoped tables, matching what
-- upstream's 00029_tenant_rls applied to every table carrying a tenant_id at
-- the time it ran.
--
-- 00029 walked the schema once, and this file now runs after it, so these three
-- tables arrived with no policy at all: the second layer under the developer
-- portal's WHERE tenant_id was missing for codes, tokens and consents.
-- TestEveryTenantTableHasForcedRLS is the invariant that catches exactly this,
-- and it caught it — it self-skips without a database, which is why it stayed
-- quiet locally.
--
-- oauth2_signing_keys is deliberately absent: the keys are the provider's, not
-- a tenant's, and the table has no tenant_id for a policy to key on.
--
-- The token and code endpoints are unaffected. They are reached by a client
-- with no platform session, so no tenant is in context and dbguard leaves the
-- connection on the login role, which these policies do not apply to. What they
-- do cover is the developer portal, which always runs with a tenant bound.
ALTER TABLE oauth2_authorization_codes ENABLE ROW LEVEL SECURITY;
ALTER TABLE oauth2_authorization_codes FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON oauth2_authorization_codes;
CREATE POLICY tenant_isolation ON oauth2_authorization_codes TO gerege_nexus_app
    USING (tenant_id IS NULL OR tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid);

ALTER TABLE oauth2_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE oauth2_tokens FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON oauth2_tokens;
CREATE POLICY tenant_isolation ON oauth2_tokens TO gerege_nexus_app
    USING (tenant_id IS NULL OR tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid);

ALTER TABLE oauth2_consents ENABLE ROW LEVEL SECURITY;
ALTER TABLE oauth2_consents FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON oauth2_consents;
CREATE POLICY tenant_isolation ON oauth2_consents TO gerege_nexus_app
    USING (tenant_id IS NULL OR tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid);

-- +goose Down
DROP TABLE IF EXISTS oauth2_signing_keys;
DROP TABLE IF EXISTS oauth2_consents;
DROP TABLE IF EXISTS oauth2_tokens;
DROP TABLE IF EXISTS oauth2_authorization_codes;
-- oauth2_clients belongs to 00004; this migration only widened it, so down
-- takes the added columns back off rather than dropping the table.
ALTER TABLE oauth2_clients
    DROP CONSTRAINT IF EXISTS oauth2_clients_secret_matches_type,
    DROP CONSTRAINT IF EXISTS oauth2_clients_type_check,
    DROP COLUMN IF EXISTS client_type,
    DROP COLUMN IF EXISTS client_uri,
    DROP COLUMN IF EXISTS logo_uri,
    DROP COLUMN IF EXISTS disabled,
    DROP COLUMN IF EXISTS created_by,
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS secret_rotated_at,
    DROP COLUMN IF EXISTS last_used_at,
    ALTER COLUMN tenant_id DROP NOT NULL;
DROP INDEX IF EXISTS idx_oauth2_clients_tenant;
