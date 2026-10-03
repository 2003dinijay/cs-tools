-- Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com). All Rights Reserved.
--
-- This software is the property of WSO2 LLC. and its suppliers, if any.
-- Dissemination of any information or reproduction of any material contained
-- herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
-- You may not alter or remove any copyright or other notice from copies of this content.

CREATE TABLE IF NOT EXISTS timezone (
    value      VARCHAR(64)  NOT NULL PRIMARY KEY,
    label      VARCHAR(128) NOT NULL,
    utc_offset VARCHAR(16)  NOT NULL,
    dst        BOOLEAN      NOT NULL
);
