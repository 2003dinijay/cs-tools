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

-- Move the GitHub issue link from "case" up to work_item.
--
-- WHY IT MOVES. 000067 put github_issue_number on "case" because a case was
-- the only thing that could carry one. Reading the integration that actually
-- runs -- servicenow-integration's issue_servicenow.yml -- shows an issue
-- becomes a SERVICE REQUEST or an INCIDENT, not a case:
--
--     [CR]: / [ECR]: title  -> Service Request, catalog "Generic Requests"
--     Type/ServiceRequest   -> Service Request, catalog "General Requests"
--     Type/Incident         -> Incident
--
-- Service requests live in service_request and incidents in incident; neither
-- is in "case". So the column was on the one table the integration never
-- writes to.
--
-- ON work_item RATHER THAN ON EACH EXTENSION TABLE. Three types need it now
-- and the four outbound triggers currently join "case" only because that was
-- the only linkable type. One column on the parent table lets them join
-- work_item once, instead of growing a branch and an index per type.
--
-- The data moves with it: "case" rows that carry a number keep it.

BEGIN;

ALTER TABLE work_item ADD COLUMN IF NOT EXISTS github_issue_number INTEGER;

UPDATE work_item wi
SET github_issue_number = c.github_issue_number
FROM "case" c
WHERE c.id = wi.id AND c.github_issue_number IS NOT NULL;

-- Partial, like the one it replaces: most work items have no linked issue, and
-- every lookup asks only about the ones that do.
CREATE INDEX IF NOT EXISTS idx_work_item_github_issue_number
    ON work_item (github_issue_number) WHERE github_issue_number IS NOT NULL;

DROP INDEX IF EXISTS idx_case_github_issue_number;
ALTER TABLE "case" DROP COLUMN IF EXISTS github_issue_number;

COMMIT;
