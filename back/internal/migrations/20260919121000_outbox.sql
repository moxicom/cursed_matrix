-- +goose Up
CREATE TABLE outbox_events (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    aggregate_id UUID NOT NULL,
    user_id      UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    type         TEXT NOT NULL,
    payload      JSONB NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ,
    attempts     INT NOT NULL DEFAULT 0,
    last_error   TEXT
);

CREATE INDEX outbox_events_pending_idx ON outbox_events (created_at)
    WHERE processed_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS outbox_events;
