-- +goose Up

-- PDF E-Sign v2. The app gains a second signing rail — eID Mongolia qualified
-- remote signing (PIN2 on the citizen's phone) alongside the existing Gerege
-- eSign HSM — plus the tenant configuration and batch runs the module screens
-- are built on.

-- ─── eID identity linkage ────────────────────────────────────────────────────

-- Qualified remote signing addresses the citizen by their ETSI semantics
-- identifier (PNOMN-<civilId>), but sign-in only ever kept an email and a
-- tenant, so nothing on the platform knew who a logged-in user *is* to eID.
-- Without this, every signature would make the citizen retype the registration
-- number they had just authenticated with — and a typo would push the PIN2
-- prompt at somebody else's phone.
--
-- Held apart from `users` on purpose: eID identity is optional (password and
-- DAN accounts have none), it has its own lifecycle, and keeping it out of the
-- core table means a national identifier is never selected by accident.
CREATE TABLE IF NOT EXISTS user_eid_identities (
    user_id         UUID        PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    civil_id        VARCHAR(32),
    reg_number      VARCHAR(32),
    person_etsi     VARCHAR(64) NOT NULL,
    -- The device that completed authentication. Pushing a signature at it
    -- directly is more precise than a person-wide push when the citizen holds
    -- several enrolled devices.
    document_number VARCHAR(64),
    given_name      VARCHAR(255),
    surname         VARCHAR(255),
    linked_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- One eID citizen resolves to one ERP account. A second account claiming the
-- same identifier is a linkage bug, and it should fail loudly at write time
-- rather than silently splitting a person's signing history in two.
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_eid_identities_etsi
    ON user_eid_identities (person_etsi);

-- ─── Documents ───────────────────────────────────────────────────────────────

-- Provenance and integrity columns. checksum is the SHA-256 of original_pdf: it
-- is the same digest eID asks the citizen's device to sign, so storing it makes
-- an after-the-fact "was this the document that was signed?" answerable without
-- re-reading the blob.
ALTER TABLE esign_documents
    ADD COLUMN IF NOT EXISTS checksum          CHAR(64),
    ADD COLUMN IF NOT EXISTS byte_size         BIGINT      NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS uploaded_by       UUID        REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS provider          VARCHAR(16) NOT NULL DEFAULT 'HSM',
    ADD COLUMN IF NOT EXISTS signer_etsi       VARCHAR(64),
    ADD COLUMN IF NOT EXISTS on_behalf_of_etsi VARCHAR(64),
    ADD COLUMN IF NOT EXISTS on_behalf_of_name VARCHAR(255),
    ADD COLUMN IF NOT EXISTS certificate_level VARCHAR(16),
    ADD COLUMN IF NOT EXISTS deleted_at        TIMESTAMPTZ;

-- Signed documents are evidence, so deletion is a soft archive. Every listing
-- filters on deleted_at IS NULL; this index keeps that the cheap path.
CREATE INDEX IF NOT EXISTS idx_esign_documents_tenant_live
    ON esign_documents (tenant_id, created_at DESC)
    WHERE deleted_at IS NULL;

-- ─── Signing sessions ────────────────────────────────────────────────────────

-- One remote-signing ceremony. The id is 32 lowercase hex rather than a UUID
-- because that is the identifier format the signing client contract uses, and
-- the browser validates it against /^[a-f0-9]{32}$/ before polling.
--
-- The original PDF is copied onto the session. A ceremony can outlive an edit
-- to the document row, and eID's stamp endpoint must be handed back exactly the
-- bytes whose digest the citizen approved — hashing a changed document would
-- produce a PDF whose signature does not verify.
CREATE TABLE IF NOT EXISTS esign_sign_sessions (
    id                  CHAR(32)    PRIMARY KEY,
    tenant_id           UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    document_id         UUID        REFERENCES esign_documents(id) ON DELETE SET NULL,
    provider            VARCHAR(16) NOT NULL DEFAULT 'EID',

    -- The upstream eID session id. Kept separate from our own id so a support
    -- request can be correlated with eID's logs.
    eid_session_id      TEXT,

    -- pending | completed | failed | expired | rejected — the vocabulary the
    -- signing UI polls for.
    state               VARCHAR(16) NOT NULL DEFAULT 'pending',
    failure_reason      VARCHAR(64),

    file_name           VARCHAR(255) NOT NULL,
    document_hash       CHAR(64)    NOT NULL,
    verification_code   VARCHAR(8),

    signer_user_id      UUID        REFERENCES users(id) ON DELETE SET NULL,
    signer_etsi         VARCHAR(64),
    signer_name         VARCHAR(255),
    on_behalf_of_etsi   VARCHAR(64),
    on_behalf_of_name   VARCHAR(255),
    certificate_level   VARCHAR(16),
    signature_algorithm VARCHAR(64),

    original_pdf        BYTEA       NOT NULL,
    signed_pdf          BYTEA,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at        TIMESTAMPTZ,
    expires_at          TIMESTAMPTZ NOT NULL,

    CONSTRAINT esign_sign_sessions_state_check
        CHECK (state IN ('pending', 'completed', 'failed', 'expired', 'rejected'))
);

CREATE INDEX IF NOT EXISTS idx_esign_sign_sessions_tenant
    ON esign_sign_sessions (tenant_id, created_at DESC);

-- Sweeping expired ceremonies walks pending rows past their deadline only.
CREATE INDEX IF NOT EXISTS idx_esign_sign_sessions_pending
    ON esign_sign_sessions (expires_at)
    WHERE state = 'pending';

-- ─── Tenant configuration ────────────────────────────────────────────────────

-- Backs the three settings screens. The payloads are JSONB because each is a
-- small, self-contained document that the app validates on write — a column per
-- field would mean a migration every time a placement gained an option.
--
-- No secret belongs in `hsm`: the HSM token is an environment variable, and
-- this row is readable by anyone holding esign.manage.
CREATE TABLE IF NOT EXISTS esign_settings (
    tenant_id  UUID        PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    placement  JSONB       NOT NULL DEFAULT '{}'::jsonb,
    hsm        JSONB       NOT NULL DEFAULT '{}'::jsonb,
    policy     JSONB       NOT NULL DEFAULT '{}'::jsonb,
    updated_by UUID        REFERENCES users(id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── Batch signing ───────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS esign_batches (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name        VARCHAR(255) NOT NULL,
    provider    VARCHAR(16) NOT NULL DEFAULT 'EID',
    status      VARCHAR(16) NOT NULL DEFAULT 'DRAFT',
    created_by  UUID        REFERENCES users(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at  TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,

    CONSTRAINT esign_batches_status_check
        CHECK (status IN ('DRAFT', 'RUNNING', 'COMPLETED', 'FAILED', 'CANCELLED'))
);

CREATE INDEX IF NOT EXISTS idx_esign_batches_tenant
    ON esign_batches (tenant_id, created_at DESC);

CREATE TABLE IF NOT EXISTS esign_batch_items (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id    UUID        NOT NULL REFERENCES esign_batches(id) ON DELETE CASCADE,
    document_id UUID        NOT NULL REFERENCES esign_documents(id) ON DELETE CASCADE,
    position    INT         NOT NULL DEFAULT 0,
    status      VARCHAR(16) NOT NULL DEFAULT 'PENDING',
    session_id  CHAR(32)    REFERENCES esign_sign_sessions(id) ON DELETE SET NULL,
    error       TEXT,
    signed_at   TIMESTAMPTZ,

    CONSTRAINT esign_batch_items_status_check
        CHECK (status IN ('PENDING', 'RUNNING', 'SIGNED', 'FAILED', 'SKIPPED')),
    -- A document may appear in many batches over time but never twice in one.
    CONSTRAINT esign_batch_items_unique UNIQUE (batch_id, document_id)
);

CREATE INDEX IF NOT EXISTS idx_esign_batch_items_batch
    ON esign_batch_items (batch_id, position);

-- ─── Signature log ───────────────────────────────────────────────────────────

-- The log gains the columns the signature-log screen filters on. `outcome`
-- matters most: the original table only ever recorded successes, so a refused
-- or expired ceremony left no trace, which is exactly the event an auditor
-- looks for.
ALTER TABLE esign_signature_logs
    ADD COLUMN IF NOT EXISTS provider       VARCHAR(16) NOT NULL DEFAULT 'HSM',
    ADD COLUMN IF NOT EXISTS outcome        VARCHAR(16) NOT NULL DEFAULT 'OK',
    ADD COLUMN IF NOT EXISTS session_id     CHAR(32),
    ADD COLUMN IF NOT EXISTS actor_user_id  UUID        REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS document_title VARCHAR(255),
    ADD COLUMN IF NOT EXISTS detail         TEXT;

CREATE INDEX IF NOT EXISTS idx_esign_signature_logs_tenant_time
    ON esign_signature_logs (tenant_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_esign_signature_logs_document
    ON esign_signature_logs (document_id)
    WHERE document_id IS NOT NULL;

-- ─── Permission backfill ─────────────────────────────────────────────────────

-- The app declared esign.read / esign.sign / esign.manage from the start but
-- never asserted them: io.example.esign is absent from the platform's blanket
-- app gate, and the handlers only checked the tenant. Anyone in the tenant
-- could sign. The handlers now enforce all three, which would silently take
-- signing away from every non-administrator on an existing deployment.
--
-- So the grants existing roles should already have had are backfilled here.
-- esign.sign reaches ordinary users on purpose: the authority to sign is the
-- citizen's own — eID signs with their PIN2 on their own phone — while
-- uploading and configuring stay behind esign.manage.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
  FROM roles r
  JOIN permissions p ON p.code = ANY (ARRAY['esign.read', 'esign.sign', 'esign.manage'])
 WHERE r.code = 'manager'
   AND r.active
   AND EXISTS (SELECT 1 FROM app_installations i
                WHERE i.tenant_id = r.tenant_id AND i.app_id = 'io.example.esign')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
  FROM roles r
  JOIN permissions p ON p.code = ANY (ARRAY['esign.read', 'esign.sign'])
 WHERE r.code = 'user'
   AND r.active
   AND EXISTS (SELECT 1 FROM app_installations i
                WHERE i.tenant_id = r.tenant_id AND i.app_id = 'io.example.esign')
ON CONFLICT DO NOTHING;

-- +goose Down

DROP INDEX IF EXISTS idx_esign_signature_logs_document;
DROP INDEX IF EXISTS idx_esign_signature_logs_tenant_time;
ALTER TABLE esign_signature_logs
    DROP COLUMN IF EXISTS detail,
    DROP COLUMN IF EXISTS document_title,
    DROP COLUMN IF EXISTS actor_user_id,
    DROP COLUMN IF EXISTS session_id,
    DROP COLUMN IF EXISTS outcome,
    DROP COLUMN IF EXISTS provider;

DROP TABLE IF EXISTS esign_batch_items;
DROP TABLE IF EXISTS esign_batches;
DROP TABLE IF EXISTS esign_settings;
DROP TABLE IF EXISTS esign_sign_sessions;

DROP INDEX IF EXISTS idx_user_eid_identities_etsi;
DROP TABLE IF EXISTS user_eid_identities;

DROP INDEX IF EXISTS idx_esign_documents_tenant_live;
ALTER TABLE esign_documents
    DROP COLUMN IF EXISTS deleted_at,
    DROP COLUMN IF EXISTS certificate_level,
    DROP COLUMN IF EXISTS on_behalf_of_name,
    DROP COLUMN IF EXISTS on_behalf_of_etsi,
    DROP COLUMN IF EXISTS signer_etsi,
    DROP COLUMN IF EXISTS provider,
    DROP COLUMN IF EXISTS uploaded_by,
    DROP COLUMN IF EXISTS byte_size,
    DROP COLUMN IF EXISTS checksum;
