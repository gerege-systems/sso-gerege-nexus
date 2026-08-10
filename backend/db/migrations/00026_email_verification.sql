-- +goose Up

-- Proving that somebody controls an email address is not one app's problem.
-- Contacts wants it before it trusts an address, Documents wants it before it
-- sends a signing link to an outsider, and a platform running next to Gerege
-- Nexus wants it over HTTP for users who never sign in here at all. It is
-- therefore platform furniture — one flow, one audit trail, one place an
-- administrator can see who is sending what.
--
-- A client is a key issued to a caller outside the browser: another platform,
-- a mobile backend, a partner. The key itself is never stored. key_hash is a
-- SHA-256 of it, so a database dump is not a set of working credentials, and
-- key_prefix is the first few characters kept in the clear purely so an
-- administrator can tell two keys apart on screen.
CREATE TABLE IF NOT EXISTS email_verification_clients (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name         VARCHAR(255) NOT NULL,
    key_prefix   VARCHAR(16) NOT NULL,
    key_hash     CHAR(64) NOT NULL UNIQUE,
    -- The administrator's switch. A disabled client's key stops working on the
    -- next request; there is no cached copy of it anywhere to outlive this.
    status       VARCHAR(16) NOT NULL DEFAULT 'ACTIVE',
    hourly_limit INT NOT NULL DEFAULT 60,
    -- Empty means "any HTTPS destination". Otherwise a comma-separated host
    -- allowlist: the caller names the address the recipient is returned to, so
    -- without this the platform is an open redirector wearing its own domain.
    allowed_redirect_hosts TEXT NOT NULL DEFAULT '',
    last_used_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT email_verification_clients_status_known CHECK (status IN ('ACTIVE', 'DISABLED')),
    CONSTRAINT email_verification_clients_limit_sane CHECK (hourly_limit BETWEEN 1 AND 100000),
    -- The name is what an administrator revokes by, and two clients called
    -- "Mobile app" is how the wrong key gets switched off during an incident.
    CONSTRAINT email_verification_clients_name_unique_per_tenant UNIQUE (tenant_id, name)
);

CREATE INDEX IF NOT EXISTS idx_email_verification_clients_tenant
    ON email_verification_clients(tenant_id, created_at DESC);

-- One issued link. token_hash is a SHA-256 of the token in the mail for the
-- same reason as the key above: the row is a record of a link, not a copy of
-- one, so nobody reading the table can complete somebody else's verification.
--
-- source is a label kept at send time — the client name, an app module id, or
-- "portal". It is denormalised on purpose: a deleted client leaves its history
-- behind, and "who sent this" must still answer after the key is gone.
CREATE TABLE IF NOT EXISTS email_verifications (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    client_id    UUID REFERENCES email_verification_clients(id) ON DELETE SET NULL,
    source       VARCHAR(128) NOT NULL DEFAULT '',
    purpose      VARCHAR(64) NOT NULL DEFAULT '',
    email        VARCHAR(320) NOT NULL,
    token_hash   CHAR(64) NOT NULL UNIQUE,
    redirect_url TEXT NOT NULL DEFAULT '',
    status       VARCHAR(16) NOT NULL DEFAULT 'PENDING',
    requested_ip VARCHAR(64) NOT NULL DEFAULT '',
    expires_at   TIMESTAMPTZ NOT NULL,
    verified_at  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT email_verifications_status_known CHECK (status IN ('PENDING', 'VERIFIED', 'EXPIRED'))
);

-- The Overview screen reads the newest rows for one tenant.
CREATE INDEX IF NOT EXISTS idx_email_verifications_tenant
    ON email_verifications(tenant_id, created_at DESC);

-- Both rate limits count recent sends: one per client, one per recipient.
CREATE INDEX IF NOT EXISTS idx_email_verifications_client_recent
    ON email_verifications(client_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_email_verifications_recipient
    ON email_verifications(tenant_id, email, created_at DESC);

-- The sweep only ever looks at links still waiting to be followed.
CREATE INDEX IF NOT EXISTS idx_email_verifications_pending_expiry
    ON email_verifications(expires_at) WHERE status = 'PENDING';

-- +goose Down
DROP TABLE IF EXISTS email_verifications;
DROP TABLE IF EXISTS email_verification_clients;
