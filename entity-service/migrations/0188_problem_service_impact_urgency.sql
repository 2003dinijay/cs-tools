-- Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com). All Rights Reserved.
--
-- This software is the property of WSO2 LLC. and its suppliers, if any.
-- Dissemination of any information or reproduction of any material contained
-- herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
-- You may not alter or remove any copyright or other notice from copies of this content.

-- problem.business_service, impact and urgency, for "[WSO2 Cloud Ops] Post
-- resolution tasks": the problem it creates for an incident resolved with a
-- workaround copies all three from the incident, and Postgres had nowhere to
-- put them. Same choice list as incident (1 - High, 2 - Medium, 3 - Low), but
-- an enum per table, as incident and incident_task already have.
--
-- All nullable with no default: existing problems keep NULL, as ServiceNow
-- leaves a problem nobody filled in.

DO $$ BEGIN
    CREATE TYPE problem_impact_enum AS ENUM ('HIGH', 'MEDIUM', 'LOW');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE problem_urgency_enum AS ENUM ('HIGH', 'MEDIUM', 'LOW');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

ALTER TABLE problem ADD COLUMN IF NOT EXISTS service_id UUID REFERENCES service(id) ON DELETE SET NULL;
ALTER TABLE problem ADD COLUMN IF NOT EXISTS impact problem_impact_enum;
ALTER TABLE problem ADD COLUMN IF NOT EXISTS urgency problem_urgency_enum;

CREATE INDEX IF NOT EXISTS idx_problem_service_id ON problem (service_id);
