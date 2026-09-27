-- Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com). All Rights Reserved.
--
-- This software is the property of WSO2 LLC. and its suppliers, if any.
-- Dissemination of any information or reproduction of any material contained
-- herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
-- You may not alter or remove any copyright or other notice from copies of this content.

-- Brings every control-plane table onto one shared csm_migration_ prefix, so
-- the whole control plane reads as one family of tables. The 6th
-- control-plane table, csm_migration_applied_migration (make migrate's own
-- tracking table), is renamed the same way but via Makefile, not here - see
-- CLAUDE.md §4. IF EXISTS makes each rename a no-op on an environment where
-- it's already run (or where 0001_control_plane.sql, in a future fresh
-- install, is edited to create the new names directly) - safe to re-run.
ALTER TABLE IF EXISTS migration_job RENAME TO csm_migration_job;
ALTER TABLE IF EXISTS migration_run RENAME TO csm_migration_run;
ALTER TABLE IF EXISTS migration_row_error RENAME TO csm_migration_row_error;
ALTER TABLE IF EXISTS sync_checkpoint RENAME TO csm_migration_checkpoint;
ALTER TABLE IF EXISTS schema_version RENAME TO csm_migration_schema_version;
