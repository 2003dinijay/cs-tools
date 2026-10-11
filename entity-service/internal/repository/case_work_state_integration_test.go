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

// Regression tests for the work_state lifecycle: work_state is only valid
// while state is work_in_progress, so a state change away from it must pause
// (awaiting_info) or clear (everything else) work_state in the same statement,
// and the one-Ongoing-per-engineer guard must only count work_in_progress
// items. Runs against a real Postgres with all migrations applied. Skipped
// without CASE_STATS_TEST_DSN.
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run CaseWorkState

package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	cwsCase     = "73333333-0000-0000-0000-000000000004"
	cwsEngineer = "74444444-0000-0000-0000-000000000002"
)

// seedCaseWorkStateFixture extends the case-like extension fixture with a
// "case" work item and one assigned engineer, and assigns every work item to
// that engineer.
func seedCaseWorkStateFixture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	seedCaseLikeExtensionFixture(t, pool)
	sys := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	cleanup := func() {
		_, _ = scoped.Exec(sys, `UPDATE work_item SET assigned_to_id = NULL WHERE assigned_to_id = $1`, cwsEngineer)
		_, _ = scoped.Exec(sys, `DELETE FROM "case" WHERE id = $1`, cwsCase)
		_, _ = scoped.Exec(sys, `DELETE FROM work_item WHERE id = $1`, cwsCase)
		_, _ = pool.Exec(sys, `DELETE FROM "user" WHERE id = $1`, cwsEngineer)
	}
	cleanup()
	t.Cleanup(cleanup)
	now := time.Now().UTC()
	if _, err := pool.Exec(sys, `INSERT INTO "user" (id, created_on, updated_on, user_name, email, is_active)
		VALUES ($1, now(), now(), 'cws-engineer@test.local', 'cws-engineer@test.local', true)`, cwsEngineer); err != nil {
		t.Fatalf("seed engineer: %v", err)
	}
	if _, err := scoped.Exec(sys, `INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, project_id)
		VALUES ($1, $2, $2, 'test', 'test', 'CWS-CASE-1', 'CWS-WSO2-CASE-1', 'cws case', 'CASE', $3)`, cwsCase, now, cleProjectID); err != nil {
		t.Fatalf("seed case work item: %v", err)
	}
	if _, err := scoped.Exec(sys, `INSERT INTO "case" (id, severity, issue_type, state) VALUES ($1, 'S3', 'ERROR', 'OPEN')`, cwsCase); err != nil {
		t.Fatalf("seed case: %v", err)
	}
	if _, err := scoped.Exec(sys, `UPDATE work_item SET assigned_to_id = $1 WHERE id = ANY($2::uuid[])`,
		cwsEngineer, []string{cwsCase, cleServiceReq, cleEngagement, cleSecReport}); err != nil {
		t.Fatalf("assign: %v", err)
	}
}

func cwsWorkState(t *testing.T, pool *pgxpool.Pool, table, id string) string {
	t.Helper()
	var got *string
	sys := repository.WithSystemIdentity(context.Background())
	if err := repository.NewScoped(pool).QueryRow(sys, `SELECT work_state::TEXT FROM `+table+` WHERE id = $1`, id).Scan(&got); err != nil {
		t.Fatalf("read %s.work_state: %v", table, err)
	}
	if got == nil {
		return "NULL"
	}
	return *got
}

func TestCaseWorkStateIntegration_StateChangeSyncsWorkState(t *testing.T) {
	pool := caseStatsPool(t)
	seedCaseWorkStateFixture(t, pool)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	ctx := repository.WithCallerIdentity(context.Background(), repository.SearchScope{Unrestricted: false, ViewerEmail: cleMember})

	types := []struct{ name, table, id string }{
		{"case", `"case"`, cwsCase},
		{"service_request", "service_request", cleServiceReq},
		{"engagement", "engagement", cleEngagement},
		{"security_report_analysis", "security_report_analysis", cleSecReport},
	}
	setState := func(t *testing.T, id string, s domain.CaseState) {
		t.Helper()
		if _, _, err := repo.UpdateCase(ctx, domain.UpdateCaseRequest{ID: id, State: &s}, nil); err != nil {
			t.Fatalf("UpdateCase state %s: %v", s, err)
		}
	}
	setOngoing := func(t *testing.T, id string) {
		t.Helper()
		ws := domain.CaseWorkStateOngoing
		if _, _, err := repo.UpdateCase(ctx, domain.UpdateCaseRequest{ID: id, WorkState: &ws}, nil); err != nil {
			t.Fatalf("UpdateCase workState ongoing: %v", err)
		}
	}

	for _, ty := range types {
		t.Run(ty.name, func(t *testing.T) {
			setState(t, ty.id, domain.CaseStateWorkInProgress)
			setOngoing(t, ty.id)
			if got := cwsWorkState(t, pool, ty.table, ty.id); got != "ONGOING" {
				t.Fatalf("after ongoing: work_state = %s, want ONGOING", got)
			}

			// (iii) a state PATCH to work_in_progress leaves work_state alone.
			setState(t, ty.id, domain.CaseStateWorkInProgress)
			if got := cwsWorkState(t, pool, ty.table, ty.id); got != "ONGOING" {
				t.Errorf("after work_in_progress re-PATCH: work_state = %s, want ONGOING", got)
			}

			// (i) awaiting_info pauses.
			setState(t, ty.id, domain.CaseStateAwaitingInfo)
			if got := cwsWorkState(t, pool, ty.table, ty.id); got != "PAUSED" {
				t.Errorf("after awaiting_info: work_state = %s, want PAUSED", got)
			}

			// (iii) resuming to work_in_progress keeps the paused value.
			setState(t, ty.id, domain.CaseStateWorkInProgress)
			if got := cwsWorkState(t, pool, ty.table, ty.id); got != "PAUSED" {
				t.Errorf("after resume to work_in_progress: work_state = %s, want PAUSED", got)
			}

			// (ii) any other non-work_in_progress state clears it.
			setOngoing(t, ty.id)
			setState(t, ty.id, domain.CaseStateOpen)
			if got := cwsWorkState(t, pool, ty.table, ty.id); got != "NULL" {
				t.Errorf("after open: work_state = %s, want NULL", got)
			}
		})
	}
}

func TestCaseWorkStateIntegration_OneOngoingGuardIgnoresNonWorkInProgress(t *testing.T) {
	pool := caseStatsPool(t)
	seedCaseWorkStateFixture(t, pool)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	ctx := repository.WithCallerIdentity(context.Background(), repository.SearchScope{Unrestricted: false, ViewerEmail: cleMember})
	sys := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)

	// A stale row: awaiting_info yet work_state ONGOING, as written before the fix.
	if _, err := scoped.Exec(sys, `UPDATE service_request SET state = 'AWAITING_INFO', work_state = 'ONGOING' WHERE id = $1`, cleServiceReq); err != nil {
		t.Fatalf("seed stale row: %v", err)
	}
	inProgress := domain.CaseStateWorkInProgress
	ongoing := domain.CaseWorkStateOngoing
	if _, _, err := repo.UpdateCase(ctx, domain.UpdateCaseRequest{ID: cleEngagement, State: &inProgress}, nil); err != nil {
		t.Fatalf("engagement work_in_progress: %v", err)
	}
	if _, _, err := repo.UpdateCase(ctx, domain.UpdateCaseRequest{ID: cleEngagement, WorkState: &ongoing}, nil); err != nil {
		t.Fatalf("engagement Ongoing must not trip on the stale awaiting_info row: %v", err)
	}

	// A genuine work_in_progress + ONGOING row still trips the guard.
	if _, _, err := repo.UpdateCase(ctx, domain.UpdateCaseRequest{ID: cleSecReport, State: &inProgress}, nil); err != nil {
		t.Fatalf("security report work_in_progress: %v", err)
	}
	_, _, err := repo.UpdateCase(ctx, domain.UpdateCaseRequest{ID: cleSecReport, WorkState: &ongoing}, nil)
	var conflict *apierror.ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("Ongoing beside a real work_in_progress Ongoing item: err = %v, want *apierror.ConflictError", err)
	}
}
