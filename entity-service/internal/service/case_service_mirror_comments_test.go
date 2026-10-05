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
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// mirrorSNCaseService is the ServiceNow-side CaseService whose comment search
// returns the comments ServiceNow generated for a just-created case.
type mirrorSNCaseService struct {
	CaseService
	comments []domain.CaseComment
}

func (m *mirrorSNCaseService) SearchCaseComments(_ context.Context, _ domain.SearchCaseCommentsRequest) (domain.SearchCaseCommentsResponse, error) {
	return domain.SearchCaseCommentsResponse{Comments: m.comments}, nil
}

// ServiceNow can generate WORK_NOTE comments for a new case, and the customer
// who created the case is the caller here. A WORK_NOTE is refused for an
// external identity (migration 0191), so the mirror write must run as the
// system. If it did not, ServiceNow-originated work notes would be silently
// lost (the failure is logged and does not fail the case creation).
func TestMirrorInitialSNComments_WritesAsSystemIdentity(t *testing.T) {
	const caseID = "44444444-4444-4444-4444-444444444444"
	created := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	var gotCtx context.Context
	var gotReqs []domain.CreateCaseCommentRequest

	repo := &stubCaseRepo{
		createCaseComment: func(ctx context.Context, req domain.CreateCaseCommentRequest, _ *time.Time) (domain.CaseComment, error) {
			gotCtx = ctx
			gotReqs = append(gotReqs, req)
			return domain.CaseComment{}, nil
		},
	}
	svc := &caseService{
		repo: repo,
		snMirror: &mirrorSNCaseService{comments: []domain.CaseComment{
			{Type: domain.CommentTypeWorkNote, Content: "auto-generated note", CreatedOn: created},
			{Type: domain.CommentTypeActivity, Content: "audit trail", CreatedOn: created},
			{Type: domain.CommentTypeComment, Content: "title and description", CreatedOn: created},
		}},
	}

	svc.mirrorInitialSNComments(externalCustomerContext(), caseID)

	if len(gotReqs) != 2 {
		t.Fatalf("expected the work note and the comment to be mirrored (the audit entry is skipped), got %d writes", len(gotReqs))
	}
	if gotReqs[0].Type != domain.CommentTypeWorkNote || gotReqs[0].CaseID != caseID {
		t.Fatalf("expected the WORK_NOTE to be written to case %s, got %+v", caseID, gotReqs[0])
	}
	id, ok := repository.CallerIdentityFromContext(gotCtx)
	if !ok || !id.Unrestricted {
		t.Fatalf("the mirror write must run as the system identity, got %+v (ok=%v)", id, ok)
	}
}
