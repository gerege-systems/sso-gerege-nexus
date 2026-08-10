-- +goose Up

-- Replays the two upstream migrations this fork's numbering caused goose to
-- skip, and gives the tables they create the row-level policies they missed.
--
-- The situation this repairs, exactly:
--
-- This fork wrote 00022_oauth2_provider and 00023_gerege_sso_rebrand while
-- upstream was writing 00022_integrations and 00023_integration_status_is_intent.
-- Every deployed fork database therefore has version rows 22 and 23 recorded
-- for *this fork's* files. When the merge renumbered ours to 00032 and 00033,
-- goose read 22 and 23 as already applied and skipped upstream's two files —
-- not with an error, which would have stopped the deploy, but silently, exiting
-- zero and reporting "successfully migrated". The integrations, integration_
-- oauth_states and integration_deliveries tables were never created, and
-- gov_appointments never got its mode, meeting_url and meeting_provider
-- columns. The API boots; Settings → Integrations, webhook dispatch and the
-- Drive, Dropbox and Meet connectors then fail against tables that do not exist.
--
-- Deleting the two version rows by hand before the deploy would also work, and
-- was the first plan. This is better: it needs no one to run anything at the
-- right moment, it repairs every copy of this database rather than the one
-- somebody remembered, and it is checked into the history where the next person
-- can read why. The cost is that it duplicates statements that already live in
-- upstream's files — which is why every one of them is written to be a no-op
-- against a database that already has them, and why this file is numbered
-- above the sweep rather than editing anything below it.
--
-- On a database built from scratch, upstream's 00022 and 00023 run in their own
-- place and everything below is a no-op. That path is tested too.

-- ── from 00022_integrations ──────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS integrations (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    provider          VARCHAR(32) NOT NULL,
    name              VARCHAR(255) NOT NULL,
    target_url        TEXT NOT NULL DEFAULT '',
    status            VARCHAR(16) NOT NULL DEFAULT 'ACTIVE',
    config            JSONB NOT NULL DEFAULT '{}'::jsonb,
    secret_ciphertext BYTEA,
    oauth_ciphertext  BYTEA,
    account_label     VARCHAR(255) NOT NULL DEFAULT '',
    connected_at      TIMESTAMPTZ,
    last_ping_at      TIMESTAMPTZ,
    last_error        TEXT NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- 'ERROR' is deliberately absent: 00023_integration_status_is_intent
    -- removed it, and a fresh table has no rows carrying it to migrate.
    CONSTRAINT integrations_status_known CHECK (status IN ('ACTIVE', 'INACTIVE')),
    CONSTRAINT integrations_provider_known CHECK (provider IN (
        'webhook', 'government', 'payment', 'custom_rest',
        'google_drive', 'dropbox', 'google_meet')),
    CONSTRAINT integrations_name_unique_per_tenant UNIQUE (tenant_id, name)
);

CREATE INDEX IF NOT EXISTS idx_integrations_tenant ON integrations(tenant_id, provider);
CREATE INDEX IF NOT EXISTS idx_integrations_dispatch
    ON integrations(tenant_id) WHERE provider = 'webhook' AND status = 'ACTIVE';

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

ALTER TABLE gov_appointments
    ADD COLUMN IF NOT EXISTS mode VARCHAR(16) NOT NULL DEFAULT 'IN_PERSON',
    ADD COLUMN IF NOT EXISTS meeting_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS meeting_provider VARCHAR(32) NOT NULL DEFAULT '';

-- +goose StatementBegin
DO $mode$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'gov_appointments_mode_known'
    ) THEN
        ALTER TABLE gov_appointments
            ADD CONSTRAINT gov_appointments_mode_known CHECK (mode IN ('IN_PERSON', 'ONLINE'));
    END IF;
END
$mode$;
-- +goose StatementEnd

-- ── from 00023_integration_status_is_intent ──────────────────────────────────
--
-- status is the administrator's intent, not the last delivery outcome. A
-- transient failure used to write 'ERROR' into it, and since every selection
-- predicate reads ACTIVE, one 503 switched a connector off for good. Health
-- lives in last_error instead.
--
-- Both statements are no-ops where 00023 already ran: nothing is left in
-- 'ERROR', and the constraint already excludes it.

UPDATE integrations SET status = 'ACTIVE' WHERE status = 'ERROR';

ALTER TABLE integrations DROP CONSTRAINT IF EXISTS integrations_status_known;
ALTER TABLE integrations
    ADD CONSTRAINT integrations_status_known CHECK (status IN ('ACTIVE', 'INACTIVE'));

-- ── the policies 00029_tenant_rls could not have applied ─────────────────────
--
-- 00029 walked the schema once. On a repaired database the three tables above
-- did not exist when it ran, so they would carry no policy at all — the same
-- gap 00032 closed for the OAuth2 tables. Idempotent, so the fresh-database
-- path simply re-states what 00029 already did.

ALTER TABLE integrations ENABLE ROW LEVEL SECURITY;
ALTER TABLE integrations FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON integrations;
CREATE POLICY tenant_isolation ON integrations TO gerege_nexus_app
    USING (tenant_id IS NULL OR tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid);

ALTER TABLE integration_oauth_states ENABLE ROW LEVEL SECURITY;
ALTER TABLE integration_oauth_states FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON integration_oauth_states;
CREATE POLICY tenant_isolation ON integration_oauth_states TO gerege_nexus_app
    USING (tenant_id IS NULL OR tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid);

ALTER TABLE integration_deliveries ENABLE ROW LEVEL SECURITY;
ALTER TABLE integration_deliveries FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON integration_deliveries;
CREATE POLICY tenant_isolation ON integration_deliveries TO gerege_nexus_app
    USING (tenant_id IS NULL OR tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid);

-- The app role's privileges came from a GRANT ON ALL TABLES in 00029, which
-- covers what existed then, plus ALTER DEFAULT PRIVILEGES for what the
-- migrating role creates afterwards. A table created here is covered by the
-- second; stated explicitly anyway, because a deployment whose default
-- privileges were set by a different role would otherwise have policies over
-- tables the app role cannot read at all.
GRANT SELECT, INSERT, UPDATE, DELETE ON integrations TO gerege_nexus_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON integration_oauth_states TO gerege_nexus_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON integration_deliveries TO gerege_nexus_app;

-- +goose Down

-- Deliberately empty.
--
-- Everything above belongs to upstream's 00022 and 00023, whose own Down
-- sections drop these tables. Dropping them here as well would destroy a
-- tenant's connectors on a rollback of a migration that, on a healthy
-- database, did nothing at all.
SELECT 1;
