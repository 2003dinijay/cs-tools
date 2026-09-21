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

-- user_filter.user_id must be "user".id. 000076 originally stored the
-- Asgardeo JWT userid with no FK; goose will not re-run that file, so
-- this migration adds the constraint when it is still missing. Skip when
-- 000076 already created the table with REFERENCES "user"(id).

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'user_filter_user_id_fkey'
    ) THEN
        -- Rows stored as Asgardeo userid cannot satisfy the FK.
        TRUNCATE TABLE user_filter;
        ALTER TABLE user_filter
            ADD CONSTRAINT user_filter_user_id_fkey
            FOREIGN KEY (user_id) REFERENCES "user"(id) ON DELETE CASCADE;
    END IF;
END $$;
