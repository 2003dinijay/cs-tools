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

-- Give problems created without a priority ServiceNow's: every new problem
-- there is impact 3 - Low, urgency 3 - Low, priority 5 - Planning (discovery
-- script 63: all 57 problems created in the last 180 days). Postgres-only
-- creates never set one, and dual-write creates dropped the one ServiceNow
-- returned; both now set it at create time, and this fills in the problems
-- created before that.
--
-- Only rows with no priority are touched: a synced problem always carries
-- ServiceNow's own. impact and urgency are filled only on those same rows,
-- and only where empty. Re-running it changes nothing.

UPDATE problem
SET priority = 'PLANNING'::problem_priority_enum,
    impact   = COALESCE(impact, 'LOW'::problem_impact_enum),
    urgency  = COALESCE(urgency, 'LOW'::problem_urgency_enum)
WHERE priority IS NULL;
