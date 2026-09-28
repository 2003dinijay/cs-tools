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

DROP POLICY IF EXISTS time_card_approver_delete ON time_card_approver;
DROP POLICY IF EXISTS time_card_approver_write ON time_card_approver;
DROP POLICY IF EXISTS time_card_approver_visibility ON time_card_approver;
ALTER TABLE time_card_approver NO FORCE ROW LEVEL SECURITY;
ALTER TABLE time_card_approver DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS time_card_delete ON time_card;
DROP POLICY IF EXISTS time_card_update ON time_card;
DROP POLICY IF EXISTS time_card_write ON time_card;
DROP POLICY IF EXISTS time_card_visibility ON time_card;
ALTER TABLE time_card NO FORCE ROW LEVEL SECURITY;
ALTER TABLE time_card DISABLE ROW LEVEL SECURITY;
