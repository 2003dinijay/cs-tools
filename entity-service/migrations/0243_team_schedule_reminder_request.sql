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

-- Rota reminders asked for by hand ("Send now" on the Team Schedule's Rota
-- Access tab), queued for csm-scheduled-tasks, which has no address of its
-- own to call: its rota_manual_reminder job claims the oldest queued request
-- at each run, sends it with the same code as the automatic reminders and
-- reports how it went.
--
--   kind    'weekday' (the next working day's Morning and Evening) or
--           'weekend' (the coming weekend, sent even if not fully marked)
--   status  QUEUED -> SENDING (claimed) -> SENT or FAILED. A SENDING request
--           not reported within 30 minutes is claimed again.
--   test    "Send a test to me": sent only to requested_by, marked as a
--           test copy, with test calendar event ids; not recorded, so the
--           real reminders are not affected.
--
-- id keys the Team Schedule audit trigger (team_schedule_audit_row), so who
-- asked and what happened is in team_schedule_audit too.
--
-- Additive and safe to re-run: nothing existing is touched.
CREATE TABLE IF NOT EXISTS team_schedule_reminder_request (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    family       VARCHAR(16)  NOT NULL,
    kind         VARCHAR(16)  NOT NULL,
    status       VARCHAR(16)  NOT NULL DEFAULT 'QUEUED',
    test         BOOLEAN      NOT NULL DEFAULT FALSE,
    requested_by VARCHAR(255) NOT NULL,
    requested_on TIMESTAMPTZ  NOT NULL DEFAULT now(),
    claimed_on   TIMESTAMPTZ,
    handled_on   TIMESTAMPTZ,
    result       TEXT,
    created_on   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_on   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    created_by   VARCHAR(255) NOT NULL,
    updated_by   VARCHAR(255) NOT NULL,
    CONSTRAINT team_schedule_reminder_request_family_check CHECK (family IN ('CRE', 'SRE', 'SME')),
    CONSTRAINT team_schedule_reminder_request_kind_check CHECK (kind IN ('weekday', 'weekend')),
    CONSTRAINT team_schedule_reminder_request_status_check CHECK (status IN ('QUEUED', 'SENDING', 'SENT', 'FAILED'))
);

-- For a database that applied this file before test was added (it was not
-- released then): the column, as above.
ALTER TABLE team_schedule_reminder_request ADD COLUMN IF NOT EXISTS test BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS team_schedule_reminder_request_family_on_idx
    ON team_schedule_reminder_request (family, requested_on DESC);

COMMENT ON TABLE team_schedule_reminder_request IS
    'Rota reminders asked for by hand on the Team Schedule Rota Access tab; sent by csm-scheduled-tasks.';

DROP TRIGGER IF EXISTS team_schedule_reminder_request_audit ON team_schedule_reminder_request;
CREATE TRIGGER team_schedule_reminder_request_audit
    AFTER INSERT OR UPDATE OR DELETE ON team_schedule_reminder_request
    FOR EACH ROW EXECUTE FUNCTION team_schedule_audit_row();
