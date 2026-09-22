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

package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// alwaysUnrestrictedAccess is an AccessService stub for tests that exercise
// CaseService/ProjectService logic unrelated to access scoping -- every
// caller sees everything, so these tests aren't coupled to AccessService's
// own rules (see access_service_test.go for those).
type alwaysUnrestrictedAccess struct{}

func (alwaysUnrestrictedAccess) ResolveScope(context.Context) (AccessScope, error) {
	return AccessScope{Unrestricted: true}, nil
}

// stubCaseRepo is a minimal repository.CaseRepository whose SearchCases
// panics if called: tests using it prove ParseCaseFieldFilters' rejection of
// an unsupported field happens before the Postgres backend ever reaches the
// repository, not merely that the repository ignores the field.
type stubCaseRepo struct {
	searchCases              func(ctx context.Context, req domain.SearchCasesRequest) ([]domain.SearchCaseView, int, error)
	createCaseAttachment     func(ctx context.Context, req domain.CreateAttachmentRequest) (domain.Attachment, error)
	searchCaseAttachments    func(ctx context.Context, caseID string, pagination domain.Pagination) ([]domain.Attachment, int, error)
	getCaseAttachmentByID    func(ctx context.Context, id string) (domain.Attachment, error)
	deleteCaseAttachment     func(ctx context.Context, id string) error
	updateAttachmentName     func(ctx context.Context, id, name, updatedBy string) (time.Time, error)
	confirmCaseAttachment    func(ctx context.Context, id string) (domain.Attachment, error)
	searchCaseComments       func(ctx context.Context, req domain.SearchCaseCommentsRequest) ([]domain.CaseComment, int, error)
	updateCase               func(ctx context.Context, req domain.UpdateCaseRequest) (domain.Case, *domain.CaseSeverity, error)
	createCaseFromServiceNow func(ctx context.Context, req domain.CreateCaseRequest, id, number, wso2ID, createdBy, announcementState string) (domain.Case, error)
}

func (s *stubCaseRepo) CreateCase(context.Context, domain.CreateCaseRequest) (domain.Case, error) {
	panic("not implemented")
}
func (s *stubCaseRepo) CreateCaseFromServiceNow(ctx context.Context, req domain.CreateCaseRequest, id, number, wso2ID, createdBy, announcementState string) (domain.Case, error) {
	if s.createCaseFromServiceNow != nil {
		return s.createCaseFromServiceNow(ctx, req, id, number, wso2ID, createdBy, announcementState)
	}
	panic("not implemented")
}
func (s *stubCaseRepo) GetCaseByID(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
	panic("not implemented")
}
func (s *stubCaseRepo) SearchCases(ctx context.Context, req domain.SearchCasesRequest, scope repository.SearchScope) ([]domain.SearchCaseView, int, error) {
	if s.searchCases != nil {
		return s.searchCases(ctx, req)
	}
	panic("SearchCases called unexpectedly: the unsupported-field check should have short-circuited before reaching the repository")
}
func (s *stubCaseRepo) CreateCaseComment(context.Context, domain.CreateCaseCommentRequest) (domain.CaseComment, error) {
	panic("not implemented")
}
func (s *stubCaseRepo) SearchCaseComments(ctx context.Context, req domain.SearchCaseCommentsRequest) ([]domain.CaseComment, int, error) {
	if s.searchCaseComments != nil {
		return s.searchCaseComments(ctx, req)
	}
	panic("not implemented")
}
func (s *stubCaseRepo) UpdateCase(ctx context.Context, req domain.UpdateCaseRequest) (domain.Case, *domain.CaseSeverity, error) {
	if s.updateCase != nil {
		return s.updateCase(ctx, req)
	}
	panic("not implemented")
}
func (s *stubCaseRepo) CreateCaseAttachment(ctx context.Context, req domain.CreateAttachmentRequest) (domain.Attachment, error) {
	if s.createCaseAttachment != nil {
		return s.createCaseAttachment(ctx, req)
	}
	panic("not implemented")
}
func (s *stubCaseRepo) SearchCaseAttachments(ctx context.Context, caseID string, pagination domain.Pagination) ([]domain.Attachment, int, error) {
	if s.searchCaseAttachments != nil {
		return s.searchCaseAttachments(ctx, caseID, pagination)
	}
	panic("not implemented")
}
func (s *stubCaseRepo) GetCaseAttachmentByID(ctx context.Context, id string) (domain.Attachment, error) {
	if s.getCaseAttachmentByID != nil {
		return s.getCaseAttachmentByID(ctx, id)
	}
	panic("not implemented")
}
func (s *stubCaseRepo) DeleteCaseAttachment(ctx context.Context, id string) error {
	if s.deleteCaseAttachment != nil {
		return s.deleteCaseAttachment(ctx, id)
	}
	panic("not implemented")
}
func (s *stubCaseRepo) UpdateCaseAttachmentName(ctx context.Context, id, name, updatedBy string) (time.Time, error) {
	if s.updateAttachmentName != nil {
		return s.updateAttachmentName(ctx, id, name, updatedBy)
	}
	panic("not implemented")
}
func (s *stubCaseRepo) ConfirmCaseAttachment(ctx context.Context, id string) (domain.Attachment, error) {
	if s.confirmCaseAttachment != nil {
		return s.confirmCaseAttachment(ctx, id)
	}
	panic("not implemented")
}
func (s *stubCaseRepo) AddCaseTag(context.Context, string, string, string) (domain.Tag, error) {
	panic("not implemented")
}
func (s *stubCaseRepo) RemoveCaseTag(context.Context, string, string, string) error {
	panic("not implemented")
}
func (s *stubCaseRepo) SearchTags(context.Context, string, string, int) ([]domain.Tag, error) {
	panic("not implemented")
}
func (s *stubCaseRepo) SetCaseWatchList(context.Context, string, []string, string) ([]domain.WatchListUser, time.Time, error) {
	panic("not implemented")
}
func (s *stubCaseRepo) SearchCaseActivities(context.Context, domain.SearchCaseActivitiesRequest) ([]domain.CaseActivity, int, error) {
	panic("not implemented")
}

// stubUserRepo is a minimal repository.UserRepository; SearchCases doesn't
// exercise it beyond the createdBy-current-user path, which these tests don't
// use.
type stubUserRepo struct {
	getUserByEmail       func(ctx context.Context, email string) (domain.User, error)
	searchUsers          func(ctx context.Context, req domain.SearchUsersRequest) ([]domain.User, int, error)
	getUserDetail        func(ctx context.Context, id string) (domain.UserDetail, error)
	getUserRoles         func(ctx context.Context, id string) ([]string, error)
	getUserGroups        func(ctx context.Context, id string) ([]domain.UserGroupRef, error)
	getUserProjectAccess func(ctx context.Context, email string) ([]domain.UserContactAccess, error)
}

func (s stubUserRepo) GetUserDetail(ctx context.Context, id string) (domain.UserDetail, error) {
	if s.getUserDetail != nil {
		return s.getUserDetail(ctx, id)
	}
	panic("not implemented")
}
func (s stubUserRepo) GetUserProjectAccess(ctx context.Context, email string) ([]domain.UserContactAccess, error) {
	if s.getUserProjectAccess != nil {
		return s.getUserProjectAccess(ctx, email)
	}
	panic("not implemented")
}

func (s stubUserRepo) SearchUsers(ctx context.Context, req domain.SearchUsersRequest) ([]domain.User, int, error) {
	if s.searchUsers != nil {
		return s.searchUsers(ctx, req)
	}
	panic("not implemented")
}
func (s stubUserRepo) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	if s.getUserByEmail != nil {
		return s.getUserByEmail(ctx, email)
	}
	panic("not implemented")
}

// GetUserRoles/GetUserGroups return empty rather than panicking: GetMe calls
// both unconditionally after GetUserByEmail succeeds, and none of this
// stub's existing test cases care about their contents.
func (s stubUserRepo) GetUserRoles(ctx context.Context, id string) ([]string, error) {
	if s.getUserRoles != nil {
		return s.getUserRoles(ctx, id)
	}
	return nil, nil
}
func (s stubUserRepo) GetUserGroups(ctx context.Context, id string) ([]domain.UserGroupRef, error) {
	if s.getUserGroups != nil {
		return s.getUserGroups(ctx, id)
	}
	return nil, nil
}

// TestCaseService_SearchCases_RejectsUnsupportedPostgresFields proves the
// Postgres-backed SearchCases path rejects each of these filter fields
// ParseCaseFieldFilters accepts but the Postgres repository has no query
// support for (they dot-walk into SN-specific concepts with no Postgres
// query support yet), rather than silently accepting the request and
// returning a broader-than-requested result set.
func TestCaseService_SearchCases_RejectsUnsupportedPostgresFields(t *testing.T) {
	svc := NewCaseService(&stubCaseRepo{}, stubUserRepo{}, nil, alwaysUnrestrictedAccess{})
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	cases := []struct {
		name   string
		filter domain.CaseFieldFilter
	}{
		{name: "parentId", filter: domain.CaseFieldFilter{Field: "parentId", Op: "eq", Values: []string{"00000000-0000-0000-0000-000000000000"}}},
		{name: "product", filter: domain.CaseFieldFilter{Field: "product", Op: "in", Values: []string{"API Manager"}}},
		{name: "projectType", filter: domain.CaseFieldFilter{Field: "projectType", Op: "in", Values: []string{"Subscription"}}},
		{name: "creTeam", filter: domain.CaseFieldFilter{Field: "creTeam", Op: "in", Values: []string{"00000000-0000-0000-0000-000000000000"}}},
		{name: "sreTeam", filter: domain.CaseFieldFilter{Field: "sreTeam", Op: "in", Values: []string{"00000000-0000-0000-0000-000000000000"}}},
		{name: "assignedUserId isEmpty (Unassigned)", filter: domain.CaseFieldFilter{Field: "assignedUserId", Op: "isEmpty"}},
		{name: "resolutionNotes isEmpty", filter: domain.CaseFieldFilter{Field: "resolutionNotes", Op: "isEmpty"}},
		// state+in IS supported by this backend; only the exclusion is not.
		{name: "state notIn", filter: domain.CaseFieldFilter{Field: "state", Op: "notIn", Values: []string{"closed"}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := domain.SearchCasesRequest{Filters: domain.SearchCasesFilters{
				Filters: []domain.CaseFieldFilter{tc.filter},
			}}
			_, err := svc.SearchCases(ctx, req)
			var ve *apierror.ValidationError
			if !asValidationError(err, &ve) {
				t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
			}
		})
	}
}

// TestCaseService_SearchCases_SupportedFieldsStillReachRepository proves the
// fields the Postgres repository does support are not caught by the
// unsupported-field rejection: each reaches repo.SearchCases unchanged.
func TestCaseService_SearchCases_SupportedFieldsStillReachRepository(t *testing.T) {
	uuid1 := "00000000-0000-0000-0000-000000000001"

	cases := []struct {
		name   string
		filter domain.CaseFieldFilter
	}{
		{name: "type", filter: domain.CaseFieldFilter{Field: "type", Op: "in", Values: []string{"case"}}},
		{name: "projectId", filter: domain.CaseFieldFilter{Field: "projectId", Op: "in", Values: []string{uuid1}}},
		{name: "deploymentId", filter: domain.CaseFieldFilter{Field: "deploymentId", Op: "in", Values: []string{uuid1}}},
		{name: "state", filter: domain.CaseFieldFilter{Field: "state", Op: "in", Values: []string{"open"}}},
		{name: "severity", filter: domain.CaseFieldFilter{Field: "severity", Op: "in", Values: []string{"high"}}},
		{name: "issueType", filter: domain.CaseFieldFilter{Field: "issueType", Op: "in", Values: []string{"error"}}},
		{name: "engagementType", filter: domain.CaseFieldFilter{Field: "engagementType", Op: "in", Values: []string{"migration"}}},
		{name: "createdBy", filter: domain.CaseFieldFilter{Field: "createdBy", Op: "in", Values: []string{"a@example.com"}}},
		{name: "workState", filter: domain.CaseFieldFilter{Field: "workState", Op: "in", Values: []string{"ongoing"}}},
		{name: "assignedUserId in", filter: domain.CaseFieldFilter{Field: "assignedUserId", Op: "in", Values: []string{uuid1}}},
		{name: "createdOn gte", filter: domain.CaseFieldFilter{Field: "createdOn", Op: "gte", Values: []string{"2026-01-01"}}},
		{name: "escalationLevel in", filter: domain.CaseFieldFilter{Field: "escalationLevel", Op: "in", Values: []string{"1", "2"}}},
		{name: "escalation isNotEmpty", filter: domain.CaseFieldFilter{Field: "escalation", Op: "isNotEmpty"}},
		{name: "escalation isEmpty", filter: domain.CaseFieldFilter{Field: "escalation", Op: "isEmpty"}},
		{name: "tag in", filter: domain.CaseFieldFilter{Field: "tag", Op: "in", Values: []string{"patch"}}},
		{name: "tag notIn", filter: domain.CaseFieldFilter{Field: "tag", Op: "notIn", Values: []string{"s_dip", "patch"}}},
		{name: "projectOnboardingStatus in", filter: domain.CaseFieldFilter{Field: "projectOnboardingStatus", Op: "in", Values: []string{"Completed"}}},
		{name: "projectOnboardingStatus notIn", filter: domain.CaseFieldFilter{Field: "projectOnboardingStatus", Op: "notIn", Values: []string{"In-Progress"}}},
		{name: "taskSLABusinessElapsedPercent gte", filter: domain.CaseFieldFilter{Field: "taskSLABusinessElapsedPercent", Op: "gte", Values: []string{"80"}}},
		{name: "taskSLABusinessElapsedPercent lte 0", filter: domain.CaseFieldFilter{Field: "taskSLABusinessElapsedPercent", Op: "lte", Values: []string{"0"}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			repo := &stubCaseRepo{
				searchCases: func(ctx context.Context, req domain.SearchCasesRequest) ([]domain.SearchCaseView, int, error) {
					called = true
					return nil, 0, nil
				},
			}
			svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{})
			ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

			req := domain.SearchCasesRequest{Filters: domain.SearchCasesFilters{
				Filters: []domain.CaseFieldFilter{tc.filter},
			}}
			if _, err := svc.SearchCases(ctx, req); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !called {
				t.Fatalf("expected repo.SearchCases to be called for supported field %q", tc.name)
			}
		})
	}
}

// TestCaseService_SearchCases_RejectsServiceNowOnlyOptions proves the Postgres
// path rejects the search options that only snCaseService implements: the
// slaBreached and account-escalation filters, and grouped counts.
// caseRepo.SearchCases models none of them, so accepting the request
// would silently drop the predicate and return a wider result set with a 200.
// The stub repository panics if reached, so a passing test proves the
// short-circuit, not merely that the repository ignored the option.
func TestCaseService_SearchCases_RejectsServiceNowOnlyOptions(t *testing.T) {
	svc := NewCaseService(&stubCaseRepo{}, stubUserRepo{}, nil, alwaysUnrestrictedAccess{})
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	cases := []struct {
		name    string
		req     domain.SearchCasesRequest
		wantMsg string
	}{
		{
			name: "slaBreached",
			req: domain.SearchCasesRequest{Filters: domain.SearchCasesFilters{
				Filters: []domain.CaseFieldFilter{{Field: "slaBreached", Op: "eq", Values: []string{"true"}}},
			}},
			wantMsg: `field "slaBreached" is not supported by this data source`,
		},
		{
			name: "accountEscalationActive",
			req: domain.SearchCasesRequest{Filters: domain.SearchCasesFilters{
				Filters: []domain.CaseFieldFilter{{Field: "accountEscalationActive", Op: "eq", Values: []string{"true"}}},
			}},
			wantMsg: `field "accountEscalationActive" is not supported by this data source`,
		},
		{
			name: "resolvedOn gte",
			req: domain.SearchCasesRequest{Filters: domain.SearchCasesFilters{
				Filters: []domain.CaseFieldFilter{{Field: "resolvedOn", Op: "gte", Values: []string{"2026-01-01"}}},
			}},
			wantMsg: `field "resolvedOn" is not supported by this data source`,
		},
		{
			name: "resolvedOn lte",
			req: domain.SearchCasesRequest{Filters: domain.SearchCasesFilters{
				Filters: []domain.CaseFieldFilter{{Field: "resolvedOn", Op: "lte", Values: []string{"2026-01-31"}}},
			}},
			wantMsg: `field "resolvedOn" is not supported by this data source`,
		},
		{
			name:    "groupBy",
			req:     domain.SearchCasesRequest{GroupBy: "state"},
			wantMsg: "groupBy is not supported by this data source",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.SearchCases(ctx, tc.req)
			var ve *apierror.ValidationError
			if !asValidationError(err, &ve) {
				t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
			}
			if ve.Msg != tc.wantMsg {
				t.Errorf("Msg = %q, want %q", ve.Msg, tc.wantMsg)
			}
		})
	}
}

// TestCaseService_SearchCaseComments covers the Postgres-backed comment
// listing path added to close the gap where POST /cases/{id}/comments/search
// was never registered in routes.go, even though comment creation worked and
// this service method (plus its repository query) was already fully
// implemented. Exercises: empty result, a single comment, multiple comments
// with the repository's most-recent-first ordering preserved through to the
// response, and pagination bookkeeping (hasMore).
func TestCaseService_SearchCaseComments(t *testing.T) {
	caseID := "11111111-1111-1111-1111-111111111111"
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)

	t.Run("empty case has no comments", func(t *testing.T) {
		repo := &stubCaseRepo{
			searchCaseComments: func(_ context.Context, req domain.SearchCaseCommentsRequest) ([]domain.CaseComment, int, error) {
				if req.CaseID != caseID {
					t.Fatalf("CaseID = %q, want %q", req.CaseID, caseID)
				}
				return nil, 0, nil
			},
		}
		svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{})

		resp, err := svc.SearchCaseComments(context.Background(), domain.SearchCaseCommentsRequest{CaseID: caseID})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Comments) != 0 {
			t.Errorf("Comments = %v, want empty", resp.Comments)
		}
		if resp.Total != 0 || resp.HasMore {
			t.Errorf("Total = %d, HasMore = %v, want 0/false", resp.Total, resp.HasMore)
		}
	})

	t.Run("single comment", func(t *testing.T) {
		want := domain.CaseComment{
			ID:        "c1",
			CaseID:    caseID,
			Type:      domain.CommentTypeComment,
			Content:   "hello",
			CreatedBy: domain.NewUserReference("u1", "jane.doe@example.com", "Jane Doe"),
			CreatedOn: now,
		}
		repo := &stubCaseRepo{
			searchCaseComments: func(context.Context, domain.SearchCaseCommentsRequest) ([]domain.CaseComment, int, error) {
				return []domain.CaseComment{want}, 1, nil
			},
		}
		svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{})

		resp, err := svc.SearchCaseComments(context.Background(), domain.SearchCaseCommentsRequest{CaseID: caseID})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Comments) != 1 || resp.Comments[0].ID != "c1" {
			t.Fatalf("Comments = %+v, want [%+v]", resp.Comments, want)
		}
		if resp.Total != 1 || resp.HasMore {
			t.Errorf("Total = %d, HasMore = %v, want 1/false", resp.Total, resp.HasMore)
		}
	})

	t.Run("multiple comments preserve repository order and compute hasMore", func(t *testing.T) {
		// The repository orders by created_at DESC (most recent first); the
		// service must not re-sort, only pass the slice through.
		newest := domain.CaseComment{ID: "c3", CaseID: caseID, CreatedOn: now}
		middle := domain.CaseComment{ID: "c2", CaseID: caseID, CreatedOn: now.Add(-time.Hour)}
		oldest := domain.CaseComment{ID: "c1", CaseID: caseID, CreatedOn: now.Add(-2 * time.Hour)}
		repo := &stubCaseRepo{
			searchCaseComments: func(_ context.Context, req domain.SearchCaseCommentsRequest) ([]domain.CaseComment, int, error) {
				if req.Pagination.Limit != 2 {
					t.Fatalf("Pagination.Limit = %d, want 2 (page size requested)", req.Pagination.Limit)
				}
				// total (5) exceeds what's returned on this page (2 of the 3
				// shown here is illustrative; assert against the 5 below).
				return []domain.CaseComment{newest, middle, oldest}, 5, nil
			},
		}
		svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{})

		resp, err := svc.SearchCaseComments(context.Background(), domain.SearchCaseCommentsRequest{
			CaseID:     caseID,
			Pagination: domain.Pagination{Limit: 2},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		gotIDs := []string{resp.Comments[0].ID, resp.Comments[1].ID, resp.Comments[2].ID}
		wantIDs := []string{"c3", "c2", "c1"}
		for i := range wantIDs {
			if gotIDs[i] != wantIDs[i] {
				t.Errorf("Comments[%d].ID = %q, want %q (order must not be reshuffled)", i, gotIDs[i], wantIDs[i])
			}
		}
		if resp.Total != 5 {
			t.Errorf("Total = %d, want 5", resp.Total)
		}
		if !resp.HasMore {
			t.Errorf("HasMore = false, want true (offset 0 + 3 returned < total 5)")
		}
	})

	t.Run("invalid case id is rejected before reaching the repository", func(t *testing.T) {
		repo := &stubCaseRepo{
			searchCaseComments: func(context.Context, domain.SearchCaseCommentsRequest) ([]domain.CaseComment, int, error) {
				t.Fatal("repository should not be reached for an invalid caseId")
				return nil, 0, nil
			},
		}
		svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{})

		_, err := svc.SearchCaseComments(context.Background(), domain.SearchCaseCommentsRequest{CaseID: "not-a-uuid"})
		var ve *apierror.ValidationError
		if !asValidationError(err, &ve) {
			t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
		}
	})
}

// TestCaseService_UpdateCase_RejectsTypeTransferFields proves the case-type
// transfer fields are rejected before the
// Postgres-backed UpdateCase ever reaches the repository -- stubCaseRepo's
// UpdateCase panics if called, so a passing test here proves the rejection,
// not just a repository that happens to ignore the field. Postgres-backed
// cases have no engagement_type column and are always type "case" (see
// CreateCase's own "only type \"case\" is supported" guard), so none of these
// fields have anywhere to go on this data source.
func TestCaseService_UpdateCase_RejectsTypeTransferFields(t *testing.T) {
	svc := NewCaseService(&stubCaseRepo{}, stubUserRepo{}, nil, alwaysUnrestrictedAccess{})
	ctx := context.Background()
	strPtr := func(s string) *string { return &s }
	engagement := domain.EngagementTypeMigration

	cases := []struct {
		name string
		req  domain.UpdateCaseRequest
	}{
		{name: "type", req: domain.UpdateCaseRequest{ID: testDeploymentUUID, Type: strPtr("engagement")}},
		{
			name: "engagementType",
			req:  domain.UpdateCaseRequest{ID: testDeploymentUUID, EngagementType: &engagement},
		},
		{
			name: "catalogId",
			req:  domain.UpdateCaseRequest{ID: testDeploymentUUID, CatalogID: strPtr(testDeploymentUUID)},
		},
		{
			name: "catalogItemId",
			req:  domain.UpdateCaseRequest{ID: testDeploymentUUID, CatalogItemID: strPtr(testDeploymentUUID)},
		},
		{
			name: "variables",
			req: domain.UpdateCaseRequest{
				ID:        testDeploymentUUID,
				Variables: []domain.Variable{{ID: testDeploymentUUID, Value: "x"}},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.UpdateCase(ctx, tc.req)
			var ve *apierror.ValidationError
			if !asValidationError(err, &ve) {
				t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
			}
		})
	}
}

// TestCaseService_SearchCases_AnyOfReachesRepository proves an anyOf request is
// parsed into OR groups and handed to the repository (the Postgres path used to
// reject it outright), and that a field not allowed inside a branch is still a
// validation error rather than silently dropped.
func TestCaseService_SearchCases_AnyOfReachesRepository(t *testing.T) {
	var got domain.SearchCasesRequest
	repo := &stubCaseRepo{
		searchCases: func(ctx context.Context, req domain.SearchCasesRequest) ([]domain.SearchCaseView, int, error) {
			got = req
			return nil, 0, nil
		},
	}
	svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{})
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	req := domain.SearchCasesRequest{Filters: domain.SearchCasesFilters{
		Filters: []domain.CaseFieldFilter{{Field: "state", Op: "in", Values: []string{"open"}}},
		AnyOf: []domain.CaseFilterBranch{
			{Filters: []domain.CaseFieldFilter{{Field: "severity", Op: "in", Values: []string{"critical"}}}},
			{Filters: []domain.CaseFieldFilter{{Field: "escalationLevel", Op: "in", Values: []string{"3", "4"}}}},
		},
	}}
	if _, err := svc.SearchCases(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Parsed.OrGroups) != 2 {
		t.Fatalf("OrGroups = %d, want 2", len(got.Parsed.OrGroups))
	}
	if g := got.Parsed.OrGroups[1]; len(g.EscalationLevels) != 2 || g.EscalationLevels[0] != "3" {
		t.Errorf("second branch escalation levels = %v, want [3 4]", g.EscalationLevels)
	}

	bad := domain.SearchCasesRequest{Filters: domain.SearchCasesFilters{
		AnyOf: []domain.CaseFilterBranch{
			{Filters: []domain.CaseFieldFilter{{Field: "projectOnboardingStatus", Op: "in", Values: []string{"Completed"}}}},
		},
	}}
	_, err := svc.SearchCases(ctx, bad)
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("a field not allowed inside a branch must be a validation error, got %v", err)
	}
}

// stubMirrorCaseService is a minimal CaseService stub that only implements
// UpdateCase/CreateCase — the two methods a NewCaseServiceWithSNWriteback
// mirror is ever called on (see that constructor's own doc comment).
type stubMirrorCaseService struct {
	CaseService
	updateCase func(ctx context.Context, req domain.UpdateCaseRequest) (domain.UpdateCaseResponse, error)
	createCase func(ctx context.Context, req domain.CreateCaseRequest) (domain.CreateCaseResponse, error)
}

func (s *stubMirrorCaseService) UpdateCase(ctx context.Context, req domain.UpdateCaseRequest) (domain.UpdateCaseResponse, error) {
	return s.updateCase(ctx, req)
}

func (s *stubMirrorCaseService) CreateCase(ctx context.Context, req domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
	return s.createCase(ctx, req)
}

// TestCaseService_UpdateCase_MirrorsWorkStateToServiceNow is the pilot's core
// regression guard: a WorkState-only update must reach the mirror
// CaseService with State and Severity both nil (see UpdateCase's own doc
// comment for why that matters — the ServiceNow-mode UpdateCase this mirrors
// reads from ServiceNow first whenever either is set, which this mode must
// never do).
func TestCaseService_UpdateCase_MirrorsWorkStateToServiceNow(t *testing.T) {
	var mu sync.Mutex
	var gotReq domain.UpdateCaseRequest
	called := make(chan struct{})
	mirror := &stubMirrorCaseService{
		updateCase: func(_ context.Context, req domain.UpdateCaseRequest) (domain.UpdateCaseResponse, error) {
			mu.Lock()
			gotReq = req
			mu.Unlock()
			close(called)
			return domain.UpdateCaseResponse{}, nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)

	workState := domain.CaseWorkStateOngoing
	repo := &stubCaseRepo{
		updateCase: func(_ context.Context, req domain.UpdateCaseRequest) (domain.Case, *domain.CaseSeverity, error) {
			return domain.Case{ID: req.ID, WorkState: req.WorkState}, nil, nil
		},
	}
	svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, dispatcher, mirror)

	if _, err := svc.UpdateCase(context.Background(), domain.UpdateCaseRequest{ID: testDeploymentUUID, WorkState: &workState}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatal("mirror.UpdateCase was never called")
	}

	mu.Lock()
	defer mu.Unlock()
	if gotReq.ID != testDeploymentUUID {
		t.Errorf("mirror got ID %q, want %q", gotReq.ID, testDeploymentUUID)
	}
	if gotReq.WorkState == nil || *gotReq.WorkState != workState {
		t.Errorf("mirror got WorkState %v, want %v", gotReq.WorkState, workState)
	}
	if gotReq.State != nil {
		t.Errorf("mirror got non-nil State %v, want nil — this must never carry State into the SN mirror call", gotReq.State)
	}
	if gotReq.Severity != nil {
		t.Errorf("mirror got non-nil Severity %v, want nil — this must never carry Severity into the SN mirror call", gotReq.Severity)
	}
	if got := failures.count(); got != 0 {
		t.Errorf("expected 0 sn_writeback_failures records for a successful mirror, got %d", got)
	}
}

// TestCaseService_UpdateCase_DoesNotMirrorStateOrSeverity guards the other
// half of the same invariant: a State or Severity update must NOT dispatch
// to the mirror at all (not "dispatch with a stripped request" — no
// dispatch), since neither is in this pilot's scope.
func TestCaseService_UpdateCase_DoesNotMirrorStateOrSeverity(t *testing.T) {
	mirrorCalled := make(chan struct{}, 1)
	mirror := &stubMirrorCaseService{
		updateCase: func(context.Context, domain.UpdateCaseRequest) (domain.UpdateCaseResponse, error) {
			mirrorCalled <- struct{}{}
			return domain.UpdateCaseResponse{}, nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)

	repo := &stubCaseRepo{
		updateCase: func(_ context.Context, req domain.UpdateCaseRequest) (domain.Case, *domain.CaseSeverity, error) {
			return domain.Case{ID: req.ID, State: req.State, Severity: req.Severity}, req.Severity, nil
		},
	}
	svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, dispatcher, mirror)

	state := domain.CaseStateOpen
	if _, err := svc.UpdateCase(context.Background(), domain.UpdateCaseRequest{ID: testDeploymentUUID, State: &state}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	severity := domain.CaseSeverityHigh
	if _, err := svc.UpdateCase(context.Background(), domain.UpdateCaseRequest{ID: testDeploymentUUID, Severity: &severity}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case <-mirrorCalled:
		t.Fatal("mirror.UpdateCase was called for a State/Severity update — only WorkState is in scope")
	case <-time.After(100 * time.Millisecond):
		// expected: no dispatch
	}
}

// TestCaseService_UpdateCase_RecordsSNWritebackFailureOnMirrorError covers
// the failure path: Postgres already committed by the time Dispatch runs, so
// a failed mirror write must not surface as an UpdateCase error — it's
// recorded to sn_writeback_failures instead (see SNWritebackDispatcher).
func TestCaseService_UpdateCase_RecordsSNWritebackFailureOnMirrorError(t *testing.T) {
	mirror := &stubMirrorCaseService{
		updateCase: func(context.Context, domain.UpdateCaseRequest) (domain.UpdateCaseResponse, error) {
			return domain.UpdateCaseResponse{}, errors.New("sn downstream unreachable")
		},
	}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)

	workState := domain.CaseWorkStatePaused
	repo := &stubCaseRepo{
		updateCase: func(_ context.Context, req domain.UpdateCaseRequest) (domain.Case, *domain.CaseSeverity, error) {
			return domain.Case{ID: req.ID, WorkState: req.WorkState}, nil, nil
		},
	}
	svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, dispatcher, mirror)

	resp, err := svc.UpdateCase(context.Background(), domain.UpdateCaseRequest{ID: testDeploymentUUID, WorkState: &workState})
	if err != nil {
		t.Fatalf("UpdateCase must still succeed on a failed best-effort mirror write, got: %v", err)
	}
	if resp.Case.WorkState == nil || *resp.Case.WorkState != workState {
		t.Errorf("UpdateCase response WorkState = %v, want %v", resp.Case.WorkState, workState)
	}

	waitFor(t, func() bool { return failures.count() == 1 })
	req := failures.calls[0]
	if req.EntityType != "case" || req.EntityID != testDeploymentUUID || req.Operation != "update" {
		t.Errorf("unexpected failure record: %+v", req)
	}
}

// validCreateCaseRequest returns a minimally valid, type="case" CreateCase
// request — one of the two types this data source supports, alongside
// "announcement" (see CreateCase's own "only type \"case\" or \"announcement\""
// check; validAnnouncementCreateCaseRequest below covers the other one).
func validCreateCaseRequest() domain.CreateCaseRequest {
	return domain.CreateCaseRequest{
		Type:              "case",
		ProjectID:         testDeploymentUUID,
		DeploymentID:      testDeploymentUUID,
		DeployedProductID: testDeploymentUUID,
		Subject:           "Something is broken",
		Description:       "Steps to reproduce the problem",
		Severity:          domain.CaseSeverityHigh,
		IssueType:         domain.CaseIssueTypeQuestion,
	}
}

// validAnnouncementCreateCaseRequest returns a minimally valid,
// type="announcement" CreateCase request -- no deploymentId/deployedProductId,
// same as validateCreateCaseRequest's own announcement conditional requires.
func validAnnouncementCreateCaseRequest() domain.CreateCaseRequest {
	return domain.CreateCaseRequest{
		Type:        "announcement",
		ProjectID:   testDeploymentUUID,
		Subject:     "Scheduled maintenance window",
		Description: "Maintenance details go here",
	}
}

// TestCaseService_CreateCase_SNFailureLeavesPostgresUntouched is the pilot's
// core regression guard for the orphan bug this whole design change exists
// to fix: if ServiceNow never accepts the case (even after the retry), the
// Postgres repository must never be called at all — no row, no orphan.
func TestCaseService_CreateCase_SNFailureLeavesPostgresUntouched(t *testing.T) {
	var mu sync.Mutex
	attempts := 0
	mirror := &stubMirrorCaseService{
		createCase: func(context.Context, domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
			mu.Lock()
			attempts++
			mu.Unlock()
			return domain.CreateCaseResponse{}, errors.New("sn downstream unreachable")
		},
	}
	// No createCaseFromServiceNow override -- stubCaseRepo panics if it's
	// ever called, which is exactly the assertion: Postgres must stay
	// untouched.
	repo := &stubCaseRepo{}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, dispatcher, mirror)

	_, err := svc.CreateCase(context.Background(), validCreateCaseRequest())
	if err == nil {
		t.Fatal("expected an error when ServiceNow never accepts the case")
	}

	mu.Lock()
	defer mu.Unlock()
	if attempts != snCaseCreateAttempts {
		t.Errorf("expected %d SN attempts (bounded retry, both transient), got %d", snCaseCreateAttempts, attempts)
	}
}

// TestCaseService_CreateCase_SNSuccessCreatesPostgresRowWithMatchingIdentity
// covers the other half: on ServiceNow success, the Postgres insert must use
// EXACTLY the id/number/internalId/createdBy ServiceNow returned -- not
// anything generated locally -- so both systems agree on identity from the
// moment the Postgres row exists.
func TestCaseService_CreateCase_SNSuccessCreatesPostgresRowWithMatchingIdentity(t *testing.T) {
	const (
		snID         = "33333333-3333-3333-3333-333333333333"
		snNumber     = "CS0023001"
		snInternalID = "WSO2-CS-1"
		snCreatedBy  = "jane.doe@example.com"
	)
	createdOn := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	mirror := &stubMirrorCaseService{
		createCase: func(_ context.Context, req domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
			return domain.CreateCaseResponse{
				Message: "Case created successfully.",
				Case: domain.CreateCaseDetails{
					ID: snID, InternalID: snInternalID, Number: snNumber,
					CreatedBy: snCreatedBy, CreatedOn: createdOn, State: "Open",
				},
			}, nil
		},
	}

	var mu sync.Mutex
	var gotID, gotNumber, gotWso2ID, gotCreatedBy string
	repo := &stubCaseRepo{
		createCaseFromServiceNow: func(_ context.Context, req domain.CreateCaseRequest, id, number, wso2ID, createdBy, announcementState string) (domain.Case, error) {
			mu.Lock()
			gotID, gotNumber, gotWso2ID, gotCreatedBy = id, number, wso2ID, createdBy
			mu.Unlock()
			state := domain.CaseStateOpen
			return domain.Case{
				ID: id, Number: number, InternalID: wso2ID, CreatedBy: createdBy,
				CreatedOn: createdOn, State: &state,
			}, nil
		},
	}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, dispatcher, mirror)

	resp, err := svc.CreateCase(context.Background(), validCreateCaseRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if gotID != snID || gotNumber != snNumber || gotWso2ID != snInternalID || gotCreatedBy != snCreatedBy {
		t.Errorf("CreateCaseFromServiceNow got (%q, %q, %q, %q), want (%q, %q, %q, %q)",
			gotID, gotNumber, gotWso2ID, gotCreatedBy, snID, snNumber, snInternalID, snCreatedBy)
	}
	if resp.Case.ID != snID || resp.Case.Number != snNumber || resp.Case.InternalID != snInternalID {
		t.Errorf("CreateCase response = %+v, want identity matching ServiceNow's (%q, %q, %q)", resp.Case, snID, snNumber, snInternalID)
	}
}

// TestCaseService_CreateCase_RetriesTransientSNFailureThenSucceeds covers the
// retry itself: a first attempt that fails transiently must not surface as
// an error if the second attempt succeeds.
func TestCaseService_CreateCase_RetriesTransientSNFailureThenSucceeds(t *testing.T) {
	var mu sync.Mutex
	attempts := 0
	mirror := &stubMirrorCaseService{
		createCase: func(context.Context, domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
			mu.Lock()
			attempts++
			n := attempts
			mu.Unlock()
			if n == 1 {
				return domain.CreateCaseResponse{}, errors.New("sn downstream: timeout")
			}
			return domain.CreateCaseResponse{
				Case: domain.CreateCaseDetails{ID: testDeploymentUUID, Number: "CS0001", InternalID: "WSO2-1", CreatedBy: "jane.doe@example.com"},
			}, nil
		},
	}
	repo := &stubCaseRepo{
		createCaseFromServiceNow: func(_ context.Context, req domain.CreateCaseRequest, id, number, wso2ID, createdBy, announcementState string) (domain.Case, error) {
			return domain.Case{ID: id, Number: number, InternalID: wso2ID, CreatedBy: createdBy}, nil
		},
	}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, dispatcher, mirror)

	resp, err := svc.CreateCase(context.Background(), validCreateCaseRequest())
	if err != nil {
		t.Fatalf("expected the retried attempt to succeed, got error: %v", err)
	}
	if resp.Case.ID != testDeploymentUUID {
		t.Errorf("CreateCase response ID = %q, want %q", resp.Case.ID, testDeploymentUUID)
	}

	mu.Lock()
	defer mu.Unlock()
	if attempts != 2 {
		t.Errorf("expected exactly 2 SN attempts (1 transient failure + 1 success), got %d", attempts)
	}
}

// TestCaseService_CreateCase_DoesNotRetryValidationError guards against
// wasted latency on a deterministic client error: retrying the exact same
// invalid input can't produce a different outcome.
func TestCaseService_CreateCase_DoesNotRetryValidationError(t *testing.T) {
	var mu sync.Mutex
	attempts := 0
	mirror := &stubMirrorCaseService{
		createCase: func(context.Context, domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
			mu.Lock()
			attempts++
			mu.Unlock()
			return domain.CreateCaseResponse{}, &apierror.ValidationError{Msg: "severity contains invalid value"}
		},
	}
	repo := &stubCaseRepo{}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, dispatcher, mirror)

	_, err := svc.CreateCase(context.Background(), validCreateCaseRequest())
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}

	mu.Lock()
	defer mu.Unlock()
	if attempts != 1 {
		t.Errorf("expected exactly 1 SN attempt (validation errors are not retried), got %d", attempts)
	}
}

// TestCaseService_CreateCase_RejectsUnsupportedTypesOnPostgres is the control
// half of the type-guard widening: proves service_request/engagement/
// security_report_analysis are still rejected outright (never reaching
// snMirror or the repository) even though "case" and "announcement" are now
// both accepted -- guards against the guard having been widened too far.
func TestCaseService_CreateCase_RejectsUnsupportedTypesOnPostgres(t *testing.T) {
	mirror := &stubMirrorCaseService{
		createCase: func(context.Context, domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
			t.Fatal("snMirror.CreateCase should not be reached for an unsupported type")
			return domain.CreateCaseResponse{}, nil
		},
	}
	repo := &stubCaseRepo{}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, dispatcher, mirror)

	for _, typ := range []string{"service_request", "engagement", "security_report_analysis"} {
		t.Run(typ, func(t *testing.T) {
			req := domain.CreateCaseRequest{
				Type:                  typ,
				ProjectID:             testDeploymentUUID,
				DeploymentID:          testDeploymentUUID,
				DeployedProductID:     testDeploymentUUID,
				Subject:               "x",
				Description:           "y",
				CatalogID:             testDeploymentUUID,
				CatalogItemID:         testDeploymentUUID,
				Variables:             []domain.Variable{{ID: testDeploymentUUID, Value: "v"}},
				EngagementType:        domain.EngagementTypeMigration,
				EngagementPaymentType: domain.EngagementPaymentTypePaid,
			}
			_, err := svc.CreateCase(context.Background(), req)
			var ve *apierror.ValidationError
			if !asValidationError(err, &ve) {
				t.Fatalf("expected *apierror.ValidationError for type %q, got %T: %v", typ, err, err)
			}
		})
	}
}

// TestCaseService_CreateCase_Announcement_SNSuccessStoresAnnouncementType
// covers the announcement branch of the SN-first path end to end: the
// Postgres insert must receive req.Type == "announcement" and store the
// real ANNOUNCEMENT work_item type plus the state ServiceNow's create
// response actually returned (mapped through snAnnouncementStateToEnum),
// not just "no error" -- CreateCaseFromServiceNow's stub here asserts on
// req.Type directly, which is what the repository branches on.
func TestCaseService_CreateCase_Announcement_SNSuccessStoresAnnouncementType(t *testing.T) {
	const (
		snID         = "44444444-4444-4444-4444-444444444444"
		snNumber     = "AN0001001"
		snInternalID = "WSO2-AN-1"
		snCreatedBy  = "jane.doe@example.com"
	)
	mirror := &stubMirrorCaseService{
		createCase: func(_ context.Context, req domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
			if req.Type != "announcement" {
				t.Fatalf("snMirror.CreateCase got type %q, want \"announcement\"", req.Type)
			}
			return domain.CreateCaseResponse{
				Message: "Case created successfully.",
				Case: domain.CreateCaseDetails{
					ID: snID, InternalID: snInternalID, Number: snNumber,
					CreatedBy: snCreatedBy, CreatedOn: time.Now(), State: "Open",
				},
			}, nil
		},
	}

	var mu sync.Mutex
	var gotType, gotAnnouncementState string
	repo := &stubCaseRepo{
		createCaseFromServiceNow: func(_ context.Context, req domain.CreateCaseRequest, id, number, wso2ID, createdBy, announcementState string) (domain.Case, error) {
			mu.Lock()
			gotType, gotAnnouncementState = req.Type, announcementState
			mu.Unlock()
			state := domain.CaseState(strings.ToLower(announcementState))
			return domain.Case{ID: id, Number: number, InternalID: wso2ID, CreatedBy: createdBy, State: &state}, nil
		},
	}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, dispatcher, mirror)

	resp, err := svc.CreateCase(context.Background(), validAnnouncementCreateCaseRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if gotType != "announcement" {
		t.Errorf("CreateCaseFromServiceNow got req.Type %q, want \"announcement\"", gotType)
	}
	if gotAnnouncementState != "OPEN" {
		t.Errorf("CreateCaseFromServiceNow got announcementState %q, want \"OPEN\" (mapped from ServiceNow's \"Open\" label)", gotAnnouncementState)
	}
	if resp.Case.State != "open" {
		t.Errorf("CreateCase response state = %q, want \"open\"", resp.Case.State)
	}
}

// TestCaseService_CreateCase_Announcement_UnknownSNStateFailsClosed proves an
// unrecognized ServiceNow state label on an announcement create response is
// a hard error -- not silently defaulted to OPEN -- and that the Postgres
// repository is never reached in that case (ServiceNow already has the
// announcement at that point, which is real drift needing operator
// attention, but the caller must still see an error rather than a
// fabricated success).
func TestCaseService_CreateCase_Announcement_UnknownSNStateFailsClosed(t *testing.T) {
	mirror := &stubMirrorCaseService{
		createCase: func(context.Context, domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
			return domain.CreateCaseResponse{
				Case: domain.CreateCaseDetails{ID: testDeploymentUUID, Number: "AN0002", InternalID: "WSO2-AN-2", CreatedBy: "jane.doe@example.com", State: "Pending Review"},
			}, nil
		},
	}
	// No createCaseFromServiceNow override -- stubCaseRepo panics if it's
	// ever called, proving Postgres is never reached for an unmappable state.
	repo := &stubCaseRepo{}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, dispatcher, mirror)

	_, err := svc.CreateCase(context.Background(), validAnnouncementCreateCaseRequest())
	if err == nil {
		t.Fatal("expected an error for an unrecognized ServiceNow announcement state")
	}
}

// TestCaseService_CreateCase_Announcement_SNFailureLeavesPostgresUntouched
// mirrors the case-flavored orphan guard (TestCaseService_CreateCase_SNFailureLeavesPostgresUntouched)
// for announcement: if ServiceNow never accepts the announcement, Postgres
// must never be touched, same as case.
func TestCaseService_CreateCase_Announcement_SNFailureLeavesPostgresUntouched(t *testing.T) {
	mirror := &stubMirrorCaseService{
		createCase: func(context.Context, domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
			return domain.CreateCaseResponse{}, errors.New("sn downstream unreachable")
		},
	}
	repo := &stubCaseRepo{}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, dispatcher, mirror)

	_, err := svc.CreateCase(context.Background(), validAnnouncementCreateCaseRequest())
	if err == nil {
		t.Fatal("expected an error when ServiceNow never accepts the announcement")
	}
}
