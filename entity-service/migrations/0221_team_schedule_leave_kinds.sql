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

-- Team Schedule leave tags, as the leads asked for them:
--   * Annual leave is named "Annual/Casual leave" (casual leave is marked with it);
--   * Sabbatical leave is added under Leave;
--   * Sick leave is no longer offered.
--
-- Data only, no schema change. Sick leave is RETIRED (is_active FALSE), not
-- deleted: days already marked with it keep their label on the roster, and
-- new ones can no longer be marked (the absence write only takes an active
-- kind). Safe to re-run: every statement changes nothing once applied.

UPDATE team_schedule_absence_kind
   SET label = 'Annual/Casual leave',
       updated_on = now(),
       updated_by = 'migration:0221_team_schedule_leave_kinds'
 WHERE code = 'ANNUAL_LEAVE'
   AND label IS DISTINCT FROM 'Annual/Casual leave';

UPDATE team_schedule_absence_kind
   SET is_active = FALSE,
       updated_on = now(),
       updated_by = 'migration:0221_team_schedule_leave_kinds'
 WHERE code = 'SICK_LEAVE'
   AND is_active;

-- Built-in like the others (created_by 'migration'), so it is not a custom
-- tag a lead could delete. Sorted after paternity leave, where sick leave sat.
INSERT INTO team_schedule_absence_kind
    (code, short_code, label, bucket, colour_token, sort_order, is_active, family, created_by, updated_by)
VALUES
    ('SABBATICAL_LEAVE', 'SAB', 'Sabbatical leave', 'LEAVE', 'SAB', 23, TRUE, NULL, 'migration', 'migration')
ON CONFLICT (code) DO NOTHING;
