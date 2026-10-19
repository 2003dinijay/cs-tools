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
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

const (
	apcAttachmentID = "11111111-1111-1111-1111-111111111111"
	apcCaseID       = "22222222-2222-2222-2222-222222222222"
	apcCommentID    = "33333333-3333-3333-3333-333333333333"
	apcOtherCaseID  = "44444444-4444-4444-4444-444444444444"
)

// apcCaseService resolves an attachment to attachmentRef and only knows the
// cases listed in visibleCases (anything else is a 404, as for a caller who
// cannot see the case).
type apcCaseService struct {
	service.CaseService
	attachmentRef string
	visibleCases  map[string]bool
}

func (s *apcCaseService) GetAttachmentByID(_ context.Context, id string) (domain.AttachmentDetails, error) {
	return domain.AttachmentDetails{ID: id, ReferenceID: s.attachmentRef}, nil
}

func (s *apcCaseService) GetCaseByID(_ context.Context, id string) (domain.CaseView, error) {
	if !s.visibleCases[id] {
		return domain.CaseView{}, &apierror.NotFoundError{Msg: "case not found: " + id}
	}
	return domain.CaseView{ID: id}, nil
}

type apcComments struct {
	comment domain.Comment
	err     error
}

func (c *apcComments) GetComment(context.Context, string) (domain.Comment, error) {
	return c.comment, c.err
}

func TestGetAttachmentParentCase_CommentFallback(t *testing.T) {
	notFound := &apierror.NotFoundError{Msg: "comment not found"}
	tests := map[string]struct {
		attachmentRef string
		visible       map[string]bool
		comments      *apcComments // nil = no resolver configured
		wantStatus    int
		wantCaseID    string
	}{
		"case-referenced attachment: unchanged, no comment lookup needed": {
			attachmentRef: apcCaseID, visible: map[string]bool{apcCaseID: true},
			wantStatus: http.StatusOK, wantCaseID: apcCaseID,
		},
		"comment-referenced, comment on a visible case: allowed": {
			attachmentRef: apcCommentID, visible: map[string]bool{apcCaseID: true},
			comments:   &apcComments{comment: domain.Comment{ID: apcCommentID, ReferenceID: apcCaseID, Type: domain.CommentTypeComment}},
			wantStatus: http.StatusOK, wantCaseID: apcCaseID,
		},
		"comment-referenced, comment's case not visible to caller: denied": {
			attachmentRef: apcCommentID, visible: map[string]bool{apcCaseID: true},
			comments:   &apcComments{comment: domain.Comment{ID: apcCommentID, ReferenceID: apcOtherCaseID, Type: domain.CommentTypeComment}},
			wantStatus: http.StatusNotFound,
		},
		"comment not visible to caller (not found): denied": {
			attachmentRef: apcCommentID, visible: map[string]bool{apcCaseID: true},
			comments: &apcComments{err: notFound}, wantStatus: http.StatusNotFound,
		},
		"work note: denied": {
			attachmentRef: apcCommentID, visible: map[string]bool{apcCaseID: true},
			comments:   &apcComments{comment: domain.Comment{ReferenceID: apcCaseID, Type: domain.CommentTypeWorkNote}},
			wantStatus: http.StatusNotFound,
		},
		"deleted comment: denied": {
			attachmentRef: apcCommentID, visible: map[string]bool{apcCaseID: true},
			comments:   &apcComments{comment: domain.Comment{ReferenceID: apcCaseID, Type: domain.CommentTypeComment, IsDeleted: true}},
			wantStatus: http.StatusNotFound,
		},
		"no comment resolver configured: denied as before": {
			attachmentRef: apcCommentID, visible: map[string]bool{apcCaseID: true},
			wantStatus: http.StatusNotFound,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			h := NewCaseHandler(&apcCaseService{attachmentRef: tc.attachmentRef, visibleCases: tc.visible}, nil)
			if tc.comments != nil {
				h.WithCommentResolver(tc.comments)
			}
			mux := http.NewServeMux()
			mux.HandleFunc("GET /attachments/{id}/case", h.GetAttachmentParentCase)

			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/attachments/"+apcAttachmentID+"/case", nil))

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantCaseID != "" && !strings.Contains(rec.Body.String(), tc.wantCaseID) {
				t.Errorf("body %s does not name case %s", rec.Body.String(), tc.wantCaseID)
			}
		})
	}
}
