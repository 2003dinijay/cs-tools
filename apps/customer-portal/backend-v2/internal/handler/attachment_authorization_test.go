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

package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// GET /attachments/{id}/content, GET /attachments/{id}, and DELETE
// /attachments/{id} are not nested under any project/case path, so unlike
// every other route in this backend there is no trusted path segment to
// scope them — before this fix, any authenticated caller could read or
// delete any other customer's attachment just by knowing or guessing its
// UUID. These tests cover GetAttachment/GetAttachmentContent specifically
// (DeleteAttachment's own equivalent coverage lives in
// closed_case_attachments_test.go, TestDeleteAttachment_ClosedCase).

// testCommentID stands in for a ServiceNow comment/journal-entry sysid --
// distinct from testCaseID/testDeploymentID, used by
// TestGetAttachmentContent_CommentFallback to simulate an attachment whose
// ReferenceID is a comment, not a case or deployment.
const testCommentID = "44444444-4444-4444-4444-444444444444"

// fakeAttachmentAuthzClient serves
// GetAttachment/GetAttachmentContent/GetAttachmentCase/SearchDeployments from
// canned values and records whether the guarded upstream call (content fetch,
// or the metadata response itself) was reached.
type fakeAttachmentAuthzClient struct {
	entityAttachmentClient
	referenceID       string
	getAttachmentErr  error
	getCaseErr        error
	searchDeploysErr  error
	deploymentVisible bool
	// unrelatedDeploymentID, when set, makes SearchDeployments return a
	// deployment with THIS id regardless of what the request's ids filter
	// asked for -- simulating snDeploymentService (plain
	// DATA_SOURCE=servicenow), which never forwards the ids filter at all.
	unrelatedDeploymentID string
	contentReached        bool
	// hintCaseErr, when set, makes GetCase (the caseHint access check inside
	// commentAttachmentIsVisible) fail -- distinct from getCaseErr, which
	// controls GetAttachmentCase instead.
	hintCaseErr error
	// commentInlineAttachmentIDs, when non-empty, makes SearchComments return
	// one comment carrying an InlineAttachment per id listed here.
	commentInlineAttachmentIDs []string
}

func (f *fakeAttachmentAuthzClient) GetAttachment(ctx context.Context, id string) (entity.AttachmentDetails, error) {
	if f.getAttachmentErr != nil {
		return entity.AttachmentDetails{}, f.getAttachmentErr
	}
	return entity.AttachmentDetails{ID: id, ReferenceID: f.referenceID, Name: "logs.txt"}, nil
}

func (f *fakeAttachmentAuthzClient) GetAttachmentCase(ctx context.Context, id string) (entity.CaseView, error) {
	if f.getCaseErr != nil {
		return entity.CaseView{}, f.getCaseErr
	}
	return entity.CaseView{ID: id, State: "open"}, nil
}

func (f *fakeAttachmentAuthzClient) SearchDeployments(ctx context.Context, req entity.SearchDeploymentsRequest) (entity.SearchDeploymentsResponse, error) {
	if f.searchDeploysErr != nil {
		return entity.SearchDeploymentsResponse{}, f.searchDeploysErr
	}
	if f.unrelatedDeploymentID != "" {
		return entity.SearchDeploymentsResponse{Deployments: []entity.DeploymentView{{ID: f.unrelatedDeploymentID}}, Total: 1}, nil
	}
	if !f.deploymentVisible {
		return entity.SearchDeploymentsResponse{}, nil
	}
	return entity.SearchDeploymentsResponse{Deployments: []entity.DeploymentView{{ID: req.IDs[0]}}, Total: 1}, nil
}

func (f *fakeAttachmentAuthzClient) GetAttachmentContent(ctx context.Context, id string) ([]byte, string, error) {
	f.contentReached = true
	return []byte("file bytes"), "text/plain", nil
}

func (f *fakeAttachmentAuthzClient) GetCase(ctx context.Context, id string) (entity.CaseView, error) {
	if f.hintCaseErr != nil {
		return entity.CaseView{}, f.hintCaseErr
	}
	return entity.CaseView{ID: id, State: "open"}, nil
}

func (f *fakeAttachmentAuthzClient) SearchComments(ctx context.Context, req entity.SearchCommentsRequest) (entity.SearchCommentsResponse, error) {
	var atts []entity.InlineAttachment
	for _, id := range f.commentInlineAttachmentIDs {
		atts = append(atts, entity.InlineAttachment{ID: id})
	}
	return entity.SearchCommentsResponse{Comments: []entity.CommentView{{InlineAttachments: atts}}}, nil
}

func attachmentAuthzTestCases() map[string]struct {
	client     fakeAttachmentAuthzClient
	wantStatus int
	wantAllow  bool
} {
	return map[string]struct {
		client     fakeAttachmentAuthzClient
		wantStatus int
		wantAllow  bool
	}{
		"caller can see the referenced case: allowed": {
			client:     fakeAttachmentAuthzClient{referenceID: testCaseID},
			wantStatus: http.StatusOK,
			wantAllow:  true,
		},
		"referenced case is outside the caller's scope: denied": {
			client:     fakeAttachmentAuthzClient{referenceID: testCaseID, getCaseErr: &apierror.Error{StatusCode: http.StatusNotFound}},
			wantStatus: http.StatusNotFound,
		},
		"no reference at all: denied": {
			client:     fakeAttachmentAuthzClient{referenceID: ""},
			wantStatus: http.StatusNotFound,
		},
		"attachment lookup itself fails: denied": {
			client:     fakeAttachmentAuthzClient{getAttachmentErr: &apierror.Error{StatusCode: http.StatusNotFound}},
			wantStatus: http.StatusNotFound,
		},
		// Deployment-referenced attachments are never resolvable via
		// GetAttachmentCase (a deployment id is never a real case), so these
		// all set getCaseErr
		// to a 404 -- the same 404 entity-service genuinely returns for one
		// live today (see authorizeAttachmentAccess's own doc comment for why
		// ReferenceType can't be used to route these directly instead).
		"deployment-referenced attachment visible to the caller: allowed": {
			client:     fakeAttachmentAuthzClient{referenceID: testDeploymentID, getCaseErr: &apierror.Error{StatusCode: http.StatusNotFound}, deploymentVisible: true},
			wantStatus: http.StatusOK,
			wantAllow:  true,
		},
		"deployment-referenced attachment outside the caller's scope: denied": {
			client:     fakeAttachmentAuthzClient{referenceID: testDeploymentID, getCaseErr: &apierror.Error{StatusCode: http.StatusNotFound}, deploymentVisible: false},
			wantStatus: http.StatusNotFound,
		},
		"deployment lookup itself fails: denied": {
			client:     fakeAttachmentAuthzClient{referenceID: testDeploymentID, getCaseErr: &apierror.Error{StatusCode: http.StatusNotFound}, searchDeploysErr: &apierror.Error{StatusCode: http.StatusServiceUnavailable}},
			wantStatus: http.StatusServiceUnavailable,
		},
		// CodeRabbit finding on PR #2432: the ServiceNow-backed SearchDeployments
		// adapter doesn't forward the ids filter at all, so a non-empty result
		// alone doesn't prove it's THIS deployment -- an unrelated deployment
		// the caller can see must not authorize access to a different one's
		// attachment.
		"deployment lookup returns an unrelated deployment: denied": {
			client:     fakeAttachmentAuthzClient{referenceID: testDeploymentID, getCaseErr: &apierror.Error{StatusCode: http.StatusNotFound}, unrelatedDeploymentID: "99999999-9999-9999-9999-999999999999"},
			wantStatus: http.StatusNotFound,
		},
	}
}

// TestGetAttachment_Authorization covers GET /attachments/{id}.
func TestGetAttachment_Authorization(t *testing.T) {
	for name, tc := range attachmentAuthzTestCases() {
		t.Run(name, func(t *testing.T) {
			fake := tc.client
			h := NewAttachmentHandler(&fake)

			mux := http.NewServeMux()
			mux.HandleFunc("GET /attachments/{id}", h.GetAttachment)

			req := authedRequest(http.MethodGet, "/attachments/"+testAttachmentID, "")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
		})
	}
}

// TestGetAttachmentContent_Authorization covers GET /attachments/{id}/content
// — the raw-binary-download route, distinct from metadata above. The
// authorization check must run before the content fetch, not after: an
// unauthorized caller must never reach entity-service's own binary download
// at all.
func TestGetAttachmentContent_Authorization(t *testing.T) {
	for name, tc := range attachmentAuthzTestCases() {
		t.Run(name, func(t *testing.T) {
			fake := tc.client
			h := NewAttachmentHandler(&fake)

			mux := http.NewServeMux()
			mux.HandleFunc("GET /attachments/{id}/content", h.GetAttachmentContent)

			req := authedRequest(http.MethodGet, "/attachments/"+testAttachmentID+"/content", "")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if fake.contentReached != tc.wantAllow {
				t.Errorf("entity GetAttachmentContent reached = %v, want %v", fake.contentReached, tc.wantAllow)
			}
		})
	}
}

// TestGetAttachmentContent_CommentFallback covers the ?caseId= hint and
// commentAttachmentIsVisible: an attachment pasted inline into a case comment
// has ReferenceID set to the comment's id, not the case's, so GetAttachmentCase
// always 404s for it -- these are only resolvable via the caller-supplied
// caseId hint plus the caller's own comments actually carrying the attachment
// as an inline attachment.
func TestGetAttachmentContent_CommentFallback(t *testing.T) {
	tests := map[string]struct {
		client      fakeAttachmentAuthzClient
		caseIDParam string
		wantStatus  int
		wantAllow   bool
	}{
		"no caseId hint: denied, unchanged from before this fallback existed": {
			client:     fakeAttachmentAuthzClient{referenceID: testCommentID, getCaseErr: &apierror.Error{StatusCode: http.StatusNotFound}},
			wantStatus: http.StatusNotFound,
		},
		"caseId hint, caller can see the case, attachment is one of its inline attachments: allowed": {
			client: fakeAttachmentAuthzClient{
				referenceID:                testCommentID,
				getCaseErr:                 &apierror.Error{StatusCode: http.StatusNotFound},
				commentInlineAttachmentIDs: []string{testAttachmentID},
			},
			caseIDParam: testCaseID,
			wantStatus:  http.StatusOK,
			wantAllow:   true,
		},
		"caseId hint, caller can see the case, but attachment is not among its inline attachments: denied": {
			client: fakeAttachmentAuthzClient{
				referenceID:                testCommentID,
				getCaseErr:                 &apierror.Error{StatusCode: http.StatusNotFound},
				commentInlineAttachmentIDs: []string{"99999999-9999-9999-9999-999999999999"},
			},
			caseIDParam: testCaseID,
			wantStatus:  http.StatusNotFound,
		},
		"caseId hint, but caller cannot see that case at all: denied": {
			client: fakeAttachmentAuthzClient{
				referenceID:                testCommentID,
				getCaseErr:                 &apierror.Error{StatusCode: http.StatusNotFound},
				hintCaseErr:                &apierror.Error{StatusCode: http.StatusNotFound},
				commentInlineAttachmentIDs: []string{testAttachmentID},
			},
			caseIDParam: testCaseID,
			wantStatus:  http.StatusNotFound,
		},
		"malformed caseId hint: rejected before any lookup": {
			client:      fakeAttachmentAuthzClient{referenceID: testCommentID},
			caseIDParam: "not-a-uuid",
			wantStatus:  http.StatusBadRequest,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			fake := tc.client
			h := NewAttachmentHandler(&fake)

			mux := http.NewServeMux()
			mux.HandleFunc("GET /attachments/{id}/content", h.GetAttachmentContent)

			url := "/attachments/" + testAttachmentID + "/content"
			if tc.caseIDParam != "" {
				url += "?caseId=" + tc.caseIDParam
			}
			req := authedRequest(http.MethodGet, url, "")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if fake.contentReached != tc.wantAllow {
				t.Errorf("entity GetAttachmentContent reached = %v, want %v", fake.contentReached, tc.wantAllow)
			}
		})
	}
}
