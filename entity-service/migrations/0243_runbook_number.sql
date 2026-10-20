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

-- A runbook is known by a number (RB0000001, the RB series of migration 0180) instead of a name; existing runbooks are numbered oldest first.
CREATE SEQUENCE IF NOT EXISTS rb_number_seq START 1;

INSERT INTO number_series (series, digits, sequence_name) VALUES ('RB', 7, 'rb_number_seq')
ON CONFLICT (series) DO NOTHING;

ALTER TABLE runbook ADD COLUMN IF NOT EXISTS number TEXT;

DO $$
DECLARE
    r RECORD;
BEGIN
    FOR r IN SELECT id FROM runbook WHERE number IS NULL ORDER BY created_on, id LOOP
        UPDATE runbook SET number = next_series_number('RB') WHERE id = r.id;
    END LOOP;
END $$;

ALTER TABLE runbook ALTER COLUMN number SET DEFAULT next_series_number('RB');
ALTER TABLE runbook ALTER COLUMN number SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS runbook_number_key ON runbook (number);

DROP INDEX IF EXISTS runbook_name_key;
ALTER TABLE runbook DROP CONSTRAINT IF EXISTS runbook_name_check;
ALTER TABLE runbook DROP COLUMN IF EXISTS name;

COMMENT ON TABLE runbook IS
    'Runbook catalog: a numbered runbook link (NULL when not written yet) for the alert names in runbook_alert.';
