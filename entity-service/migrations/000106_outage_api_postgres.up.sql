-- Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com). All Rights Reserved.
--
-- This software is the property of WSO2 LLC. and its suppliers, if any.
-- Dissemination of any information or reproduction of any material contained
-- herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
-- You may not alter or remove any copyright or other notice from copies of this content.

-- Everything the Postgres outage API needs that the mirror does not already
-- carry. Both statements were applied by hand to the dev database first; this
-- is what carries them to staging and production, where the API 404s without
-- them.

BEGIN;

-- The communication journal was built for the public status page and holds
-- only external entries. The API has three channels and isPublic is derived
-- from which one, so the channel has to be stored rather than assumed.
--
-- The existing rows came from u_external_outage_communications, so 'external'
-- is their true value and not a placeholder default.
ALTER TABLE outage_communication
  ADD COLUMN IF NOT EXISTS channel VARCHAR(20) NOT NULL DEFAULT 'external';

DO $$ BEGIN
  ALTER TABLE outage_communication
    ADD CONSTRAINT outage_communication_channel_chk
    CHECK (channel IN ('external','internal','additional'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- Numbers for outages created natively, as OUT + seven zero-padded digits.
--
-- *** IT STARTS AT 10000 ON PURPOSE. *** ServiceNow is still allocating
-- outage numbers while dual-write is on, and had reached OUT0001887. Starting
-- at 1 would collide immediately; starting at 10000 leaves ServiceNow room to
-- reach 9999 and keeps the number the same width either side of the cutover.
CREATE SEQUENCE IF NOT EXISTS outage_number_seq START WITH 10000;

COMMIT;
