-- A session has an end but no notion of being left alone.
--
-- expires_at bounds one sign-in to twelve hours whatever happens in them, which
-- is the right ceiling and the wrong floor: an unlocked machine in a shared
-- office keeps a working session for the rest of that window, and the person
-- who walked away from it has no way to say so.
--
-- last_seen_at is what an idle timeout needs. It is not written on every
-- request — that would be a write per read, on the busiest statement in the
-- platform — but at most once a minute per session, which is enough resolution
-- for a timeout measured in tens of minutes.
--
-- +goose Up
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- Sessions already open when this lands have never been touched, so they get
-- their creation time. Anything genuinely idle since then is then already
-- expired by the timeout rather than granted a fresh window by it.
UPDATE sessions SET last_seen_at = created_at WHERE last_seen_at < created_at;

-- The sweep that closes idle sessions reads this and nothing else.
CREATE INDEX IF NOT EXISTS idx_sessions_last_seen ON sessions(last_seen_at)
    WHERE revoked_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_sessions_last_seen;
ALTER TABLE sessions DROP COLUMN IF EXISTS last_seen_at;
