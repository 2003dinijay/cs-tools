-- Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com). All Rights Reserved.
--
-- This software is the property of WSO2 LLC. and its suppliers, if any.
-- Dissemination of any information or reproduction of any material contained
-- herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
-- You may not alter or remove any copyright or other notice from copies of this content.

-- Two junction tables fanned out from change_request.u_deployment_products via
-- expand_list. Each sys_id in the list resolves to exactly one table: if it
-- exists in product it lands in change_request_product; if it exists in
-- product_version it lands in change_request_product_version. A UUID present
-- in neither produces a row error in both mappings (genuine missing data).
CREATE TABLE IF NOT EXISTS change_request_product (
    id                UUID PRIMARY KEY,
    change_request_id UUID NOT NULL REFERENCES change_request(id) ON DELETE CASCADE,
    product_id        UUID NOT NULL REFERENCES product(id) ON DELETE CASCADE,
    UNIQUE (change_request_id, product_id)
);

CREATE INDEX IF NOT EXISTS idx_change_request_product_change_request_id ON change_request_product (change_request_id);
CREATE INDEX IF NOT EXISTS idx_change_request_product_product_id ON change_request_product (product_id);

CREATE TABLE IF NOT EXISTS change_request_product_version (
    id                UUID PRIMARY KEY,
    change_request_id UUID NOT NULL REFERENCES change_request(id) ON DELETE CASCADE,
    product_version_id UUID NOT NULL REFERENCES product_version(id) ON DELETE CASCADE,
    UNIQUE (change_request_id, product_version_id)
);

CREATE INDEX IF NOT EXISTS idx_change_request_product_version_change_request_id ON change_request_product_version (change_request_id);
CREATE INDEX IF NOT EXISTS idx_change_request_product_version_product_version_id ON change_request_product_version (product_version_id);
