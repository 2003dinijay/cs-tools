-- Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com). All Rights Reserved.
--
-- This software is the property of WSO2 LLC. and its suppliers, if any.
-- Dissemination of any information or reproduction of any material contained
-- herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
-- You may not alter or remove any copyright or other notice from copies of this content.

-- u_work_state and resolution_code live on sn_customerservice_case, the one
-- table behind every case type, so service requests, engagements and security
-- report analyses carry them too, not just cases. Same enum types as "case"
-- (0023), since the values come from the same ServiceNow choice lists.
-- Same DDL as cs-tools entity-service 0184.

ALTER TABLE service_request
  ADD COLUMN IF NOT EXISTS work_state case_work_state_enum,
  ADD COLUMN IF NOT EXISTS resolution_code case_resolution_code_enum;

ALTER TABLE engagement
  ADD COLUMN IF NOT EXISTS work_state case_work_state_enum,
  ADD COLUMN IF NOT EXISTS resolution_code case_resolution_code_enum;

ALTER TABLE security_report_analysis
  ADD COLUMN IF NOT EXISTS work_state case_work_state_enum,
  ADD COLUMN IF NOT EXISTS resolution_code case_resolution_code_enum;
