-- +goose Up
CREATE TABLE xp_transactions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    task_id    UUID REFERENCES tasks (id) ON DELETE SET NULL,
    amount     INT NOT NULL,
    source     xp_source_enum NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    metadata   JSONB NOT NULL DEFAULT '{}',

    CONSTRAINT xp_transactions_amount_nonzero CHECK (amount <> 0)
);

CREATE UNIQUE INDEX xp_transactions_grant_once
    ON xp_transactions (task_id, source)
    WHERE source IN ('TASK_COMPLETED', 'SUBTASK_COMPLETED');

CREATE INDEX xp_transactions_user_time_idx ON xp_transactions (user_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS xp_transactions;
