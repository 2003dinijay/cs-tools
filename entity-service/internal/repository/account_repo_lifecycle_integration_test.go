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

// Regression test for SearchAccountsFilters.LifeCycle/HasProject -- added to
// faithfully reproduce the legacy system's own base query for the Customer
// Health dashboard's account list (life cycle = Customer plus a
// join-existence check against the account's projects), found live: the CSM
// portal backend's own "active accounts" filter (account.deactivation_date
// IS NULL) matched far more accounts than the legacy implementation answered
// for the identical request -- account.life_cycle (not
// account.classification, a different, business-segment dimension) is what
// mirrors the legacy rule's own status field, and a live reproduction
// against the real staging database confirmed `life_cycle = 'Customer' AND
// has at least one project` reproduces the legacy implementation's exact
// account count.
//
// Runs against a real Postgres with every migration applied. Skipped without
// CASE_STATS_TEST_DSN, so an ordinary `go test ./...` stays hermetic.
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run AccountLifeCycle

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	alcAccountCustomerWithProject    = "a6000000-0000-0000-0000-000000000001" // life_cycle=Customer, has a project -- must match
	alcAccountCustomerNoProject      = "a6000000-0000-0000-0000-000000000002" // life_cycle=Customer, no project -- must NOT match
	alcAccountProspectWithProject    = "a6000000-0000-0000-0000-000000000003" // life_cycle=Prospect, has a project -- must NOT match
	alcAccountCustomerDeactivated    = "a6000000-0000-0000-0000-000000000004" // life_cycle=Customer, has a project, deactivated -- MUST still match (the real script has no active/inactive filter at all)
	alcProjectForCustomerWithProject = "a6100000-0000-0000-0000-000000000001"
	alcProjectForProspectWithProject = "a6100000-0000-0000-0000-000000000003"
	alcProjectForDeactivated         = "a6100000-0000-0000-0000-000000000004"
	alcCreatedBy                     = "alc-rls-test"
)

func seedAccountLifeCycleFixture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()

	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM project WHERE id IN ($1, $2, $3)`,
			alcProjectForCustomerWithProject, alcProjectForProspectWithProject, alcProjectForDeactivated)
		_, _ = pool.Exec(ctx, `DELETE FROM account WHERE created_by = $1`, alcCreatedBy)
	}
	cleanup()
	t.Cleanup(cleanup)

	now := time.Now().UTC()
	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed (%.80s): %v", sql, err)
		}
	}

	insertAccount := func(id, number, lifeCycle string, deactivated bool) {
		var deactivationDate any
		if deactivated {
			deactivationDate = now.AddDate(0, -1, 0)
		}
		mustExec(`INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number, sf_id, life_cycle, deactivation_date)
			VALUES ($1, $2, $2, $3, $3, $4, $5, $6, $7, $8)`,
			id, now, alcCreatedBy, "ALC Test "+number, number, "sf-"+number, lifeCycle, deactivationDate)
	}
	insertAccount(alcAccountCustomerWithProject, "ALC-CUST-PROJ", "Customer", false)
	insertAccount(alcAccountCustomerNoProject, "ALC-CUST-NOPROJ", "Customer", false)
	insertAccount(alcAccountProspectWithProject, "ALC-PROSPECT", "Prospect", false)
	insertAccount(alcAccountCustomerDeactivated, "ALC-CUST-DEACT", "Customer", true)

	insertProject := func(id, key, accountID string) {
		mustExec(`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, name, sf_id, account_id)
			VALUES ($1, $2, $2, $3, $3, $4, $4, $5, $6)`,
			id, now, alcCreatedBy, key, "sf-"+key, accountID)
	}
	insertProject(alcProjectForCustomerWithProject, "ALCPROJ1", alcAccountCustomerWithProject)
	insertProject(alcProjectForProspectWithProject, "ALCPROJ3", alcAccountProspectWithProject)
	insertProject(alcProjectForDeactivated, "ALCPROJ4", alcAccountCustomerDeactivated)
}

func alcIDs(views []repository.AccountRow) map[string]bool {
	out := map[string]bool{}
	for _, v := range views {
		out[v.ID] = true
	}
	return out
}

// TestAccountLifeCycleIntegration_LifeCycleAndHasProject pins the exact
// filter combination that reproduces the legacy system's account count:
// life_cycle = "Customer" AND HasProject -- and specifically that it does
// NOT also require the account to be active, since the legacy rule's own
// base query has no such condition at all.
func TestAccountLifeCycleIntegration_LifeCycleAndHasProject(t *testing.T) {
	pool := caseStatsPool(t)
	seedAccountLifeCycleFixture(t, pool)
	repo := repository.NewAccountRepository(repository.NewScoped(pool))

	req := domain.SearchAccountsRequest{
		Pagination: domain.Pagination{Limit: 50},
		Filters:    domain.SearchAccountsFilters{LifeCycle: "Customer", HasProject: true},
	}
	rows, _, err := repo.SearchAccounts(repository.WithSystemIdentity(context.Background()), req)
	if err != nil {
		t.Fatalf("SearchAccounts: %v", err)
	}
	got := alcIDs(rows)

	if !got[alcAccountCustomerWithProject] {
		t.Errorf("want %s (Customer, has a project) to match, got %v", alcAccountCustomerWithProject, got)
	}
	if !got[alcAccountCustomerDeactivated] {
		t.Errorf("want %s (Customer, has a project, DEACTIVATED) to still match -- the real script has no active/inactive filter, got %v", alcAccountCustomerDeactivated, got)
	}
	if got[alcAccountCustomerNoProject] {
		t.Errorf("account %s (Customer, no project) must NOT match", alcAccountCustomerNoProject)
	}
	if got[alcAccountProspectWithProject] {
		t.Errorf("account %s (Prospect, has a project) must NOT match", alcAccountProspectWithProject)
	}
}

// TestAccountLifeCycleIntegration_LifeCycleAloneIgnoresProjectExistence
// pins that LifeCycle and HasProject are independent filters -- omitting
// HasProject must not implicitly require a project.
func TestAccountLifeCycleIntegration_LifeCycleAloneIgnoresProjectExistence(t *testing.T) {
	pool := caseStatsPool(t)
	seedAccountLifeCycleFixture(t, pool)
	repo := repository.NewAccountRepository(repository.NewScoped(pool))

	req := domain.SearchAccountsRequest{
		Pagination: domain.Pagination{Limit: 50},
		Filters:    domain.SearchAccountsFilters{LifeCycle: "Customer"},
	}
	rows, _, err := repo.SearchAccounts(repository.WithSystemIdentity(context.Background()), req)
	if err != nil {
		t.Fatalf("SearchAccounts: %v", err)
	}
	got := alcIDs(rows)
	for _, id := range []string{alcAccountCustomerWithProject, alcAccountCustomerNoProject, alcAccountCustomerDeactivated} {
		if !got[id] {
			t.Errorf("want %s (life_cycle=Customer) to match with no HasProject filter, got %v", id, got)
		}
	}
	if got[alcAccountProspectWithProject] {
		t.Errorf("account %s (Prospect) must NOT match a LifeCycle=Customer filter", alcAccountProspectWithProject)
	}
}
