-- +goose Up

-- Integrations were a process-global Go map. Two consequences, both bad in a
-- multi-tenant platform: every tenant administrator saw and could dispatch to
-- every other tenant's connectors, and a restart lost all of them. This is that
-- map, given a tenant and a disk.
--
-- Credentials never sit here in the clear. secret_ciphertext (the webhook HMAC
-- key) and oauth_ciphertext (the token bundle for a SaaS provider) are AES-GCM
-- sealed by the application under INTEGRATION_ENCRYPTION_KEY, so a database
-- dump is not a set of working credentials for someone's Google Drive.
CREATE TABLE IF NOT EXISTS integrations (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    -- webhook | government | payment | custom_rest | google_drive | dropbox | google_meet
    provider          VARCHAR(32) NOT NULL,
    name              VARCHAR(255) NOT NULL,
    target_url        TEXT NOT NULL DEFAULT '',
    status            VARCHAR(16) NOT NULL DEFAULT 'ACTIVE',
    -- Non-secret provider settings: destination folder, Dropbox path, calendar id.
    config            JSONB NOT NULL DEFAULT '{}'::jsonb,
    secret_ciphertext BYTEA,
    oauth_ciphertext  BYTEA,
    -- Which account the OAuth grant belongs to, so an administrator can tell
    -- whose Drive a document is about to land in before it does.
    account_label     VARCHAR(255) NOT NULL DEFAULT '',
    connected_at      TIMESTAMPTZ,
    last_ping_at      TIMESTAMPTZ,
    last_error        TEXT NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT integrations_status_known CHECK (status IN ('ACTIVE', 'INACTIVE', 'ERROR')),
    CONSTRAINT integrations_provider_known CHECK (provider IN (
        'webhook', 'government', 'payment', 'custom_rest',
        'google_drive', 'dropbox', 'google_meet')),
    -- One name per tenant: the name is what an operator picks the destination
    -- by, and two connectors called "Archive" is a way to file a signed
    -- document into the wrong account.
    CONSTRAINT integrations_name_unique_per_tenant UNIQUE (tenant_id, name)
);

CREATE INDEX IF NOT EXISTS idx_integrations_tenant ON integrations(tenant_id, provider);

-- Dispatch reads this on every event, and only ACTIVE webhooks are targets.
CREATE INDEX IF NOT EXISTS idx_integrations_dispatch
    ON integrations(tenant_id) WHERE provider = 'webhook' AND status = 'ACTIVE';

-- The OAuth `state` parameter, held rather than signed so that it is
-- single-use: a signed state that leaks is replayable until it expires, and
-- replaying an authorization-code callback is how a grant gets bound to the
-- wrong account. Rows are deleted when redeemed and swept when stale.
CREATE TABLE IF NOT EXISTS integration_oauth_states (
    state          VARCHAR(64) PRIMARY KEY,
    tenant_id      UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    integration_id UUID NOT NULL REFERENCES integrations(id) ON DELETE CASCADE,
    user_id        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    redirect_uri   TEXT NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at     TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_integration_oauth_states_expiry
    ON integration_oauth_states(expires_at);

-- A record of what actually left the platform. A signed PDF leaving for
-- somebody's Drive is a disclosure, and "we think it uploaded" is not an
-- answer anyone can act on afterwards.
CREATE TABLE IF NOT EXISTS integration_deliveries (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id      UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    integration_id UUID NOT NULL REFERENCES integrations(id) ON DELETE CASCADE,
    kind           VARCHAR(32) NOT NULL,
    reference      VARCHAR(255) NOT NULL DEFAULT '',
    outcome        VARCHAR(16) NOT NULL,
    detail         TEXT NOT NULL DEFAULT '',
    external_id    VARCHAR(255) NOT NULL DEFAULT '',
    external_url   TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT integration_deliveries_outcome_known CHECK (outcome IN ('OK', 'FAILED'))
);

CREATE INDEX IF NOT EXISTS idx_integration_deliveries_tenant
    ON integration_deliveries(tenant_id, created_at DESC);

-- An appointment can be held online. The link is stored rather than derived
-- because it is issued once by the conferencing provider and must survive the
-- integration later being disconnected — a citizen holding a meeting link for
-- next Tuesday should still be able to attend.
ALTER TABLE gov_appointments
    ADD COLUMN IF NOT EXISTS mode VARCHAR(16) NOT NULL DEFAULT 'IN_PERSON',
    ADD COLUMN IF NOT EXISTS meeting_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS meeting_provider VARCHAR(32) NOT NULL DEFAULT '';

-- goose splits on semicolons, so a dollar-quoted block has to be marked or the
-- body arrives as several broken statements.
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'gov_appointments_mode_known'
    ) THEN
        ALTER TABLE gov_appointments
            ADD CONSTRAINT gov_appointments_mode_known CHECK (mode IN ('IN_PERSON', 'ONLINE'));
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
ALTER TABLE gov_appointments DROP CONSTRAINT IF EXISTS gov_appointments_mode_known;
ALTER TABLE gov_appointments
    DROP COLUMN IF EXISTS meeting_provider,
    DROP COLUMN IF EXISTS meeting_url,
    DROP COLUMN IF EXISTS mode;
DROP TABLE IF EXISTS integration_deliveries;
DROP TABLE IF EXISTS integration_oauth_states;
DROP TABLE IF EXISTS integrations;
