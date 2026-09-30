-- The outage COMMUNICATION LOG: one row per email the outage-communication
-- flows have sent. The Go port of ServiceNow's
-- `sn_comm_management_outage_communication`.
--
-- *** THIS IS NOT `outage_communication`. *** That table (migration 000105)
-- is the public JOURNAL -- the external updates the status page renders
-- verbatim. This one is an internal send-log: email bodies, recipient lists,
-- subjects. The names are one character apart and the contents are on
-- opposite sides of a disclosure boundary, so the suffix is load-bearing.
-- Nothing public should ever read this table.
--
-- *** IT IS ALSO THIS PORT'S IDEMPOTENCY KEY, WHICH IS WHY IT IS A
-- FIRST-CLASS TABLE AND NOT A MIRROR. *** ServiceNow's flow is
-- "Run Trigger: Once" -- the platform starts at most one instance per
-- outage, ever, and that instance parks until begin, sends, parks until
-- end, sends again. Nothing is written to the outage record at all (its two
-- Update Outage steps are configured with no fields). So the platform's
-- execution model IS the guard.
--
-- A sweep cannot inherit that: it re-reads every qualifying row on every
-- tick. Asking "is there already a DECLARE row for this outage?" reproduces
-- the guarantee using state the flow was already writing.

CREATE TABLE IF NOT EXISTS outage_communication_log (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- *** JOIN ON THE NUMBER, NOT A FOREIGN KEY. *** ServiceNow's table has
    -- a real `u_outage` reference column and it is empty on all 336 rows;
    -- the link has always been carried by the number string. Mirroring the
    -- reference would produce a column that is null forever, so this stores
    -- what is actually populated.
    outage_number   VARCHAR(32) NOT NULL,

    -- *** FREE TEXT, DELIBERATELY NOT AN ENUM. *** The live data holds six
    -- distinct values -- Update 155, Declared 88, Resolve 72, RCA 13,
    -- Declare 7, and one blank. Two of those are spellings of each other and
    -- 13 come from a flow that is inactive today. A CREATE TYPE would reject
    -- real rows at import. Readers normalise; see email_type_norm below.
    email_type      TEXT,

    -- Normalised for querying, so the idempotency check cannot be defeated
    -- by the "Declare"/"Declared" split. Generated rather than written, so
    -- it can never drift from email_type.
    --
    -- Matching on the PREFIX is the point: `= 'DECLARED'` would miss the 7
    -- rows spelled "Declare" and re-announce those outages at cutover.
    email_type_norm TEXT GENERATED ALWAYS AS (
        CASE
            WHEN email_type IS NULL THEN NULL
            WHEN upper(email_type) LIKE 'DECLARE%' THEN 'DECLARED'
            WHEN upper(email_type) LIKE 'RESOLV%'  THEN 'RESOLVED'
            WHEN upper(email_type) LIKE 'UPDATE%'  THEN 'UPDATE'
            WHEN upper(email_type) LIKE 'RCA%'     THEN 'RCA'
            ELSE upper(email_type)
        END
    ) STORED,

    outage_status   VARCHAR(40),
    subject         VARCHAR(100),
    recipients      VARCHAR(4000),
    email_content   TEXT,
    main_content    VARCHAR(1000),
    sent_on         TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    created_on      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- The sweep's only question: what have we already said about this outage?
CREATE INDEX IF NOT EXISTS idx_outage_communication_log_guard
    ON outage_communication_log (outage_number, email_type_norm);

-- Operational reads are newest-first over a window.
CREATE INDEX IF NOT EXISTS idx_outage_communication_log_sent
    ON outage_communication_log (sent_on DESC);
