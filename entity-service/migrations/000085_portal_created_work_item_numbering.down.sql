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

DROP FUNCTION IF EXISTS next_portal_wso2_id(UUID);
-- portal_wso2_id_counter is intentionally NOT dropped: it is each project's
-- current allocation position. If portal-created work items already exist,
-- dropping it and letting the up migration recreate it at 0 would let the
-- next create reuse a wso2_id an existing item already has -- Postgres has
-- no way to recover a dropped counter's value, so the only safe rollback is
-- to leave it exactly where it is.

DROP FUNCTION IF EXISTS next_portal_work_item_number();
-- portal_work_item_number_seq is intentionally NOT dropped either -- same
-- allocation-state-loss reasoning as portal_wso2_id_counter above.

-- Restore the two vestigial sequences this migration's .up dropped, at the
-- same value they were left at (63), so this migration reverses cleanly.
CREATE SEQUENCE IF NOT EXISTS cases_number_seq START 63;
CREATE SEQUENCE IF NOT EXISTS cases_wso2_id_seq START 63;
