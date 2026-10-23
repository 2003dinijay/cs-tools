-- Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
--
-- WSO2 LLC. licenses this file to you under the Apache License,
-- Version 2.0 (the "License"); you may not use this file except
-- in compliance with the License.
-- You may obtain a copy of the License at
--
-- http://www.apache.org/licenses/LICENSE-2.0
--
-- Unless required by applicable law or agreed to in writing,
-- software distributed under the License is distributed on an
-- "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
-- KIND, either express or implied.  See the License for the
-- specific language governing permissions and limitations
-- under the License.

-- Set-based rollups of the usage tables, run inside the database so no usage row
-- travels through the service: hourly_usage_summary -> daily_usage_summary, and
-- daily_usage_summary -> monthly_usage_summary. Both are idempotent: running one
-- again for the same period changes nothing unless its source rows changed.
-- Safe to run again (CREATE OR REPLACE; the one-argument rollup_daily_usage of an earlier
-- draft is dropped first).

-- A counter is a per-period delta a product publishes as the amount since its
-- previous publish, so it adds up over a day and a month. Every other count type is
-- a level (users, organisations, cores, MAU, API count) whose value for a period is
-- its latest reading. Keep in step with isCounterCountType in
-- internal/repository/instance_ingestion_repo.go.
CREATE OR REPLACE FUNCTION usage_count_type_is_counter(p_count_type text)
RETURNS boolean
LANGUAGE sql IMMUTABLE PARALLEL SAFE
AS $$
    SELECT p_count_type IN ('TRANSACTION_COUNT', 'M2M_TOKEN') OR p_count_type LIKE 'AGENT\_%'
$$;

-- Recomputes the API_CALL rows of one UTC day from that day's hourly rows: the sum
-- for a counter (capped at what the INTEGER column holds), the latest report by
-- counted_on for any other type. Returns the number of rows written. FILE_UPLOAD
-- rows are a different source and are never touched.
--
-- It takes the same per-node advisory lock ingestion takes before it reads or writes
-- a node, in byte order (COLLATE "C", the order Go's sort.Strings gives the batch
-- import, whatever the database collation), so a report being ingested while this runs
-- is either included here or recomputed by the ingestion afterwards, and no two
-- transactions take the same pair of node locks in opposite orders.
--
-- p_node_ids limits the run to those node ids (deployment_node.node_id); NULL means
-- every node with hourly rows that day. The service passes chunks of nodes, one
-- transaction each, so a busy day never holds thousands of advisory locks at once
-- (they share the lock table) or blocks every node's ingestion for the whole run.
DROP FUNCTION IF EXISTS rollup_daily_usage(date);
CREATE OR REPLACE FUNCTION rollup_daily_usage(p_day date, p_node_ids text[] DEFAULT NULL)
RETURNS integer
LANGUAGE plpgsql
AS $$
DECLARE
    v_start   timestamptz := p_day::timestamp AT TIME ZONE 'UTC';
    v_end     timestamptz := (p_day + 1)::timestamp AT TIME ZONE 'UTC';
    v_written integer;
BEGIN
    PERFORM pg_advisory_xact_lock(hashtextextended('deployment-node:' || n.node_id, 0::bigint))
    FROM (
        SELECT DISTINCT dn.node_id COLLATE "C" AS node_id
        FROM hourly_usage_summary h
        JOIN deployment_node dn ON dn.id = h.deployment_node_id
        WHERE h.counted_on >= v_start AND h.counted_on < v_end
          AND (p_node_ids IS NULL OR dn.node_id = ANY (p_node_ids))
        ORDER BY 1
    ) n;

    WITH day_rows AS (
        -- One row per node, type and counted_on: a report stored twice (a retry from
        -- before ingestion skipped them, or synced data) counts once.
        SELECT DISTINCT ON (h.deployment_node_id, h.count_type, h.counted_on)
               h.deployment_node_id AS node_uuid, h.count_type, h.count, h.counted_on, h.created_on
        FROM hourly_usage_summary h
        WHERE h.deployment_node_id IS NOT NULL
          AND h.counted_on >= v_start AND h.counted_on < v_end
          AND (p_node_ids IS NULL OR h.deployment_node_id IN (
                SELECT dn.id FROM deployment_node dn WHERE dn.node_id = ANY (p_node_ids)))
        ORDER BY h.deployment_node_id, h.count_type, h.counted_on, h.created_on DESC, h.id
    ), computed AS (
        SELECT node_uuid, count_type, LEAST(SUM(count), 2147483647)::integer AS value
        FROM day_rows
        WHERE usage_count_type_is_counter(count_type)
        GROUP BY node_uuid, count_type
        UNION ALL
        (SELECT DISTINCT ON (node_uuid, count_type) node_uuid, count_type, count AS value
         FROM day_rows
         WHERE NOT usage_count_type_is_counter(count_type)
         ORDER BY node_uuid, count_type, counted_on DESC, created_on DESC)
    ), updated AS (
        UPDATE daily_usage_summary d
        SET value = c.value, updated_by = 'usage-rollup', updated_on = NOW()
        FROM computed c
        WHERE d.deployment_node_id = c.node_uuid
          AND d.summary_date = p_day
          AND d.count_type = c.count_type
          AND d.data_source = 'API_CALL'
          AND d.value IS DISTINCT FROM c.value
        RETURNING d.id
    ), inserted AS (
        INSERT INTO daily_usage_summary (
            id, created_on, updated_on, created_by, updated_by,
            deployment_node_id, deployment_number, summary_date, count_type, value, data_source
        )
        SELECT gen_random_uuid(), NOW(), NOW(), 'usage-rollup', 'usage-rollup',
               c.node_uuid, dn.deployment_number, p_day, c.count_type, c.value, 'API_CALL'
        FROM computed c
        JOIN deployment_node dn ON dn.id = c.node_uuid
        WHERE NOT EXISTS (
            SELECT 1 FROM daily_usage_summary d
            WHERE d.deployment_node_id = c.node_uuid
              AND d.summary_date = p_day
              AND d.count_type = c.count_type
              AND d.data_source = 'API_CALL')
        RETURNING id
    )
    SELECT (SELECT count(*) FROM updated) + (SELECT count(*) FROM inserted) INTO v_written;

    RETURN v_written;
END;
$$;

-- Recomputes one calendar month of monthly_usage_summary from daily_usage_summary:
-- the sum of the days for a counter (capped at what the INTEGER column holds), the
-- latest day's value for any other type. A node-type-day with both a live (API_CALL)
-- row and an uploaded (FILE_UPLOAD) row counts once, the live one; a node that only
-- has uploaded days (an air-gapped product) is included from them. A count type
-- longer than the monthly column holds (50) is skipped. Returns the number of rows
-- written; months other than the one given are never touched.
CREATE OR REPLACE FUNCTION rollup_monthly_usage(p_year integer, p_month integer)
RETURNS integer
LANGUAGE plpgsql
AS $$
DECLARE
    v_first   date := make_date(p_year, p_month, 1);
    v_next    date := (make_date(p_year, p_month, 1) + INTERVAL '1 month')::date;
    v_written integer;
BEGIN
    -- monthly_usage_summary has no unique key, so two runs for the same month would
    -- both pass the NOT EXISTS check and insert twice. Serialise them per month.
    PERFORM pg_advisory_xact_lock(hashtextextended('usage-rollup-month:' || p_year || '-' || p_month, 0));

    WITH per_day AS (
        SELECT DISTINCT ON (d.deployment_node_id, d.count_type, d.summary_date)
               d.deployment_node_id AS node_uuid, d.count_type, d.summary_date, d.value
        FROM daily_usage_summary d
        WHERE d.deployment_node_id IS NOT NULL
          AND d.value IS NOT NULL
          AND length(d.count_type) <= 50
          AND d.summary_date >= v_first AND d.summary_date < v_next
        ORDER BY d.deployment_node_id, d.count_type, d.summary_date,
                 (d.data_source = 'API_CALL') DESC, d.updated_on DESC, d.id
    ), per_node AS (
        SELECT node_uuid, count_type, LEAST(SUM(value), 2147483647)::integer AS value
        FROM per_day
        WHERE usage_count_type_is_counter(count_type)
        GROUP BY node_uuid, count_type
        UNION ALL
        (SELECT DISTINCT ON (node_uuid, count_type) node_uuid, count_type, value
         FROM per_day
         WHERE NOT usage_count_type_is_counter(count_type)
         ORDER BY node_uuid, count_type, summary_date DESC)
    ), computed AS (
        -- monthly_usage_summary is keyed by the node's own id string, which is not
        -- unique in deployment_node: fold nodes sharing one into a single row.
        SELECT dn.node_id,
               p.count_type,
               CASE WHEN usage_count_type_is_counter(p.count_type)
                    THEN LEAST(SUM(p.value::bigint), 2147483647)::integer
                    ELSE MAX(p.value) END AS value
        FROM per_node p
        JOIN deployment_node dn ON dn.id = p.node_uuid
        GROUP BY dn.node_id, p.count_type
    ), updated AS (
        UPDATE monthly_usage_summary m
        SET count = c.value, updated_by = 'usage-rollup', updated_on = NOW()
        FROM computed c
        WHERE m.node_id = c.node_id
          AND m.count_type = c.count_type
          AND m.year = p_year AND m.month = p_month
          AND m.count IS DISTINCT FROM c.value
        RETURNING m.id
    ), inserted AS (
        INSERT INTO monthly_usage_summary (
            id, created_on, updated_on, created_by, updated_by,
            node_id, count_type, count, year, month
        )
        SELECT gen_random_uuid(), NOW(), NOW(), 'usage-rollup', 'usage-rollup',
               c.node_id, c.count_type, c.value, p_year, p_month
        FROM computed c
        WHERE NOT EXISTS (
            SELECT 1 FROM monthly_usage_summary m
            WHERE m.node_id = c.node_id
              AND m.count_type = c.count_type
              AND m.year = p_year AND m.month = p_month)
        RETURNING id
    )
    SELECT (SELECT count(*) FROM updated) + (SELECT count(*) FROM inserted) INTO v_written;

    RETURN v_written;
END;
$$;
