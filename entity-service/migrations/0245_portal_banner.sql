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

-- portal_banner: the top, maintenance, and announcement banners both portals
-- show. One row is one banner. The two visibility flags replace the copies
-- that used to live in each portal's config.js, which a CDN kept serving
-- after the file changed. Both false keeps the row and shows it nowhere.
--
-- Entity-service owns this table. It is not a ServiceNow sync table, so
-- operations/csm-sync-service does not carry this file.
--
-- starts_on / expires_on follow the schema's _on timestamp names. The API
-- can still call them startsAt / expiresAt. Dismissal stays in the browser.
-- Kind-specific required fields (html, severity) are checked by the service
-- when the write API lands; this file only stores the row.

BEGIN;
SET LOCAL lock_timeout = '5s';

CREATE TABLE IF NOT EXISTS portal_banner (
    id                            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    kind                          TEXT        NOT NULL CHECK (kind IN ('top', 'maintenance', 'announcement')),
    sort_order                    INTEGER     NOT NULL DEFAULT 0 CHECK (sort_order >= 0),
    visible_in_customer_portal    BOOLEAN     NOT NULL DEFAULT FALSE,
    visible_in_csm_portal         BOOLEAN     NOT NULL DEFAULT FALSE,
    closeable                     BOOLEAN     NOT NULL DEFAULT FALSE,
    storage_key                   TEXT        NOT NULL DEFAULT '',
    html                          TEXT        NOT NULL DEFAULT '',
    starts_on                     TIMESTAMPTZ NULL,
    expires_on                    TIMESTAMPTZ NULL,
    severity                      TEXT        NULL CHECK (severity IS NULL OR severity IN ('info', 'warning', 'error', 'success')),
    title                         TEXT        NOT NULL DEFAULT '',
    message                       TEXT        NOT NULL DEFAULT '',
    action_label                  TEXT        NULL,
    action_url                    TEXT        NULL CHECK (action_url IS NULL OR action_url ~* '^https?://'),
    created_on                    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by                    TEXT        NOT NULL,
    updated_on                    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by                    TEXT        NOT NULL
);

COMMENT ON TABLE portal_banner IS
    'Portal banners (top, maintenance, announcement). visible_in_customer_portal and visible_in_csm_portal decide where a row is shown; both false hides it.';

COMMIT;
