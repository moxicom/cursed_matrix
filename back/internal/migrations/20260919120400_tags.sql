-- +goose Up
CREATE TABLE tags (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    color      TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT tags_name_len CHECK (char_length(name) BETWEEN 1 AND 24),
    CONSTRAINT tags_id_user_key UNIQUE (id, user_id)
);

CREATE UNIQUE INDEX tags_user_name_key ON tags (user_id, lower(name));

CREATE TABLE task_tags (
    task_id    UUID NOT NULL,
    tag_id     UUID NOT NULL,
    user_id    UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (task_id, tag_id),
    FOREIGN KEY (task_id, user_id) REFERENCES tasks (id, user_id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id, user_id)  REFERENCES tags  (id, user_id) ON DELETE CASCADE
);

CREATE INDEX task_tags_tag_idx ON task_tags (tag_id);

-- +goose Down
DROP TABLE IF EXISTS task_tags;
DROP TABLE IF EXISTS tags;
