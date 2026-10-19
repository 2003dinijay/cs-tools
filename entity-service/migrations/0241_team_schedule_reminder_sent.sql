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

-- Team Schedule reminders: what csm-scheduled-tasks last sent for one rota
-- period, so a run can tell "already sent" from "not yet" and cancel a
-- calendar invite for somebody taken off the rota since.
--
-- One row per (family, kind, period_start). kind is the reminder, e.g.
-- 'weekend_notice'; period_start the first rota day it covers (a weekend's
-- Saturday). invites is the last set of calendar events sent:
-- [{"uid": "...", "sequence": 3, "attendees": ["a@wso2.com", ...]}].
--
-- Additive and safe to re-run: nothing existing is touched.
CREATE TABLE IF NOT EXISTS team_schedule_reminder_sent (
    family        VARCHAR(16)  NOT NULL,
    kind          VARCHAR(32)  NOT NULL,
    period_start  DATE         NOT NULL,
    first_sent_on TIMESTAMPTZ  NOT NULL DEFAULT now(),
    last_sent_on  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    send_count    INTEGER      NOT NULL DEFAULT 1,
    invites       JSONB        NOT NULL DEFAULT '[]'::jsonb,
    sent_by       VARCHAR(255) NOT NULL,
    PRIMARY KEY (family, kind, period_start),
    CONSTRAINT team_schedule_reminder_sent_family_check CHECK (family IN ('CRE', 'SRE', 'SME')),
    CONSTRAINT team_schedule_reminder_sent_kind_check CHECK (kind ~ '^[a-z_]{1,32}$')
);
