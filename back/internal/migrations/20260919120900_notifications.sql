-- +goose Up
CREATE TABLE notifications (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    type       notification_type_enum NOT NULL,
    payload    JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    read_at    TIMESTAMPTZ
);

CREATE INDEX notifications_user_unread_idx ON notifications (user_id, created_at DESC)
    WHERE read_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS notifications;
