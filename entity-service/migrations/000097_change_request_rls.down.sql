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

DROP POLICY IF EXISTS approval_stage_approver_delete_internal_only ON approval_stage_approver;
DROP POLICY IF EXISTS approval_stage_approver_write_internal_only ON approval_stage_approver;
DROP POLICY IF EXISTS approval_stage_approver_update ON approval_stage_approver;
DROP POLICY IF EXISTS approval_stage_approver_visibility ON approval_stage_approver;
ALTER TABLE approval_stage_approver NO FORCE ROW LEVEL SECURITY;
ALTER TABLE approval_stage_approver DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS approval_stage_delete_internal_only ON approval_stage;
DROP POLICY IF EXISTS approval_stage_update_internal_only ON approval_stage;
DROP POLICY IF EXISTS approval_stage_write_internal_only ON approval_stage;
DROP POLICY IF EXISTS approval_stage_visibility ON approval_stage;
ALTER TABLE approval_stage NO FORCE ROW LEVEL SECURITY;
ALTER TABLE approval_stage DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS change_request_write_internal_only ON change_request;
DROP POLICY IF EXISTS change_request_update ON change_request;
DROP POLICY IF EXISTS change_request_visibility ON change_request;
ALTER TABLE change_request NO FORCE ROW LEVEL SECURITY;
ALTER TABLE change_request DISABLE ROW LEVEL SECURITY;
