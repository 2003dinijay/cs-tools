-- Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com). All Rights Reserved.
--
-- This software is the property of WSO2 LLC. and its suppliers, if any.
-- Dissemination of any information or reproduction of any material contained
-- herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
-- You may not alter or remove any copyright or other notice from copies of this content.

-- Reversing this loses which channel each journal entry belonged to. That is
-- acceptable only because every row predating the column is external, which
-- is exactly what the up migration's default encodes.

BEGIN;

DROP SEQUENCE IF EXISTS outage_number_seq;

ALTER TABLE outage_communication
  DROP CONSTRAINT IF EXISTS outage_communication_channel_chk;

ALTER TABLE outage_communication
  DROP COLUMN IF EXISTS channel;

COMMIT;
