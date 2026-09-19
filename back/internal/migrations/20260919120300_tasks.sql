-- +goose Up
CREATE TABLE tasks (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    parent_task_id UUID REFERENCES tasks (id) ON DELETE CASCADE,

    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',

    quadrant quadrant_enum,
    position INT  NOT NULL,
    color TEXT NOT NULL DEFAULT 'NONE',

    deadline_at       TIMESTAMPTZ,
    deadline_has_time BOOLEAN NOT NULL DEFAULT FALSE,

    status     task_status_enum NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    completed_at           TIMESTAMPTZ,
    xp_awarded             INT,
    quadrant_at_completion quadrant_enum,
    completed_via          completion_source_enum,

    deleted_at TIMESTAMPTZ,

    CONSTRAINT tasks_quadrant_xor_parent
        CHECK ((parent_task_id IS NULL) = (quadrant IS NOT NULL)),

    CONSTRAINT tasks_completion_snapshot CHECK (
        (status = 'COMPLETED') = (completed_at IS NOT NULL)
        AND (completed_at IS NOT NULL) = (xp_awarded IS NOT NULL)
        AND (completed_at IS NOT NULL) = (quadrant_at_completion IS NOT NULL)
        AND (completed_at IS NOT NULL) = (completed_via IS NOT NULL)
    ),

    CONSTRAINT tasks_not_own_parent CHECK (parent_task_id <> id),
    CONSTRAINT tasks_title_len       CHECK (char_length(title) BETWEEN 1 AND 100),
    CONSTRAINT tasks_description_len CHECK (char_length(description) <= 2000),
    CONSTRAINT tasks_color_known CHECK (
        color IN ('NONE', 'CYAN', 'VIOLET', 'AMBER', 'ROSE', 'TEAL', 'SLATE')
    ),

    CONSTRAINT tasks_id_user_key UNIQUE (id, user_id)
);

ALTER TABLE tasks ADD CONSTRAINT tasks_parent_same_user
    FOREIGN KEY (parent_task_id, user_id) REFERENCES tasks (id, user_id) ON DELETE CASCADE;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION tasks_enforce_one_level() RETURNS TRIGGER AS $$
DECLARE
    grandparent  UUID;
    has_children BOOLEAN;
BEGIN
    IF NEW.parent_task_id IS NULL THEN
        RETURN NEW;
    END IF;

    SELECT parent_task_id INTO grandparent FROM tasks WHERE id = NEW.parent_task_id;
    IF grandparent IS NOT NULL THEN
        RAISE EXCEPTION 'NESTING_NOT_ALLOWED: task % already has a parent', NEW.parent_task_id
            USING ERRCODE = 'check_violation';
    END IF;

    SELECT EXISTS (SELECT 1 FROM tasks WHERE parent_task_id = NEW.id) INTO has_children;
    IF has_children THEN
        RAISE EXCEPTION 'NESTING_NOT_ALLOWED: task % has subtasks of its own', NEW.id
            USING ERRCODE = 'check_violation';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER tasks_one_level
    BEFORE INSERT OR UPDATE OF parent_task_id ON tasks
    FOR EACH ROW EXECUTE FUNCTION tasks_enforce_one_level();

CREATE TRIGGER tasks_touch BEFORE UPDATE ON tasks
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

CREATE INDEX tasks_board_idx ON tasks (user_id, quadrant, position)
    WHERE parent_task_id IS NULL AND status = 'ACTIVE' AND deleted_at IS NULL;

CREATE INDEX tasks_subtasks_idx ON tasks (parent_task_id, position)
    WHERE deleted_at IS NULL;

CREATE INDEX tasks_user_status_idx ON tasks (user_id, status) WHERE deleted_at IS NULL;

CREATE INDEX tasks_user_deadline_idx ON tasks (user_id, deadline_at)
    WHERE deadline_at IS NOT NULL AND deleted_at IS NULL;

CREATE INDEX tasks_title_trgm_idx       ON tasks USING gin (title gin_trgm_ops);
CREATE INDEX tasks_description_trgm_idx ON tasks USING gin (description gin_trgm_ops);

-- +goose Down
DROP TRIGGER IF EXISTS tasks_touch ON tasks;
DROP TRIGGER IF EXISTS tasks_one_level ON tasks;
DROP FUNCTION IF EXISTS tasks_enforce_one_level();
DROP TABLE IF EXISTS tasks;
