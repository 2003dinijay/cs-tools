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

-- Which customer groups belong to which customer project.
--
-- A change request's Customer Group (change_request.customer_group_id) names
-- the customer-side people who approve / review the change (their
-- team_member.group_id members get Approve / Reject on the Customer Approval
-- and Customer Review stages). Until now it was a free choice among every
-- "group" row, so a change request of customer A's project could be pointed at
-- customer B's group and ask B's people to approve it. A customer group is only
-- meaningful within the project(s) it serves, so the data model gets an explicit
-- association and the change request rules (change_request_links.go) require
-- the chosen group to be associated with the chosen project.
--
-- Nothing in the existing schema models that association: "project_group"
-- (migration 0029) is the portal's access-level vocabulary (Full Access, Admin,
-- Security Only, ...) for project contacts, not a set of customer approver
-- groups; project.assignment_group_id / deployment.*_group_id /
-- account.cre_team_id / account.sre_team_id are the INTERNAL support teams. So
-- this is a new, minimal many-to-many link: a project may have several customer
-- groups (e.g. a production and a non-production approver group), and the same
-- group may serve more than one project of the same customer.
--
-- Population. No ServiceNow table feeds this yet: the SN example is project
-- "Pekin - Managed Cloud Subscription" with customer group "PEKINPROD_customer",
-- which is a sys_user_group the customer's contacts belong to, tied to the
-- project on the ServiceNow side. The csm-sync mapping that mirrors that tie
-- (or an operator, until it exists) inserts one row per (project, group) pair.
-- The migration deliberately backfills nothing: inferring pairs from existing
-- change requests would turn the very mismatches this table exists to prevent
-- into "valid" associations. Until a project has rows here no customer group
-- can be chosen for its change requests and the manual Customer Approval /
-- Customer Review paths apply.
--
-- Rows are removed with the project or the group (ON DELETE CASCADE: an
-- association to a record that no longer exists has no meaning).
--
-- FORCE ROW LEVEL SECURITY, same shape as migration 0191's link tables: an
-- internal caller sees every row, a project member the rows of their own
-- project (that is what the form's lookup and the write-time check need).
-- Writing is internal-only (the sync / an operator); the change request code
-- only ever reads this table.
--
-- Idempotent: IF NOT EXISTS / DROP POLICY IF EXISTS throughout.

CREATE TABLE IF NOT EXISTS project_customer_group (
    project_id UUID NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    group_id UUID NOT NULL REFERENCES "group"(id) ON DELETE CASCADE,
    created_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (project_id, group_id)
);
CREATE INDEX IF NOT EXISTS idx_project_customer_group_group_id
    ON project_customer_group (group_id);

ALTER TABLE project_customer_group ENABLE ROW LEVEL SECURITY;
ALTER TABLE project_customer_group FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS project_customer_group_visibility ON project_customer_group;
CREATE POLICY project_customer_group_visibility ON project_customer_group
    FOR SELECT USING (
        (SELECT current_setting('app.is_internal', true) = 'true')
        OR is_project_member(project_customer_group.project_id)
    );
DROP POLICY IF EXISTS project_customer_group_write_internal_only ON project_customer_group;
CREATE POLICY project_customer_group_write_internal_only ON project_customer_group
    FOR INSERT WITH CHECK (current_setting('app.is_internal', true) = 'true');
DROP POLICY IF EXISTS project_customer_group_delete_internal_only ON project_customer_group;
CREATE POLICY project_customer_group_delete_internal_only ON project_customer_group
    FOR DELETE USING (current_setting('app.is_internal', true) = 'true');
