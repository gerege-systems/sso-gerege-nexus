-- +goose Up

-- E-Signature workflow for io.example.documents.
--
-- The documents module shipped with a signed_by column but no way to populate
-- it: there was no sign endpoint and status never left PENDING_APPROVAL. These
-- columns record who signed a document, through which national channel (E-ID or
-- DAN), the returned digital-signature hash, and when it happened.
ALTER TABLE document_records
    ADD COLUMN IF NOT EXISTS signature_hash    VARCHAR(255),
    ADD COLUMN IF NOT EXISTS signer_reg_number VARCHAR(64),
    ADD COLUMN IF NOT EXISTS signer_method     VARCHAR(32),
    ADD COLUMN IF NOT EXISTS signed_at         TIMESTAMPTZ;

-- +goose Down
ALTER TABLE document_records
    DROP COLUMN IF EXISTS signature_hash,
    DROP COLUMN IF EXISTS signer_reg_number,
    DROP COLUMN IF EXISTS signer_method,
    DROP COLUMN IF EXISTS signed_at;
