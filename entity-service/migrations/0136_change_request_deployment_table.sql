-- Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com). All Rights Reserved.
--
-- This software is the property of WSO2 LLC. and its suppliers, if any.
-- Dissemination of any information or reproduction of any material contained
-- herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
-- You may not alter or remove any copyright or other notice from copies of this content.

-- Junction table fanned out from change_request.u_deployments via expand_list,
-- same shape as work_item_watcher: one row per (change_request, deployment) pair.
-- No DEFAULT on id: expand_list always supplies a deterministic id from Go,
-- matching the work_item_watcher pattern (see 0042_work_item_watcher_table.sql).
CREATE TABLE IF NOT EXISTS change_request_deployment (
    id                UUID PRIMARY KEY,
    change_request_id UUID NOT NULL REFERENCES change_request(id) ON DELETE CASCADE,
    deployment_id     UUID NOT NULL REFERENCES deployment(id) ON DELETE CASCADE,
    UNIQUE (change_request_id, deployment_id)
);

CREATE INDEX IF NOT EXISTS idx_change_request_deployment_change_request_id ON change_request_deployment (change_request_id);
CREATE INDEX IF NOT EXISTS idx_change_request_deployment_deployment_id ON change_request_deployment (deployment_id);
