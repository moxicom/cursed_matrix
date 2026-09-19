-- Completing a task that was reopened must be able to earn XP again.
--
-- The original index made (task_id, source) unique, which says a task may be
-- rewarded once ever. That was right while completion was final; with reopen
-- in the product it means a user who reopens something loses its XP for good,
-- because the compensating transaction is recorded and the second completion
-- is then refused by the index.
--
-- grant_seq counts the completions of one task, so each one is rewarded once
-- and no more: the guard against a double grant survives, and an honest
-- re-completion is allowed.

-- +goose Up
ALTER TABLE xp_transactions ADD COLUMN grant_seq INT NOT NULL DEFAULT 1;

ALTER TABLE xp_transactions
    ADD CONSTRAINT xp_transactions_grant_seq_positive CHECK (grant_seq >= 1);

DROP INDEX xp_transactions_grant_once;
CREATE UNIQUE INDEX xp_transactions_grant_once
    ON xp_transactions (task_id, source, grant_seq)
    WHERE source IN ('TASK_COMPLETED', 'SUBTASK_COMPLETED');

-- +goose Down
DROP INDEX xp_transactions_grant_once;

-- Collapsing back to one grant per task would fail on any task rewarded more
-- than once, so the later grants go first.
DELETE FROM xp_transactions WHERE grant_seq > 1;

CREATE UNIQUE INDEX xp_transactions_grant_once
    ON xp_transactions (task_id, source)
    WHERE source IN ('TASK_COMPLETED', 'SUBTASK_COMPLETED');

ALTER TABLE xp_transactions DROP CONSTRAINT xp_transactions_grant_seq_positive;
ALTER TABLE xp_transactions DROP COLUMN grant_seq;
