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

// A write to a case-like work item dispatches on the extension row that exists,
// not on work_item.type. The two disagree on some rows (a work item typed CASE
// whose only row is a service request's, or the reverse): every read COALESCEs
// across the extension tables and shows such a ticket with its state, while a
// write that picked its table from work_item.type updated zero rows and
// answered 404 "case not found" for a ticket the same caller had just read --
// confirmed live on a service request. These tests seed both shapes and run
// against a real Postgres, as a superuser and as the plain application role
// alike. Skipped without CASE_STATS_TEST_DSN.
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run CaseUpdateDispatchIntegration

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
	udSRUnderCaseID = "93000000-0000-0000-0000-000000000001" // work_item CASE, only a service_request row
	udCaseUnderSRID = "93000000-0000-0000-0000-000000000002" // work_item SERVICE_REQUEST, only a "case" row
	udNoRowID       = "93000000-0000-0000-0000-000000000003" // work_item CASE, no extension row at all
	udMissingID     = "93000000-0000-0000-0000-0000000000ff" // no work_item at all
	udUserID        = "93000000-0000-0000-0000-0000000000a1"
	udActor         = "dispatch.tester@example.com"
)

var udAllIDs = []string{udSRUnderCaseID, udCaseUnderSRID, udNoRowID}

type udFixture struct {
	pool   *pgxpool.Pool
	scoped *repository.Scoped
	repo   repository.CaseRepository
	ctx    context.Context
}

func (f *udFixture) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := f.scoped.Exec(f.ctx, sql, args...); err != nil {
		t.Fatalf("exec (%.90s): %v", sql, err)
	}
}

func (f *udFixture) clean() {
	for _, id := range udAllIDs {
		_, _ = f.scoped.Exec(f.ctx, `DELETE FROM work_item WHERE id = $1`, id)
	}
	_, _ = f.scoped.Exec(f.ctx, `DELETE FROM "user" WHERE id = $1`, udUserID)
}

// newUDFixture seeds the two disagreeing shapes and a work item with no
// extension row at all. The extension rows are inserted directly: no create path
// of this service writes them this way, the sync does.
func newUDFixture(t *testing.T) *udFixture {
	t.Helper()
	pool := caseStatsPool(t)
	f := &udFixture{
		pool:   pool,
		scoped: repository.NewScoped(pool),
		ctx:    repository.WithSystemIdentity(context.Background()),
	}
	f.repo = repository.NewCaseRepository(f.scoped)
	f.clean()
	t.Cleanup(f.clean)

	now := time.Now().UTC()
	// closed_by_user_id references "user", and a freshly migrated database has none.
	f.exec(t, `INSERT INTO "user" (id, created_on, updated_on, user_name, email, is_active)
		VALUES ($1, $2, $2, 'update.dispatch.fixture', $3, TRUE)`, udUserID, now, udActor)
	workItem := func(id, number, kind string) {
		f.exec(t, `INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type)
			VALUES ($1, $2, $2, 'test', 'test', $3, $3, 'update dispatch fixture', $4::work_item_type_enum)`, id, now, number, kind)
	}

	workItem(udSRUnderCaseID, "UD-TEST-0001", "CASE")
	f.exec(t, `INSERT INTO service_request (id, state) VALUES ($1, 'WORK_IN_PROGRESS')`, udSRUnderCaseID)

	workItem(udCaseUnderSRID, "UD-TEST-0002", "SERVICE_REQUEST")
	f.exec(t, `INSERT INTO "case" (id, severity, issue_type, state) VALUES ($1, 'S3', 'ERROR', 'OPEN')`, udCaseUnderSRID)

	workItem(udNoRowID, "UD-TEST-0003", "CASE")
	return f
}

func (f *udFixture) column(t *testing.T, table, column, id string) *string {
	t.Helper()
	var got *string
	if err := f.scoped.QueryRow(f.ctx, `SELECT `+column+`::TEXT FROM `+table+` WHERE id = $1`, id).Scan(&got); err != nil {
		t.Fatalf("read %s.%s: %v", table, column, err)
	}
	return got
}

func (f *udFixture) view(t *testing.T, id string) domain.CaseView {
	t.Helper()
	v, err := f.repo.GetCaseByID(f.ctx, id, repository.SearchScope{Unrestricted: true})
	if err != nil {
		t.Fatalf("GetCaseByID(%s): %v", id, err)
	}
	return v
}

func udState(s domain.CaseState) *domain.CaseState { return &s }

func udStr(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}

// A service request whose work_item.type says CASE reads with its state, and
// every state change the portal makes -- waiting on WSO2, solution proposed,
// closed -- lands on the service_request row. Before the fix each one was a 404.
// work_item.type is left as it was: the sync owns it.
func TestCaseUpdateDispatchIntegration_ServiceRequestRowUnderCaseType(t *testing.T) {
	f := newUDFixture(t)
	actor := udUserID

	if v := f.view(t, udSRUnderCaseID); v.State == nil || *v.State != domain.CaseStateWorkInProgress {
		t.Fatalf("GetCaseByID state = %v, want work_in_progress (read from the service_request row)", v.State)
	}

	for _, want := range []domain.CaseState{domain.CaseStateWaitingOnWSO2, domain.CaseStateSolutionProposed, domain.CaseStateClosed} {
		c, _, err := f.repo.UpdateCase(f.ctx, domain.UpdateCaseRequest{ID: udSRUnderCaseID, State: udState(want)}, &actor)
		if err != nil {
			t.Fatalf("UpdateCase to %s: %v", want, err)
		}
		if c.State == nil || *c.State != want {
			t.Fatalf("UpdateCase to %s returned state %v", want, c.State)
		}
		if got := f.column(t, "service_request", "state", udSRUnderCaseID); udStr(got) != string(upper(string(want))) {
			t.Errorf("service_request.state after %s = %s", want, udStr(got))
		}
		if v := f.view(t, udSRUnderCaseID); v.State == nil || *v.State != want {
			t.Errorf("GetCaseByID after %s shows state %v", want, v.State)
		}
	}
	if got := f.column(t, "service_request", "closed_by_user_id", udSRUnderCaseID); udStr(got) != udUserID {
		t.Errorf("service_request.closed_by_user_id = %s, want the actor", udStr(got))
	}
	if got := f.column(t, "service_request", "closed_on", udSRUnderCaseID); got == nil {
		t.Error("service_request.closed_on is still NULL after closing")
	}
	if got := f.column(t, "work_item", "type", udSRUnderCaseID); udStr(got) != "CASE" {
		t.Errorf("work_item.type was rewritten to %s; the sync owns it", udStr(got))
	}
}

// The ongoing work state (serialized per assignee) and the auto-closure hold
// (UpdateCaseFields) pick their table the same way.
func TestCaseUpdateDispatchIntegration_WorkStateAndAutocloseHoldFollowTheRow(t *testing.T) {
	f := newUDFixture(t)
	actor := udUserID

	ws := domain.CaseWorkStateOngoing
	c, _, err := f.repo.UpdateCase(f.ctx, domain.UpdateCaseRequest{ID: udSRUnderCaseID, WorkState: &ws}, &actor)
	if err != nil {
		t.Fatalf("UpdateCase workState ongoing: %v", err)
	}
	if c.WorkState == nil || *c.WorkState != ws {
		t.Errorf("UpdateCase returned work state %v", c.WorkState)
	}
	if got := f.column(t, "service_request", "work_state", udSRUnderCaseID); udStr(got) != "ONGOING" {
		t.Errorf("service_request.work_state = %s", udStr(got))
	}

	hold := time.Date(2026, time.December, 24, 10, 0, 0, 0, time.UTC)
	if _, err := f.repo.UpdateCaseFields(f.ctx, domain.UpdateCaseRequest{ID: udSRUnderCaseID, AutocloseHoldUntil: &hold}, actor, udActor); err != nil {
		t.Fatalf("UpdateCaseFields autocloseHoldUntil: %v", err)
	}
	if got := f.column(t, "service_request", "autoclosure_step", udSRUnderCaseID); udStr(got) != "ON_HOLD" {
		t.Errorf("service_request.autoclosure_step = %s", udStr(got))
	}
}

// The mirror shape: a "case" row under a work item typed SERVICE_REQUEST. The
// state and the severity (a column only "case" has) both land on the "case" row.
func TestCaseUpdateDispatchIntegration_CaseRowUnderServiceRequestType(t *testing.T) {
	f := newUDFixture(t)
	actor := udUserID

	if v := f.view(t, udCaseUnderSRID); v.State == nil || *v.State != domain.CaseStateOpen {
		t.Fatalf("GetCaseByID state = %v, want open (read from the \"case\" row)", v.State)
	}
	c, _, err := f.repo.UpdateCase(f.ctx, domain.UpdateCaseRequest{ID: udCaseUnderSRID, State: udState(domain.CaseStateWaitingOnWSO2)}, &actor)
	if err != nil {
		t.Fatalf("UpdateCase to waiting_on_wso2: %v", err)
	}
	if c.State == nil || *c.State != domain.CaseStateWaitingOnWSO2 {
		t.Errorf("UpdateCase returned state %v", c.State)
	}
	if got := f.column(t, `"case"`, "state", udCaseUnderSRID); udStr(got) != "WAITING_ON_WSO2" {
		t.Errorf(`"case".state = %s`, udStr(got))
	}

	sev := domain.CaseSeverityHigh
	c, previous, err := f.repo.UpdateCase(f.ctx, domain.UpdateCaseRequest{ID: udCaseUnderSRID, Severity: &sev}, &actor)
	if err != nil {
		t.Fatalf("UpdateCase severity: %v", err)
	}
	if c.Severity == nil || *c.Severity != sev {
		t.Errorf("UpdateCase returned severity %v", c.Severity)
	}
	if previous == nil || *previous != domain.CaseSeverityMedium {
		t.Errorf("previous severity = %v, want medium (S3)", previous)
	}
	if got := f.column(t, "work_item", "type", udCaseUnderSRID); udStr(got) != "SERVICE_REQUEST" {
		t.Errorf("work_item.type was rewritten to %s; the sync owns it", udStr(got))
	}
}

// A work item with no extension row at all, and an id that does not exist, are
// still 404: dispatching on the row that exists never invents one.
func TestCaseUpdateDispatchIntegration_StillNotFoundWithoutARow(t *testing.T) {
	f := newUDFixture(t)
	actor := udUserID
	hold := time.Date(2026, time.December, 24, 10, 0, 0, 0, time.UTC)

	for _, id := range []string{udNoRowID, udMissingID} {
		var nf *apierror.NotFoundError
		if _, _, err := f.repo.UpdateCase(f.ctx, domain.UpdateCaseRequest{ID: id, State: udState(domain.CaseStateClosed)}, &actor); !errors.As(err, &nf) {
			t.Errorf("UpdateCase(%s) = %v, want NotFoundError", id, err)
		}
		if _, err := f.repo.UpdateCaseFields(f.ctx, domain.UpdateCaseRequest{ID: id, AutocloseHoldUntil: &hold}, actor, udActor); !errors.As(err, &nf) {
			t.Errorf("UpdateCaseFields(%s) = %v, want NotFoundError", id, err)
		}
	}
	if got := f.column(t, "work_item", "type", udNoRowID); udStr(got) != "CASE" {
		t.Errorf("work_item.type = %s, want CASE untouched", udStr(got))
	}
}

// The type transfer reads and replaces the row that exists too. A service
// request under a CASE work item moved to an engagement carries its state over
// and leaves no service_request row behind; asked to become the type its row
// already has, it is refused rather than given a second row.
func TestCaseUpdateDispatchIntegration_TransferFollowsTheRow(t *testing.T) {
	f := newUDFixture(t)
	transfer, ok := f.repo.(repository.CaseTypeTransferRepository)
	if !ok {
		t.Fatal("the case repository does not implement CaseTypeTransferRepository")
	}

	var ve *apierror.ValidationError
	same := repository.CaseTypeTransfer{CaseID: udSRUnderCaseID, TargetType: "service_request", ActorEmail: udActor}
	if _, err := transfer.TransferCaseType(f.ctx, same, nil); !errors.As(err, &ve) {
		t.Errorf("transfer to the type the row already has = %v, want ValidationError", err)
	}

	result, err := transfer.TransferCaseType(f.ctx, ctEngagementPlan(udSRUnderCaseID), nil)
	if err != nil {
		t.Fatalf("transfer to engagement: %v", err)
	}
	if result.PreviousType != "service_request" {
		t.Errorf("PreviousType = %q, want service_request (the row that existed)", result.PreviousType)
	}
	if result.State == nil || *result.State != domain.CaseStateWorkInProgress {
		t.Errorf("State = %v, want work_in_progress carried over", result.State)
	}
	if got := f.column(t, "engagement", "state", udSRUnderCaseID); udStr(got) != "WORK_IN_PROGRESS" {
		t.Errorf("engagement.state = %s, want WORK_IN_PROGRESS carried over", udStr(got))
	}
	var leftBehind int
	if err := f.scoped.QueryRow(f.ctx, `SELECT COUNT(*) FROM service_request WHERE id = $1`, udSRUnderCaseID).Scan(&leftBehind); err != nil {
		t.Fatalf("count service_request rows: %v", err)
	}
	if leftBehind != 0 {
		t.Errorf("service_request still holds %d row(s) after the transfer", leftBehind)
	}
	if got := f.column(t, "work_item", "type", udSRUnderCaseID); udStr(got) != "ENGAGEMENT" {
		t.Errorf("work_item.type = %s, want ENGAGEMENT", udStr(got))
	}
}
