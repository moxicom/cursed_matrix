-- +goose Up
CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username      CITEXT NOT NULL,
    email         CITEXT NOT NULL,
    password_hash TEXT   NOT NULL,
    avatar_url    TEXT,

    plan            plan_enum NOT NULL DEFAULT 'FREE',
    plan_expires_at TIMESTAMPTZ,

    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_login_at TIMESTAMPTZ,
    deleted_at    TIMESTAMPTZ,

    CONSTRAINT users_username_len CHECK (char_length(username) BETWEEN 3 AND 32),
    CONSTRAINT users_email_len    CHECK (char_length(email) BETWEEN 3 AND 254)
);

CREATE UNIQUE INDEX users_username_key ON users (username) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX users_email_key    ON users (email)    WHERE deleted_at IS NULL;

ALTER TABLE users ADD CONSTRAINT users_id_self_key UNIQUE (id);

CREATE TRIGGER users_touch BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

CREATE TABLE user_settings (
    user_id               UUID PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    language              language_enum NOT NULL DEFAULT 'EN',
    timezone              TEXT    NOT NULL DEFAULT 'UTC',
    show_in_leaderboard   BOOLEAN NOT NULL DEFAULT FALSE,
    notifications_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT user_settings_timezone_len CHECK (char_length(timezone) BETWEEN 1 AND 64)
);

CREATE TRIGGER user_settings_touch BEFORE UPDATE ON user_settings
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

CREATE TABLE user_stats (
    user_id               UUID PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    lifetime_xp           BIGINT NOT NULL DEFAULT 0,
    level                 INT    NOT NULL DEFAULT 1,
    current_streak        INT    NOT NULL DEFAULT 0,
    longest_streak        INT    NOT NULL DEFAULT 0,
    last_streak_date      DATE,
    tasks_created         INT    NOT NULL DEFAULT 0,
    tasks_completed       INT    NOT NULL DEFAULT 0,
    subtasks_completed    INT    NOT NULL DEFAULT 0,
    links_created         INT    NOT NULL DEFAULT 0,
    achievements_unlocked INT    NOT NULL DEFAULT 0,
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT user_stats_level_positive CHECK (level >= 1),
    CONSTRAINT user_stats_streaks_sane   CHECK (current_streak >= 0 AND longest_streak >= current_streak)
);

CREATE TRIGGER user_stats_touch BEFORE UPDATE ON user_stats
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

-- +goose Down
DROP TABLE IF EXISTS user_stats;
DROP TABLE IF EXISTS user_settings;
DROP TABLE IF EXISTS users;
