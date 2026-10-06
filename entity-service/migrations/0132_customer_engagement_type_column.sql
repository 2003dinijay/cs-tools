-- Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com). All Rights Reserved.
--
-- This software is the property of WSO2 LLC. and its suppliers, if any.
-- Dissemination of any information or reproduction of any material contained
-- herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
-- You may not alter or remove any copyright or other notice from copies of this content.

DO $$ BEGIN
    CREATE TYPE customer_engagement_type_enum AS ENUM (
        'FIREFIGHTING',
        'CONSULTANCY',
        'TRAINING',
        'ENTERPRISE_CSM_TAM',
        'ARCHITECTURE_REVIEW',
        'CUSTOMER_ONBOARDING',
        'QSP'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- Rename from engagement_type_id (raw sys_id) to engagement_type (enum label).
-- Existing rows held sys_ids that cannot cast to enum values; NULL them out so
-- the next field_backfill repopulates correctly via u_type.u_name.
ALTER TABLE customer_engagement
    RENAME COLUMN engagement_type_id TO engagement_type;

ALTER TABLE customer_engagement
    ALTER COLUMN engagement_type TYPE customer_engagement_type_enum
        USING NULL::customer_engagement_type_enum;
