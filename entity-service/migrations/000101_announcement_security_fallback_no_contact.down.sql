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

DROP POLICY IF EXISTS announcement_visibility ON announcement;

-- Restores the pre-fallback policy shape this database actually had before
-- this migration (confirmed live via pg_policies), not migration 000085's
-- own version -- that one's announcement_is_security(id, type) function
-- was never actually applied here, so reverting to a call to it would not
-- restore a state this database was ever really in.
CREATE POLICY announcement_visibility ON announcement
  FOR SELECT
  USING (
    current_setting('app.is_internal', true) = 'true'
    OR EXISTS (
      SELECT 1
      FROM work_item wi
      JOIN project_contact pc ON pc.project_id = wi.project_id
      JOIN project_contact_group pcg ON pcg.project_contact_id = pc.id
      JOIN project_group_role pgr ON pgr.project_group_id = pcg.project_group_id
      JOIN project_role pr ON pr.id = pgr.project_role_id
      WHERE wi.id = announcement.id
        AND pc.email = current_setting('app.viewer_email', true)
        AND (
          (NOT announcement.is_security_announcement AND pr.role IN ('PORTAL_USER', 'LEAD_USER'))
          OR (announcement.is_security_announcement AND pr.role = 'SECURITY_CONTACT')
        )
    )
  );

DROP FUNCTION IF EXISTS project_has_security_contact(UUID);
DROP FUNCTION IF EXISTS announcement_is_security(UUID, announcement_type_enum);
