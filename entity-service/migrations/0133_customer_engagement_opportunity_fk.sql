-- Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com). All Rights Reserved.
--
-- This software is the property of WSO2 LLC. and its suppliers, if any.
-- Dissemination of any information or reproduction of any material contained
-- herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
-- You may not alter or remove any copyright or other notice from copies of this content.

-- opportunity_id was VARCHAR(32) holding a raw sys_id; convert to a typed UUID
-- FK referencing sf_opportunity. Existing values cannot be cast (wrong format);
-- NULL them so the next field_backfill repopulates via sysid_to_uuid.
ALTER TABLE customer_engagement
    ALTER COLUMN opportunity_id TYPE UUID
        USING NULL::UUID;

ALTER TABLE customer_engagement
    ADD CONSTRAINT fk_customer_engagement_opportunity
        FOREIGN KEY (opportunity_id) REFERENCES sf_opportunity(id) ON DELETE SET NULL;
