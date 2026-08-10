-- +goose Up

-- status carried two meanings at once, and they contradicted each other.
--
-- It was the administrator's intent — is this connector switched on — and it
-- was also the last delivery outcome, because a failure wrote 'ERROR' into it.
-- Every selection predicate reads the first meaning: webhook dispatch takes
-- WHERE status = 'ACTIVE', and file export and meeting booking refuse anything
-- that is not ACTIVE. So one transient failure switched the connector off
-- permanently: a subscriber that answered 503 once was never sent another
-- event, which meant the success that would have cleared the flag could never
-- happen. Nothing short of an administrator re-saving the row brought it back.
--
-- status is now intent alone. Whether a connector is healthy is last_error,
-- which is already recorded, already returned by the API and already shown on
-- the settings screen.
UPDATE integrations SET status = 'ACTIVE' WHERE status = 'ERROR';

ALTER TABLE integrations DROP CONSTRAINT IF EXISTS integrations_status_known;
ALTER TABLE integrations
    ADD CONSTRAINT integrations_status_known CHECK (status IN ('ACTIVE', 'INACTIVE'));

-- +goose Down
ALTER TABLE integrations DROP CONSTRAINT IF EXISTS integrations_status_known;
ALTER TABLE integrations
    ADD CONSTRAINT integrations_status_known CHECK (status IN ('ACTIVE', 'INACTIVE', 'ERROR'));
