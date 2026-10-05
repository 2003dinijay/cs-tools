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

-- change_request.is_customer_approved/is_customer_reviewed (migration 0043 --
-- the authorized, one-way-locked customer OUTCOME flags, see change_request_repo.go's
-- own doc comment) were renamed directly on the real staging database, out of
-- band, to is_customer_approval_required/is_customer_review_required --
-- confirmed: no migration in this repo ever renamed them, and migration 0189's
-- own doc comment explicitly states its similarly-named customer_approval_required/
-- customer_review_required (no is_ prefix) are a deliberately DIFFERENT concept
-- (the REQUIREMENT, not the outcome). The code was updated to match staging's
-- real column names in a separate change; this migration catches this repo's
-- own migration history up to that same reality, so a database built fresh
-- from migrations/ (local dev, CI) ends up with the column names the code
-- actually expects, instead of 500ing on "column does not exist" the same way
-- staging did before the code was fixed.
--
-- Conditional, not a bare RENAME COLUMN: staging itself already has the new
-- names (the rename already happened there), so a bare rename would fail with
-- "column is_customer_approved does not exist" if this migration ever runs
-- against it. Each rename only fires when the old name is present AND the new
-- name is not already there -- safe to re-run, and a no-op on staging.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'change_request' AND column_name = 'is_customer_approved'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'change_request' AND column_name = 'is_customer_approval_required'
    ) THEN
        ALTER TABLE change_request RENAME COLUMN is_customer_approved TO is_customer_approval_required;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'change_request' AND column_name = 'is_customer_reviewed'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'change_request' AND column_name = 'is_customer_review_required'
    ) THEN
        ALTER TABLE change_request RENAME COLUMN is_customer_reviewed TO is_customer_review_required;
    END IF;
END $$;
