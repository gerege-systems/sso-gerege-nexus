-- +goose Up

-- The four remaining Documents screens (templates, workflows, signature
-- policies, retention rules) had a sidebar entry each but no storage. This adds
-- it, and turns one implicit rule explicit: a document used to carry exactly one
-- signature, in columns on document_records. Approval that needs two or three
-- signers had nowhere to record them, so document_signatures becomes the ledger
-- and document_records keeps the newest signature for the list query.

-- Reusable presets a clerk starts a document from. A document is a title and a
-- type today, so that is all a template can carry — there is no body to hold.
CREATE TABLE IF NOT EXISTS document_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    doc_type VARCHAR(64) NOT NULL DEFAULT 'CONTRACT',
    title_pattern VARCHAR(255) NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT document_templates_name_unique UNIQUE (tenant_id, name)
);
CREATE INDEX IF NOT EXISTS idx_document_templates_tenant ON document_templates(tenant_id);

-- Who must sign a document type, in order. No steps for a type means one
-- signature approves it, which is how the app behaved before this table.
CREATE TABLE IF NOT EXISTS document_workflow_steps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    doc_type VARCHAR(64) NOT NULL,
    step_order SMALLINT NOT NULL,
    name VARCHAR(255) NOT NULL,
    -- Empty means "anyone who may sign": the step counts, but no particular
    -- citizen is named for it.
    signer_reg_number VARCHAR(64) NOT NULL DEFAULT '',
    CONSTRAINT document_workflow_steps_order_unique UNIQUE (tenant_id, doc_type, step_order),
    CONSTRAINT document_workflow_steps_order_positive CHECK (step_order >= 1)
);
CREATE INDEX IF NOT EXISTS idx_document_workflow_steps_type ON document_workflow_steps(tenant_id, doc_type);

-- Every signature applied to a document.
CREATE TABLE IF NOT EXISTS document_signatures (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    document_id UUID NOT NULL REFERENCES document_records(id) ON DELETE CASCADE,
    signer_name VARCHAR(255) NOT NULL,
    signer_reg_number VARCHAR(64) NOT NULL,
    signer_method VARCHAR(32) NOT NULL,
    signature_hash VARCHAR(255) NOT NULL,
    signed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- One citizen signs a given document once. A second attempt is the same
    -- approval twice over, not progress through the workflow.
    CONSTRAINT document_signatures_once_per_signer UNIQUE (document_id, signer_reg_number)
);
CREATE INDEX IF NOT EXISTS idx_document_signatures_document ON document_signatures(document_id);

-- How a document type may be signed.
CREATE TABLE IF NOT EXISTS document_signature_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    doc_type VARCHAR(64) NOT NULL,
    allow_eid BOOLEAN NOT NULL DEFAULT TRUE,
    allow_dan BOOLEAN NOT NULL DEFAULT TRUE,
    -- When set, only a registration number named by a workflow step may sign.
    require_named_signer BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT document_signature_policies_type_unique UNIQUE (tenant_id, doc_type),
    -- A policy that forbids both channels would make the type unsignable.
    CONSTRAINT document_signature_policies_one_method CHECK (allow_eid OR allow_dan)
);

-- How long a document of each type is kept. Nothing deletes on this schedule:
-- the retention screen reports what is past its term and leaves the decision to
-- a person.
CREATE TABLE IF NOT EXISTS document_retention_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    doc_type VARCHAR(64) NOT NULL,
    retain_years SMALLINT NOT NULL,
    note VARCHAR(255) NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT document_retention_rules_type_unique UNIQUE (tenant_id, doc_type),
    CONSTRAINT document_retention_rules_years_sane CHECK (retain_years BETWEEN 1 AND 100)
);

-- Carry the signature each already-signed document holds into the ledger, so a
-- document signed before this migration is not absent from its own history.
INSERT INTO document_signatures (tenant_id, document_id, signer_name, signer_reg_number,
                                 signer_method, signature_hash, signed_at)
SELECT tenant_id, id, COALESCE(signed_by, ''), COALESCE(signer_reg_number, ''),
       COALESCE(signer_method, ''), COALESCE(signature_hash, ''), signed_at
  FROM document_records
 WHERE signed_at IS NOT NULL
    ON CONFLICT (document_id, signer_reg_number) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS document_retention_rules;
DROP TABLE IF EXISTS document_signature_policies;
DROP TABLE IF EXISTS document_signatures;
DROP TABLE IF EXISTS document_workflow_steps;
DROP TABLE IF EXISTS document_templates;
