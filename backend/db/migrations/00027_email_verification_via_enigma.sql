-- +goose Up

-- The platform stopped sending this mail itself.
--
-- Delivering mail that arrives is not a matter of holding an SMTP password: it
-- is SPF, DKIM, DMARC, reverse DNS and a sending reputation, maintained
-- continuously. That is run by the hosted verification service, so this
-- platform now asks it for a link and holds no mailbox credential, no sender
-- address and no key of its own to issue.
--
-- Which makes email_verification_clients — one row per API key this platform
-- handed out to an outside caller — a table for a job it no longer does. An
-- outside caller now goes to the service directly, and keys are administered
-- there. Dropping it also drops the last place this database stored a
-- credential hash for this feature.
ALTER TABLE email_verifications DROP COLUMN IF EXISTS client_id;
DROP INDEX IF EXISTS idx_email_verifications_client_recent;
DROP TABLE IF EXISTS email_verification_clients;

-- token_hash keeps its name and its meaning — a SHA-256 of a single-use string
-- that this row is claimed by — but the string changed hands. It used to be the
-- token in a mail this platform composed; it is now the reference this platform
-- puts in the return address it hands to the service, and matches when somebody
-- comes back. Renaming the column would rewrite an index for a synonym.
COMMENT ON COLUMN email_verifications.token_hash IS
    'SHA-256 of the single-use reference carried in the return address given to the verification service.';

-- +goose Down
ALTER TABLE email_verifications ADD COLUMN IF NOT EXISTS client_id UUID;

CREATE TABLE IF NOT EXISTS email_verification_clients (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name         VARCHAR(255) NOT NULL,
    key_prefix   VARCHAR(16) NOT NULL,
    key_hash     CHAR(64) NOT NULL UNIQUE,
    status       VARCHAR(16) NOT NULL DEFAULT 'ACTIVE',
    hourly_limit INT NOT NULL DEFAULT 60,
    allowed_redirect_hosts TEXT NOT NULL DEFAULT '',
    last_used_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT email_verification_clients_status_known CHECK (status IN ('ACTIVE', 'DISABLED')),
    CONSTRAINT email_verification_clients_limit_sane CHECK (hourly_limit BETWEEN 1 AND 100000),
    CONSTRAINT email_verification_clients_name_unique_per_tenant UNIQUE (tenant_id, name)
);

CREATE INDEX IF NOT EXISTS idx_email_verification_clients_tenant
    ON email_verification_clients(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_email_verifications_client_recent
    ON email_verifications(client_id, created_at DESC);
