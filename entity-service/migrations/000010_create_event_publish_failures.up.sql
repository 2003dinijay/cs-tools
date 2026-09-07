CREATE TABLE IF NOT EXISTS event_publish_failures (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  event_type    TEXT NOT NULL,
  entity_id     TEXT NOT NULL,
  payload       JSONB NOT NULL,
  error         TEXT NOT NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  resolved_at   TIMESTAMPTZ
);

-- Index to optimize the common "show me the unresolved backlog" query —
-- resolved_at IS NULL, ordered the same way Search's query is (created_at
-- DESC, id) so Postgres can satisfy that ordering directly from the index
-- instead of sorting equal-timestamp rows itself.

CREATE INDEX IF NOT EXISTS idx_event_publish_failures_unresolved
  ON event_publish_failures(created_at DESC, id)
  WHERE resolved_at IS NULL;
