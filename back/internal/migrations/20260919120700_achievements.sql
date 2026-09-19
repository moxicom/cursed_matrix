-- +goose Up
CREATE TABLE achievements (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code       TEXT NOT NULL UNIQUE,
    category   achievement_category_enum NOT NULL,
    condition  JSONB NOT NULL,
    reward_xp  INT     NOT NULL DEFAULT 0,
    repeatable BOOLEAN NOT NULL DEFAULT FALSE,
    icon       TEXT,
    sort_order INT NOT NULL DEFAULT 0,

    CONSTRAINT achievements_reward_nonnegative CHECK (reward_xp >= 0)
);

CREATE TABLE user_achievements (
    user_id           UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    achievement_id    UUID NOT NULL REFERENCES achievements (id) ON DELETE CASCADE,
    unlocked_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    progress_snapshot JSONB,

    PRIMARY KEY (user_id, achievement_id)
);

CREATE INDEX user_achievements_user_time_idx ON user_achievements (user_id, unlocked_at DESC);

-- +goose Down
DROP TABLE IF EXISTS user_achievements;
DROP TABLE IF EXISTS achievements;
