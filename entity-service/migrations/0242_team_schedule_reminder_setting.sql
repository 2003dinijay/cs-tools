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

-- Rota reminder settings, one row per rota family, saved from the Team
-- Schedule's Rota Access tab by a CSM admin or the family's rota admin and
-- read by csm-scheduled-tasks at the start of every reminder run. A family
-- with no row keeps the task's own configuration.
--
--   weekday_enabled / weekend_enabled  whether those reminders are sent
--   invites_enabled                    whether they carry calendar invitations
--   weekend_required                   {"CRE_WEEKEND": 3, ...} people each
--                                      weekend shift needs on each day
--   windows                            {"CRE_WEEKEND_NIGHT": {"start": "08:00",
--                                      "end": "18:00", "timeZone":
--                                      "America/Los_Angeles"}}: a rotation's
--                                      hours on its own clock
--   copy_to                            addresses copied on every reminder
--
-- id is there for the Team Schedule audit trigger (team_schedule_audit_row),
-- which keys its rows by it, so every change is in team_schedule_audit.
--
-- Additive and safe to re-run: nothing existing is touched.
CREATE TABLE IF NOT EXISTS team_schedule_reminder_setting (
    id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    family           VARCHAR(16)  NOT NULL,
    weekday_enabled  BOOLEAN      NOT NULL DEFAULT TRUE,
    weekend_enabled  BOOLEAN      NOT NULL DEFAULT TRUE,
    invites_enabled  BOOLEAN      NOT NULL DEFAULT TRUE,
    weekend_required JSONB        NOT NULL DEFAULT '{}'::jsonb,
    windows          JSONB        NOT NULL DEFAULT '{}'::jsonb,
    copy_to          TEXT[]       NOT NULL DEFAULT '{}',
    created_on       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_on       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    created_by       VARCHAR(255) NOT NULL,
    updated_by       VARCHAR(255) NOT NULL,
    CONSTRAINT team_schedule_reminder_setting_family_uq UNIQUE (family),
    CONSTRAINT team_schedule_reminder_setting_family_check CHECK (family IN ('CRE', 'SRE', 'SME')),
    CONSTRAINT team_schedule_reminder_setting_required_check CHECK (jsonb_typeof(weekend_required) = 'object'),
    CONSTRAINT team_schedule_reminder_setting_windows_check CHECK (jsonb_typeof(windows) = 'object')
);

COMMENT ON TABLE team_schedule_reminder_setting IS
    'Rota reminder settings per family, set on the Team Schedule Rota Access tab; read by csm-scheduled-tasks.';

DROP TRIGGER IF EXISTS team_schedule_reminder_setting_audit ON team_schedule_reminder_setting;
CREATE TRIGGER team_schedule_reminder_setting_audit
    AFTER INSERT OR UPDATE OR DELETE ON team_schedule_reminder_setting
    FOR EACH ROW EXECUTE FUNCTION team_schedule_audit_row();
