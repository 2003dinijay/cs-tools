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
--
-- Rows whose user_id already exists in "user" are already the platform
-- id (JWT email → GetUserByEmail) and are kept. There is no SQL join
-- from Asgardeo userid to "user".id: user_filter does not store email
-- and "user" has no IdP subject column. Unmapped rows abort the
-- migration instead of TRUNCATE, so mapped filters are not deleted.

DO $$
DECLARE
    unmapped_count integer;
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'user_filter_user_id_fkey'
    ) THEN
        SELECT COUNT(*) INTO unmapped_count
        FROM user_filter uf
        WHERE NOT EXISTS (
            SELECT 1 FROM "user" u WHERE u.id = uf.user_id
        );

        IF unmapped_count > 0 THEN
            RAISE EXCEPTION
                'user_filter has % row(s) whose user_id is not "user".id; cannot add FK. Those values were stored as Asgardeo JWT userid and cannot be mapped (user_filter has no email). Delete them or set user_id to "user".id, then rerun.',
                unmapped_count;
        END IF;

        ALTER TABLE user_filter
            ADD CONSTRAINT user_filter_user_id_fkey
            FOREIGN KEY (user_id) REFERENCES "user"(id) ON DELETE CASCADE;
    END IF;
END $$;
