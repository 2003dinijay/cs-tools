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

-- What a node said it runs, kept as reported. product_version_id is the catalogue
-- match, and it is NULL when the reported product or version is not in the catalogue;
-- without these the reported text was thrown away, so nobody could tell what an
-- unresolved node claimed to be or which catalogue entry is missing. ServiceNow emailed
-- a fixed address on every such miss; GET /instances/unresolved-products lists them
-- instead. Safe to run again.
ALTER TABLE deployment_node ADD COLUMN IF NOT EXISTS reported_product VARCHAR(255);
ALTER TABLE deployment_node ADD COLUMN IF NOT EXISTS reported_version VARCHAR(100);

-- When the node's current report stopped matching the catalogue: set when it goes
-- unresolved, kept while it stays so, cleared when it matches again. updated_on moves
-- with every heartbeat and created_on is when the node first reported at all, so
-- neither says how long a catalogue entry has been missing.
ALTER TABLE deployment_node ADD COLUMN IF NOT EXISTS unresolved_since TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_deployment_node_unresolved_product
    ON deployment_node (updated_on)
    WHERE product_version_id IS NULL AND reported_product IS NOT NULL;
