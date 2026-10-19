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

-- Renamed from the old-style 000075_pending_verification_table.up.sql (its
-- .down.sql counterpart is deleted, not mirrored here). Six-digit
-- 0NNNNN_*.up.sql/.down.sql names are this repo's legacy, pre-NNNN-scheme
-- convention -- entity-service's Makefile migrate target globs only the
-- four-digit NNNN_*.sql files first, then the dedicated 1NNNNN_*.sql RLS
-- track, then the six-digit 0NNNNN_*.up.sql stragglers last (.down.sql
-- files are never globbed by anything in this repo at all); a brand new
-- 2026 migration should never have used that legacy convention in the
-- first place. See the top-level CLAUDE.md's "Migrations are keyed by
-- filename" for why a bare rename is still safe here: this file guards
-- both CREATE TYPEs so re-running it against an environment that already
-- recorded the old filename as applied is a no-op.
DO $$ BEGIN
    CREATE TYPE pending_verification_added_reason_enum AS ENUM ('AUTO_CLOSED', 'MANUAL');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE pending_verification_previous_status_enum AS ENUM ('SOLUTION_PROPOSED', 'AWAITING_INFO');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS pending_verification (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    work_item_id UUID NOT NULL REFERENCES work_item(id),
    added_reason pending_verification_added_reason_enum NOT NULL,
    previous_status pending_verification_previous_status_enum,
    added_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    added_by VARCHAR(255) NOT NULL,
    verified_on TIMESTAMPTZ,
    verified_by VARCHAR(255),
    note TEXT,

    -- previous_status is set iff the entry is auto-closed
    CONSTRAINT chk_previous_status_matches_reason
        CHECK ((added_reason = 'AUTO_CLOSED') = (previous_status IS NOT NULL)),

    -- verified_by is set iff verified_on is set
    CONSTRAINT chk_verified_by_matches_verified_on
        CHECK ((verified_on IS NULL) = (verified_by IS NULL))
);

CREATE INDEX IF NOT EXISTS idx_pending_verification_work_item_id ON pending_verification (work_item_id);
CREATE INDEX IF NOT EXISTS idx_pending_verification_verified_on ON pending_verification (verified_on);
