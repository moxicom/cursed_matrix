-- Opening the graph is recorded once per day, not once per click.
--
-- The exploration achievements count the distinct days a user went looking, so
-- a row per visit would let someone unlock "open the graph on fourteen days"
-- in fourteen seconds. The index makes the rule the database's, not the
-- caller's; the delete is what lets the index be created at all, and it keeps
-- the earliest visit of each day.

-- +goose Up
DELETE FROM activity_events a
WHERE a.type = 'GRAPH_OPENED'
  AND EXISTS (
      SELECT 1 FROM activity_events b
      WHERE b.type = 'GRAPH_OPENED'
        AND b.user_id = a.user_id
        AND b.local_date = a.local_date
        AND b.occurred_at < a.occurred_at
  );

CREATE UNIQUE INDEX activity_events_graph_day_key
    ON activity_events (user_id, local_date)
    WHERE type = 'GRAPH_OPENED';

-- +goose Down
DROP INDEX IF EXISTS activity_events_graph_day_key;
