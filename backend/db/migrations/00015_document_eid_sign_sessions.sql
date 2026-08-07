-- +goose Up

-- E-ID signing is an approval the citizen gives on their own device, and the only
-- thing that tells them what they are approving is the display text we push. That
-- makes the pairing of session to document security-critical: without it, an
-- approval given for one document could be presented as a signature on another.
-- This table is that pairing, and it is what makes the session single-use.
CREATE TABLE IF NOT EXISTS document_eid_sign_sessions (
    session_id VARCHAR(190) PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    document_id UUID NOT NULL REFERENCES document_records(id) ON DELETE CASCADE,
    -- The registration number the push was addressed to. The identity that comes
    -- back has to match it, or somebody else approved.
    reg_number VARCHAR(64) NOT NULL,
    -- Kept so a dispute can be answered with the exact words the citizen saw.
    display_text VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Set once the approval has been turned into a signature. A consumed session
    -- can never produce a second one.
    consumed_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_document_eid_sign_sessions_document
    ON document_eid_sign_sessions(document_id);

-- What the citizen approved with. A session id records that an approval happened;
-- the certificate records whose key gave it, which is what a dispute turns on.
-- eID returns these only on a completed session, and only when the certificate
-- could be parsed, so both stay nullable.
ALTER TABLE document_signatures
    ADD COLUMN IF NOT EXISTS certificate_serial VARCHAR(190),
    ADD COLUMN IF NOT EXISTS certificate_issuer VARCHAR(255);

-- +goose Down
ALTER TABLE document_signatures
    DROP COLUMN IF EXISTS certificate_serial,
    DROP COLUMN IF EXISTS certificate_issuer;

DROP TABLE IF EXISTS document_eid_sign_sessions;
