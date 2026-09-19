-- The account is identified by its username; the address is optional.
--
-- An address is only needed for things that send mail or come from an identity
-- provider — the Google Calendar integration will supply one. Requiring it at
-- registration would make the user invent one to get past the form, and an
-- invented address is worse than none: it looks deliverable.

-- +goose Up
ALTER TABLE users ALTER COLUMN email DROP NOT NULL;

-- Two accounts may both have no address, so the uniqueness rule applies only
-- where there is one. PostgreSQL treats NULLs as distinct in a unique index
-- anyway; saying it in the predicate keeps the intent readable.
DROP INDEX users_email_key;
CREATE UNIQUE INDEX users_email_key ON users (email)
    WHERE deleted_at IS NULL AND email IS NOT NULL;

-- +goose Down
DROP INDEX users_email_key;
CREATE UNIQUE INDEX users_email_key ON users (email) WHERE deleted_at IS NULL;

UPDATE users SET email = id::text || '@placeholder.invalid' WHERE email IS NULL;
ALTER TABLE users ALTER COLUMN email SET NOT NULL;
