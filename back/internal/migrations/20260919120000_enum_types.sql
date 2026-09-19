-- +goose Up
CREATE TYPE quadrant_enum AS ENUM (
    'IMPORTANT_URGENT',
    'IMPORTANT_NOT_URGENT',
    'NOT_IMPORTANT_URGENT',
    'NOT_IMPORTANT_NOT_URGENT'
);

CREATE TYPE task_status_enum AS ENUM ('ACTIVE', 'COMPLETED');

CREATE TYPE completion_source_enum AS ENUM ('DIRECT', 'PARENT_CASCADE');

CREATE TYPE language_enum AS ENUM ('EN', 'RU');

CREATE TYPE plan_enum AS ENUM ('FREE', 'PRO', 'SELF_HOSTED');

CREATE TYPE link_type_enum AS ENUM ('RELATED', 'CONNECTED_TO', 'BLOCKS', 'DEPENDS_ON');

CREATE TYPE xp_source_enum AS ENUM (
    'TASK_COMPLETED',
    'SUBTASK_COMPLETED',
    'ACHIEVEMENT_REWARD',
    'TASK_REOPENED',
    'ADMIN_ADJUSTMENT'
);

CREATE TYPE activity_event_type_enum AS ENUM (
    'TASK_CREATED',
    'TASK_COMPLETED',
    'SUBTASK_COMPLETED',
    'TASK_LINKED',
    'LEVEL_UP',
    'ACHIEVEMENT_UNLOCKED',
    'STREAK_EXTENDED',
    'GRAPH_OPENED'
);

CREATE TYPE achievement_category_enum AS ENUM (
    'TASKS', 'XP', 'LEVEL', 'STREAK', 'LINKS', 'EXPLORATION'
);

CREATE TYPE notification_type_enum AS ENUM (
    'DEADLINE_APPROACHING',
    'TASK_OVERDUE',
    'ACHIEVEMENT_UNLOCKED',
    'LEVEL_UP',
    'STREAK_EXTENDED'
);

-- +goose Down
DROP TYPE IF EXISTS notification_type_enum;
DROP TYPE IF EXISTS achievement_category_enum;
DROP TYPE IF EXISTS activity_event_type_enum;
DROP TYPE IF EXISTS xp_source_enum;
DROP TYPE IF EXISTS link_type_enum;
DROP TYPE IF EXISTS plan_enum;
DROP TYPE IF EXISTS language_enum;
DROP TYPE IF EXISTS completion_source_enum;
DROP TYPE IF EXISTS task_status_enum;
DROP TYPE IF EXISTS quadrant_enum;
