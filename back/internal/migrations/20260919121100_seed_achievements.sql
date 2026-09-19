-- +goose Up
INSERT INTO achievements (code, category, condition, reward_xp, sort_order) VALUES
    ('FIRST_BLOOD',     'TASKS',       '{"metric": "TASKS_COMPLETED",    "threshold": 1}',     0, 10),
    ('CENTURION',       'TASKS',       '{"metric": "TASKS_COMPLETED",    "threshold": 100}',   0, 20),
    ('FIREFIGHTER',     'TASKS',       '{"metric": "Q1_TASKS_COMPLETED", "threshold": 25}',    0, 30),
    ('XP_10K',          'XP',          '{"metric": "LIFETIME_XP",        "threshold": 10000}', 0, 40),
    ('ARCHITECT',       'LEVEL',       '{"metric": "LEVEL",              "threshold": 20}',    0, 50),
    ('STREAK_30',       'STREAK',      '{"metric": "LONGEST_STREAK",     "threshold": 30}',    0, 60),
    ('NETWORK_BUILDER', 'LINKS',       '{"metric": "LINKS_CREATED",      "threshold": 50}',    0, 70),
    ('CARTOGRAPHER',    'EXPLORATION', '{"metric": "GRAPH_OPENED_DAYS",  "threshold": 14}',    0, 80);

-- +goose Down
DELETE FROM achievements WHERE code IN (
    'FIRST_BLOOD', 'CENTURION', 'FIREFIGHTER', 'XP_10K',
    'ARCHITECT', 'STREAK_30', 'NETWORK_BUILDER', 'CARTOGRAPHER'
);
