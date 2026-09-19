-- Deleting a completed task withdraws its XP, and the ledger has to say why.
--
-- Reusing TASK_REOPENED would record a reopening that never happened, and
-- ADMIN_ADJUSTMENT would claim a human intervened. Both would mislead an
-- audit of where a user's XP went.

-- +goose NO TRANSACTION
-- +goose Up
ALTER TYPE xp_source_enum ADD VALUE IF NOT EXISTS 'TASK_DELETED';

-- +goose Down
-- PostgreSQL cannot remove a value from an enum type; the value is left in
-- place, which is harmless: nothing writes it once this migration is rolled
-- back.
SELECT 1;
