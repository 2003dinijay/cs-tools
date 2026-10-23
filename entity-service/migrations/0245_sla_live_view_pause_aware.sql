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

-- sla_live (migration 0204) computed a source='CSM' row's live elapsed
-- percentage/remaining duration as flat wall-clock time since start_on,
-- with no account at all for time the clock spent paused -- documented
-- there as a deliberate carry-over of RecomputeActive's own crudeness.
-- That was only half-true: it wasn't a choice to ignore pause time, it was
-- that nothing ever wrote pause_duration for a source='CSM' row in the
-- first place (SLAEngineRepository.SetPaused only ever touched paused_on,
-- the table's own pause_duration/business_pause_duration columns from
-- migration 0048 sat unused) -- there was no accumulated value for the view
-- to subtract even if it wanted to.
--
-- Now that SetPaused accumulates pause_duration on every resume (see that
-- function's own doc comment), the view can -- and should -- actually use
-- it: "elapsed working time" for an IN_PROGRESS/BREACHED row is wall-clock
-- time since start_on MINUS every completed pause segment so far. A clock
-- currently paused has no live branch at all here (stage is 'PAUSED', not
-- 'IN_PROGRESS'/'BREACHED'), so its own in-progress pause isn't double
-- counted -- only pause_duration's already-closed segments are subtracted.
--
-- business_pause_duration (a business-hours-aware pause total) is left out
-- of this subtraction on purpose: this engine has no business-hours
-- calendar at all (see RecomputeActive's own doc comment), so there is no
-- correct business-hours value to subtract with either -- subtracting the
-- wall-clock pause_duration from a wall-clock elapsed time is the one
-- internally-consistent combination available without inventing a
-- calendar this engine doesn't have.
BEGIN;
SET LOCAL lock_timeout = '5s';

CREATE OR REPLACE VIEW sla_live AS
SELECT s.*,
    CASE
        WHEN s.stage IN ('IN_PROGRESS', 'BREACHED') AND s.start_on IS NOT NULL AND s.duration IS NOT NULL
        THEN GREATEST(0,
                EXTRACT(EPOCH FROM (NOW() - s.start_on - COALESCE(s.pause_duration, INTERVAL '0')))
                / NULLIF(EXTRACT(EPOCH FROM s.duration), 0) * 100
             )
        ELSE s.business_elapsed_percentage
    END AS live_elapsed_percentage,
    CASE
        WHEN s.stage IN ('IN_PROGRESS', 'BREACHED') AND s.start_on IS NOT NULL
        THEN GREATEST(NOW() - s.start_on - COALESCE(s.pause_duration, INTERVAL '0'), INTERVAL '0')
        ELSE s.business_duration
    END AS live_business_duration,
    CASE
        WHEN s.stage IN ('IN_PROGRESS', 'BREACHED') AND s.start_on IS NOT NULL AND s.duration IS NOT NULL
        THEN GREATEST(s.duration - (NOW() - s.start_on - COALESCE(s.pause_duration, INTERVAL '0')), INTERVAL '0')
        ELSE s.remaining_business_duration
    END AS live_remaining_business_duration,
    CASE
        WHEN s.stage = 'IN_PROGRESS' AND s.start_on IS NOT NULL AND s.duration IS NOT NULL
             AND EXTRACT(EPOCH FROM (NOW() - s.start_on - COALESCE(s.pause_duration, INTERVAL '0'))) >= EXTRACT(EPOCH FROM s.duration)
        THEN TRUE
        ELSE COALESCE(s.has_breached, FALSE)
    END AS live_has_breached,
    CASE
        WHEN s.stage = 'IN_PROGRESS' AND s.start_on IS NOT NULL AND s.duration IS NOT NULL
             AND EXTRACT(EPOCH FROM (NOW() - s.start_on - COALESCE(s.pause_duration, INTERVAL '0'))) >= EXTRACT(EPOCH FROM s.duration)
        THEN 'BREACHED'::sla_stage_enum
        ELSE s.stage
    END AS live_stage
FROM sla s;

COMMIT;
