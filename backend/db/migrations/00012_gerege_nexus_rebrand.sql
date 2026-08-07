-- +goose Up

-- Gerege Nexus rebrand for the AI prompts shipped as defaults in 00005.
--
-- The old wording is not only in the source: 00005 inserted it into ai_prompts,
-- so every database created before this migration serves "You are Gerege ERP AI
-- Copilot" to users regardless of what the Go defaults now say. Editing 00005
-- would fix nothing for those databases, because goose will not re-run an
-- applied version. The rebrand therefore has to travel as its own migration.
--
-- The predicate is deliberately narrow. It matches only rows that are BOTH
-- global (tenant_id IS NULL — a tenant's own prompt always carries its id) AND
-- still byte-identical to the default 00005 shipped. An operator who edited the
-- global prompt, or any tenant who customised theirs, keeps their text: the
-- content comparison fails and the row is skipped. That is the intended
-- behaviour, not a limitation — we are correcting our own default, not
-- overwriting someone's configuration.
--
-- The safety clauses are carried over verbatim in meaning: answer only from
-- approved tools, never invent database values, never cross the tenant
-- boundary. Only the product name and the "ERP operations/data" framing change,
-- since the platform is no longer positioned as an ERP.

UPDATE ai_prompts
   SET content = 'You are Gerege Nexus AI Copilot. Answer only about platform operations and the information returned by approved tools. Never invent database values or expose another tenant''s data.',
       updated_at = NOW()
 WHERE tenant_id IS NULL
   AND prompt_key = 'scope'
   AND content = 'You are Gerege ERP AI Copilot. Answer only about ERP operations and the information returned by approved tools. Never invent database values or expose another tenant''s data.';

UPDATE ai_prompts
   SET content = 'Be concise and practical. Reply in the requested language. Use tools whenever a question depends on live platform data.',
       updated_at = NOW()
 WHERE tenant_id IS NULL
   AND prompt_key = 'instructions'
   AND content = 'Be concise and practical. Reply in the requested language. Use tools whenever a question depends on live ERP data.';

-- +goose Down

-- Symmetrical, and just as narrow: restore the 00005 wording only where the row
-- still holds exactly what Up wrote. Anything edited after the rebrand is left
-- alone, so rolling back cannot discard newer work.

UPDATE ai_prompts
   SET content = 'You are Gerege ERP AI Copilot. Answer only about ERP operations and the information returned by approved tools. Never invent database values or expose another tenant''s data.',
       updated_at = NOW()
 WHERE tenant_id IS NULL
   AND prompt_key = 'scope'
   AND content = 'You are Gerege Nexus AI Copilot. Answer only about platform operations and the information returned by approved tools. Never invent database values or expose another tenant''s data.';

UPDATE ai_prompts
   SET content = 'Be concise and practical. Reply in the requested language. Use tools whenever a question depends on live ERP data.',
       updated_at = NOW()
 WHERE tenant_id IS NULL
   AND prompt_key = 'instructions'
   AND content = 'Be concise and practical. Reply in the requested language. Use tools whenever a question depends on live platform data.';
