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

-- Internal-only policies for the seven table/command pairs that had no policy at all.
--
-- Background: with FORCE ROW LEVEL SECURITY, a command that has no policy is refused for every
-- caller, internal ones included. The RLS series (0141-0178) left these seven commands without a
-- policy on purpose, because entity-service never issues them:
--   UPDATE  work_item_watcher, work_item_activity, comment_edit_history, time_card_approver
--           (append-only or delete-and-reinsert from entity-service's side)
--   DELETE  change_request, conversation, customer_call
-- csm-sync-service does issue them (INSERT ... ON CONFLICT DO UPDATE re-writes existing rows; the
-- delete-sync job removes rows deleted in the source), and since digiops-cs PR #3267 it connects with
-- app.is_internal=true, which passes every policy that has an internal branch but cannot pass a
-- command that has no policy. Observed on the staging database: a task_watch_list run recorded
-- thousands of `new row violates row-level security policy (USING expression)` failures on
-- work_item_watcher, while the same run had no failures before RLS.
--
-- Each policy below allows its command for internal sessions only. There is deliberately no
-- project-member branch, so a customer session is still refused exactly as before.
-- Trade-off: the database no longer refuses these seven commands for an internal session. Nothing in
-- entity-service issues them, so its behaviour is unchanged; it simply stops being a guard against an
-- internal-level bug. The test TestRLSSchemaIntegration_EveryProtectedTableHasAPolicyForEveryCommand
-- now keeps every protected table covered for all four commands.
--
-- The internal check uses the planner-friendly scalar sub-select introduced by migration 0154.
-- Idempotent (each policy is dropped and recreated) and one transaction, because `make migrate` runs
-- each file with `psql -f` and no --single-transaction. CREATE POLICY does not block reads or writes.
BEGIN;

DROP POLICY IF EXISTS work_item_watcher_update_internal_only ON work_item_watcher;
CREATE POLICY work_item_watcher_update_internal_only ON work_item_watcher
  FOR UPDATE
  USING ((SELECT current_setting('app.is_internal', true) = 'true'))
  WITH CHECK ((SELECT current_setting('app.is_internal', true) = 'true'));

DROP POLICY IF EXISTS work_item_activity_update_internal_only ON work_item_activity;
CREATE POLICY work_item_activity_update_internal_only ON work_item_activity
  FOR UPDATE
  USING ((SELECT current_setting('app.is_internal', true) = 'true'))
  WITH CHECK ((SELECT current_setting('app.is_internal', true) = 'true'));

DROP POLICY IF EXISTS comment_edit_history_update_internal_only ON comment_edit_history;
CREATE POLICY comment_edit_history_update_internal_only ON comment_edit_history
  FOR UPDATE
  USING ((SELECT current_setting('app.is_internal', true) = 'true'))
  WITH CHECK ((SELECT current_setting('app.is_internal', true) = 'true'));

DROP POLICY IF EXISTS time_card_approver_update_internal_only ON time_card_approver;
CREATE POLICY time_card_approver_update_internal_only ON time_card_approver
  FOR UPDATE
  USING ((SELECT current_setting('app.is_internal', true) = 'true'))
  WITH CHECK ((SELECT current_setting('app.is_internal', true) = 'true'));

DROP POLICY IF EXISTS change_request_delete_internal_only ON change_request;
CREATE POLICY change_request_delete_internal_only ON change_request
  FOR DELETE
  USING ((SELECT current_setting('app.is_internal', true) = 'true'));

DROP POLICY IF EXISTS conversation_delete_internal_only ON conversation;
CREATE POLICY conversation_delete_internal_only ON conversation
  FOR DELETE
  USING ((SELECT current_setting('app.is_internal', true) = 'true'));

DROP POLICY IF EXISTS customer_call_delete_internal_only ON customer_call;
CREATE POLICY customer_call_delete_internal_only ON customer_call
  FOR DELETE
  USING ((SELECT current_setting('app.is_internal', true) = 'true'));

COMMIT;
