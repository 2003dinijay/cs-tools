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

DROP POLICY IF EXISTS problem_deny_all_delete ON problem;
DROP POLICY IF EXISTS problem_deny_all_update ON problem;
DROP POLICY IF EXISTS problem_deny_all_insert ON problem;
DROP POLICY IF EXISTS problem_deny_all_select ON problem;
ALTER TABLE problem NO FORCE ROW LEVEL SECURITY;
ALTER TABLE problem DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS incident_task_deny_all_delete ON incident_task;
DROP POLICY IF EXISTS incident_task_deny_all_update ON incident_task;
DROP POLICY IF EXISTS incident_task_deny_all_insert ON incident_task;
DROP POLICY IF EXISTS incident_task_deny_all_select ON incident_task;
ALTER TABLE incident_task NO FORCE ROW LEVEL SECURITY;
ALTER TABLE incident_task DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS incident_deny_all_delete ON incident;
DROP POLICY IF EXISTS incident_deny_all_update ON incident;
DROP POLICY IF EXISTS incident_deny_all_insert ON incident;
DROP POLICY IF EXISTS incident_deny_all_select ON incident;
ALTER TABLE incident NO FORCE ROW LEVEL SECURITY;
ALTER TABLE incident DISABLE ROW LEVEL SECURITY;
