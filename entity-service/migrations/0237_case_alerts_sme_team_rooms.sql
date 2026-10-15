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

-- Case Alerts: SME teams' chat rooms (0233). Each SME team's Google Chat
-- space can be saved on the Alert Settings tab, as each SRE team's already
-- can, instead of a GOOGLE_CHAT_SPACES entry in Choreo. Only the room: SME
-- has no shared room, routes or ladder values here.
--
-- paging_chat_room takes ladder SME, and team rooms for SME teams. teamless
-- and channel stay SRE-only (entity-service refuses them on an SME room).
-- Nothing else changes: no row is added, and csm-notification-service still
-- reads no room until PAGING_CONFIG_SOURCE is set.
--
-- Additive and re-runnable: each widened check is dropped by name and added
-- again, so a re-run lands on the same two checks. One transaction with a
-- short lock timeout, as 0233.

BEGIN;
SET LOCAL lock_timeout = '5s';

ALTER TABLE paging_chat_room DROP CONSTRAINT IF EXISTS paging_chat_room_ladder_check;
ALTER TABLE paging_chat_room ADD CONSTRAINT paging_chat_room_ladder_check
    CHECK (ladder IN ('CRE', 'SRE', 'SME'));

ALTER TABLE paging_chat_room DROP CONSTRAINT IF EXISTS paging_chat_room_team_rooms_sre_check;
ALTER TABLE paging_chat_room ADD CONSTRAINT paging_chat_room_team_rooms_sre_check
    CHECK (team_key IS NULL OR ladder IN ('SRE', 'SME'));

-- An SME room is always a team's: SME has no shared room here.
ALTER TABLE paging_chat_room DROP CONSTRAINT IF EXISTS paging_chat_room_sme_team_check;
ALTER TABLE paging_chat_room ADD CONSTRAINT paging_chat_room_sme_team_check
    CHECK (ladder <> 'SME' OR (team_key IS NOT NULL AND NOT teamless AND channel IS NULL));

COMMIT;
