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

DROP POLICY IF EXISTS case_escalation_notification_list_delete_internal_only ON case_escalation_notification_list;
DROP POLICY IF EXISTS case_escalation_notification_list_update_internal_only ON case_escalation_notification_list;
DROP POLICY IF EXISTS case_escalation_notification_list_write_internal_only ON case_escalation_notification_list;
DROP POLICY IF EXISTS case_escalation_notification_list_visibility ON case_escalation_notification_list;
ALTER TABLE case_escalation_notification_list NO FORCE ROW LEVEL SECURITY;
ALTER TABLE case_escalation_notification_list DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS case_escalation_delete_internal_only ON case_escalation;
DROP POLICY IF EXISTS case_escalation_update_internal_only ON case_escalation;
DROP POLICY IF EXISTS case_escalation_write_internal_only ON case_escalation;
DROP POLICY IF EXISTS case_escalation_visibility ON case_escalation;
ALTER TABLE case_escalation NO FORCE ROW LEVEL SECURITY;
ALTER TABLE case_escalation DISABLE ROW LEVEL SECURITY;

DROP FUNCTION IF EXISTS is_project_member(UUID);
