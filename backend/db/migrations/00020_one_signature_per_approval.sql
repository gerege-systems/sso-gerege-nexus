-- +goose Up

-- One approval, one signature. The ledger already refused the same citizen twice
-- (`document_signatures_once_per_signer`), but nothing stopped two DIFFERENT citizens
-- being recorded against the same step of the same document — and when that happened
-- it happened silently, leaving the trail crediting a named step to whoever landed on
-- it second.
--
-- Two ways it could arise, both now fixed in the code, both invisible without this:
--
--   * migration 00017 §4 parked a left-over signature at "number of steps + n", which
--     on a chain whose numbers are not 1..n lands ON an existing step;
--   * recordSignature numbered a signature that filled no step "applied + 1", with
--     the same collision at runtime.
--
-- A constraint is worth more than both fixes. It turns a numbering mistake into a
-- failed write — loud, and before anything is attributed to the wrong person —
-- instead of a ledger that reads plausibly and says the wrong thing.

-- +goose StatementBegin
DO $$
DECLARE
    repaired INTEGER;
BEGIN
    -- Any duplicate that already exists is renumbered past the end of its document's
    -- chain. That is what §4 should have done with it: the signature is real and
    -- counts toward what the document holds, it just cannot claim an approval another
    -- signature already claimed.
    --
    -- WHICH one keeps the step is decided by the step itself, not by the clock. The
    -- collision this repairs is the one 00017 §4 produced: a signature from somebody
    -- no step names, parked onto a real step next to the signature of the citizen that
    -- step DOES name. The stranger's is often the older of the two — legacy signatures
    -- are what §4 was placing — so keeping the oldest would strip the named signer of
    -- their own approval and freeze the stranger into it, which is the misattribution
    -- this migration exists to end. So: the citizen the step names first, then the
    -- clock, then the id.
    WITH ranked AS (
        SELECT s.id, s.document_id, s.step_order,
               row_number() OVER (
                   PARTITION BY s.document_id, s.step_order
                   ORDER BY (st.signer_reg_number IS NOT NULL
                             AND st.signer_reg_number = s.signer_reg_number) DESC,
                            s.signed_at, s.id) AS n
          FROM document_signatures s
          LEFT JOIN document_approval_steps st
            ON st.document_id = s.document_id AND st.step_order = s.step_order
         WHERE s.step_order IS NOT NULL
    ),
    clashing AS (
        SELECT r.id, r.document_id,
               row_number() OVER (PARTITION BY r.document_id ORDER BY r.step_order, r.id) AS offset_n
          FROM ranked r
         WHERE r.n > 1
    )
    UPDATE document_signatures s
       SET step_order = GREATEST(
             COALESCE((SELECT max(x.step_order) FROM document_signatures x
                        WHERE x.document_id = c.document_id), 0),
             COALESCE((SELECT max(st.step_order) FROM document_approval_steps st
                        WHERE st.document_id = c.document_id), 0)) + c.offset_n
      FROM clashing c
     WHERE c.id = s.id;

    GET DIAGNOSTICS repaired = ROW_COUNT;
    IF repaired > 0 THEN
        RAISE NOTICE 'renumbered % signature(s) that shared an approval with another', repaired;
    END IF;
END $$;
-- +goose StatementEnd

-- NULL step_order is still allowed — 00017 fills them all in, but a row that somehow
-- arrives without one should not be forced onto an approval it did not fill. Postgres
-- treats NULLs as distinct in a unique index, which is the behaviour wanted here.
ALTER TABLE document_signatures
    ADD CONSTRAINT document_signatures_one_per_approval UNIQUE (document_id, step_order);

-- +goose Down
ALTER TABLE document_signatures
    DROP CONSTRAINT IF EXISTS document_signatures_one_per_approval;
