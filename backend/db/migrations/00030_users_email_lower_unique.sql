-- Sign-in looks an address up case-insensitively; the database has to agree.
--
-- handleLogin matches on `lower(u.email) = $1` so that somebody who capitalises
-- the first letter of their own address can still sign in. The only index on
-- users.email is the plain UNIQUE one, and a btree on email cannot answer a
-- query about lower(email): every sign-in attempt became a sequential scan of
-- the whole users table, in front of a bcrypt comparison.
--
-- The uniqueness was the other half of the same mismatch. UNIQUE(email) is
-- case-sensitive, so Bat@example.mn and bat@example.mn were two accounts, while
-- the lookup that finds them is case-insensitive and takes LIMIT 1 — which of
-- the two you signed into depended on membership order.
--
-- +goose Up

-- Fold the addresses that can be folded without colliding with an existing one.
-- +goose StatementBegin
UPDATE users u
   SET email = lower(u.email)
 WHERE u.email <> lower(u.email)
   AND NOT EXISTS (
       SELECT 1 FROM users other
        WHERE other.id <> u.id
          AND lower(other.email) = lower(u.email));
-- +goose StatementEnd

-- What is left is two real accounts whose addresses differ only in case. That
-- is a data question with a person behind it — which of the two is the account,
-- and what happens to the other one's documents — so this file does not invent
-- a rule for whose account survives.
--
-- It does not refuse to apply either, which is the shape of the deployment it
-- has to survive rather than a view about how much the collision matters. The
-- production rollout removes the API and frontend containers and then runs the
-- migrations; a migration that exits non-zero there does not stop a bad change,
-- it leaves the site down. So the index is created either way — the sequential
-- scan on every sign-in is fixed unconditionally — and only the uniqueness half
-- waits for a person, named in a warning the migration output carries.
-- +goose StatementBegin
DO $index$
DECLARE collisions TEXT;
BEGIN
    BEGIN
        CREATE UNIQUE INDEX users_email_lower_key ON users (lower(email));
        RETURN;
    EXCEPTION
        WHEN duplicate_table THEN
            RETURN; -- already applied
        WHEN unique_violation THEN
            NULL;   -- fall through to the warning below
    END;

    SELECT string_agg(duplicated.address, ', ')
      INTO collisions
      FROM (
          SELECT lower(email) AS address
            FROM users
           GROUP BY lower(email)
          HAVING count(*) > 1
      ) AS duplicated;

    RAISE WARNING
        'e-mail uniqueness is still case-sensitive: these addresses exist more than once, differing only in case: %',
        collisions
    USING HINT = 'Merge or rename those accounts, then: CREATE UNIQUE INDEX users_email_lower_key ON users (lower(email)); DROP INDEX users_email_lower_idx;';

    -- Not unique, but it is the index sign-in needs, which is the half of this
    -- that is about whether the platform works rather than about the data.
    CREATE INDEX IF NOT EXISTS users_email_lower_idx ON users (lower(email));
END
$index$;
-- +goose StatementEnd

-- +goose Down
DROP INDEX IF EXISTS users_email_lower_key;
DROP INDEX IF EXISTS users_email_lower_idx;
