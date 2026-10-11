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

// The customer-reply state transition end to end on a real database: the
// real caseService over the real CaseRepository/UserRepository, a registered
// project contact posting a comment through CreateCaseComment, and the rows
// it leaves behind -- the record's state, the activity-feed entry and the
// comment itself. Run as the superuser and as the application role
// (csm_app, row-level security enforced). Skipped unless
// ENTITY_TEST_DATABASE_URL is set.
//
//	ENTITY_TEST_DATABASE_URL=postgres://... go test ./internal/service/ -run CustomerReplyIntegration

package service

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	crAccountID       = "c0ee0000-0000-4000-8000-000000000001"
	crContactID       = "c0ee0000-0000-4000-8000-000000000002"
	crProjectID       = "c0ee0000-0000-4000-8000-000000000003"
	crCustomerUserID  = "c0ee0000-0000-4000-8000-000000000004"
	crStaffUserID     = "c0ee0000-0000-4000-8000-000000000005"
	crCaseAwaiting    = "c0ee0000-0000-4000-8000-000000000011"
	crCaseProposed    = "c0ee0000-0000-4000-8000-000000000012"
	crSRAwaiting      = "c0ee0000-0000-4000-8000-000000000013"
	crSRProposed      = "c0ee0000-0000-4000-8000-000000000014"
	crCaseClosed      = "c0ee0000-0000-4000-8000-000000000015"
	crCaseWaiting     = "c0ee0000-0000-4000-8000-000000000016"
	crMismatchedCase  = "c0ee0000-0000-4000-8000-000000000017"
	crCaseOpen        = "c0ee0000-0000-4000-8000-000000000018"
	crStaffCase       = "c0ee0000-0000-4000-8000-000000000019"
	crWorkNoteCase    = "c0ee0000-0000-4000-8000-00000000001a"
	crCustomerEmail   = "cr-customer@test.local"
	crStaffEmail      = "cr-staff@test.local"
	crWorkItemIDsList = `('` + crCaseAwaiting + `','` + crCaseProposed + `','` + crSRAwaiting + `','` + crSRProposed + `','` +
		crCaseClosed + `','` + crCaseWaiting + `','` + crMismatchedCase + `','` + crCaseOpen + `','` + crStaffCase + `','` + crWorkNoteCase + `')`
)

// ctxIdentityAccess resolves the scope the identity middleware stamped on the
// context (repository.WithCallerIdentity), exactly as the real
// accessService.ResolveScope does first; nothing stamped is an error, as for
// a request with no verified identity.
type ctxIdentityAccess struct{}

func (ctxIdentityAccess) ResolveScope(ctx context.Context) (AccessScope, error) {
	if scope, ok := repository.CallerIdentityFromContext(ctx); ok {
		return scope, nil
	}
	return AccessScope{}, repository.ErrNoCallerIdentity
}

func newCustomerReplyIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL is not set; skipping the live-database tests")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// seedCustomerReplyFixture: one project with one REGISTERED contact (the
// customer, who also has a "user" row so the comment's author resolves), one
// staff user, and one case-like work item per scenario. Row-level-security
// tables are written as the system identity; the rest straight on the pool.
func seedCustomerReplyFixture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)

	cleanup := func() {
		for _, stmt := range []string{
			`DELETE FROM work_item_activity WHERE work_item_id IN ` + crWorkItemIDsList,
			`DELETE FROM comment WHERE work_item_id IN ` + crWorkItemIDsList,
			`DELETE FROM work_item_watcher WHERE work_item_id IN ` + crWorkItemIDsList,
			`DELETE FROM "case" WHERE id IN ` + crWorkItemIDsList,
			`DELETE FROM service_request WHERE id IN ` + crWorkItemIDsList,
			`DELETE FROM work_item WHERE id IN ` + crWorkItemIDsList,
		} {
			_, _ = scoped.Exec(ctx, stmt)
		}
		_, _ = pool.Exec(ctx, `DELETE FROM project_contact WHERE project_id = $1`, crProjectID)
		_, _ = pool.Exec(ctx, `DELETE FROM project WHERE id = $1`, crProjectID)
		_, _ = pool.Exec(ctx, `DELETE FROM account_contact WHERE id = $1`, crContactID)
		_, _ = pool.Exec(ctx, `DELETE FROM account WHERE id = $1`, crAccountID)
		_, _ = pool.Exec(ctx, `DELETE FROM "user" WHERE id IN ($1, $2)`, crCustomerUserID, crStaffUserID)
	}
	cleanup()
	t.Cleanup(cleanup)

	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed (%.80s): %v", sql, err)
		}
	}
	mustExecScoped := func(sql string, args ...any) {
		t.Helper()
		if _, err := scoped.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed scoped (%.80s): %v", sql, err)
		}
	}
	now := time.Now().UTC()

	mustExec(`INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number, sf_id)
		VALUES ($1, $2, $2, 'test', 'test', 'Customer Reply Test Account', 'CR-ACC-1', 'sf-cr-acc-1')`, crAccountID, now)
	mustExec(`INSERT INTO account_contact (id, created_on, updated_on, created_by, updated_by, user_name, account_id)
		VALUES ($1, $2, $2, 'test', 'test', 'Customer Reply Test Contact', $3)`, crContactID, now, crAccountID)
	mustExec(`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, sf_id, account_id)
		VALUES ($1, $2, $2, 'test', 'test', 'CRTEST', 'sf-crtest', $3)`, crProjectID, now, crAccountID)
	mustExec(`INSERT INTO project_contact (id, created_on, updated_on, created_by, updated_by, email, account_contact_id, project_id, state)
		VALUES (gen_random_uuid(), $1, $1, 'test', 'test', $2, $3, $4, 'REGISTERED')`, now, crCustomerEmail, crContactID, crProjectID)
	mustExec(`INSERT INTO "user" (id, created_on, updated_on, user_name, first_name, last_name, email, user_type)
		VALUES ($1, $2, $2, 'cr-customer', 'Dave', 'Mendis', $3, 'EXTERNAL')`, crCustomerUserID, now, crCustomerEmail)
	mustExec(`INSERT INTO "user" (id, created_on, updated_on, user_name, first_name, last_name, email, user_type)
		VALUES ($1, $2, $2, 'cr-staff', 'Jane', 'Doe', $3, 'INTERNAL')`, crStaffUserID, now, crStaffEmail)

	workItem := func(id, number, typ string) {
		mustExecScoped(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, project_id)
			VALUES ($1, $2, $2, 'test', 'test', $3, $4, 'customer reply integration test fixture', $5::work_item_type_enum, $6)`,
			id, now, number, "WSO2-"+number, typ, crProjectID)
	}
	workItem(crCaseAwaiting, "CR-CASE-AWAITING", "CASE")
	mustExecScoped(`INSERT INTO "case" (id, state) VALUES ($1, 'AWAITING_INFO')`, crCaseAwaiting)
	workItem(crCaseProposed, "CR-CASE-PROPOSED", "CASE")
	mustExecScoped(`INSERT INTO "case" (id, state) VALUES ($1, 'SOLUTION_PROPOSED')`, crCaseProposed)
	workItem(crSRAwaiting, "CR-SR-AWAITING", "SERVICE_REQUEST")
	mustExecScoped(`INSERT INTO service_request (id, state) VALUES ($1, 'AWAITING_INFO')`, crSRAwaiting)
	workItem(crSRProposed, "CR-SR-PROPOSED", "SERVICE_REQUEST")
	mustExecScoped(`INSERT INTO service_request (id, state) VALUES ($1, 'SOLUTION_PROPOSED')`, crSRProposed)
	workItem(crCaseClosed, "CR-CASE-CLOSED", "CASE")
	mustExecScoped(`INSERT INTO "case" (id, state, closed_on) VALUES ($1, 'CLOSED', $2)`, crCaseClosed, now)
	workItem(crCaseWaiting, "CR-CASE-WAITING", "CASE")
	mustExecScoped(`INSERT INTO "case" (id, state) VALUES ($1, 'WAITING_ON_WSO2')`, crCaseWaiting)
	workItem(crCaseOpen, "CR-CASE-OPEN", "CASE")
	mustExecScoped(`INSERT INTO "case" (id, state) VALUES ($1, 'OPEN')`, crCaseOpen)
	workItem(crStaffCase, "CR-CASE-STAFF", "CASE")
	mustExecScoped(`INSERT INTO "case" (id, state) VALUES ($1, 'AWAITING_INFO')`, crStaffCase)
	workItem(crWorkNoteCase, "CR-CASE-WORKNOTE", "CASE")
	mustExecScoped(`INSERT INTO "case" (id, state) VALUES ($1, 'AWAITING_INFO')`, crWorkNoteCase)
	// The pre-prod shape: work_item.type says CASE but the only extension row
	// is a service request's. UpdateCase dispatches on the row that exists.
	workItem(crMismatchedCase, "CR-MISMATCHED", "CASE")
	mustExecScoped(`INSERT INTO service_request (id, state) VALUES ($1, 'AWAITING_INFO')`, crMismatchedCase)
}

// crRecordState reads the state of a case-like record from whichever
// extension row it has, as the system identity.
func crRecordState(t *testing.T, scoped *repository.Scoped, id string) string {
	t.Helper()
	var state string
	err := scoped.QueryRow(repository.WithSystemIdentity(context.Background()), `
		SELECT COALESCE(c.state::TEXT, sr.state::TEXT, '')
		FROM work_item wi
		LEFT JOIN "case" c ON c.id = wi.id
		LEFT JOIN service_request sr ON sr.id = wi.id
		WHERE wi.id = $1`, id).Scan(&state)
	if err != nil {
		t.Fatalf("read state of %s: %v", id, err)
	}
	return state
}

type crActivityRow struct {
	oldValue, newValue, userEmail string
}

// crStateActivity returns every "state" activity-feed entry of the record.
func crStateActivity(t *testing.T, scoped *repository.Scoped, id string) []crActivityRow {
	t.Helper()
	rows, err := scoped.Query(repository.WithSystemIdentity(context.Background()), `
		SELECT old_value, new_value, user_email FROM work_item_activity
		WHERE work_item_id = $1 AND field_name = 'state' ORDER BY created_on`, id)
	if err != nil {
		t.Fatalf("read activity of %s: %v", id, err)
	}
	defer rows.Close()
	var out []crActivityRow
	for rows.Next() {
		var r crActivityRow
		if err := rows.Scan(&r.oldValue, &r.newValue, &r.userEmail); err != nil {
			t.Fatalf("scan activity: %v", err)
		}
		out = append(out, r)
	}
	return out
}

func crCommentCount(t *testing.T, scoped *repository.Scoped, id string) int {
	t.Helper()
	var n int
	if err := scoped.QueryRow(repository.WithSystemIdentity(context.Background()),
		`SELECT count(*) FROM comment WHERE work_item_id = $1`, id).Scan(&n); err != nil {
		t.Fatalf("count comments of %s: %v", id, err)
	}
	return n
}

func newCustomerReplyIntegrationService(pool *pgxpool.Pool) CaseService {
	scoped := repository.NewScoped(pool)
	return NewCaseService(repository.NewCaseRepository(scoped), repository.NewUserRepository(pool), nil, ctxIdentityAccess{}, repository.NewProjectContactRepository(pool))
}

// crCustomerContext is the request context of the registered contact posting
// through the customer portal: their x-user-id-token (the comment's author)
// and the project-scoped, non-internal identity the middleware stamps.
func crCustomerContext(t *testing.T) context.Context {
	t.Helper()
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, crCustomerEmail))
	return repository.WithCallerIdentity(ctx, repository.SearchScope{ProjectIDs: []string{crProjectID}, ViewerEmail: crCustomerEmail, ViaCustomerPortal: true})
}

func crStaffContext(t *testing.T) context.Context {
	t.Helper()
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, crStaffEmail))
	return repository.WithCallerIdentity(ctx, repository.SearchScope{Unrestricted: true, HasInternalAccess: true, ViewerEmail: crStaffEmail})
}

// TestCustomerReplyIntegration_CustomerCommentMovesAwaitingRecordsToWaitingOnWSO2
// is the rule's table against real rows: a case and a service request, from
// each of the two states, move and get the one lifecycle entry a manual move
// writes (with the customer as the actor); a record whose work_item.type
// disagrees with its extension row moves too; and Open, Closed and already
// Waiting on WSO2 records are left exactly as they were -- with the comment
// itself written in every case.
func TestCustomerReplyIntegration_CustomerCommentMovesAwaitingRecordsToWaitingOnWSO2(t *testing.T) {
	pool := newCustomerReplyIntegrationPool(t)
	seedCustomerReplyFixture(t, pool)
	scoped := repository.NewScoped(pool)
	svc := newCustomerReplyIntegrationService(pool)

	rows := []struct {
		name  string
		id    string
		from  string
		label string
		moves bool
	}{
		{"case from Awaiting Info", crCaseAwaiting, "AWAITING_INFO", "Awaiting Info", true},
		{"case from Solution Proposed", crCaseProposed, "SOLUTION_PROPOSED", "Solution Proposed", true},
		{"service request from Awaiting Info", crSRAwaiting, "AWAITING_INFO", "Awaiting Info", true},
		{"service request from Solution Proposed", crSRProposed, "SOLUTION_PROPOSED", "Solution Proposed", true},
		{"work_item typed CASE whose only row is a service request's", crMismatchedCase, "AWAITING_INFO", "Awaiting Info", true},
		{"case already Waiting on WSO2", crCaseWaiting, "WAITING_ON_WSO2", "", false},
		{"case Open", crCaseOpen, "OPEN", "", false},
		{"case Closed", crCaseClosed, "CLOSED", "", false},
	}
	for _, tc := range rows {
		t.Run(tc.name, func(t *testing.T) {
			if got := crRecordState(t, scoped, tc.id); got != tc.from {
				t.Fatalf("fixture state = %q, want %q", got, tc.from)
			}
			resp, err := svc.CreateCaseComment(crCustomerContext(t), domain.CreateCaseCommentRequest{
				CaseID: tc.id, Type: domain.CommentTypeComment, Content: "Here is the information you asked for",
			})
			if err != nil {
				t.Fatalf("CreateCaseComment: %v", err)
			}
			if resp.Comment.CreatedBy != crCustomerEmail {
				t.Errorf("comment CreatedBy = %q, want %q", resp.Comment.CreatedBy, crCustomerEmail)
			}
			if n := crCommentCount(t, scoped, tc.id); n != 1 {
				t.Errorf("comment rows = %d, want 1 (the comment write itself is unchanged)", n)
			}

			state := crRecordState(t, scoped, tc.id)
			activity := crStateActivity(t, scoped, tc.id)
			if !tc.moves {
				if state != tc.from {
					t.Errorf("state = %q after the comment, want it unchanged at %q", state, tc.from)
				}
				if len(activity) != 0 {
					t.Errorf("activity = %+v, want no state entry", activity)
				}
				return
			}
			if state != "WAITING_ON_WSO2" {
				t.Errorf("state = %q after the customer's comment, want WAITING_ON_WSO2", state)
			}
			want := crActivityRow{tc.label, "Waiting on WSO2", crCustomerEmail}
			if len(activity) != 1 || activity[0] != want {
				t.Errorf("activity = %+v, want [%+v]", activity, want)
			}
		})
	}
}

// TestCustomerReplyIntegration_OtherCommentsLeaveTheRecordAlone: a staff
// member's customer-visible comment, and a work note from the customer
// (whether or not row-level security lets that row in), leave an Awaiting
// Info record where it is.
func TestCustomerReplyIntegration_OtherCommentsLeaveTheRecordAlone(t *testing.T) {
	pool := newCustomerReplyIntegrationPool(t)
	seedCustomerReplyFixture(t, pool)
	scoped := repository.NewScoped(pool)
	svc := newCustomerReplyIntegrationService(pool)

	t.Run("staff comment", func(t *testing.T) {
		if _, err := svc.CreateCaseComment(crStaffContext(t), domain.CreateCaseCommentRequest{
			CaseID: crStaffCase, Type: domain.CommentTypeComment, Content: "Could you also send the config?",
		}); err != nil {
			t.Fatalf("CreateCaseComment: %v", err)
		}
		if state := crRecordState(t, scoped, crStaffCase); state != "AWAITING_INFO" {
			t.Errorf("state = %q after a staff comment, want AWAITING_INFO", state)
		}
		if activity := crStateActivity(t, scoped, crStaffCase); len(activity) != 0 {
			t.Errorf("activity = %+v, want none", activity)
		}
	})

	t.Run("customer work note", func(t *testing.T) {
		_, err := svc.CreateCaseComment(crCustomerContext(t), domain.CreateCaseCommentRequest{
			CaseID: crWorkNoteCase, Type: domain.CommentTypeWorkNote, Content: "internal note",
		})
		t.Logf("work note from the customer: err = %v (whether or not the row went in, the state must not move)", err)
		if state := crRecordState(t, scoped, crWorkNoteCase); state != "AWAITING_INFO" {
			t.Errorf("state = %q after a customer's work note, want AWAITING_INFO", state)
		}
		if activity := crStateActivity(t, scoped, crWorkNoteCase); len(activity) != 0 {
			t.Errorf("activity = %+v, want none", activity)
		}
	})
}
