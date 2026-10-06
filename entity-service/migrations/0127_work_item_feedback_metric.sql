-- Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com). All Rights Reserved.
--
-- This software is the property of WSO2 LLC. and its suppliers, if any.
-- Dissemination of any information or reproduction of any material contained
-- herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
-- You may not alter or remove any copyright or other notice from copies of this content.

-- Question definitions of the case feedback form (the rating question, the
-- per-rating "reasons" checkbox questions, the free-text comment question).
-- depends_on_metric_id links a reasons question to the rating question; it is
-- deferrable so a batch holding both parent and child commits in any row order.
-- selected_image / unselected_image are the frontend asset paths of the rating
-- icon in its selected and unselected state; set only on the "<rating> - Reasons"
-- rows (NULL elsewhere), and written by the sync itself from the metric name.
CREATE TABLE IF NOT EXISTS work_item_feedback_metric (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    question TEXT,
    datatype VARCHAR(64) NOT NULL,
    display_order INTEGER,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    is_mandatory BOOLEAN NOT NULL DEFAULT FALSE,
    depends_on_metric_id UUID REFERENCES work_item_feedback_metric(id) ON DELETE SET NULL DEFERRABLE INITIALLY DEFERRED,
    selected_image VARCHAR(255),
    unselected_image VARCHAR(255),
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_work_item_feedback_metric_depends_on_metric_id ON work_item_feedback_metric (depends_on_metric_id);

-- Selectable choices of a checkbox question. value is the number a recorded
-- reason carries as option_value.
CREATE TABLE IF NOT EXISTS work_item_feedback_metric_option (
    id UUID PRIMARY KEY,
    metric_id UUID NOT NULL REFERENCES work_item_feedback_metric(id) ON DELETE CASCADE,
    label VARCHAR(255) NOT NULL,
    value INTEGER NOT NULL,
    display_order INTEGER,
    created_on TIMESTAMPTZ NOT NULL,
    updated_on TIMESTAMPTZ NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    CONSTRAINT uq_work_item_feedback_metric_option_metric_value UNIQUE (metric_id, value)
);
