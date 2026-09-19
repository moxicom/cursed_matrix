-- +goose Up
CREATE TABLE task_links (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    source_task_id UUID NOT NULL,
    target_task_id UUID NOT NULL,
    type           link_type_enum NOT NULL DEFAULT 'RELATED',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT task_links_no_self CHECK (source_task_id <> target_task_id),

    FOREIGN KEY (source_task_id, user_id) REFERENCES tasks (id, user_id) ON DELETE CASCADE,
    FOREIGN KEY (target_task_id, user_id) REFERENCES tasks (id, user_id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX task_links_directed_key
    ON task_links (source_task_id, target_task_id, type)
    WHERE type IN ('BLOCKS', 'DEPENDS_ON');

CREATE UNIQUE INDEX task_links_undirected_key
    ON task_links (
        LEAST(source_task_id, target_task_id),
        GREATEST(source_task_id, target_task_id),
        type
    )
    WHERE type IN ('RELATED', 'CONNECTED_TO');

CREATE INDEX task_links_source_idx ON task_links (source_task_id);
CREATE INDEX task_links_target_idx ON task_links (target_task_id);
CREATE INDEX task_links_user_idx   ON task_links (user_id);

-- +goose Down
DROP TABLE IF EXISTS task_links;
