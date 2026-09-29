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

-- Planner-friendly internal-caller check in every RLS policy.
--
-- Problem: for an internal caller each policy is "app.is_internal = 'true' OR
-- ...", always true, but the planner cannot know that when it plans
-- (current_setting() is opaque to cardinality estimation, so the comparison
-- gets the default 0.5% selectivity). Measured on the real-data copy:
-- 2,046 estimated vs 409,296 actual work_item rows, which pushed queries
-- into nested loops with hundreds of thousands of per-row index probes.
--
-- Fix: wrap the comparison in a scalar sub-select,
--   (SELECT current_setting('app.is_internal', true) = 'true')
-- Postgres evaluates it once per query as an InitPlan instead of once per
-- row, and estimates it at a neutral 50% rather than 0.5%. No new role, no
-- change to who can see what: the predicate's truth value is identical.
--
-- Rewrites every existing policy that contains the old form; idempotent
-- (already-wrapped policies no longer match).
DO $$
DECLARE
  old_expr CONSTANT TEXT := $e$(current_setting('app.is_internal'::text, true) = 'true'::text)$e$;
  new_expr CONSTANT TEXT := $e$(SELECT (current_setting('app.is_internal'::text, true) = 'true'::text))$e$;
  r RECORD;
  using_sql TEXT;
  check_sql TEXT;
BEGIN
  FOR r IN
    SELECT policyname, tablename, qual, with_check
    FROM pg_policies
    WHERE schemaname = current_schema()
      AND (position(old_expr IN coalesce(qual, '')) > 0 OR position(old_expr IN coalesce(with_check, '')) > 0)
  LOOP
    using_sql := CASE WHEN r.qual IS NOT NULL
      THEN format(' USING (%s)', replace(r.qual, old_expr, new_expr)) ELSE '' END;
    check_sql := CASE WHEN r.with_check IS NOT NULL
      THEN format(' WITH CHECK (%s)', replace(r.with_check, old_expr, new_expr)) ELSE '' END;
    EXECUTE format('ALTER POLICY %I ON %I%s%s', r.policyname, r.tablename, using_sql, check_sql);
  END LOOP;
END
$$;
