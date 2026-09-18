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

-- Which GitHub repository belongs to which product, and who gets assigned.
--
-- ONE TABLE, READ BOTH WAYS. Inbound, a webhook names a repository and this
-- resolves the product and the team to assign. Outbound, a case names a
-- product and this resolves where to file the issue. ServiceNow answered those
-- two questions in three different places -- hardcoded sys_ids in a script
-- (getAssigment), per-product system properties (scripted-wum.*-repo-name),
-- and a third derivation inside the case API -- which is how they drifted
-- apart.
--
-- PRODUCT-LEVEL, NOT PROJECT-LEVEL, confirmed with SRE: every customer's
-- change requests for a product go to that product's single repository. If
-- that ever stops being true this grows a project_id and a precedence rule,
-- and the UNIQUE below is what will force that conversation rather than
-- letting two rows quietly both match.
CREATE TABLE IF NOT EXISTS product_github_repo (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,

    product_id UUID NOT NULL REFERENCES product(id) ON DELETE CASCADE,
    -- The GitHub org (or user) and repository, exactly as GitHub spells them.
    -- Case matters: ServiceNow's map looked for "choreo" while the records
    -- said "Choreo", so every one of those fell through to a literal "NULL".
    owner VARCHAR(100) NOT NULL,
    repository VARCHAR(200) NOT NULL,

    -- The team a change request raised in this repository is assigned to.
    -- Nullable: a repository can be recognised before anyone decides who owns
    -- it, and a null here is a routing gap worth seeing rather than a reason
    -- to reject the event.
    team_id UUID REFERENCES team(id) ON DELETE SET NULL,

    -- Where to file when only a product is known (the outbound direction).
    -- Exactly one per product, enforced below.
    is_default BOOLEAN NOT NULL DEFAULT FALSE,

    -- Off by default so a repository can be registered before the integration
    -- is pointed at it, and switched off without deleting the mapping.
    is_active BOOLEAN NOT NULL DEFAULT TRUE,

    -- A repository belongs to exactly one product, which is what makes the
    -- inbound lookup unambiguous and removes the need for a separate
    -- allow-list. An unmapped repository is simply one we do not handle.
    CONSTRAINT uq_product_github_repo UNIQUE (owner, repository)
);

CREATE INDEX IF NOT EXISTS idx_product_github_repo_product ON product_github_repo (product_id);
CREATE INDEX IF NOT EXISTS idx_product_github_repo_team ON product_github_repo (team_id);

-- The inbound hot path: resolve a webhook's repository to a product and team.
-- Lower-cased because GitHub treats owner and repository case-insensitively
-- for routing, even though it preserves the case it was created with.
CREATE INDEX IF NOT EXISTS idx_product_github_repo_lookup
    ON product_github_repo (lower(owner), lower(repository));

-- At most one default per product: "file it against this product" must have
-- one answer, and a partial unique index says so without forbidding the other
-- repositories a product may own.
CREATE UNIQUE INDEX IF NOT EXISTS uq_product_github_repo_one_default
    ON product_github_repo (product_id) WHERE is_default;
