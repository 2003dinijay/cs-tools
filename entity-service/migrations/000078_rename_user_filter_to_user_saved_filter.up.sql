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

-- 000076/000077 may already have been applied as user_filter. Goose will
-- not re-run those versions, so this rename brings existing databases in
-- line with the repository. No-op when the table was created as
-- user_saved_filter.

DO $$
BEGIN
    IF to_regclass('public.user_filter') IS NOT NULL
       AND to_regclass('public.user_saved_filter') IS NULL THEN
        ALTER TABLE user_filter RENAME TO user_saved_filter;
    END IF;
    IF to_regclass('public.user_filter_user_list_name') IS NOT NULL
       AND to_regclass('public.user_saved_filter_user_list_name') IS NULL THEN
        ALTER INDEX user_filter_user_list_name RENAME TO user_saved_filter_user_list_name;
    END IF;
    IF to_regclass('public.user_filter_user_list_filter_position') IS NOT NULL
       AND to_regclass('public.user_saved_filter_user_list_filter_position') IS NULL THEN
        ALTER INDEX user_filter_user_list_filter_position RENAME TO user_saved_filter_user_list_filter_position;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'user_filter_user_id_fkey')
       AND NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'user_saved_filter_user_id_fkey') THEN
        ALTER TABLE user_saved_filter
            RENAME CONSTRAINT user_filter_user_id_fkey TO user_saved_filter_user_id_fkey;
    END IF;
END $$;
