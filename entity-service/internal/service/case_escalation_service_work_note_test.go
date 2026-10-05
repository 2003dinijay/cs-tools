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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const workNoteTestCaseID = "33333333-3333-3333-3333-333333333333"

// recordingEscalations is an EscalationService whose CreateEscalation records
// the context it was called with and returns a canned escalation.
type recordingEscalations struct {
	EscalationService
	createCtx context.Context
}

func (r *recordingEscalations) CreateEscalation(ctx context.Context, _ domain.CreateEscalationRequest) (domain.CreateEscalationResponse, error) {
	r.createCtx = ctx
	return domain.CreateEscalationResponse{Escalation: domain.CreatedEscalation{ID: "esc-1"}}, nil
}

// recordingCaseService is a CaseService whose CreateCaseComment records the
// context and request it was called with.
type recordingCaseService struct {
	CaseService
	commentCtx context.Context
	comment    domain.CreateCaseCommentRequest
	err        error
}

func (r *recordingCaseService) CreateCaseComment(ctx context.Context, req domain.CreateCaseCommentRequest) (domain.CreateCaseCommentResponse, error) {
	r.commentCtx = ctx
	r.comment = req
	return domain.CreateCaseCommentResponse{}, r.err
}

func externalCustomerContext() context.Context {
	return repository.WithCallerIdentity(context.Background(), repository.SearchScope{
		ProjectIDs:  []string{"22222222-2222-2222-2222-222222222222"},
		ViewerEmail: "customer@test.local",
	})
}

// A customer can escalate their own case. The case work note recorded after
// the escalation is a WORK_NOTE comment, which an external caller can neither
// read nor write (migration 0191), so that one write must run as the system
// identity while the escalation itself keeps the caller's identity.
func TestCreateCaseEscalation_WorkNoteRunsAsSystemEscalationKeepsCaller(t *testing.T) {
	escalations := &recordingEscalations{}
	caseSvc := &recordingCaseService{}
	svc := NewCaseEscalationService(escalations, caseSvc)

	if _, err := svc.CreateCaseEscalation(externalCustomerContext(), workNoteTestCaseID, nil, nil); err != nil {
		t.Fatalf("CreateCaseEscalation: %v", err)
	}

	// the escalation was created with the customer's own identity, unchanged
	got, ok := repository.CallerIdentityFromContext(escalations.createCtx)
	if !ok || got.Unrestricted || got.ViewerEmail != "customer@test.local" {
		t.Fatalf("escalation must keep the caller's identity, got %+v (ok=%v)", got, ok)
	}

	// the work note is a WORK_NOTE on the same case, written as the system
	if caseSvc.comment.Type != domain.CommentTypeWorkNote || caseSvc.comment.CaseID != workNoteTestCaseID {
		t.Fatalf("expected a WORK_NOTE on case %s, got %+v", workNoteTestCaseID, caseSvc.comment)
	}
	note, ok := repository.CallerIdentityFromContext(caseSvc.commentCtx)
	if !ok || !note.Unrestricted {
		t.Fatalf("work note must run as the system identity, got %+v (ok=%v)", note, ok)
	}
}

// The note is bookkeeping: if writing it fails, the escalation the customer
// asked for has already happened and must still be reported as successful.
func TestCreateCaseEscalation_WorkNoteFailureDoesNotFailEscalation(t *testing.T) {
	caseSvc := &recordingCaseService{err: errors.New("boom")}
	svc := NewCaseEscalationService(&recordingEscalations{}, caseSvc)

	got, err := svc.CreateCaseEscalation(externalCustomerContext(), workNoteTestCaseID, nil, nil)
	if err != nil {
		t.Fatalf("a failed work note must not fail the escalation: %v", err)
	}
	if got.ID != "esc-1" {
		t.Fatalf("expected the created escalation back, got %+v", got)
	}
}
