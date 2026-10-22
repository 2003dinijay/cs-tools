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

-- portal_banner has no project, so is_project_member does not apply. A
-- customer may read only the rows marked visible in the customer portal.
-- An internal caller may read every row (the Settings page lists hidden
-- ones too). Writes are internal-only here; the service still requires a
-- CSM admin role before it inserts or updates.
--
-- Runs after 0245_portal_banner.sql because the RLS track always follows
-- the 4-digit schema files. DROP POLICY IF EXISTS makes a re-run a no-op.
-- Every command has a policy: FORCE with a missing command refuses that
-- command for every caller, internal ones included.

BEGIN;

ALTER TABLE portal_banner ENABLE ROW LEVEL SECURITY;
ALTER TABLE portal_banner FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS portal_banner_select ON portal_banner;
CREATE POLICY portal_banner_select ON portal_banner
  FOR SELECT USING (
    current_setting('app.is_internal', true) = 'true'
    OR visible_in_customer_portal
  );

DROP POLICY IF EXISTS portal_banner_insert ON portal_banner;
CREATE POLICY portal_banner_insert ON portal_banner
  FOR INSERT WITH CHECK (current_setting('app.is_internal', true) = 'true');

DROP POLICY IF EXISTS portal_banner_update ON portal_banner;
CREATE POLICY portal_banner_update ON portal_banner
  FOR UPDATE USING (current_setting('app.is_internal', true) = 'true')
  WITH CHECK (current_setting('app.is_internal', true) = 'true');

DROP POLICY IF EXISTS portal_banner_delete ON portal_banner;
CREATE POLICY portal_banner_delete ON portal_banner
  FOR DELETE USING (current_setting('app.is_internal', true) = 'true');

COMMIT;
