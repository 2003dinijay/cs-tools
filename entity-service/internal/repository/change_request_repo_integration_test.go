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
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// Skipped without CHANGE_REQUEST_TEST_DSN, the same convention every other
// repository integration test in this package uses.
//
//	CHANGE_REQUEST_TEST_DSN=postgres://... go test ./internal/repository/ -run ChangeRequestIntegration

const (
	changeRequestApprovalTestID         = "36666666-0000-0000-0000-000000000001"
	changeRequestApprovalApproverUserID = "36666666-0000-0000-0000-000000000003"
)

// seedApprovalUserForDecisionTest inserts the one "user" row
// approval_stage_approver.approver_user_id's FK needs -- deliberately its own
// fresh row (not relying on any of the local docker-compose stack's own
// incidental seed users) so this test only depends on migrations having run,
// the same assumption every other integration test in this package makes.
func seedApprovalUserForDecisionTest(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()

	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM "user" WHERE id = $1`, changeRequestApprovalApproverUserID)
	}
	cleanup()
	t.Cleanup(cleanup)

	if _, err := pool.Exec(ctx,
		`INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by, user_name, name, first_name, last_name, email, is_active, is_system_user)
		 VALUES ($1, now(), now(), 'cr-approval-test', 'cr-approval-test', 'cr-approval-test@example.com', 'CR Approval Test', 'CR', 'Approval Test', 'cr-approval-test@example.com', true, false)`,
		changeRequestApprovalApproverUserID); err != nil {
		t.Fatalf("seed approver user: %v", err)
	}
}

// seedChangeRequestForApprovalTest inserts a minimal work_item/change_request
// pair in the given state -- enough for PatchChangeRequest's own read (via
// GetChangeRequestByID at the end of a successful patch) to resolve, since
// every other join in changeRequestFromJoins is a LEFT JOIN.
func seedChangeRequestForApprovalTest(t *testing.T, pool *pgxpool.Pool, state string) {
	t.Helper()
	ctx := context.Background()

	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM work_item WHERE id = $1`, changeRequestApprovalTestID)
	}
	cleanup()
	t.Cleanup(cleanup)

	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed (%.60s): %v", sql, err)
		}
	}

	mustExec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type)
	          VALUES ($1, now(), now(), 'cr-approval-test', 'cr-approval-test', 'CRAPPRV01', 'approval guard test', 'CHANGE_REQUEST')`,
		changeRequestApprovalTestID)
	mustExec(`INSERT INTO change_request (id, state) VALUES ($1, $2::change_request_state_enum)`,
		changeRequestApprovalTestID, state)
}

// TestChangeRequestIntegration_RequestApprovalIsBookkeepingOnly confirms the
// corrected behavior: {requestApproval: true} always sets
// change_request.approval = 'REQUESTED' and never touches state, regardless
// of the change request's current state. This replaces two prior tests
// (TestChangeRequestIntegration_RequestApprovalRejectsNonNewState/
// TestChangeRequestIntegration_RequestApprovalAdvancesNewToAssess) that
// asserted the earlier, now-confirmed-wrong model -- PatchChangeRequest used
// to also force state=ASSESS for a change request in New (and reject the
// request with a ConflictError for any other state) modeling New->Assess as
// an approval-gated ceremony. Checked against the real ServiceNow instance:
// New->Assess is a plain, ungated state change like every other transition,
// unrelated to approval at all -- the one real approval-gated transition is
// Assess->Authorize, handled entirely by DecideChangeRequestApproval, which
// this test does not touch.
func TestChangeRequestIntegration_RequestApprovalIsBookkeepingOnly(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	repo := repository.NewChangeRequestRepository(pool)

	for _, seedState := range []string{"NEW", "REVIEW", "CLOSED"} {
		t.Run(seedState, func(t *testing.T) {
			seedChangeRequestForApprovalTest(t, pool, seedState)

			yes := true
			_, err := repo.PatchChangeRequest(context.Background(), changeRequestApprovalTestID,
				domain.PatchChangeRequestRequest{RequestApproval: &yes}, "cr-approval-test")
			if err != nil {
				t.Fatalf("PatchChangeRequest(requestApproval=true) on a %s-state change request: %v", seedState, err)
			}

			var gotState, gotApproval *string
			if scanErr := pool.QueryRow(context.Background(),
				`SELECT state::TEXT, approval::TEXT FROM change_request WHERE id = $1`, changeRequestApprovalTestID).
				Scan(&gotState, &gotApproval); scanErr != nil {
				t.Fatalf("read back state/approval: %v", scanErr)
			}
			if gotState == nil || *gotState != seedState {
				t.Fatalf("state after requestApproval=true = %v, want unchanged %q", gotState, seedState)
			}
			if gotApproval == nil || *gotApproval != "REQUESTED" {
				t.Fatalf("approval after requestApproval=true = %v, want \"REQUESTED\"", gotApproval)
			}
		})
	}
}

// seedApprovalStageForDecisionTest inserts one approval_stage plus one or
// more approval_stage_approver rows for changeRequestApprovalTestID -- the
// same shared seed changeRequestForApprovalTest/seedChangeRequestForApprovalTest
// use, extended with an actual approval stage so DecideChangeRequestApproval
// has something real to act on. Each entry in approverUserIDs gets its own
// requested approver row; the returned stage id lets a test seed additional
// rows (e.g. a second approver already rejected) directly.
func seedApprovalStageForDecisionTest(t *testing.T, pool *pgxpool.Pool, approverUserIDs ...string) string {
	t.Helper()
	ctx := context.Background()

	stageID := "36666666-0000-0000-0000-000000000002"
	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed (%.60s): %v", sql, err)
		}
	}
	mustExec(`INSERT INTO approval_stage (id, created_on, updated_on, created_by, updated_by, work_item_id, raw_status)
	          VALUES ($1, now(), now(), 'cr-approval-test', 'cr-approval-test', $2, 'requested')`,
		stageID, changeRequestApprovalTestID)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM approval_stage WHERE id = $1`, stageID)
	})

	for i, userID := range approverUserIDs {
		mustExec(`INSERT INTO approval_stage_approver (id, created_on, updated_on, created_by, updated_by, stage_id, work_item_id, approver_user_id, status)
		          VALUES ($1, now(), now(), 'cr-approval-test', 'cr-approval-test', $2, $3, $4, 'requested')`,
			fmt.Sprintf("36666666-0000-0000-0000-00000000%04d", i+10), stageID, changeRequestApprovalTestID, userID)
	}
	return stageID
}

// TestChangeRequestIntegration_DecideApprovalCascadesAssessToAuthorize is the
// regression guard for a real, reported gap: DecideChangeRequestApproval used
// to only flip the one approval_stage_approver row and never touch
// change_request.state at all -- confirmed live against a real approval on
// this data source. Approving the sole (or last-standing) approver on a
// change request's Assess-stage approval, while it's actually sitting in
// Assess, must now advance it to Authorize.
func TestChangeRequestIntegration_DecideApprovalCascadesAssessToAuthorize(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	repo := repository.NewChangeRequestRepository(pool)
	seedApprovalUserForDecisionTest(t, pool)
	seedChangeRequestForApprovalTest(t, pool, "ASSESS")
	seedApprovalStageForDecisionTest(t, pool, changeRequestApprovalApproverUserID)

	if _, err := repo.DecideChangeRequestApproval(context.Background(), changeRequestApprovalTestID,
		changeRequestApprovalApproverUserID, "approved", "cr-approval-test"); err != nil {
		t.Fatalf("DecideChangeRequestApproval(approved): %v", err)
	}

	var gotState string
	if scanErr := pool.QueryRow(context.Background(),
		`SELECT state::TEXT FROM change_request WHERE id = $1`, changeRequestApprovalTestID).Scan(&gotState); scanErr != nil {
		t.Fatalf("read back state: %v", scanErr)
	}
	if gotState != "AUTHORIZE" {
		t.Fatalf("state after approval = %q, want \"AUTHORIZE\"", gotState)
	}
}

// TestChangeRequestIntegration_DecideApprovalRejectionDoesNotCascade confirms
// a rejection never advances state, even though it resolves the stage (the
// same first-responder-wins quorum rule that lets a single approval resolve
// one also lets a single rejection resolve one) -- there is no confirmed
// target for a rejected Assess stage in this schema, so this must be a
// pure no-op on change_request.state.
func TestChangeRequestIntegration_DecideApprovalRejectionDoesNotCascade(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	repo := repository.NewChangeRequestRepository(pool)
	seedApprovalUserForDecisionTest(t, pool)
	seedChangeRequestForApprovalTest(t, pool, "ASSESS")
	seedApprovalStageForDecisionTest(t, pool, changeRequestApprovalApproverUserID)

	if _, err := repo.DecideChangeRequestApproval(context.Background(), changeRequestApprovalTestID,
		changeRequestApprovalApproverUserID, "rejected", "cr-approval-test"); err != nil {
		t.Fatalf("DecideChangeRequestApproval(rejected): %v", err)
	}

	var gotState string
	if scanErr := pool.QueryRow(context.Background(),
		`SELECT state::TEXT FROM change_request WHERE id = $1`, changeRequestApprovalTestID).Scan(&gotState); scanErr != nil {
		t.Fatalf("read back state: %v", scanErr)
	}
	if gotState != "ASSESS" {
		t.Fatalf("state after rejection = %q, want unchanged \"ASSESS\"", gotState)
	}
}

// TestChangeRequestIntegration_DecideApprovalDoesNotCascadeOutsideAssess
// confirms the cascade is scoped exactly to Assess->Authorize: approving an
// approver on a change request that isn't currently in Assess (e.g. one
// already sitting in Authorize, mid its own separate approval stage) must
// leave state untouched -- this repository deliberately does not attempt
// Authorize's own outgoing cascade yet.
func TestChangeRequestIntegration_DecideApprovalDoesNotCascadeOutsideAssess(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	repo := repository.NewChangeRequestRepository(pool)
	seedApprovalUserForDecisionTest(t, pool)
	seedChangeRequestForApprovalTest(t, pool, "AUTHORIZE")
	seedApprovalStageForDecisionTest(t, pool, changeRequestApprovalApproverUserID)

	if _, err := repo.DecideChangeRequestApproval(context.Background(), changeRequestApprovalTestID,
		changeRequestApprovalApproverUserID, "approved", "cr-approval-test"); err != nil {
		t.Fatalf("DecideChangeRequestApproval(approved): %v", err)
	}

	var gotState string
	if scanErr := pool.QueryRow(context.Background(),
		`SELECT state::TEXT FROM change_request WHERE id = $1`, changeRequestApprovalTestID).Scan(&gotState); scanErr != nil {
		t.Fatalf("read back state: %v", scanErr)
	}
	if gotState != "AUTHORIZE" {
		t.Fatalf("state after approval outside Assess = %q, want unchanged \"AUTHORIZE\"", gotState)
	}
}
