-- +goose Up

-- Ceremony state for the shared eID signing usecase (open-gerege-core's
-- core/business/usecases/sign), which expects a Redis-like key/value cache to
-- carry a session between init, poll and download.
--
-- This deployment runs no Redis, and adding one to hold a signing ceremony
-- would be a new piece of infrastructure to operate for state that Postgres
-- already stores durably. A table also survives a restart mid-ceremony, which
-- a cache with an eviction policy does not — and losing the state means losing
-- the PDF the citizen has already approved on their phone.
--
-- Deliberately separate from esign_sign_sessions: that table is the ERP's
-- tenant-scoped record (which document, which batch, whose signature, for the
-- log), while this is the shared library's own opaque state. Overloading one
-- table for both would tie the ERP's schema to the library's internals.
CREATE TABLE IF NOT EXISTS eid_sign_state (
    key        TEXT        PRIMARY KEY,
    value      TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- The value holds the base64 PDF being signed, so abandoned rows are the
    -- only unbounded growth here. Swept on the same schedule as the app's own
    -- abandoned ceremonies.
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_eid_sign_state_expiry ON eid_sign_state (expires_at);

-- Signing moved onto the shared usecase, which holds the document being signed
-- in eid_sign_state for the life of the ceremony. Keeping a second copy on
-- esign_sign_sessions would double the storage for every signature and, worse,
-- create two answers to "which bytes did the citizen approve?".
--
-- The columns stay for the rows already written by the previous release; they
-- simply stop being required.
ALTER TABLE esign_sign_sessions ALTER COLUMN original_pdf DROP NOT NULL;

-- +goose Down

ALTER TABLE esign_sign_sessions ALTER COLUMN original_pdf SET NOT NULL;

DROP INDEX IF EXISTS idx_eid_sign_state_expiry;
DROP TABLE IF EXISTS eid_sign_state;
