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

-- runbook: the alert-name to runbook catalog (Operations > Runbooks); a new incident whose subject contains an alert_name gets its runbook as a work note.
CREATE TABLE IF NOT EXISTS runbook (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    alert_name  TEXT        NOT NULL CHECK (btrim(alert_name) <> '' AND char_length(alert_name) <= 1000),
    runbook_url TEXT        CHECK (runbook_url IS NULL OR runbook_url ~* '^https?://'),
    created_on  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by  TEXT        NOT NULL,
    updated_on  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by  TEXT        NOT NULL
);

-- One entry per alert name, ignoring case and surrounding spaces.
CREATE UNIQUE INDEX IF NOT EXISTS runbook_alert_name_key ON runbook (lower(btrim(alert_name)));

COMMENT ON TABLE runbook IS
    'Alert name to runbook link catalog. A NULL runbook_url records an alert known to have no runbook yet.';
