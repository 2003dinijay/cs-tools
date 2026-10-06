-- Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com). All Rights Reserved.
--
-- This software is the property of WSO2 LLC. and its suppliers, if any.
-- Dissemination of any information or reproduction of any material contained
-- herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
-- You may not alter or remove any copyright or other notice from copies of this content.

DO $$ BEGIN
    CREATE TYPE comment_attachment_state_enum AS ENUM (
        'AVAILABLE', 'AVAILABLE_CONDITIONALLY', 'NOT_AVAILABLE', 'PENDING'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS comment_attachment (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    name VARCHAR(255),
    content_type VARCHAR(255),
    comment_id UUID NOT NULL REFERENCES comment(id) ON DELETE CASCADE,
    size_bytes BIGINT,
    compressed_size_bytes BIGINT,
    is_compressed BOOLEAN,
    hash VARCHAR(100),
    state comment_attachment_state_enum,
    chunk_size_bytes INTEGER
);

CREATE INDEX IF NOT EXISTS idx_comment_attachment_comment_id ON comment_attachment (comment_id);
