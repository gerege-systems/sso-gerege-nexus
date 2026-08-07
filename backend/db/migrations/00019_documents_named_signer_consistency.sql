-- +goose Up

-- require_named_signer used to be checked at signing time, and its guard only
-- asked that SOME step name a signer. A tenant could therefore store the flag
-- against a chain with an open step, or with one citizen named twice — pairs that
-- under the new rules cannot be saved at all, because a named step now binds on its
-- own and every step must name a distinct citizen for the flag to mean anything.
--
-- The flag is no longer consulted when a signature is applied, so leaving such a
-- pair stored would have the screen advertise "Named signer only" over a chain that
-- lets anyone fill its open steps. Enforcing it at signing time instead would strand
-- every document of that type, which is the outcome the save-time guards exist to
-- prevent.
--
-- Migration 00017 has already repaired the chains themselves — where a citizen was
-- named more than once, the later steps were opened, before anything was copied onto
-- a document or any signature placed. What is left here is the flag.
--
-- The flag is cleared wherever the repaired chain still could not satisfy it: a
-- chain with no steps at all, or one carrying an open step — including a step 00017
-- opened on its way past.
UPDATE document_signature_policies p
   SET require_named_signer = FALSE, updated_at = NOW()
 WHERE p.require_named_signer
   AND (
        -- no steps at all: nobody could sign
        NOT EXISTS (SELECT 1 FROM document_workflow_steps w
                     WHERE w.tenant_id = p.tenant_id AND w.doc_type = p.doc_type)
        -- a step naming nobody: it could never be filled under the flag
        OR EXISTS (SELECT 1 FROM document_workflow_steps w
                    WHERE w.tenant_id = p.tenant_id AND w.doc_type = p.doc_type
                      AND w.signer_reg_number = '')
        -- one citizen named twice: 00017 will have opened the repeat, so this is a
        -- belt to that brace — a chain that somehow still carries one could not be
        -- completed under the flag either
        OR EXISTS (SELECT w.signer_reg_number
                     FROM document_workflow_steps w
                    WHERE w.tenant_id = p.tenant_id AND w.doc_type = p.doc_type
                      AND w.signer_reg_number <> ''
                    GROUP BY w.signer_reg_number HAVING count(*) > 1)
   );

-- +goose Down

-- Nothing to undo. The flag was cleared because the chain beneath it could not be
-- completed by anybody; turning it back on would restore a state no screen will save
-- and no citizen could satisfy.
SELECT 1;
