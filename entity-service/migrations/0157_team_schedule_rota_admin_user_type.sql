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

-- A rota admin is internal staff, so recompute_user_type() has to say so.
--
-- Without this the feature does not work at all, and fails in a way that looks
-- like something else: recompute_user_type() (0011) derives INTERNAL from
-- the names 'admin' and 'internal' only, and anything it does not recognise
-- falls through to NOT_AVAILABLE. The schedule's own requireInternalCaller
-- refuses a NOT_AVAILABLE caller with a 403 before it ever asks whether they
-- may edit this team -- so granting cre_rota_admin to somebody who does not
-- already hold 'admin' or 'internal' would lock them out rather than let them
-- in, and the 403 says nothing about roles.
--
-- The function is rewritten in full rather than altered. A plpgsql body is
-- text, resolved at runtime, so nothing about it follows a change made
-- elsewhere -- the same property that has made a table rename here need an
-- explicit CREATE OR REPLACE of a trigger function it did not appear to
-- touch. Everything here is 0011's body verbatim except the two added
-- names; the EXTERNAL branch and the SYSTEM precedence are unchanged.
CREATE OR REPLACE FUNCTION recompute_user_type(p_user_id UUID) RETURNS void AS $$
BEGIN
    UPDATE "user"
    SET user_type = (CASE
        WHEN is_system_user THEN 'SYSTEM'
        WHEN EXISTS (
            SELECT 1 FROM user_role ur JOIN role r ON r.id = ur.role_id
            WHERE ur.user_id = p_user_id
              AND r.name IN ('admin', 'internal', 'cre_rota_admin', 'sre_rota_admin')
        ) THEN 'INTERNAL'
        WHEN EXISTS (
            SELECT 1 FROM user_role ur JOIN role r ON r.id = ur.role_id
            WHERE ur.user_id = p_user_id
              AND r.name IN ('external', 'partner', 'customer', 'partner_admin', 'customer_admin')
        ) THEN 'EXTERNAL'
        ELSE 'NOT_AVAILABLE'
    END)::user_type_enum
    WHERE id = p_user_id;
END;
$$ LANGUAGE plpgsql;

-- Backfill, for anyone already holding one of the two roles when this runs.
--
-- Narrow on purpose, unlike 0011's own whole-table backfill. This change
-- only ADDS names to the INTERNAL branch, so no user can move away from
-- INTERNAL by it -- only towards. That makes "holds a rota admin role and is
-- not already INTERNAL" the complete set of rows whose derived value changed,
-- and rewriting the other few thousand would be churn with no result.
--
-- is_system_user is checked the way the CASE above checks it: SYSTEM wins over
-- every role, and IS NOT TRUE (rather than = FALSE) keeps a NULL falling
-- through to the role branches exactly as the CASE does.
--
-- This UPDATE does not re-enter the trigger it looks like it might:
-- users_is_system_user_change is AFTER INSERT OR UPDATE **OF is_system_user**,
-- and this writes user_type only.
UPDATE "user" u
   SET user_type = 'INTERNAL'::user_type_enum
 WHERE u.is_system_user IS NOT TRUE
   AND u.user_type IS DISTINCT FROM 'INTERNAL'::user_type_enum
   AND EXISTS (
       SELECT 1 FROM user_role ur JOIN role r ON r.id = ur.role_id
        WHERE ur.user_id = u.id
          AND r.name IN ('cre_rota_admin', 'sre_rota_admin')
   );
