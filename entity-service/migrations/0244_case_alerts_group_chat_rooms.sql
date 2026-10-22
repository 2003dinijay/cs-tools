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

-- Case Alerts: a chat space for an SRE assignment group.
--
-- A paging_chat_room (0233) belonged to a ladder (its shared room) or to a
-- team. The Alert Settings tab now gives each SRE assignment group its own
-- webhook, so a group's incident card and its rung replies go to that group's
-- space while its calls still go to the team the group is routed to. group_id
-- names the "group" row; such a room has no team, teamless flag or channel.
--
-- The room scope index now includes group_id, so one room per group can sit
-- beside the ladder's shared room (team_key NULL as well). A room is deleted
-- with its group: a room for a group that no longer exists reaches nobody.
--
-- Additive and re-runnable: IF NOT EXISTS throughout, the old index dropped by
-- name, the check dropped and added again. No row is added or rewritten.

BEGIN;
SET LOCAL lock_timeout = '5s';

ALTER TABLE paging_chat_room
    ADD COLUMN IF NOT EXISTS group_id UUID NULL REFERENCES "group"(id) ON DELETE CASCADE;

-- A group's room is an SRE room and nothing but its URL.
ALTER TABLE paging_chat_room DROP CONSTRAINT IF EXISTS paging_chat_room_group_room_check;
ALTER TABLE paging_chat_room ADD CONSTRAINT paging_chat_room_group_room_check
    CHECK (group_id IS NULL OR (ladder = 'SRE' AND team_key IS NULL AND NOT teamless AND channel IS NULL));

-- One shared room per ladder, one room per team, one room per group.
CREATE UNIQUE INDEX IF NOT EXISTS paging_chat_room_scope_group_uq
    ON paging_chat_room (ladder, COALESCE(team_key, ''), COALESCE(group_id::text, ''));
DROP INDEX IF EXISTS paging_chat_room_scope_uq;

COMMIT;
