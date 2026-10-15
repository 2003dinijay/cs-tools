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

-- Team Schedule: U2 and B-Central's SME rotations move to Day 06:00-18:00 and
-- Night 18:00-06:00 (LK), from the 10:00-22:00 / 22:00-10:00 that 0217 gave
-- them.
--
-- Data only: no table, column, type or constraint is created or changed.
--
-- A window's hours cannot change once a turn uses it (the used-shift guard,
-- 0153): every turn holds the instants it was resolved to, and the roster is
-- the record of who was on at the time. So, as that guard asks:
--
--   * four new windows with the new hours, in the same zones, with the same
--     labels, short codes and colours, so the roster and the picker read as
--     before;
--   * from a clean cutover -- the day it runs on (LK date), so the new hours
--     are what the roster and the day view show that same day -- every
--     U2 / B-Central turn moves to the new window of its zone, with its
--     starts_at / ends_at resolved again from the new hours (the instants
--     Case Paging reads). Turns before the cutover keep the hours they were
--     worked; the one night that crosses it (the previous night's 22:00, to
--     10:00 on the cutover day) is shortened to end at 06:00, when the cutover
--     day's new day turn starts, so the cover runs on with no gap and no
--     overlap. Moving turn by
--     turn instead cannot work: the same engineers hold day and night, so one
--     turn kept on the old hours collides with the next, all the way down;
--   * a moved turn that would still overlap another turn of the same engineer
--     (on another rota) is left on the old window and named in a NOTICE,
--     rather than failing the whole migration on the no-overlap rule; a lead
--     re-marks it;
--   * the old windows are retired (is_active FALSE): no longer offered, and
--     still served to nothing new. Past turns on them keep showing their tier.
--
-- Times are minutes from midnight in the windows' authoring time zone
-- (Asia/Colombo); an end past 1440 runs into the next day.
--
-- Re-runnable: the windows are added only when their code is new; once the old
-- ones are retired nothing new is written against them, so a second run moves
-- nothing and retires nothing. One transaction with a short lock timeout, as
-- the other schedule migrations.

BEGIN;
SET LOCAL lock_timeout = '5s';

-- ── The new windows ───────────────────────────────────────────────────────
INSERT INTO team_schedule_shift
    (code, label, family, zone_id, tier, day_scope, start_minute, end_minute, authoring_time_zone,
     is_on_call, is_escalation, is_rotation, required_headcount, shared_across_teams,
     short_code, colour_token, sort_order, is_active, created_by, updated_by)
SELECT v.new_code, o.label, o.family, o.zone_id, o.tier, o.day_scope, v.start_minute, v.end_minute, o.authoring_time_zone,
       o.is_on_call, o.is_escalation, o.is_rotation, o.required_headcount, o.shared_across_teams,
       o.short_code, o.colour_token, o.sort_order, TRUE, 'migration', 'migration'
  FROM (VALUES
    ('SME_U2_DAY',    'SME_U2_DAY_06_18',     360, 1080),
    ('SME_U2_NIGHT',  'SME_U2_NIGHT_18_06',  1080, 1800),
    ('SME_BCN_DAY',   'SME_BCN_DAY_06_18',    360, 1080),
    ('SME_BCN_NIGHT', 'SME_BCN_NIGHT_18_06', 1080, 1800)
  ) AS v(old_code, new_code, start_minute, end_minute)
  JOIN team_schedule_shift o ON o.code = v.old_code
ON CONFLICT (code) DO NOTHING;

-- ── From the cutover: onto the new window, instants resolved again ────────
CREATE TEMP TABLE _sme_move ON COMMIT DROP AS
SELECT a.id, a.user_id, n.id AS new_shift_id,
       (a.rota_date::timestamp + make_interval(mins => n.start_minute)) AT TIME ZONE n.authoring_time_zone AS new_starts,
       (a.rota_date::timestamp + make_interval(mins => n.end_minute))   AT TIME ZONE n.authoring_time_zone AS new_ends
  FROM team_schedule_assignment a
  JOIN team_schedule_shift o ON o.id = a.shift_id
  JOIN (VALUES
    ('SME_U2_DAY',    'SME_U2_DAY_06_18'),
    ('SME_U2_NIGHT',  'SME_U2_NIGHT_18_06'),
    ('SME_BCN_DAY',   'SME_BCN_DAY_06_18'),
    ('SME_BCN_NIGHT', 'SME_BCN_NIGHT_18_06')
  ) AS v(old_code, new_code) ON v.old_code = o.code
  JOIN team_schedule_shift n ON n.code = v.new_code
 WHERE a.rota_date >= (now() AT TIME ZONE 'Asia/Colombo')::date;

-- The night that crosses the cutover ends when the new day begins.
UPDATE team_schedule_assignment a
   SET ends_at = ((a.rota_date + 1)::timestamp + make_interval(mins => d.start_minute)) AT TIME ZONE d.authoring_time_zone,
       updated_on = NOW(), updated_by = 'migration'
  FROM team_schedule_shift o, team_schedule_shift d
 WHERE o.id = a.shift_id
   AND (o.code, d.code) IN (('SME_U2_NIGHT', 'SME_U2_DAY_06_18'), ('SME_BCN_NIGHT', 'SME_BCN_DAY_06_18'))
   AND a.rota_date = (now() AT TIME ZONE 'Asia/Colombo')::date - 1
   AND a.ends_at > ((a.rota_date + 1)::timestamp + make_interval(mins => d.start_minute)) AT TIME ZONE d.authoring_time_zone;

-- A move that would overlap a turn of the same engineer staying where it is
-- stays too -- and then may itself be what a later move overlaps (tonight's
-- night kept on the old hours runs to 10:00, past tomorrow's 06:00 day), so
-- this repeats until no move overlaps anything that stays.
CREATE TEMP TABLE _sme_clash (id UUID PRIMARY KEY) ON COMMIT DROP;

DO $$
DECLARE n INT; r RECORD;
BEGIN
    LOOP
        WITH kept AS (
            DELETE FROM _sme_move m
             WHERE EXISTS (
                SELECT 1 FROM team_schedule_assignment b
                 WHERE b.user_id = m.user_id AND b.is_rotation AND b.id <> m.id
                   AND NOT EXISTS (SELECT 1 FROM _sme_move o WHERE o.id = b.id)
                   AND tstzrange(b.starts_at, b.ends_at, '[)') && tstzrange(m.new_starts, m.new_ends, '[)'))
            RETURNING m.id)
        INSERT INTO _sme_clash SELECT id FROM kept;
        GET DIAGNOSTICS n = ROW_COUNT;
        EXIT WHEN n = 0;
    END LOOP;
    FOR r IN
        SELECT u.email, a.rota_date, s.code
          FROM _sme_clash c
          JOIN team_schedule_assignment a ON a.id = c.id
          JOIN team_schedule_shift s ON s.id = a.shift_id
          JOIN "user" u ON u.id = a.user_id
         ORDER BY a.rota_date, s.code
    LOOP
        RAISE NOTICE '0236: % on % (%) left on the old hours: the new ones overlap another of their turns', r.email, r.rota_date, r.code;
    END LOOP;
END
$$;

-- In two steps. The no-overlap rule is checked row by row, and the same
-- engineers hold day and night: a day moved to 06:00 before the night ahead of
-- it has moved off 22:00-10:00 overlaps it for a moment. So every moving turn
-- is first parked in a minute of its own, far ahead where nothing else is, and
-- only then given its new window and hours, which abut.
UPDATE team_schedule_assignment a
   SET starts_at = timestamptz '2999-01-01 00:00+00' + m.rn * interval '2 minutes',
       ends_at   = timestamptz '2999-01-01 00:01+00' + m.rn * interval '2 minutes'
  FROM (SELECT id, row_number() OVER (ORDER BY id) AS rn FROM _sme_move) m
 WHERE a.id = m.id;

UPDATE team_schedule_assignment a
   SET shift_id = m.new_shift_id, starts_at = m.new_starts, ends_at = m.new_ends,
       updated_on = NOW(), updated_by = 'migration'
  FROM _sme_move m
 WHERE a.id = m.id;

-- ── The old windows: no longer offered ────────────────────────────────────
UPDATE team_schedule_shift
   SET is_active = FALSE, updated_on = NOW(), updated_by = 'migration'
 WHERE code IN ('SME_U2_DAY', 'SME_U2_NIGHT', 'SME_BCN_DAY', 'SME_BCN_NIGHT')
   AND is_active
   AND EXISTS (SELECT 1 FROM team_schedule_shift n
                WHERE n.code IN ('SME_U2_DAY_06_18', 'SME_U2_NIGHT_18_06', 'SME_BCN_DAY_06_18', 'SME_BCN_NIGHT_18_06'));

COMMIT;
