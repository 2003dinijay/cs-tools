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

-- One row per offline usage report imported (POST /instances/daily-usages/batch), written
-- in the same transaction as the import, so the history and the imported rows can never
-- disagree. The previous system kept the same record (u_product_consumption_import_history:
-- deployment, export version, validity, project, uploader, export time). Safe to run again.
CREATE TABLE IF NOT EXISTS usage_report_import (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_on        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    -- the uploader's email, or the ingesting service when none was given
    created_by        VARCHAR(255) NOT NULL,
    project_key       VARCHAR(128),
    deployment_number VARCHAR(50),
    export_version    VARCHAR(20),
    exported_at       TIMESTAMPTZ,
    -- the product's own integrity verdict on the report, as it exported it
    export_valid      BOOLEAN,
    node_count        INTEGER      NOT NULL,
    imported_count    INTEGER      NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_usage_report_import_project    ON usage_report_import (project_key, created_on DESC);
CREATE INDEX IF NOT EXISTS idx_usage_report_import_deployment ON usage_report_import (deployment_number, created_on DESC);
