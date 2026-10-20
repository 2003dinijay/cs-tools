// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package repository_test

import (
	"context"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	coresDeploymentID    = "6e000000-0000-4000-8000-000000000002"
	coresDeployedProduct = "6f000000-0000-4000-8000-000000000001"
	coresNodeA           = "6f100000-0000-4000-8000-000000000001"
	coresNodeB           = "6f100000-0000-4000-8000-000000000002"
	coresNodeC           = "6f100000-0000-4000-8000-000000000003"
)

// seedCoreNodes adds a deployed product on the shared test deployment and three
// nodes that resolve to it: A reports cores only through host snapshots, B through
// both a CORES usage row and snapshots, and C reports nothing at all.
func seedCoreNodes(t *testing.T, ctx context.Context, exec func(context.Context, string, ...any) error) {
	t.Helper()
	for _, stmt := range []string{
		`CREATE TABLE deployed_product (id UUID PRIMARY KEY, name VARCHAR(255), number VARCHAR(50),
			deployment_id UUID, version_id UUID, product_id UUID)`,
		`INSERT INTO deployed_product (id, name, deployment_id, version_id, product_id) VALUES
			('` + coresDeployedProduct + `', 'WSO2 API Manager', '` + coresDeploymentID + `', '` + ingestVersionAM420 + `', '` + ingestProductAM + `')`,
		`INSERT INTO deployment_node (id, created_on, updated_on, created_by, updated_by, node_id, project_key, deployment_number, product_version_id)
			SELECT v.id::uuid, NOW(), NOW(), 't', 't', v.node_id, 'CUPRSUBDEV', 'DEP-OLD', '` + ingestVersionAM420 + `'
			FROM (VALUES ('` + coresNodeA + `', 'core-node-a'), ('` + coresNodeB + `', 'core-node-b'), ('` + coresNodeC + `', 'core-node-c')) AS v(id, node_id)`,
		// Node A: two reports on 1 June (the later one, 6 cores, is the day's reading) and one on 2 June.
		`INSERT INTO deployment_information (id, created_on, updated_on, created_by, updated_by, node_id, deployment_info, deployment_info_hash, core_count, payload_created_on, payload_updated_on)
			VALUES (gen_random_uuid(), NOW(), NOW(), 't', 't', 'core-node-a', '{}', 'h1', 4, '2026-06-01T08:00:00Z', '2026-06-01T09:00:00Z'),
			       (gen_random_uuid(), NOW(), NOW(), 't', 't', 'core-node-a', '{}', 'h2', 6, '2026-06-01T10:00:00Z', '2026-06-01T11:00:00Z'),
			       (gen_random_uuid(), NOW(), NOW(), 't', 't', 'core-node-a', '{}', 'h2', 6, '2026-06-02T10:00:00Z', '2026-06-02T11:00:00Z')`,
		// Node B: a CORES usage row of 8 on 1 June beats its 2-core snapshot that day; 3 June has the snapshot only.
		`INSERT INTO daily_usage_summary (id, created_on, updated_on, created_by, updated_by, deployment_node_id, deployment_number, summary_date, count_type, value, data_source)
			VALUES (gen_random_uuid(), NOW(), NOW(), 't', 't', '` + coresNodeB + `', 'DEP-OLD', '2026-06-01', 'CORES', 8, 'API_CALL')`,
		`INSERT INTO deployment_information (id, created_on, updated_on, created_by, updated_by, node_id, deployment_info, deployment_info_hash, core_count, payload_created_on, payload_updated_on)
			VALUES (gen_random_uuid(), NOW(), NOW(), 't', 't', 'core-node-b', '{}', 'h3', 2, '2026-06-01T10:00:00Z', '2026-06-01T11:00:00Z'),
			       (gen_random_uuid(), NOW(), NOW(), 't', 't', 'core-node-b', '{}', 'h3', 2, '2026-06-03T10:00:00Z', '2026-06-03T11:00:00Z'),
			       (gen_random_uuid(), NOW(), NOW(), 't', 't', 'core-node-b', '{}', 'h4', 99, '2026-07-15T10:00:00Z', '2026-07-15T11:00:00Z')`,
	} {
		if err := exec(ctx, stmt); err != nil {
			t.Fatalf("seed %.60q: %v", stmt, err)
		}
	}
}

// The product-level Core Usage chart must draw the core counts a node reports
// through its host snapshot, not only CORES usage rows: a CORES row wins for its
// node and day, a snapshot fills the days that have none (its latest report of the
// day), and a node that reports neither adds nothing.
func TestSearchDeployedProductMetrics_CoresFromSnapshotsAndUsageRows(t *testing.T) {
	pool := instanceIngestionTestPool(t)
	ctx := internalCtx()
	seedCoreNodes(t, ctx, func(c context.Context, q string, a ...any) error {
		_, err := pool.Exec(c, q, a...)
		return err
	})

	repo := repository.NewDeployedProductRepository(repository.NewScoped(pool))
	got, err := repo.SearchDeployedProductMetrics(ctx, coresDeployedProduct, coresDeploymentID, "2026-06-01", "2026-06-30")
	if err != nil {
		t.Fatalf("SearchDeployedProductMetrics: %v", err)
	}

	if got.Summary.TotalInstances != 3 {
		t.Errorf("totalInstances = %d, want 3 (every node the product resolves to)", got.Summary.TotalInstances)
	}
	type day struct{ total, count int }
	want := map[string]day{
		"2026-06-01": {total: 14, count: 2}, // A: 6 (latest snapshot), B: 8 (usage row beats snapshot)
		"2026-06-02": {total: 6, count: 1},  // A only
		"2026-06-03": {total: 2, count: 1},  // B's snapshot
	}
	if len(got.ChartData) != len(want) {
		t.Fatalf("chartData has %d days, want %d: %+v", len(got.ChartData), len(want), got.ChartData)
	}
	for _, e := range got.ChartData {
		w, ok := want[e.Date]
		if !ok {
			t.Errorf("unexpected day %s (a snapshot outside the range?)", e.Date)
			continue
		}
		if e.TotalCores != w.total || e.InstanceCount != w.count {
			t.Errorf("%s: totalCores=%d instanceCount=%d, want %d and %d", e.Date, e.TotalCores, e.InstanceCount, w.total, w.count)
		}
	}
	if got.Summary.MinCores == nil || got.Summary.MaxCores == nil || got.Summary.AvgCores == nil {
		t.Fatalf("summary cores missing: %+v", got.Summary)
	}
	if *got.Summary.MinCores != 2 || *got.Summary.MaxCores != 8 || *got.Summary.AvgCores != 5.5 {
		t.Errorf("summary min/max/avg = %d/%d/%v, want 2/8/5.5", *got.Summary.MinCores, *got.Summary.MaxCores, *got.Summary.AvgCores)
	}
}

// With no snapshot and no usage row in range the chart stays empty rather than
// failing, as before.
func TestSearchDeployedProductMetrics_NoCoresInRangeIsAnEmptyChart(t *testing.T) {
	pool := instanceIngestionTestPool(t)
	ctx := internalCtx()
	seedCoreNodes(t, ctx, func(c context.Context, q string, a ...any) error {
		_, err := pool.Exec(c, q, a...)
		return err
	})

	repo := repository.NewDeployedProductRepository(repository.NewScoped(pool))
	got, err := repo.SearchDeployedProductMetrics(ctx, coresDeployedProduct, coresDeploymentID, "2026-01-01", "2026-01-31")
	if err != nil {
		t.Fatalf("SearchDeployedProductMetrics: %v", err)
	}
	if len(got.ChartData) != 0 || got.Summary.MinCores != nil {
		t.Errorf("expected an empty chart and no summary cores, got %+v", got)
	}
}
