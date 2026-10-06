-- Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com). All Rights Reserved.
--
-- This software is the property of WSO2 LLC. and its suppliers, if any.
-- Dissemination of any information or reproduction of any material contained
-- herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
-- You may not alter or remove any copyright or other notice from copies of this content.

-- u_line_item_id (a plain string identifier) moves to sf_id; u_line_item (the
-- reference field) becomes a typed UUID FK on sf_opportunity_product. Existing
-- line_item_id_ref values were raw sys_ids and cannot be cast; NULL them so
-- the next field_backfill repopulates via sysid_to_uuid.
ALTER TABLE customer_engagement
    RENAME COLUMN line_item_id TO sf_id;

ALTER TABLE customer_engagement
    RENAME COLUMN line_item_id_ref TO line_item_id;

ALTER TABLE customer_engagement
    ALTER COLUMN line_item_id TYPE UUID
        USING NULL::UUID;

ALTER TABLE customer_engagement
    ADD CONSTRAINT fk_customer_engagement_line_item
        FOREIGN KEY (line_item_id) REFERENCES sf_opportunity_product(id) ON DELETE SET NULL;
