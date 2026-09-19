-- Runs once, when the pg_data volume is empty. Migrations belong to back/
-- (SPEC §0); this file only installs what a migration cannot install for
-- itself and what the whole schema then depends on.

-- gen_random_uuid() for primary keys. In PostgreSQL 16 it is built in, but the
-- extension also brings the crypt()/digest() family used for password hashing
-- if the backend chooses to hash in the database rather than in Go.
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Case-insensitive text for username and email (SPEC §3.1): two accounts that
-- differ only in capitalisation are the same account to a human, and a UNIQUE
-- constraint over citext is what enforces that.
CREATE EXTENSION IF NOT EXISTS citext;

-- Trigram indexes for the substring search of SPEC §9 — ILIKE '%term%' over
-- title and description cannot use a btree index, and GIN + pg_trgm is what
-- makes that search survive a real task list.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Query statistics. Requires shared_preload_libraries, which compose sets on
-- the postgres command; without that line this CREATE fails.
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
