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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Regression test for a bug where MarkProjectHealthy left a project's
// previously OPEN project_risk row untouched -- reachable whenever "Mark
// Healthy" is invoked without first going through CloseProjectRisk (e.g. via
// the csm-portal-backend's own "to be reviewed" display, which could show
// that option for a project that was genuinely still AT_RISK underneath, due
// to an unrelated sys_id-format bug on the caller side). The orphaned OPEN
// row then made every later OpenProjectRisk call on the same project fail
// forever with "project already has an open risk", even though the project's
// health status read back as HEALTHY with no open risk. Skipped unless
// ENTITY_TEST_DATABASE_URL is set (see account_repo_salesforce_integration_test.go
// for the setup).
func newCustomerHealthIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL is not set; skipping the live-database tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestCustomerHealthRepoIntegration_MarkProjectHealthyClosesAnOrphanedOpenRisk(t *testing.T) {
	pool := newCustomerHealthIntegrationPool(t)
	ctx := context.Background()

	accountID := "6cb00000-0000-4000-8000-000000000001"
	projectID := "6cb00000-0000-4000-8000-000000000002"

	cleanup := func() {
		if _, err := pool.Exec(ctx, `DELETE FROM project_risk WHERE project_id = $1`, projectID); err != nil {
			t.Fatalf("clean project_risk: %v", err)
		}
		if _, err := pool.Exec(ctx, `DELETE FROM project_health_status WHERE project_id = $1`, projectID); err != nil {
			t.Fatalf("clean project_health_status: %v", err)
		}
		if _, err := pool.Exec(ctx, `DELETE FROM project WHERE id = $1`, projectID); err != nil {
			t.Fatalf("clean project: %v", err)
		}
		if _, err := pool.Exec(ctx, `DELETE FROM account WHERE id = $1`, accountID); err != nil {
			t.Fatalf("clean account: %v", err)
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	now := time.Now()
	if _, err := pool.Exec(ctx, `
		INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number, sf_id)
		VALUES ($1, $2, $2, 'test', 'test', 'Orphaned Risk Test Account', 'ACCT-ORPHRISK-1', 'SFORPHRISK1')`,
		accountID, now); err != nil {
		t.Fatalf("insert account: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, sf_id, name, account_id)
		VALUES ($1, $2, $2, 'test', 'test', 'ORPHRISK-PROJ-1', 'SFORPHRISKPROJ1', 'Orphaned Risk Test Project', $3)`,
		projectID, now, accountID); err != nil {
		t.Fatalf("insert project: %v", err)
	}

	repo := NewCustomerHealthRepository(pool)

	risk, err := repo.OpenProjectRisk(ctx, projectID, "a risk nobody closes through the normal flow", "opener@example.com")
	if err != nil {
		t.Fatalf("OpenProjectRisk: %v", err)
	}

	// Simulate "Mark Healthy" being invoked directly, without first calling
	// CloseProjectRisk on the still-open risk -- the exact path the bug
	// reached.
	if _, err := repo.MarkProjectHealthy(ctx, projectID, "reviewer@example.com", nil); err != nil {
		t.Fatalf("MarkProjectHealthy: %v", err)
	}

	statuses, err := repo.GetAccountProjectHealthStatuses(ctx, accountID)
	if err != nil {
		t.Fatalf("GetAccountProjectHealthStatuses: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected exactly one project health status, got %d", len(statuses))
	}
	got := statuses[0]
	if got.HealthStatus.Status != "HEALTHY" {
		t.Errorf("expected health status HEALTHY, got %q", got.HealthStatus.Status)
	}
	if got.OpenRisk != nil {
		t.Errorf("expected no open risk after MarkProjectHealthy, got %+v", got.OpenRisk)
	}

	var riskStatus string
	if err := pool.QueryRow(ctx, `SELECT status FROM project_risk WHERE id = $1`, risk.ID).Scan(&riskStatus); err != nil {
		t.Fatalf("query risk status: %v", err)
	}
	if riskStatus != "CLOSED" {
		t.Errorf("expected the previously open risk to be CLOSED, got %q", riskStatus)
	}

	// The real-world symptom: a subsequent Mark At Risk must now succeed,
	// instead of failing forever with "project already has an open risk".
	if _, err := repo.OpenProjectRisk(ctx, projectID, "a genuinely new risk", "opener2@example.com"); err != nil {
		t.Fatalf("OpenProjectRisk after MarkProjectHealthy should succeed, got: %v", err)
	}
}
