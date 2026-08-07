-- +goose Up

-- What a document needed is a property of the document, not of today's
-- configuration. Until now required_signatures was counted live from
-- document_workflow_steps on every read, and the completion decision compared a
-- signature count taken under a row lock against a requirement read before the
-- lock was held. Three defects followed from that:
--
--   * a chain edited between the two reads could approve a document on fewer
--     signatures than its chain asked for;
--   * shortening a chain left an already-signed document stuck PENDING while the
--     screen painted a green "complete" badge beside the amber "Pending" one, and
--     no further signature could finish it;
--   * editing a chain rewrote the reported progress of documents decided months
--     earlier — an APPROVED contract would start claiming 1 of 3 signatures.
--
-- A document now carries its own copy of the chain, taken when it starts waiting
-- for approval, and its own requirement count. A later configuration change
-- cannot retroactively redefine what an in-flight or finished document needed.

ALTER TABLE document_records
    ADD COLUMN IF NOT EXISTS required_signatures SMALLINT NOT NULL DEFAULT 1;

-- The document's own approval chain. No rows means one open signature approves,
-- which is how a document of a type with no chain behaves.
CREATE TABLE IF NOT EXISTS document_approval_steps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    document_id UUID NOT NULL REFERENCES document_records(id) ON DELETE CASCADE,
    step_order SMALLINT NOT NULL,
    name VARCHAR(255) NOT NULL,
    -- Empty means the step is open: anyone allowed to sign may take it. A
    -- registration number names the one citizen whose signature fills it.
    signer_reg_number VARCHAR(64) NOT NULL DEFAULT '',
    CONSTRAINT document_approval_steps_order_unique UNIQUE (document_id, step_order),
    CONSTRAINT document_approval_steps_order_positive CHECK (step_order >= 1)
);
CREATE INDEX IF NOT EXISTS idx_document_approval_steps_document
    ON document_approval_steps(document_id);

-- Which step each signature filled. Signatures fill the chain in order, so this
-- is also the signature's ordinal — stored because a ledger that cannot say
-- which approval a signature was is a poor record of an approval chain.
ALTER TABLE document_signatures
    ADD COLUMN IF NOT EXISTS step_order SMALLINT;

-- +goose StatementBegin
DO $$
BEGIN
    -- 0. Repair the chains first, because everything below depends on their shape.
    --
    --    A chain naming ONE citizen at TWO steps was legal before this release: the
    --    registration numbers were only consulted when the signature policy demanded
    --    named signers, and then as an unordered set, so the chain simply meant "two
    --    signatures". Under the new rules a named step binds on its own, and one
    --    citizen signs a document once — so the second step would be owed to somebody
    --    who has already signed, and no document of that type could ever be approved.
    --
    --    The same applies to a step naming something that is not a registration
    --    number at all. Nothing checked the length before this release either, and
    --    the numbers were ignored unless the policy asked for them, so "AA9" was a
    --    harmless typo in a chain that just meant "two signatures". Now it names a
    --    citizen who can never present that number to a provider, so the step is
    --    unfillable and rejection is the document's only exit.
    --
    --    Both are opened. The tenant asked for that many approvals and still gets
    --    them; the step is simply fillable by whoever can actually sign, which is the
    --    only reading that leaves the chain completable. Doing this BEFORE the copy
    --    and the placement below matters: it is what lets a legacy signature land in
    --    the step it belongs in rather than being parked past the end of the chain.
    --    First, though, the numbers themselves. The save path upper-cases and trims
    --    what it stores, and the signing path upper-cases what the citizen presents, so
    --    a step holding 'aa90010111' or a padded number names somebody who can never
    --    match it — unfillable in exactly the same way. Normalising before the test
    --    below is also what makes the test right: 'AA90010111' at one step and
    --    'aa90010111' at another are ONE citizen named twice, and comparing the raw
    --    strings would not notice.
    UPDATE document_workflow_steps
       SET signer_reg_number = upper(btrim(signer_reg_number))
     WHERE signer_reg_number <> upper(btrim(signer_reg_number));

    --    One honest limit here: upper() answers to the cluster's ctype. On a database
    --    initialised with LC_CTYPE=C, upper('уб99010111') is 'уб99010111' unchanged, so
    --    a Cyrillic number stored in lower case survives this statement as it was —
    --    and, being ten characters and not a duplicate of anything the statement can
    --    see, is kept named. The runtime does not depend on that: snapshotApprovalChain
    --    normalises in Go, so every document minted after this migration carries a
    --    canonical chain whatever the locale. A chain already copied onto a waiting
    --    document below is what this cannot reach; on such a cluster it has to be saved
    --    again from the workflows screen, which normalises it properly.

    WITH unfillable AS (
        SELECT id
          FROM (SELECT id, signer_reg_number,
                       row_number() OVER (PARTITION BY tenant_id, doc_type, signer_reg_number
                                          ORDER BY step_order) AS n
                  FROM document_workflow_steps
                 WHERE signer_reg_number <> '') AS named
         WHERE n > 1                              -- one citizen signs a document once
            OR length(signer_reg_number) < 8      -- no provider would accept it
    )
    UPDATE document_workflow_steps w
       SET signer_reg_number = ''
      FROM unfillable
     WHERE unfillable.id = w.id;

    -- 1. Give every document still waiting a copy of its type's chain, unless it
    --    already carries one. That guard is what makes a partial re-run safe.
    INSERT INTO document_approval_steps (tenant_id, document_id, step_order, name, signer_reg_number)
    SELECT d.tenant_id, d.id, w.step_order, w.name, w.signer_reg_number
      FROM document_records d
      JOIN document_workflow_steps w
        ON w.tenant_id = d.tenant_id AND w.doc_type = d.doc_type
     WHERE d.status = 'PENDING_APPROVAL'
       AND NOT EXISTS (SELECT 1 FROM document_approval_steps s WHERE s.document_id = d.id);

    -- 2. Work out which step each existing signature filled.
    --
    --    Time order is NOT good enough. Before this migration the order signatures
    --    arrived in did not matter — a signer was accepted if the chain named them
    --    anywhere, and only when the policy demanded it — so a chain
    --    [1 Захирал=AA, 2 Ня-бо=BB] may well hold BB's signature first. Crediting it
    --    to step 1 by time would leave step 2 naming BB, who cannot sign twice: the
    --    document could never be approved by anybody.
    --
    --    So a signature is matched to the step that NAMES its signer first.
    WITH matched AS (
        SELECT DISTINCT ON (s.id) s.id AS signature_id, st.step_order
          FROM document_signatures s
          JOIN document_approval_steps st
            ON st.document_id = s.document_id
           AND st.signer_reg_number = s.signer_reg_number
         WHERE s.step_order IS NULL AND s.signer_reg_number <> ''
         ORDER BY s.id, st.step_order
    )
    UPDATE document_signatures s
       SET step_order = matched.step_order
      FROM matched
     WHERE matched.signature_id = s.id;

    -- 3. The rest fill whatever OPEN step numbers are still free, oldest first.
    --
    --    Only open ones. A step that names a citizen is owed to that citizen: giving
    --    it to somebody the chain never named would lock the named signer out of
    --    their own step for good, let the document complete without them, and leave
    --    the ledger recording the wrong person as having given that approval.
    WITH free_slots AS (
        SELECT st.document_id, st.step_order,
               row_number() OVER (PARTITION BY st.document_id ORDER BY st.step_order) AS slot
          FROM document_approval_steps st
         WHERE st.signer_reg_number = ''
           AND NOT EXISTS (
                 SELECT 1 FROM document_signatures s
                  WHERE s.document_id = st.document_id AND s.step_order = st.step_order)
    ),
    unplaced AS (
        SELECT s.id, s.document_id,
               row_number() OVER (PARTITION BY s.document_id ORDER BY s.signed_at, s.id) AS n
          FROM document_signatures s
         WHERE s.step_order IS NULL
    )
    UPDATE document_signatures s
       SET step_order = f.step_order
      FROM unplaced u
      JOIN free_slots f ON f.document_id = u.document_id AND f.slot = u.n
     WHERE s.id = u.id;

    -- 4. Anything left over — a document with more signatures than steps, or none
    --    at all — is numbered past the end of the chain rather than dropped.
    --
    --    Past the HIGHEST number, not past the count. Nothing this module writes
    --    produces a chain numbered other than 1..n, but the table asks only for
    --    positive distinct numbers, so a chain edited by hand can hold steps 2 and 3.
    --    Offsetting by the count would then park a signature at 2 + 1 = 3 — on top of
    --    a step that NAMES somebody, marking it filled by a citizen the chain never
    --    named, which is exactly what §3 refuses to do.
    WITH leftover AS (
        SELECT s.id,
               COALESCE((SELECT max(st.step_order) FROM document_approval_steps st
                          WHERE st.document_id = s.document_id), 0)
                 + row_number() OVER (PARTITION BY s.document_id ORDER BY s.signed_at, s.id) AS n
          FROM document_signatures s
         WHERE s.step_order IS NULL
    )
    UPDATE document_signatures s
       SET step_order = leftover.n
      FROM leftover
     WHERE leftover.id = s.id;

    -- 5. Pin the requirement.
    --
    --    An APPROVED document asked for exactly what it got: its requirement is its
    --    signature count. A PENDING or REJECTED one did not finish, so its
    --    requirement is the longest of its chain and what it already holds —
    --    clamping it to the count would make a shortened-chain document read
    --    "2/1 complete" beside an amber Pending badge, and a rejected one read as
    --    though its chain had been satisfied.
    UPDATE document_records d
       SET required_signatures = GREATEST(1, (
             SELECT count(*) FROM document_signatures s WHERE s.document_id = d.id))
     WHERE d.status = 'APPROVED';

    --    A signature that filled no step — one from a citizen no step names, with no
    --    open step to take — is numbered past the end of the chain by §4. It counts
    --    toward what the document holds but satisfies none of what it owes, so the
    --    requirement is the chain PLUS those, not the greater of the two. Taking the
    --    maximum let a document read "2 of 2" while a named step was still owed.
    --
    --    "Past the end" is the highest number here too, for the same reason as §4:
    --    against a chain numbered 2 and 3, counting would treat a signature on step 3
    --    as parked and ask for one approval more than the document owes.
    UPDATE document_records d
       SET required_signatures = GREATEST(
             1,
             (SELECT count(*) FROM document_approval_steps s WHERE s.document_id = d.id)
               + (SELECT count(*) FROM document_signatures s
                   WHERE s.document_id = d.id
                     AND s.step_order > COALESCE((SELECT max(st.step_order)
                                                    FROM document_approval_steps st
                                                   WHERE st.document_id = d.id), 0)),
             (SELECT count(*) FROM document_workflow_steps w
               WHERE w.tenant_id = d.tenant_id AND w.doc_type = d.doc_type))
     WHERE d.status = 'PENDING_APPROVAL';

    --    A rejected document did not get what it asked for, and nothing records what
    --    that was: it carries no copied chain, and its type's chain may have changed
    --    or gone since. The best available reading is the longer of that chain and
    --    what the document actually holds — never less than it holds, which would
    --    have it needing fewer signatures than it has. It cannot read as a satisfied
    --    chain either way: the screens do not call a rejected document complete, and
    --    do not show its progress at all.
    UPDATE document_records d
       SET required_signatures = GREATEST(
             1,
             (SELECT count(*) FROM document_workflow_steps w
               WHERE w.tenant_id = d.tenant_id AND w.doc_type = d.doc_type),
             (SELECT count(*) FROM document_signatures s WHERE s.document_id = d.id))
     WHERE d.status = 'REJECTED';

    -- 6. A document the old code left pending-but-complete — its chain was shortened
    --    under it — is finished here rather than left needing one more signature than
    --    it asks for. The test is the one the runtime uses: every step filled AND at
    --    least as many signatures as it asked for. Comparing the counts alone would
    --    approve a document whose named step is still owed.
    UPDATE document_records d
       SET status = 'APPROVED'
     WHERE d.status = 'PENDING_APPROVAL'
       AND (SELECT count(*) FROM document_signatures s WHERE s.document_id = d.id) >= d.required_signatures
       AND NOT EXISTS (
             SELECT 1 FROM document_approval_steps st
              WHERE st.document_id = d.id
                AND NOT EXISTS (SELECT 1 FROM document_signatures s
                                 WHERE s.document_id = st.document_id
                                   AND s.step_order = st.step_order));
END $$;
-- +goose StatementEnd

-- +goose Down

-- Rolling this back DISCARDS each document's pinned chain and requirement — the
-- only record of what a document was actually routed under. Rolling forward again
-- re-derives them from the configuration as it stands then, which is the very
-- retroactive redefinition this migration exists to stop. Down is here so the
-- schema can be unwound on a throwaway database; do not roll it back on one that
-- has routed documents.
ALTER TABLE document_signatures DROP COLUMN IF EXISTS step_order;
DROP TABLE IF EXISTS document_approval_steps;
ALTER TABLE document_records DROP COLUMN IF EXISTS required_signatures;
