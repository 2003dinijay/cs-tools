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

// Regression tests for the email-matched "user" joins: "user".email is not
// unique, and migrated records leave an inactive and an active row for the
// same address. A plain join returned one row per matching user, so lists
// grew duplicates (and a total that no longer matched the rows). Every
// created_by / user_email resolution must yield exactly one row and pick the
// active user. Runs against a real Postgres with all migrations applied.
// Skipped without CASE_STATS_TEST_DSN.
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run UserEmailJoin

package repository_test

import (
	"context"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	uejEmail      = "jane.doe@example.com"
	uejCaseID     = "b1000000-0000-0000-0000-000000000001"
	uejConvID     = "b1000000-0000-0000-0000-000000000002"
	uejIncidentID = "b1000000-0000-0000-0000-000000000003"
	uejOldUserID  = "b2000000-0000-0000-0000-000000000001"
	uejNewUserID  = "b2000000-0000-0000-0000-000000000002"
	uejConvNumber = "UEJ-CONV-1"
)

func seedUserEmailJoinFixture(t *testing.T) context.Context {
	t.Helper()
	pool := caseStatsPool(t)
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)

	cleanup := func() {
		_, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE created_by = $1`, uejEmail)
		_, _ = scoped.Exec(ctx, `DELETE FROM "user" WHERE id IN ($1, $2)`, uejOldUserID, uejNewUserID)
	}
	cleanup()
	t.Cleanup(cleanup)

	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := scoped.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed (%.80s): %v", sql, err)
		}
	}

	// Two users share one email: an older inactive record and the real active one.
	mustExec(`INSERT INTO "user" (id, created_on, updated_on, user_name, name, email, is_active)
		VALUES ($1, now() - INTERVAL '2 years', now(), 'uej-old', 'Old Inactive', $2, FALSE)`, uejOldUserID, uejEmail)
	mustExec(`INSERT INTO "user" (id, created_on, updated_on, user_name, name, email, is_active)
		VALUES ($1, now(), now(), 'uej-new', 'Jane Doe', $2, TRUE)`, uejNewUserID, uejEmail)

	insertWorkItem := func(id, number, wsoID, typ string) {
		mustExec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type)
			VALUES ($1, now(), now(), $2, $2, $3, $4, 'uej subject', $5::work_item_type_enum)`, id, uejEmail, number, wsoID, typ)
	}
	insertWorkItem(uejCaseID, "UEJ-CASE-1", "UEJ-W-1", "CASE")
	mustExec(`INSERT INTO "case" (id) VALUES ($1)`, uejCaseID)
	insertWorkItem(uejConvID, uejConvNumber, "UEJ-W-2", "CONVERSATION")
	mustExec(`INSERT INTO conversation (id) VALUES ($1)`, uejConvID)
	insertWorkItem(uejIncidentID, "UEJ-INC-1", "UEJ-W-3", "INCIDENT")
	mustExec(`INSERT INTO incident (id) VALUES ($1)`, uejIncidentID)

	for _, wi := range []string{uejCaseID, uejIncidentID} {
		mustExec(`INSERT INTO comment (id, created_on, created_by, work_item_id, content, type)
			VALUES (gen_random_uuid(), now(), $1, $2, 'uej comment', 'COMMENT')`, uejEmail, wi)
		mustExec(`INSERT INTO work_item_activity (id, created_on, created_by, work_item_id, user_email, field_name, old_value, new_value)
			VALUES (gen_random_uuid(), now(), $1, $2, $1, 'state', 'a', 'b')`, uejEmail, wi)
	}
	return ctx
}

func TestUserEmailJoinIntegration_GetCaseResolvesTheActiveCreator(t *testing.T) {
	ctx := seedUserEmailJoinFixture(t)
	repo := repository.NewCaseRepository(repository.NewScoped(caseStatsPool(t)))

	cv, err := repo.GetCaseByID(ctx, uejCaseID, repository.SearchScope{Unrestricted: true})
	if err != nil {
		t.Fatalf("GetCaseByID: %v", err)
	}
	if cv.CreatedBy == nil || cv.CreatedBy.Name != "Jane Doe" {
		t.Errorf("case creator = %+v, want the active user", cv.CreatedBy)
	}
	if cv.CreatedBy != nil && (cv.CreatedBy.ID == nil || *cv.CreatedBy.ID != uejNewUserID) {
		t.Errorf("case creator id = %v, want %s", cv.CreatedBy.ID, uejNewUserID)
	}
}

func TestUserEmailJoinIntegration_CaseCommentsAndActivitiesAreNotMultiplied(t *testing.T) {
	ctx := seedUserEmailJoinFixture(t)
	repo := repository.NewCaseRepository(repository.NewScoped(caseStatsPool(t)))
	withChanges := true

	comments, total, err := repo.SearchCaseComments(ctx, domain.SearchCaseCommentsRequest{
		CaseID: uejCaseID, Pagination: domain.Pagination{Limit: 50},
	})
	if err != nil {
		t.Fatalf("SearchCaseComments: %v", err)
	}
	if len(comments) != 1 || total != 1 {
		t.Fatalf("case comments: got %d rows, total %d; want 1 and 1", len(comments), total)
	}
	if comments[0].CreatedBy == nil || comments[0].CreatedBy.Name != "Jane Doe" {
		t.Errorf("comment author = %+v, want the active user", comments[0].CreatedBy)
	}

	acts, _, err := repo.SearchCaseActivities(ctx, domain.SearchCaseActivitiesRequest{
		CaseID: uejCaseID, Pagination: domain.Pagination{Limit: 50}, IncludeFieldChanges: &withChanges,
	})
	if err != nil {
		t.Fatalf("SearchCaseActivities: %v", err)
	}
	assertActivitiesResolveActiveUser(t, acts)
}

func TestUserEmailJoinIntegration_IncidentActivitiesAreNotMultiplied(t *testing.T) {
	ctx := seedUserEmailJoinFixture(t)
	repo := repository.NewIncidentRepository(repository.NewScoped(caseStatsPool(t)))
	withChanges := true

	acts, _, err := repo.SearchIncidentActivities(ctx, domain.SearchIncidentActivitiesRequest{
		IncidentID: uejIncidentID, Pagination: domain.Pagination{Limit: 50}, IncludeFieldChanges: &withChanges,
	})
	if err != nil {
		t.Fatalf("SearchIncidentActivities: %v", err)
	}
	assertActivitiesResolveActiveUser(t, acts)
}

func assertActivitiesResolveActiveUser(t *testing.T, acts []domain.CaseActivity) {
	t.Helper()
	// one comment + one field change
	if len(acts) != 2 {
		t.Fatalf("activities: got %d rows, want 2 (one comment, one field change)", len(acts))
	}
	for _, a := range acts {
		if a.CreatedBy == nil || a.CreatedBy.Name != "Jane Doe" {
			t.Errorf("activity %s author = %+v, want the active user", a.ID, a.CreatedBy)
		}
	}
}

func TestUserEmailJoinIntegration_GenericCommentSearchIsNotMultiplied(t *testing.T) {
	ctx := seedUserEmailJoinFixture(t)
	repo := repository.NewCommentRepository(repository.NewScoped(caseStatsPool(t)))

	rows, total, err := repo.SearchComments(ctx, uejCaseID, domain.ReferenceTypeCase, nil, false, domain.Pagination{Limit: 50})
	if err != nil {
		t.Fatalf("SearchComments: %v", err)
	}
	if len(rows) != 1 || total != 1 {
		t.Fatalf("comments: got %d rows, total %d; want 1 and 1", len(rows), total)
	}
	if rows[0].CreatedByName != "Jane Doe" {
		t.Errorf("comment author name = %q, want the active user", rows[0].CreatedByName)
	}
}

func TestUserEmailJoinIntegration_ConversationsAreNotMultiplied(t *testing.T) {
	ctx := seedUserEmailJoinFixture(t)
	repo := repository.NewConversationRepository(repository.NewScoped(caseStatsPool(t)))
	number := uejConvNumber

	views, total, err := repo.SearchConversations(ctx, domain.SearchConversationsRequest{
		Filters:    domain.SearchConversationsFilters{Number: &number},
		Pagination: domain.Pagination{Limit: 50},
	}, "")
	if err != nil {
		t.Fatalf("SearchConversations: %v", err)
	}
	if len(views) != 1 || total != 1 {
		t.Fatalf("conversations: got %d rows, total %d; want 1 and 1", len(views), total)
	}
	if views[0].CreatedBy == nil || views[0].CreatedBy.Name != "Jane Doe" {
		t.Errorf("conversation creator = %+v, want the active user", views[0].CreatedBy)
	}

	if _, err := repo.GetConversation(ctx, uejConvID); err != nil {
		t.Fatalf("GetConversation: %v", err)
	}
}

func TestUserEmailJoinIntegration_DeploymentListHasOneRowPerDeployment(t *testing.T) {
	ctx := seedUserEmailJoinFixture(t)
	pool := caseStatsPool(t)
	scoped := repository.NewScoped(pool)
	const (
		acc  = "b3000000-0000-0000-0000-000000000001"
		proj = "b3000000-0000-0000-0000-000000000002"
		dep  = "b3000000-0000-0000-0000-000000000003"
	)
	cleanup := func() {
		_, _ = scoped.Exec(ctx, `DELETE FROM deployment WHERE id = $1`, dep)
		_, _ = pool.Exec(ctx, `DELETE FROM project WHERE id = $1`, proj)
		_, _ = pool.Exec(ctx, `DELETE FROM account WHERE id = $1`, acc)
	}
	cleanup()
	t.Cleanup(cleanup)
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number, sf_id)
			VALUES ($1, now(), now(), 'test', 'test', 'UEJ Account', 'UEJ-ACC-1', 'sf-uej-acc-1')`, []any{acc}},
		{`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, name, sf_id, account_id)
			VALUES ($1, now(), now(), 'test', 'test', 'UEJPRJ', 'UEJPRJ', 'sf-uejprj', $2)`, []any{proj, acc}},
		// created_by is the shared email: the old createdBy lookup matched both user rows.
		{`INSERT INTO deployment (id, created_on, updated_on, created_by, updated_by, number, name, type, is_active, project_id)
			VALUES ($1, now(), now(), $2, $2, 'UEJ-DEP-1', 'uej dep', 'DEVELOPMENT', TRUE, $3)`, []any{dep, uejEmail, proj}},
	} {
		if _, err := scoped.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatalf("seed (%.60s): %v", q.sql, err)
		}
	}

	views, total, err := repository.NewDeploymentRepository(scoped).SearchDeployments(ctx, domain.SearchDeploymentsRequest{
		Pagination: domain.Pagination{Limit: 50}, ProjectIDs: []string{proj},
	})
	if err != nil {
		t.Fatalf("SearchDeployments: %v", err)
	}
	if len(views) != 1 || total != 1 {
		t.Fatalf("deployments: got %d rows, total %d; want 1 and 1", len(views), total)
	}
}
