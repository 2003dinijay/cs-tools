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

-- Where a specialist handoff ("Escalate to specialist team" on an incident)
-- sends the incident, as data rather than code. ServiceNow hard-codes it
-- twice -- in the "Escalate to Special Ops" UI action and in
-- IncidentHandoffUtils' IHU_SERVICE_ROUTING -- so a new service or sub-team
-- there is a code change. Here it is rows, on the Team Schedule's own team
-- model:
--
--   team                      a Special Ops team is a team row: key is the
--                             handoff's escalationTeam (choreo-runtime-team,
--                             ...), name is what the dialog shows
--   team.group_id (new)       the assignment group the team works as -- where
--                             a handed-off incident and its "[Runbook Task]"
--                             go
--   specialist_handoff_route  which teams a service hands off to: exactly one
--                             default per service, plus the sub-teams the
--                             dialog offers; and the GitHub repository the
--                             internal issue is filed in
--
-- The Special Ops teams get type SPECIAL-OPS. The Team Schedule rosters only
-- CRE/SRE-typed teams, and the call-escalation ladders read a team's family
-- from that roster, so these rows change neither: an incident in a Special
-- Ops group pages exactly as before. Rostering them is the notification
-- flow's change to make, not this one's.

ALTER TABLE team ADD COLUMN IF NOT EXISTS group_id UUID REFERENCES "group"(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_team_group_id ON team (group_id);

CREATE TABLE IF NOT EXISTS specialist_handoff_route (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id    UUID NOT NULL REFERENCES service(id) ON DELETE CASCADE,
    team_id       UUID NOT NULL REFERENCES team(id) ON DELETE CASCADE,
    is_default    BOOLEAN NOT NULL DEFAULT FALSE,
    github_owner  VARCHAR(100),
    github_repo   VARCHAR(100),
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_on    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_on    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT specialist_handoff_route_service_team UNIQUE (service_id, team_id)
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_specialist_handoff_route_default
    ON specialist_handoff_route (service_id) WHERE is_default;

-- Today's routing, from IncidentHandoffUtils. Seeded only where the groups
-- and services exist, so a database without them (a fresh test database)
-- still migrates; there the rows are added like any other route.
INSERT INTO team (id, created_on, updated_on, created_by, updated_by, name, type, key, group_id)
SELECT gen_random_uuid(), NOW(), NOW(), 'migration-0194', 'migration-0194', t.name, 'SPECIAL-OPS', t.key, t.group_id
FROM (VALUES
    ('choreo-special-ops',   'Choreo Special Ops',   'fe0d8868-1b0b-3010-d64e-64a2604bcb3c'::uuid),
    ('choreo-runtime-team',  'Choreo Runtime Team',  '80dade5d-1b70-0710-a002-c9d3604bcbd7'::uuid),
    ('choreo-apim-team',     'Choreo APIM Team',     'a79a1e9d-1b70-0710-a002-c9d3604bcb20'::uuid),
    ('asgardeo-special-ops', 'Asgardeo Special Ops', '7fb4f4c6-1b4b-3810-aea4-a936604bcb90'::uuid)
) AS t(key, name, group_id)
WHERE EXISTS (SELECT 1 FROM "group" g WHERE g.id = t.group_id)
ON CONFLICT (key) DO UPDATE SET group_id = COALESCE(team.group_id, EXCLUDED.group_id);

INSERT INTO specialist_handoff_route (service_id, team_id, is_default, github_owner, github_repo)
SELECT r.service_id, tm.id, r.is_default, 'wso2-enterprise', r.repo
FROM (VALUES
    ('b9c999f8-1b86-a010-00ae-86acdd4bcb61'::uuid, 'choreo-special-ops',   TRUE,  'choreo'),
    ('b9c999f8-1b86-a010-00ae-86acdd4bcb61'::uuid, 'choreo-runtime-team',  FALSE, 'choreo'),
    ('b9c999f8-1b86-a010-00ae-86acdd4bcb61'::uuid, 'choreo-apim-team',     FALSE, 'choreo'),
    ('97ed1b8b-1ba2-6c10-00ae-86acdd4bcbd3'::uuid, 'asgardeo-special-ops', TRUE,  'asgardeo-product')
) AS r(service_id, team_key, is_default, repo)
JOIN team tm ON tm.key = r.team_key
WHERE EXISTS (SELECT 1 FROM service s WHERE s.id = r.service_id)
ON CONFLICT (service_id, team_id) DO NOTHING;
