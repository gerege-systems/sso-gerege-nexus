-- +goose Up

-- Gerege SSO rebrand for the AI prompt shipped as a default in 00005 and
-- renamed once already in 00012.
--
-- Same reasoning as 00012: the wording lives in ai_prompts, not only in Go, so
-- a database migrated before today still serves "You are Gerege Nexus AI
-- Copilot" no matter what the source now says. Editing 00012 would fix nothing,
-- because goose will not re-run an applied version.
--
-- Only the 'scope' row needs touching. 00012 already rewrote 'instructions' to
-- "live platform data", which carries no product name — there is nothing in it
-- left to rebrand, so it is deliberately left alone rather than rewritten to
-- produce a no-op diff.
--
-- The predicate stays as narrow as 00012's: global (tenant_id IS NULL) AND
-- still byte-identical to what 00012 wrote. A tenant that customised its scope
-- prompt, or an operator who edited the global one, keeps that text. A database
-- that never reached 00012 — because its global row was already customised back
-- then — is skipped here too, which is correct: it was never ours to rewrite.
--
-- The safety clauses are carried over verbatim: answer only from approved
-- tools, never invent database values, never cross the tenant boundary. Only
-- the product name changes.

UPDATE ai_prompts
   SET content = 'You are Gerege SSO AI Copilot. Answer only about platform operations and the information returned by approved tools. Never invent database values or expose another tenant''s data.',
       updated_at = NOW()
 WHERE tenant_id IS NULL
   AND prompt_key = 'scope'
   AND content = 'You are Gerege Nexus AI Copilot. Answer only about platform operations and the information returned by approved tools. Never invent database values or expose another tenant''s data.';

-- +goose Down

-- Symmetrical and equally narrow: restore the 00012 wording only where the row
-- still holds exactly what Up wrote, so a rollback cannot discard newer edits.

UPDATE ai_prompts
   SET content = 'You are Gerege Nexus AI Copilot. Answer only about platform operations and the information returned by approved tools. Never invent database values or expose another tenant''s data.',
       updated_at = NOW()
 WHERE tenant_id IS NULL
   AND prompt_key = 'scope'
   AND content = 'You are Gerege SSO AI Copilot. Answer only about platform operations and the information returned by approved tools. Never invent database values or expose another tenant''s data.';
