-- +goose Up

-- An approval was redeemable for as long as its row survived. expires_at came
-- back from eID, was handed to the caller, and then thrown away: nothing stored
-- it and nothing checked it. The only cleanup ran when somebody started another
-- session for the same document, so a session a citizen approved days ago — its
-- id still sitting in a proxy log or a browser's network panel — could be posted
-- to the poll endpoint and turned into a signature dated today.
ALTER TABLE document_eid_sign_sessions
    ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;

-- Rows that predate the column are left NULL, and nothing is invented for them.
--
-- NULL is not "unknown" here, it is a fact: eID states no deadline for a push
-- session — it decides when one dies and reports EXPIRED — so NULL means nobody
-- has said when this session dies, and the poll bounds those by
-- signSessionBackstop measured from created_at. A pre-column row is therefore
-- bounded exactly as a new one is, without a stored figure of our own.
--
-- An earlier version of this migration wrote created_at + 2 minutes into them.
-- That was the same invented cutoff the sign-in flow had to remove: harmless for
-- rows this old, but re-running the migration set — down to 14 and up again, which
-- an operator may well do — would have stamped a two-minute deadline onto LIVE
-- sessions that eID was still waiting on, and killed them four minutes in.

-- +goose Down
ALTER TABLE document_eid_sign_sessions DROP COLUMN IF EXISTS expires_at;
