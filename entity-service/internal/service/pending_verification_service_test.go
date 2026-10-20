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

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// fakePendingVerificationRepo is a minimal repository.PendingVerificationRepository
// -- each method is a func field, nil-checked, so a test only wires up the
// ones it actually exercises; calling an unwired method panics loudly rather
// than silently returning a zero value, same convention as this package's
// other fake repos.
type fakePendingVerificationRepo struct {
	create                        func(ctx context.Context, req domain.CreatePendingVerificationRequest) (domain.PendingVerification, error)
	getWorkItemTypeAndProject     func(ctx context.Context, workItemID string) (string, string, *string, error)
	getProjectIDForEntry          func(ctx context.Context, id string) (string, error)
	getProjectVerificationEnabled func(ctx context.Context, projectID string) (bool, error)
	search                        func(ctx context.Context, req domain.SearchPendingVerificationsRequest) (domain.SearchPendingVerificationsResponse, error)
	verify                        func(ctx context.Context, req domain.VerifyPendingVerificationRequest) (domain.PendingVerification, error)
}

func (f *fakePendingVerificationRepo) Create(ctx context.Context, req domain.CreatePendingVerificationRequest) (domain.PendingVerification, error) {
	if f.create == nil {
		panic("fakePendingVerificationRepo.Create: not expected to be called by this test")
	}
	return f.create(ctx, req)
}

func (f *fakePendingVerificationRepo) GetWorkItemTypeAndProject(ctx context.Context, workItemID string) (string, string, *string, error) {
	if f.getWorkItemTypeAndProject == nil {
		panic("fakePendingVerificationRepo.GetWorkItemTypeAndProject: not expected to be called by this test")
	}
	return f.getWorkItemTypeAndProject(ctx, workItemID)
}

func (f *fakePendingVerificationRepo) GetProjectIDForEntry(ctx context.Context, id string) (string, error) {
	if f.getProjectIDForEntry == nil {
		panic("fakePendingVerificationRepo.GetProjectIDForEntry: not expected to be called by this test")
	}
	return f.getProjectIDForEntry(ctx, id)
}

func (f *fakePendingVerificationRepo) GetProjectVerificationEnabled(ctx context.Context, projectID string) (bool, error) {
	if f.getProjectVerificationEnabled == nil {
		panic("fakePendingVerificationRepo.GetProjectVerificationEnabled: not expected to be called by this test")
	}
	return f.getProjectVerificationEnabled(ctx, projectID)
}

func (f *fakePendingVerificationRepo) Search(ctx context.Context, req domain.SearchPendingVerificationsRequest) (domain.SearchPendingVerificationsResponse, error) {
	if f.search == nil {
		panic("fakePendingVerificationRepo.Search: not expected to be called by this test")
	}
	return f.search(ctx, req)
}

func (f *fakePendingVerificationRepo) Verify(ctx context.Context, req domain.VerifyPendingVerificationRequest) (domain.PendingVerification, error) {
	if f.verify == nil {
		panic("fakePendingVerificationRepo.Verify: not expected to be called by this test")
	}
	return f.verify(ctx, req)
}

const (
	pvTestWorkItemID  = "6fa0b42d-1bfa-a694-a002-c9d3604bcb77"
	pvTestEntryID     = "d121847c-58a4-4266-bf83-cffba008e1de"
	pvTestProjectID   = "11111111-2222-4333-8444-555555555555"
	pvTestCallerEmail = "jane.doe@example.com"
)

// pvEnabled is a getProjectVerificationEnabled stub that always reports the
// feature enabled -- wired into every test below that needs to reach past
// that gate to exercise something else.
func pvEnabled(context.Context, string) (bool, error) { return true, nil }

func assertValidationError(t *testing.T, err error) {
	t.Helper()
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

func assertForbiddenError(t *testing.T, err error) {
	t.Helper()
	var fe *apierror.ForbiddenError
	if !errors.As(err, &fe) {
		t.Fatalf("expected *apierror.ForbiddenError, got %T: %v", err, err)
	}
}

// --- CreatePendingVerification ---------------------------------------------

func TestPendingVerificationService_Create_InvalidWorkItemID(t *testing.T) {
	svc := NewPendingVerificationService(&fakePendingVerificationRepo{}, stubAccess{scope: AccessScope{Unrestricted: true}})
	_, err := svc.CreatePendingVerification(context.Background(), domain.CreatePendingVerificationRequest{
		WorkItemID:  "not-a-uuid",
		AddedReason: domain.PendingVerificationAddedReasonManual,
	})
	assertValidationError(t, err)
}

func TestPendingVerificationService_Create_InvalidAddedReason(t *testing.T) {
	svc := NewPendingVerificationService(&fakePendingVerificationRepo{}, stubAccess{scope: AccessScope{Unrestricted: true}})
	_, err := svc.CreatePendingVerification(context.Background(), domain.CreatePendingVerificationRequest{
		WorkItemID:  pvTestWorkItemID,
		AddedReason: "SOMETHING_ELSE",
	})
	assertValidationError(t, err)
}

func TestPendingVerificationService_Create_AutoClosedRequiresPreviousStatus(t *testing.T) {
	svc := NewPendingVerificationService(&fakePendingVerificationRepo{}, stubAccess{scope: AccessScope{Unrestricted: true}})
	_, err := svc.CreatePendingVerification(context.Background(), domain.CreatePendingVerificationRequest{
		WorkItemID:  pvTestWorkItemID,
		AddedReason: domain.PendingVerificationAddedReasonAutoClosed,
		// PreviousStatus deliberately omitted.
	})
	assertValidationError(t, err)
}

func TestPendingVerificationService_Create_ManualRejectsPreviousStatus(t *testing.T) {
	status := domain.PendingVerificationPreviousStatusAwaitingInfo
	svc := NewPendingVerificationService(&fakePendingVerificationRepo{}, stubAccess{scope: AccessScope{Unrestricted: true}})
	_, err := svc.CreatePendingVerification(context.Background(), domain.CreatePendingVerificationRequest{
		WorkItemID:     pvTestWorkItemID,
		AddedReason:    domain.PendingVerificationAddedReasonManual,
		PreviousStatus: &status,
	})
	assertValidationError(t, err)
}

func TestPendingVerificationService_Create_InvalidPreviousStatusValue(t *testing.T) {
	bad := domain.PendingVerificationPreviousStatus("CLOSED")
	svc := NewPendingVerificationService(&fakePendingVerificationRepo{}, stubAccess{scope: AccessScope{Unrestricted: true}})
	_, err := svc.CreatePendingVerification(context.Background(), domain.CreatePendingVerificationRequest{
		WorkItemID:     pvTestWorkItemID,
		AddedReason:    domain.PendingVerificationAddedReasonAutoClosed,
		PreviousStatus: &bad,
	})
	assertValidationError(t, err)
}

// TestPendingVerificationService_Create_AutoClosedRequiresInternalCaller
// locks in the CodeRabbit finding: AUTO_CLOSED claims the system itself
// closed the case, so only an internal caller may create one -- a
// registered customer (a non-Unrestricted scope) must be refused before the
// repository is ever reached, since RLS alone only checks project
// membership, never added_reason.
func TestPendingVerificationService_Create_AutoClosedRequiresInternalCaller(t *testing.T) {
	status := domain.PendingVerificationPreviousStatusAwaitingInfo
	repo := &fakePendingVerificationRepo{
		getWorkItemTypeAndProject: func(context.Context, string) (string, string, *string, error) {
			t.Fatal("the repository must not be reached for a non-internal caller's auto-closed request")
			return "", "", nil, nil
		},
		create: func(context.Context, domain.CreatePendingVerificationRequest) (domain.PendingVerification, error) {
			t.Fatal("Create must not be called for a non-internal caller's auto-closed request")
			return domain.PendingVerification{}, nil
		},
	}
	svc := NewPendingVerificationService(repo, restrictedAccess{})
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, pvTestCallerEmail))
	_, err := svc.CreatePendingVerification(ctx, domain.CreatePendingVerificationRequest{
		WorkItemID:     pvTestWorkItemID,
		AddedReason:    domain.PendingVerificationAddedReasonAutoClosed,
		PreviousStatus: &status,
	})
	assertForbiddenError(t, err)
}

// TestPendingVerificationService_Create_ManualDoesNotRequireInternalCaller
// confirms the internal-caller gate is scoped to AUTO_CLOSED only -- a
// registered (non-internal) project member can still create a MANUAL entry.
func TestPendingVerificationService_Create_ManualDoesNotRequireInternalCaller(t *testing.T) {
	wantPV := domain.PendingVerification{ID: pvTestEntryID, WorkItemID: pvTestWorkItemID}
	repo := &fakePendingVerificationRepo{
		getWorkItemTypeAndProject: func(context.Context, string) (string, string, *string, error) {
			return "CASE", pvTestProjectID, strPtr("CLOSED"), nil
		},
		getProjectVerificationEnabled: pvEnabled,
		create: func(context.Context, domain.CreatePendingVerificationRequest) (domain.PendingVerification, error) {
			return wantPV, nil
		},
	}
	svc := NewPendingVerificationService(repo, restrictedAccess{})
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, pvTestCallerEmail))
	_, err := svc.CreatePendingVerification(ctx, domain.CreatePendingVerificationRequest{
		WorkItemID:  pvTestWorkItemID,
		AddedReason: domain.PendingVerificationAddedReasonManual,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestPendingVerificationService_Create_RejectsNonCase locks in the
// 2026-10-06 Case-only scope-down: CreatePendingVerification must reject a
// workItemId whose own work_item.type isn't CASE, before ever reaching the
// repository's Create/INSERT.
func TestPendingVerificationService_Create_RejectsNonCase(t *testing.T) {
	repo := &fakePendingVerificationRepo{
		getWorkItemTypeAndProject: func(context.Context, string) (string, string, *string, error) {
			return "ENGAGEMENT", pvTestProjectID, nil, nil
		},
		create: func(context.Context, domain.CreatePendingVerificationRequest) (domain.PendingVerification, error) {
			t.Fatal("Create must not be called for a non-Case work item")
			return domain.PendingVerification{}, nil
		},
	}
	svc := NewPendingVerificationService(repo, stubAccess{scope: AccessScope{Unrestricted: true}})
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, pvTestCallerEmail))
	_, err := svc.CreatePendingVerification(ctx, domain.CreatePendingVerificationRequest{
		WorkItemID:  pvTestWorkItemID,
		AddedReason: domain.PendingVerificationAddedReasonManual,
	})
	assertValidationError(t, err)
}

// TestPendingVerificationService_Create_RejectsOpenCase locks in a real
// finding: "Add to Verification" only ever applies to a closed case, and
// that rule used to live only in the webapp (VerificationActionButtons' own
// isRecordClosed) -- any caller with case-create access could otherwise POST
// an open case straight through the API. Covers both "no case row joined"
// (nil, shouldn't happen for a CASE-typed work item, but checked anyway) and
// a real, non-CLOSED state.
func TestPendingVerificationService_Create_RejectsOpenCase(t *testing.T) {
	tests := []struct {
		name  string
		state *string
	}{
		{"open case", strPtr("OPEN")},
		{"work in progress", strPtr("WORK_IN_PROGRESS")},
		{"nil state", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakePendingVerificationRepo{
				getWorkItemTypeAndProject: func(context.Context, string) (string, string, *string, error) {
					return "CASE", pvTestProjectID, tt.state, nil
				},
				create: func(context.Context, domain.CreatePendingVerificationRequest) (domain.PendingVerification, error) {
					t.Fatal("Create must not be called for a case that isn't closed")
					return domain.PendingVerification{}, nil
				},
			}
			svc := NewPendingVerificationService(repo, stubAccess{scope: AccessScope{Unrestricted: true}})
			ctx := contextWithUserIDToken(fakeJWTWithEmail(t, pvTestCallerEmail))
			_, err := svc.CreatePendingVerification(ctx, domain.CreatePendingVerificationRequest{
				WorkItemID:  pvTestWorkItemID,
				AddedReason: domain.PendingVerificationAddedReasonManual,
			})
			assertValidationError(t, err)
		})
	}
}

// TestPendingVerificationService_Create_PropagatesWorkItemLookupError
// confirms a work item the caller can't see (RLS-filtered to zero rows,
// surfaced by the repo as NotFoundError/ValidationError) is propagated
// as-is, never masked as a different error.
func TestPendingVerificationService_Create_PropagatesWorkItemLookupError(t *testing.T) {
	wantErr := &apierror.ValidationError{Msg: "workItemId does not reference an existing record"}
	repo := &fakePendingVerificationRepo{
		getWorkItemTypeAndProject: func(context.Context, string) (string, string, *string, error) {
			return "", "", nil, wantErr
		},
	}
	svc := NewPendingVerificationService(repo, stubAccess{scope: AccessScope{Unrestricted: true}})
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, pvTestCallerEmail))
	_, err := svc.CreatePendingVerification(ctx, domain.CreatePendingVerificationRequest{
		WorkItemID:  pvTestWorkItemID,
		AddedReason: domain.PendingVerificationAddedReasonManual,
	})
	if !errors.Is(err, error(wantErr)) {
		t.Fatalf("err = %v, want the exact error GetWorkItemTypeAndProject returned (%v), propagated unchanged", err, wantErr)
	}
}

// TestPendingVerificationService_Create_RejectsWhenVerificationDisabled locks
// in the gate CodeRabbit's review found missing: RLS only checks project
// membership, not whether the project has this feature turned on at all.
func TestPendingVerificationService_Create_RejectsWhenVerificationDisabled(t *testing.T) {
	repo := &fakePendingVerificationRepo{
		getWorkItemTypeAndProject: func(context.Context, string) (string, string, *string, error) {
			return "CASE", pvTestProjectID, strPtr("CLOSED"), nil
		},
		getProjectVerificationEnabled: func(context.Context, string) (bool, error) { return false, nil },
		create: func(context.Context, domain.CreatePendingVerificationRequest) (domain.PendingVerification, error) {
			t.Fatal("Create must not be called when the project has the feature disabled")
			return domain.PendingVerification{}, nil
		},
	}
	svc := NewPendingVerificationService(repo, stubAccess{scope: AccessScope{Unrestricted: true}})
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, pvTestCallerEmail))
	_, err := svc.CreatePendingVerification(ctx, domain.CreatePendingVerificationRequest{
		WorkItemID:  pvTestWorkItemID,
		AddedReason: domain.PendingVerificationAddedReasonManual,
	})
	assertForbiddenError(t, err)
}

func TestPendingVerificationService_Create_RequiresCallerToken(t *testing.T) {
	repo := &fakePendingVerificationRepo{
		getWorkItemTypeAndProject: func(context.Context, string) (string, string, *string, error) {
			return "CASE", pvTestProjectID, strPtr("CLOSED"), nil
		},
		getProjectVerificationEnabled: pvEnabled,
	}
	svc := NewPendingVerificationService(repo, stubAccess{scope: AccessScope{Unrestricted: true}})
	ctx := contextWithUserIDToken("") // no x-user-id-token header
	_, err := svc.CreatePendingVerification(ctx, domain.CreatePendingVerificationRequest{
		WorkItemID:  pvTestWorkItemID,
		AddedReason: domain.PendingVerificationAddedReasonManual,
	})
	var unauthorized *apierror.UnauthorizedError
	if !errors.As(err, &unauthorized) {
		t.Fatalf("expected *apierror.UnauthorizedError, got %T: %v", err, err)
	}
}

// TestPendingVerificationService_Create_HappyPath confirms: the work item
// type check passes for CASE, addedBy is resolved from the caller's own
// token (never trusted from the request), and the repo's response is
// returned unchanged inside the standard success envelope.
func TestPendingVerificationService_Create_HappyPath(t *testing.T) {
	var gotReq domain.CreatePendingVerificationRequest
	wantPV := domain.PendingVerification{ID: pvTestEntryID, WorkItemID: pvTestWorkItemID, RecordNumber: "CS0441241"}
	repo := &fakePendingVerificationRepo{
		getWorkItemTypeAndProject: func(_ context.Context, workItemID string) (string, string, *string, error) {
			if workItemID != pvTestWorkItemID {
				t.Fatalf("GetWorkItemTypeAndProject called with %q, want %q", workItemID, pvTestWorkItemID)
			}
			return "CASE", pvTestProjectID, strPtr("CLOSED"), nil
		},
		getProjectVerificationEnabled: pvEnabled,
		create: func(_ context.Context, req domain.CreatePendingVerificationRequest) (domain.PendingVerification, error) {
			gotReq = req
			return wantPV, nil
		},
	}
	svc := NewPendingVerificationService(repo, stubAccess{scope: AccessScope{Unrestricted: true}})
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, pvTestCallerEmail))

	resp, err := svc.CreatePendingVerification(ctx, domain.CreatePendingVerificationRequest{
		WorkItemID:  pvTestWorkItemID,
		AddedReason: domain.PendingVerificationAddedReasonManual,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.AddedBy != pvTestCallerEmail {
		t.Fatalf("Create called with AddedBy=%q, want the caller's own email %q -- never client-supplied", gotReq.AddedBy, pvTestCallerEmail)
	}
	if resp.Message != "Added to Pending Verification" {
		t.Fatalf("Message = %q", resp.Message)
	}
	if resp.PendingVerification != wantPV {
		t.Fatalf("PendingVerification = %+v, want %+v", resp.PendingVerification, wantPV)
	}
}

// --- SearchPendingVerifications ---------------------------------------------

func TestPendingVerificationService_Search_RequiresProjectID(t *testing.T) {
	svc := NewPendingVerificationService(&fakePendingVerificationRepo{}, stubAccess{scope: AccessScope{Unrestricted: true}})
	_, err := svc.SearchPendingVerifications(context.Background(), domain.SearchPendingVerificationsRequest{
		Filters: domain.PendingVerificationSearchFilters{ProjectID: "not-a-uuid"},
	})
	assertValidationError(t, err)
}

func TestPendingVerificationService_Search_InvalidWorkItemIDFilter(t *testing.T) {
	badID := "not-a-uuid"
	svc := NewPendingVerificationService(&fakePendingVerificationRepo{}, stubAccess{scope: AccessScope{Unrestricted: true}})
	_, err := svc.SearchPendingVerifications(context.Background(), domain.SearchPendingVerificationsRequest{
		Filters: domain.PendingVerificationSearchFilters{ProjectID: pvTestWorkItemID, WorkItemID: &badID},
	})
	assertValidationError(t, err)
}

func TestPendingVerificationService_Search_SearchQueryTooLong(t *testing.T) {
	long := make([]byte, 201)
	for i := range long {
		long[i] = 'a'
	}
	svc := NewPendingVerificationService(&fakePendingVerificationRepo{}, stubAccess{scope: AccessScope{Unrestricted: true}})
	_, err := svc.SearchPendingVerifications(context.Background(), domain.SearchPendingVerificationsRequest{
		Filters: domain.PendingVerificationSearchFilters{ProjectID: pvTestWorkItemID, SearchQuery: string(long)},
	})
	assertValidationError(t, err)
}

func TestPendingVerificationService_Search_UnsupportedWorkItemType(t *testing.T) {
	svc := NewPendingVerificationService(&fakePendingVerificationRepo{}, stubAccess{scope: AccessScope{Unrestricted: true}})
	_, err := svc.SearchPendingVerifications(context.Background(), domain.SearchPendingVerificationsRequest{
		Filters: domain.PendingVerificationSearchFilters{ProjectID: pvTestWorkItemID, WorkItemTypes: []string{"INCIDENT"}},
	})
	assertValidationError(t, err)
}

// TestPendingVerificationService_Search_RejectsNonCaseWorkItemType locks in
// a real finding: buildSearchWhere unconditionally ANDs wi.type = 'CASE'
// onto every search regardless of filters.workItemTypes, so a request for
// any of the five other work_item_type_enum values the doc's §2 scope table
// originally named (the feature has since been scoped to Case only) used to
// pass this validation and then silently return zero rows forever -- never
// a 400 explaining the filter does nothing. Each must now be rejected
// outright instead.
func TestPendingVerificationService_Search_RejectsNonCaseWorkItemType(t *testing.T) {
	for _, recordType := range []string{"ENGAGEMENT", "SECURITY_REPORT_ANALYSIS", "SERVICE_REQUEST", "ANNOUNCEMENT", "CHANGE_REQUEST"} {
		t.Run(recordType, func(t *testing.T) {
			svc := NewPendingVerificationService(&fakePendingVerificationRepo{}, stubAccess{scope: AccessScope{Unrestricted: true}})
			_, err := svc.SearchPendingVerifications(context.Background(), domain.SearchPendingVerificationsRequest{
				Filters: domain.PendingVerificationSearchFilters{ProjectID: pvTestWorkItemID, WorkItemTypes: []string{recordType}},
			})
			assertValidationError(t, err)
		})
	}
}

// TestPendingVerificationService_Search_InvalidAddedReason locks in the new
// addedReason filter's own validation, added alongside the Support page's
// Auto-closed/Manual stat boxes.
func TestPendingVerificationService_Search_InvalidAddedReason(t *testing.T) {
	bad := domain.PendingVerificationAddedReason("SOMETHING_ELSE")
	svc := NewPendingVerificationService(&fakePendingVerificationRepo{}, stubAccess{scope: AccessScope{Unrestricted: true}})
	_, err := svc.SearchPendingVerifications(context.Background(), domain.SearchPendingVerificationsRequest{
		Filters: domain.PendingVerificationSearchFilters{ProjectID: pvTestWorkItemID, AddedReason: &bad},
	})
	assertValidationError(t, err)
}

func TestPendingVerificationService_Search_LimitExceedsMax(t *testing.T) {
	svc := NewPendingVerificationService(&fakePendingVerificationRepo{}, stubAccess{scope: AccessScope{Unrestricted: true}})
	_, err := svc.SearchPendingVerifications(context.Background(), domain.SearchPendingVerificationsRequest{
		Pagination: domain.Pagination{Limit: 500},
		Filters:    domain.PendingVerificationSearchFilters{ProjectID: pvTestWorkItemID},
	})
	assertValidationError(t, err)
}

// TestPendingVerificationService_Search_RejectsWhenVerificationDisabled locks
// in the gate CodeRabbit's review found missing (see the matching Create
// test's own comment).
func TestPendingVerificationService_Search_RejectsWhenVerificationDisabled(t *testing.T) {
	repo := &fakePendingVerificationRepo{
		getProjectVerificationEnabled: func(context.Context, string) (bool, error) { return false, nil },
		search: func(context.Context, domain.SearchPendingVerificationsRequest) (domain.SearchPendingVerificationsResponse, error) {
			t.Fatal("Search must not be called when the project has the feature disabled")
			return domain.SearchPendingVerificationsResponse{}, nil
		},
	}
	svc := NewPendingVerificationService(repo, stubAccess{scope: AccessScope{Unrestricted: true}})
	_, err := svc.SearchPendingVerifications(context.Background(), domain.SearchPendingVerificationsRequest{
		Filters: domain.PendingVerificationSearchFilters{ProjectID: pvTestProjectID},
	})
	assertForbiddenError(t, err)
}

// TestPendingVerificationService_Search_HappyPath confirms a valid request,
// including the new addedReason filter, reaches the repository unchanged and
// its response is returned as-is.
func TestPendingVerificationService_Search_HappyPath(t *testing.T) {
	autoClosed := domain.PendingVerificationAddedReasonAutoClosed
	var gotReq domain.SearchPendingVerificationsRequest
	wantResp := domain.SearchPendingVerificationsResponse{Total: 1}
	repo := &fakePendingVerificationRepo{
		getProjectVerificationEnabled: pvEnabled,
		search: func(_ context.Context, req domain.SearchPendingVerificationsRequest) (domain.SearchPendingVerificationsResponse, error) {
			gotReq = req
			return wantResp, nil
		},
	}
	svc := NewPendingVerificationService(repo, stubAccess{scope: AccessScope{Unrestricted: true}})

	resp, err := svc.SearchPendingVerifications(context.Background(), domain.SearchPendingVerificationsRequest{
		Filters: domain.PendingVerificationSearchFilters{ProjectID: pvTestWorkItemID, AddedReason: &autoClosed},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.Filters.AddedReason == nil || *gotReq.Filters.AddedReason != autoClosed {
		t.Fatalf("Filters.AddedReason = %v, want %q", gotReq.Filters.AddedReason, autoClosed)
	}
	if gotReq.Pagination.Limit != defaultLimit {
		t.Fatalf("Pagination.Limit = %d, want the normalized default %d", gotReq.Pagination.Limit, defaultLimit)
	}
	if resp.Total != wantResp.Total {
		t.Fatalf("response.Total = %d, want %d -- the repo's response must pass through unchanged", resp.Total, wantResp.Total)
	}
}

// --- VerifyPendingVerification -----------------------------------------------

func TestPendingVerificationService_Verify_InvalidID(t *testing.T) {
	svc := NewPendingVerificationService(&fakePendingVerificationRepo{}, stubAccess{scope: AccessScope{Unrestricted: true}})
	_, err := svc.VerifyPendingVerification(context.Background(), domain.VerifyPendingVerificationRequest{ID: "not-a-uuid"})
	assertValidationError(t, err)
}

func TestPendingVerificationService_Verify_RequiresCallerToken(t *testing.T) {
	repo := &fakePendingVerificationRepo{
		getProjectIDForEntry:          func(context.Context, string) (string, error) { return pvTestProjectID, nil },
		getProjectVerificationEnabled: pvEnabled,
	}
	svc := NewPendingVerificationService(repo, stubAccess{scope: AccessScope{Unrestricted: true}})
	ctx := contextWithUserIDToken("")
	_, err := svc.VerifyPendingVerification(ctx, domain.VerifyPendingVerificationRequest{ID: pvTestEntryID})
	var unauthorized *apierror.UnauthorizedError
	if !errors.As(err, &unauthorized) {
		t.Fatalf("expected *apierror.UnauthorizedError, got %T: %v", err, err)
	}
}

// TestPendingVerificationService_Verify_RejectsWhenVerificationDisabled locks
// in the gate CodeRabbit's review found missing (see the matching Create
// test's own comment). This request carries only the entry's id, so the
// project it belongs to has to be resolved first.
func TestPendingVerificationService_Verify_RejectsWhenVerificationDisabled(t *testing.T) {
	repo := &fakePendingVerificationRepo{
		getProjectIDForEntry:          func(context.Context, string) (string, error) { return pvTestProjectID, nil },
		getProjectVerificationEnabled: func(context.Context, string) (bool, error) { return false, nil },
		verify: func(context.Context, domain.VerifyPendingVerificationRequest) (domain.PendingVerification, error) {
			t.Fatal("Verify must not be called when the project has the feature disabled")
			return domain.PendingVerification{}, nil
		},
	}
	svc := NewPendingVerificationService(repo, stubAccess{scope: AccessScope{Unrestricted: true}})
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, pvTestCallerEmail))
	_, err := svc.VerifyPendingVerification(ctx, domain.VerifyPendingVerificationRequest{ID: pvTestEntryID})
	assertForbiddenError(t, err)
}

// TestPendingVerificationService_Verify_HappyPath confirms verifiedBy is
// resolved from the caller's own token (never trusted from the request) and
// the success message embeds the verified record's own number.
func TestPendingVerificationService_Verify_HappyPath(t *testing.T) {
	var gotReq domain.VerifyPendingVerificationRequest
	wantPV := domain.PendingVerification{ID: pvTestEntryID, RecordNumber: "CS0441241"}
	repo := &fakePendingVerificationRepo{
		getProjectIDForEntry:          func(context.Context, string) (string, error) { return pvTestProjectID, nil },
		getProjectVerificationEnabled: pvEnabled,
		verify: func(_ context.Context, req domain.VerifyPendingVerificationRequest) (domain.PendingVerification, error) {
			gotReq = req
			return wantPV, nil
		},
	}
	svc := NewPendingVerificationService(repo, stubAccess{scope: AccessScope{Unrestricted: true}})
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, pvTestCallerEmail))

	resp, err := svc.VerifyPendingVerification(ctx, domain.VerifyPendingVerificationRequest{ID: pvTestEntryID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.VerifiedBy != pvTestCallerEmail {
		t.Fatalf("Verify called with VerifiedBy=%q, want the caller's own email %q -- never client-supplied", gotReq.VerifiedBy, pvTestCallerEmail)
	}
	if resp.Message != "Marked as verified — CS0441241 has been removed from your pending verification list." {
		t.Fatalf("Message = %q", resp.Message)
	}
	if resp.PendingVerification != wantPV {
		t.Fatalf("PendingVerification = %+v, want %+v", resp.PendingVerification, wantPV)
	}
}

// TestPendingVerificationService_Verify_PropagatesNotFound confirms a
// missing-or-already-verified entry (the repo's NotFoundError) is
// propagated as-is, not masked.
func TestPendingVerificationService_Verify_PropagatesNotFound(t *testing.T) {
	wantErr := &apierror.NotFoundError{Msg: "pending_verification not found, or already verified"}
	repo := &fakePendingVerificationRepo{
		getProjectIDForEntry:          func(context.Context, string) (string, error) { return pvTestProjectID, nil },
		getProjectVerificationEnabled: pvEnabled,
		verify: func(context.Context, domain.VerifyPendingVerificationRequest) (domain.PendingVerification, error) {
			return domain.PendingVerification{}, wantErr
		},
	}
	svc := NewPendingVerificationService(repo, stubAccess{scope: AccessScope{Unrestricted: true}})
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, pvTestCallerEmail))

	_, err := svc.VerifyPendingVerification(ctx, domain.VerifyPendingVerificationRequest{ID: pvTestEntryID})
	var notFound *apierror.NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("expected *apierror.NotFoundError, got %T: %v", err, err)
	}
}
