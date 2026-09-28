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

DROP POLICY IF EXISTS work_item_activity_delete_internal_only ON work_item_activity;
DROP POLICY IF EXISTS work_item_activity_write ON work_item_activity;
DROP POLICY IF EXISTS work_item_activity_visibility ON work_item_activity;
ALTER TABLE work_item_activity NO FORCE ROW LEVEL SECURITY;
ALTER TABLE work_item_activity DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS work_item_watcher_delete ON work_item_watcher;
DROP POLICY IF EXISTS work_item_watcher_write ON work_item_watcher;
DROP POLICY IF EXISTS work_item_watcher_visibility ON work_item_watcher;
ALTER TABLE work_item_watcher NO FORCE ROW LEVEL SECURITY;
ALTER TABLE work_item_watcher DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS work_item_tag_delete ON work_item_tag;
DROP POLICY IF EXISTS work_item_tag_update ON work_item_tag;
DROP POLICY IF EXISTS work_item_tag_write ON work_item_tag;
DROP POLICY IF EXISTS work_item_tag_visibility ON work_item_tag;
ALTER TABLE work_item_tag NO FORCE ROW LEVEL SECURITY;
ALTER TABLE work_item_tag DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS case_attachment_delete ON case_attachment;
DROP POLICY IF EXISTS case_attachment_update ON case_attachment;
DROP POLICY IF EXISTS case_attachment_write ON case_attachment;
DROP POLICY IF EXISTS case_attachment_visibility ON case_attachment;
ALTER TABLE case_attachment NO FORCE ROW LEVEL SECURITY;
ALTER TABLE case_attachment DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS comment_edit_history_delete_internal_only ON comment_edit_history;
DROP POLICY IF EXISTS comment_edit_history_write ON comment_edit_history;
DROP POLICY IF EXISTS comment_edit_history_visibility ON comment_edit_history;
ALTER TABLE comment_edit_history NO FORCE ROW LEVEL SECURITY;
ALTER TABLE comment_edit_history DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS comment_delete_internal_only ON comment;
DROP POLICY IF EXISTS comment_update ON comment;
DROP POLICY IF EXISTS comment_write ON comment;
DROP POLICY IF EXISTS comment_visibility ON comment;
ALTER TABLE comment NO FORCE ROW LEVEL SECURITY;
ALTER TABLE comment DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS case_delete_internal_only ON "case";
DROP POLICY IF EXISTS case_write ON "case";
DROP POLICY IF EXISTS case_update ON "case";
DROP POLICY IF EXISTS case_visibility ON "case";
ALTER TABLE "case" NO FORCE ROW LEVEL SECURITY;
ALTER TABLE "case" DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS work_item_delete_internal_only ON work_item;
DROP POLICY IF EXISTS work_item_write ON work_item;
DROP POLICY IF EXISTS work_item_update ON work_item;
DROP POLICY IF EXISTS work_item_visibility ON work_item;
ALTER TABLE work_item NO FORCE ROW LEVEL SECURITY;
ALTER TABLE work_item DISABLE ROW LEVEL SECURITY;
