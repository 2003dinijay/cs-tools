-- Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com). All Rights Reserved.
--
-- This software is the property of WSO2 LLC. and its suppliers, if any.
-- Dissemination of any information or reproduction of any material contained
-- herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
-- You may not alter or remove any copyright or other notice from copies of this content.

-- One row per reason a user ticked alongside their rating. option_value and
-- reason are kept as recorded, with no FK to work_item_feedback_metric_option,
-- so a reason survives its option being renamed or removed.
CREATE TABLE IF NOT EXISTS work_item_feedback_reason (
    id UUID PRIMARY KEY,
    feedback_id UUID NOT NULL REFERENCES work_item_feedback(id) ON DELETE CASCADE,
    metric_id UUID NOT NULL REFERENCES work_item_feedback_metric(id),
    option_value INTEGER NOT NULL,
    reason VARCHAR(255) NOT NULL,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_work_item_feedback_reason_feedback_id ON work_item_feedback_reason (feedback_id);
CREATE INDEX IF NOT EXISTS idx_work_item_feedback_reason_metric_id ON work_item_feedback_reason (metric_id);
