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

-- Renamed from the old-style 000084_project_add_verification_enabled.up.sql
-- -- see 0176_pending_verification_table.sql's own comment for why the
-- six-digit name had to go. ADD COLUMN IF NOT EXISTS was already re-run-safe.
ALTER TABLE project ADD COLUMN IF NOT EXISTS verification_enabled BOOLEAN;
