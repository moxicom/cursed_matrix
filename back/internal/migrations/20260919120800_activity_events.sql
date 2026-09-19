-- +goose Up
CREATE TABLE activity_events (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    type        activity_event_type_enum NOT NULL,
    task_id     UUID REFERENCES tasks (id) ON DELETE SET NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    local_date  DATE NOT NULL,
    metadata    JSONB NOT NULL DEFAULT '{}'
);

CREATE INDEX activity_events_user_day_idx ON activity_events (user_id, local_date);
CREATE INDEX activity_events_user_time_idx ON activity_events (user_id, occurred_at DESC);
CREATE INDEX activity_events_user_type_day_idx ON activity_events (user_id, type, local_date);

-- +goose Down
DROP TABLE IF EXISTS activity_events;
