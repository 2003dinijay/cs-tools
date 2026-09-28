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

-- Team Schedule: who from CRE and SRE is working, when, and in which
-- escalation tier. There is no ServiceNow equivalent -- this is portal-native
-- data, like announcement_requests -- so these tables are the system of
-- record, not a mirror of one. Not to be confused with schedule /
-- user_schedule / schedule_span (0050), which mirror ServiceNow's own
-- cmn_schedule tables; every table here carries the team_schedule_ prefix so
-- the two can never be mistaken for each other.
--
-- The rota is AUTHORED in one clock (IST, the clock the shift windows are
-- written in) and READ in whatever clock the viewer is in. That is why
-- team_schedule_shift stores minutes-of-day against an authoring_time_zone,
-- while team_schedule_assignment stores the resolved absolute instants: the
-- catalogue stays human-editable, and "who is on duty at 03:14 UTC" stays a
-- plain indexed range query that no client has to re-derive.
--
-- This file is the schema only. The catalogue rows it needs to render
-- anything are in 0143, and the change history in 0144. Requires team.key
-- (0141).
--
-- It replaces the pre-restructure 000088-000107 chain with the state that
-- chain ended in. Safe to re-run, and a no-op on a database that already
-- applied that chain under its old file names.

-- ── A partial run of the old chain ────────────────────────────────────────
-- Before 000088-000104 were renumbered, `make migrate` could apply the first
-- two of them -- which created schedule_zone/_shift/_assignment/_absence under
-- their pre-rename names -- and then stop at the third. Those tables hold
-- only catalogue rows, since no code has ever written to them under those
-- names, so they are dropped and rebuilt below rather than carried forward.
-- Anything that is NOT catalogue -- a single assignment or absence -- means
-- real data, and stops the migration instead of dropping it.
DO $$
BEGIN
    IF to_regclass('team_schedule_shift') IS NULL AND to_regclass('schedule_shift') IS NOT NULL THEN
        IF (to_regclass('schedule_assignment') IS NOT NULL AND EXISTS (SELECT 1 FROM schedule_assignment))
        OR (to_regclass('schedule_absence') IS NOT NULL AND EXISTS (SELECT 1 FROM schedule_absence)) THEN
            RAISE EXCEPTION
                'schedule_assignment / schedule_absence hold rows from a partial Team Schedule install. Nothing has been changed; move or remove those rows before applying this migration.';
        END IF;
        DROP TABLE IF EXISTS schedule_assignment, schedule_absence, schedule_shift, schedule_zone;
        DROP TYPE IF EXISTS schedule_shift_family_enum, schedule_tier_enum, schedule_day_scope_enum,
                            schedule_source_enum, schedule_absence_kind_enum;
    END IF;
END $$;

-- The two EXCLUDE constraints below pair an equality test on a uuid with an
-- overlap test on a range, and a plain GiST opclass has no equality operator
-- for uuid. btree_gist ships with Postgres (contrib); on a managed server it
-- may need allow-listing first (Azure: the azure.extensions parameter).
CREATE EXTENSION IF NOT EXISTS btree_gist;

-- ── Vocabulary ────────────────────────────────────────────────────────────

DO $$ BEGIN
    CREATE TYPE team_schedule_shift_family_enum AS ENUM ('CRE', 'SRE');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- L1/L2/L3 are escalation tiers, not seniority. A shift with tier NULL is
-- ordinary working hours, or a window whose tier is a fact about the person
-- that week rather than about the window.
DO $$ BEGIN
    CREATE TYPE team_schedule_tier_enum AS ENUM ('L1', 'L2', 'L3');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- Weekday and weekend are different shapes of day, not different windows of
-- the same shape: SRE runs three zones on a weekday and two at the weekend.
DO $$ BEGIN
    CREATE TYPE team_schedule_day_scope_enum AS ENUM ('WEEKDAY', 'WEEKEND', 'ANY');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- How a row got there. GENERATED rows came from the round-robin allocator and
-- may be regenerated in bulk; MANUAL and SWAP rows were put there by a lead
-- and must survive any regeneration; IMPORTED came from a rota sheet.
DO $$ BEGIN
    CREATE TYPE team_schedule_source_enum AS ENUM ('GENERATED', 'MANUAL', 'SWAP', 'IMPORTED');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- LEAVE is time off and is never counted against a weekend; ALLOCATION is
-- work done elsewhere; EXCLUDED is off the rota entirely.
DO $$ BEGIN
    CREATE TYPE team_schedule_absence_bucket_enum AS ENUM ('LEAVE', 'ALLOCATION', 'EXCLUDED');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- ── The catalogue ─────────────────────────────────────────────────────────

-- An SRE time zone. TZ1/TZ2/TZ3 on a weekday; at the weekend the day
-- collapses to two, so each weekday zone names the weekend zone that absorbs
-- it (TZ1->TZ1, TZ2->TZ1, TZ3->TZ2). Without that mapping a Saturday's
-- 00:00-06:00 -- Friday's TZ3 crew -- belongs to no weekend zone at all.
CREATE TABLE IF NOT EXISTS team_schedule_zone (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_on          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_on          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by          VARCHAR(255),
    updated_by          VARCHAR(255),
    code                VARCHAR(8) NOT NULL UNIQUE,
    label               VARCHAR(50) NOT NULL,
    weekend_zone_id     UUID REFERENCES team_schedule_zone(id) ON DELETE SET NULL,
    sort_order          SMALLINT NOT NULL DEFAULT 0,
    is_active           BOOLEAN NOT NULL DEFAULT TRUE
);

-- A named window of the working day. Minutes count from midnight in
-- authoring_time_zone; an end past 1440 runs into the next day, so a night
-- block 21:00-06:00 is one row, 1260 -> 1800, not two a reader must stitch.
CREATE TABLE IF NOT EXISTS team_schedule_shift (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_on          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_on          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by          VARCHAR(255),
    updated_by          VARCHAR(255),
    code                VARCHAR(32) NOT NULL UNIQUE,
    label               VARCHAR(100) NOT NULL,
    family              team_schedule_shift_family_enum NOT NULL,
    zone_id             UUID REFERENCES team_schedule_zone(id) ON DELETE RESTRICT,
    tier                team_schedule_tier_enum,
    day_scope           team_schedule_day_scope_enum NOT NULL DEFAULT 'WEEKDAY',
    start_minute        SMALLINT NOT NULL,
    end_minute          SMALLINT NOT NULL,
    authoring_time_zone VARCHAR(64) NOT NULL DEFAULT 'Asia/Colombo',
    is_on_call          BOOLEAN NOT NULL DEFAULT FALSE,
    is_escalation       BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order          SMALLINT NOT NULL DEFAULT 0,
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    crosses_midnight    BOOLEAN GENERATED ALWAYS AS (end_minute > 1440) STORED,
    -- The chip a roster cell draws: a short code and a colour, both distinct
    -- from code and label.
    short_code          VARCHAR(12) NOT NULL,
    colour_token        VARCHAR(24) NOT NULL,
    is_rotation         BOOLEAN NOT NULL DEFAULT TRUE,
    required_headcount  SMALLINT,
    CONSTRAINT team_schedule_shift_start_minute_check
        CHECK (start_minute >= 0 AND start_minute < 1440),
    -- At most one midnight: > 1440 is the next day, and crosses_midnight is
    -- generated from exactly that. A typo like 2000 is refused.
    CONSTRAINT team_schedule_shift_end_minute_check
        CHECK (end_minute > start_minute AND end_minute <= start_minute + 1440),
    -- Only SRE works in time zones, and a window that hosts an escalation
    -- tier must say which zone it covers, or "who is L1 right now" has no
    -- answer.
    CONSTRAINT team_schedule_shift_zone_family_check
        CHECK (zone_id IS NULL OR family = 'SRE'),
    CONSTRAINT team_schedule_shift_escalation_zone_check
        CHECK (NOT is_escalation OR zone_id IS NOT NULL),
    CONSTRAINT team_schedule_shift_required_headcount_check
        CHECK (required_headcount IS NULL OR required_headcount > 0)
);

COMMENT ON COLUMN team_schedule_shift.is_rotation IS
  'TRUE when being on this window is a turn somebody takes; FALSE when it is simply when a team works.';
COMMENT ON COLUMN team_schedule_shift.required_headcount IS
  'Target number of engineers for this window; NULL means no target is defined, which is not the same as zero.';

CREATE INDEX IF NOT EXISTS idx_team_schedule_shift_family  ON team_schedule_shift (family);
CREATE INDEX IF NOT EXISTS idx_team_schedule_shift_zone_id ON team_schedule_shift (zone_id);

-- What takes an engineer out of the rota for whole days. Rows, not an enum:
-- a new kind is an insert a lead can ask for, not an ALTER TYPE and a deploy.
CREATE TABLE IF NOT EXISTS team_schedule_absence_kind (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_on    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_on    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by    VARCHAR(255),
    updated_by    VARCHAR(255),
    code          VARCHAR(32) NOT NULL UNIQUE,
    short_code    VARCHAR(12) NOT NULL,
    label         VARCHAR(100) NOT NULL,
    bucket        team_schedule_absence_bucket_enum NOT NULL,
    colour_token  VARCHAR(24) NOT NULL,
    sort_order    SMALLINT NOT NULL DEFAULT 0,
    -- A retired kind is deactivated, never deleted: absences point at it
    -- ON DELETE RESTRICT, and the catalogue serves active kinds only.
    is_active     BOOLEAN NOT NULL DEFAULT TRUE
);

-- ── The facts ─────────────────────────────────────────────────────────────

-- One engineer, one rota day, one window. Every assignment is stored, not
-- derived: the roster is evidence of who was responsible at a given moment,
-- so a change to the allocator must never rewrite what already happened.
--
-- rota_date is the day the CREW is rostered for, not the calendar date of
-- every hour worked: a Monday 21:00-06:00 block is Monday's even though six
-- of its hours fall on Tuesday.
--
-- starts_at/ends_at are resolved from shift + rota_date + authoring_time_zone
-- at write time. They are all a point-in-time lookup touches, and they stay
-- correct across DST because they are absolute instants.
--
-- zone_id and tier are stored rather than read through shift_id because the
-- catalogue leaves them open in places (which of L1/L2 someone is that week
-- is a fact about the person). Where the shift does fix them, the trigger
-- below holds the assignment to it.
CREATE TABLE IF NOT EXISTS team_schedule_assignment (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_on          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_on          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by          VARCHAR(255),
    updated_by          VARCHAR(255),
    user_id             UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    team_id             UUID REFERENCES team(id) ON DELETE SET NULL,
    team_key            VARCHAR(64) NOT NULL REFERENCES team(key) ON UPDATE CASCADE,
    shift_id            UUID NOT NULL REFERENCES team_schedule_shift(id) ON DELETE RESTRICT,
    zone_id             UUID REFERENCES team_schedule_zone(id) ON DELETE RESTRICT,
    tier                team_schedule_tier_enum,
    rota_date           DATE NOT NULL,
    starts_at           TIMESTAMPTZ NOT NULL,
    ends_at             TIMESTAMPTZ NOT NULL,
    is_on_call          BOOLEAN NOT NULL DEFAULT FALSE,
    source              team_schedule_source_enum NOT NULL DEFAULT 'MANUAL',
    note                TEXT,
    CONSTRAINT team_schedule_assignment_window_check CHECK (ends_at > starts_at),
    CONSTRAINT team_schedule_assignment_unique_slot UNIQUE (user_id, rota_date, shift_id),
    -- team_id is kept beside team_key so an engineer who moves team does not
    -- retroactively change who covered a past shift; the pair must agree.
    CONSTRAINT team_schedule_assignment_team_agrees
        FOREIGN KEY (team_id, team_key) REFERENCES team (id, key) ON UPDATE CASCADE,
    -- Nobody is in two places at once. Over the resolved instants, half-open,
    -- so a window ending 18:00 and one starting 18:00 are a handover.
    CONSTRAINT team_schedule_assignment_no_overlap
        EXCLUDE USING gist (user_id WITH =, tstzrange(starts_at, ends_at, '[)') WITH &&)
);

COMMENT ON COLUMN team_schedule_assignment.team_key IS
    'The team this assignment belongs to, by team.key. team_id is kept alongside it, and the pair is constrained to agree.';

CREATE INDEX IF NOT EXISTS idx_team_schedule_assignment_rota_date
    ON team_schedule_assignment (rota_date);
CREATE INDEX IF NOT EXISTS idx_team_schedule_assignment_user_date
    ON team_schedule_assignment (user_id, rota_date);
CREATE INDEX IF NOT EXISTS idx_team_schedule_assignment_team_date
    ON team_schedule_assignment (team_key, rota_date);
CREATE INDEX IF NOT EXISTS idx_team_schedule_assignment_zone_tier_date
    ON team_schedule_assignment (zone_id, tier, rota_date);
-- "Who is on duty at this instant" -- the question an alert escalation asks.
CREATE INDEX IF NOT EXISTS idx_team_schedule_assignment_window
    ON team_schedule_assignment USING GIST (tstzrange(starts_at, ends_at, '[)'));

-- Where the shift fixes a zone or a tier, the assignment must match it;
-- where it leaves one open, the assignment may fill it in. A CHECK cannot
-- reach another table, so this is a trigger.
CREATE OR REPLACE FUNCTION team_schedule_assignment_matches_shift()
RETURNS TRIGGER AS $$
DECLARE
    shift_zone UUID;
    shift_tier team_schedule_tier_enum;
    shift_code TEXT;
BEGIN
    SELECT s.zone_id, s.tier, s.code INTO shift_zone, shift_tier, shift_code
      FROM team_schedule_shift s WHERE s.id = NEW.shift_id;

    IF shift_zone IS NOT NULL AND NEW.zone_id IS DISTINCT FROM shift_zone THEN
        RAISE EXCEPTION
            'assignment zone does not match shift % (shift fixes zone %, assignment says %)',
            shift_code, shift_zone, NEW.zone_id
            USING ERRCODE = 'check_violation';
    END IF;

    IF shift_tier IS NOT NULL AND NEW.tier IS DISTINCT FROM shift_tier THEN
        RAISE EXCEPTION
            'assignment tier does not match shift % (shift fixes tier %, assignment says %)',
            shift_code, shift_tier, NEW.tier
            USING ERRCODE = 'check_violation';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS team_schedule_assignment_matches_shift_trigger ON team_schedule_assignment;
CREATE TRIGGER team_schedule_assignment_matches_shift_trigger
    BEFORE INSERT OR UPDATE OF shift_id, zone_id, tier ON team_schedule_assignment
    FOR EACH ROW EXECUTE FUNCTION team_schedule_assignment_matches_shift();

-- Whole days an engineer is not available to the rota: leave, or time given
-- to R&D or to a customer. A range, because that is how it is granted -- one
-- row for "12-19 March", not eight. ends_on NULL is open-ended ("allocated
-- until further notice"). A range may span a weekend; which days inside it
-- count is a service-layer rule.
CREATE TABLE IF NOT EXISTS team_schedule_absence (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_on          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_on          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by          VARCHAR(255),
    updated_by          VARCHAR(255),
    user_id             UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    team_key            VARCHAR(64) NOT NULL REFERENCES team(key) ON UPDATE CASCADE,
    starts_on           DATE NOT NULL,
    ends_on             DATE,
    note                TEXT,
    approved_by         VARCHAR(255),
    approved_on         TIMESTAMPTZ,
    kind_id             UUID NOT NULL REFERENCES team_schedule_absence_kind(id) ON DELETE RESTRICT,
    allocated_to        VARCHAR(100),
    CONSTRAINT team_schedule_absence_range_check CHECK (ends_on IS NULL OR ends_on >= starts_on),
    -- Not away twice for two reasons. Closed at both ends: a leave day is a
    -- whole day, so 1st-3rd and 3rd-5th is a real clash.
    CONSTRAINT team_schedule_absence_no_overlap
        EXCLUDE USING gist (user_id WITH =, daterange(starts_on, ends_on, '[]') WITH &&)
);

COMMENT ON COLUMN team_schedule_absence.team_key IS
    'The team this absence belongs to, by team.key. No team_id column on purpose: nothing joins an absence to a team row, and the foreign key on this column is what integrity the id would have added.';
COMMENT ON COLUMN team_schedule_absence.allocated_to IS
  'Who an allocation is for: the customer for a customer allocation, the product team for RnD. NULL when not known or not an allocation.';

CREATE INDEX IF NOT EXISTS idx_team_schedule_absence_user
    ON team_schedule_absence (user_id, starts_on, ends_on);
CREATE INDEX IF NOT EXISTS idx_team_schedule_absence_team
    ON team_schedule_absence (team_key, starts_on);
CREATE INDEX IF NOT EXISTS idx_team_schedule_absence_kind_id
    ON team_schedule_absence (kind_id);
CREATE INDEX IF NOT EXISTS idx_team_schedule_absence_span
    ON team_schedule_absence USING GIST (daterange(starts_on, ends_on, '[]'));

-- ── What leads changed ────────────────────────────────────────────────────
-- Append-only, one row per change, read by the portal's "Recent changes"
-- panel. Deliberately no foreign keys: a delete is the change most worth
-- keeping, and a cascade would erase exactly that record. Everything a reader
-- needs is denormalised onto the row so it survives the row it describes.

CREATE TABLE IF NOT EXISTS team_schedule_assignment_activity (
    id              UUID PRIMARY KEY,
    created_on      TIMESTAMPTZ NOT NULL,
    created_by      VARCHAR(255) NOT NULL,
    assignment_id   UUID NOT NULL,
    user_id         UUID NOT NULL,
    team_key        VARCHAR(100) NOT NULL,
    rota_date       DATE NOT NULL,
    shift_code      VARCHAR(100) NOT NULL,
    action          VARCHAR(20) NOT NULL,
    field_name      VARCHAR(255),
    old_value       VARCHAR(255),
    new_value       VARCHAR(255),
    -- The person who made the change, not the person the row is about.
    actor_email     VARCHAR(255) NOT NULL,
    note            TEXT,
    CONSTRAINT team_schedule_assignment_activity_action_check
        CHECK (action IN ('CREATED', 'UPDATED', 'DELETED'))
);

CREATE INDEX IF NOT EXISTS idx_team_schedule_assignment_activity_assignment
    ON team_schedule_assignment_activity (assignment_id, created_on DESC);
CREATE INDEX IF NOT EXISTS idx_team_schedule_assignment_activity_user_date
    ON team_schedule_assignment_activity (user_id, rota_date);
CREATE INDEX IF NOT EXISTS idx_team_schedule_assignment_activity_team_date
    ON team_schedule_assignment_activity (team_key, rota_date);
CREATE INDEX IF NOT EXISTS idx_team_schedule_assignment_activity_date
    ON team_schedule_assignment_activity (rota_date, created_on DESC);

CREATE TABLE IF NOT EXISTS team_schedule_absence_activity (
    id              UUID PRIMARY KEY,
    created_on      TIMESTAMPTZ NOT NULL,
    created_by      VARCHAR(255) NOT NULL,
    absence_id      UUID NOT NULL,
    user_id         UUID NOT NULL,
    team_key        VARCHAR(100) NOT NULL,
    kind_code       VARCHAR(64) NOT NULL,
    starts_on       DATE NOT NULL,
    ends_on         DATE,
    -- TRIMMED: part of a span was cleared, so the absence was shortened
    -- rather than removed. Recording it as a delete would say the leave was
    -- cancelled when most of it still stands.
    action          VARCHAR(20) NOT NULL,
    field_name      VARCHAR(255),
    old_value       VARCHAR(255),
    new_value       VARCHAR(255),
    actor_email     VARCHAR(255) NOT NULL,
    note            TEXT,
    CONSTRAINT team_schedule_absence_activity_action_check
        CHECK (action IN ('CREATED', 'UPDATED', 'DELETED', 'TRIMMED'))
);

CREATE INDEX IF NOT EXISTS idx_team_schedule_absence_activity_absence
    ON team_schedule_absence_activity (absence_id, created_on DESC);
CREATE INDEX IF NOT EXISTS idx_team_schedule_absence_activity_user
    ON team_schedule_absence_activity (user_id, starts_on);
CREATE INDEX IF NOT EXISTS idx_team_schedule_absence_activity_team
    ON team_schedule_absence_activity (team_key, starts_on);
