DROP TRIGGER IF EXISTS outage_affected_ci_outbox_update ON outage_affected_ci;
DROP TRIGGER IF EXISTS outage_affected_ci_outbox ON outage_affected_ci;
DROP TRIGGER IF EXISTS outage_outbox_insert ON outage;
DROP TRIGGER IF EXISTS outage_outbox ON outage;
DROP FUNCTION IF EXISTS trg_event_outbox_insert();
