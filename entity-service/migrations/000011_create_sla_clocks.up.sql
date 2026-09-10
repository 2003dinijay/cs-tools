CREATE TABLE IF NOT EXISTS sla_clocks (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  case_id        TEXT NOT NULL,
  clock_type     TEXT NOT NULL,
  started_at     TIMESTAMPTZ NOT NULL,
  due_at         TIMESTAMPTZ NOT NULL,
  paused_at      TIMESTAMPTZ,
  reached_50_at  TIMESTAMPTZ,
  reached_75_at  TIMESTAMPTZ,
  reached_100_at TIMESTAMPTZ,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (case_id, clock_type)
);

-- case_id is a plain TEXT, not a FK to cases(id): a case may be
-- ServiceNow-backed (no local cases row at all), same reasoning as
-- event_publish_failures.entity_id.

CREATE INDEX IF NOT EXISTS idx_sla_clocks_case_id ON sla_clocks(case_id);
