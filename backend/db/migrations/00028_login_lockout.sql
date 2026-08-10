-- +goose Up
ALTER TABLE users ADD COLUMN IF NOT EXISTS failed_login_attempts INTEGER NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS locked_until TIMESTAMPTZ;
ALTER TABLE users ADD CONSTRAINT users_failed_login_attempts_nonnegative CHECK (failed_login_attempts >= 0);
CREATE INDEX IF NOT EXISTS idx_users_locked_until ON users(locked_until) WHERE locked_until IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_users_locked_until;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_failed_login_attempts_nonnegative;
ALTER TABLE users DROP COLUMN IF EXISTS locked_until;
ALTER TABLE users DROP COLUMN IF EXISTS failed_login_attempts;
