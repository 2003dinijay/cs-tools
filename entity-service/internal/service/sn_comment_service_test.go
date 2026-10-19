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
	"net/http"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestCreateCommentReferenceTypes pins which reference types the reference-generic
// create-comment path accepts. "case" must be rejected before any downstream call:
// case comments belong to the dedicated case route, and accepting them here as well
// would leave two paths to the same outcome, free to drift apart.
func TestCreateCommentReferenceTypes(t *testing.T) {
	t.Parallel()

	newSvc := func(t *testing.T, called *bool) CommentService {
		t.Helper()
		client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			*called = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"message":"created","comment":{"id":"abc","createdOn":"2026-08-01 10:00:00","createdBy":"jane.doe@example.com"}}`))
		}))
		return NewServiceNowCommentService(client, nil)
	}

	req := func(refType domain.ReferenceType) domain.CreateCommentRequest {
		return domain.CreateCommentRequest{
			ReferenceID:   "00000000-0000-0000-0000-000000000000",
			ReferenceType: refType,
			Type:          domain.CommentTypeComment,
			Content:       "a comment",
		}
	}

	t.Run("rejects case without calling downstream", func(t *testing.T) {
		called := false
		svc := newSvc(t, &called)

		_, err := svc.CreateComment(context.Background(), req(domain.ReferenceTypeCase))
		if err == nil {
			t.Fatal("expected an error for referenceType case, got none")
		}
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("expected *apierror.ValidationError, got %T", err)
		}
		if want := "referenceType must be one of: conversation, change_request, deployment, incident"; ve.Msg != want {
			t.Errorf("got msg %q, want %q", ve.Msg, want)
		}
		if called {
			t.Error("downstream was called for referenceType case; it must be rejected before the request")
		}
	})

	t.Run("accepts every other supported reference type", func(t *testing.T) {
		for _, refType := range []domain.ReferenceType{
			domain.ReferenceTypeConversation,
			domain.ReferenceTypeChangeRequest,
			domain.ReferenceTypeDeployment,
			domain.ReferenceTypeIncident,
		} {
			called := false
			svc := newSvc(t, &called)

			resp, err := svc.CreateComment(context.Background(), req(refType))
			if err != nil {
				t.Errorf("referenceType %q: unexpected error: %v", refType, err)
				continue
			}
			if !called {
				t.Errorf("referenceType %q: downstream was not called", refType)
			}
			if resp.Message != "created" {
				t.Errorf("referenceType %q: got message %q, want %q", refType, resp.Message, "created")
			}
		}
	})
}

// TestSNCommentSearchService_EditDeleteUnsupported covers the ServiceNow data
// source's CommentService interface-satisfaction stub: comment edit/delete is
// a net-new Postgres-only capability (ServiceNow's own sys_journal_field is
// append-only), so every method must reject explicitly with a
// ServiceUnavailableError rather than silently succeeding or panicking. None
// of these methods touch the injected client or the event publisher, so nil
// is fine for both here.
func TestSNCommentSearchService_EditDeleteUnsupported(t *testing.T) {
	svc := NewServiceNowCommentService(nil, nil)
	ctx := context.Background()

	t.Run("UpdateComment", func(t *testing.T) {
		_, err := svc.UpdateComment(ctx, domain.UpdateCommentRequest{ID: testUUID, Content: "x"})
		var sue *apierror.ServiceUnavailableError
		if !errors.As(err, &sue) {
			t.Fatalf("expected ServiceUnavailableError, got %v (%T)", err, err)
		}
	})

	t.Run("DeleteComment", func(t *testing.T) {
		err := svc.DeleteComment(ctx, testUUID)
		var sue *apierror.ServiceUnavailableError
		if !errors.As(err, &sue) {
			t.Fatalf("expected ServiceUnavailableError, got %v (%T)", err, err)
		}
	})

	t.Run("GetCommentEditHistory", func(t *testing.T) {
		_, err := svc.GetCommentEditHistory(ctx, testUUID)
		var sue *apierror.ServiceUnavailableError
		if !errors.As(err, &sue) {
			t.Fatalf("expected ServiceUnavailableError, got %v (%T)", err, err)
		}
	})
}

// TestSNCommentSearchService_SearchComments_MapsInlineAttachments pins that
// the generic SearchComments (POST /comments/search) carries inline
// attachments through, same as snCaseService.SearchCaseComments already does
// for the case-nested POST /cases/{id}/comments/search. Found live: this
// generic path parsed the identical upstream response into domain.Comment,
// which had no InlineAttachments field at all, so the data was silently
// dropped -- backend-v2's commentAttachmentIsVisible authorization fallback
// (customer-portal, internal/handler/attachments.go) depends on this field to
// authorize an attachment pasted inline into a case comment, and always
// returned "not found" with it missing.
func TestSNCommentSearchService_SearchComments_MapsInlineAttachments(t *testing.T) {
	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"comments": [{
				"id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"referenceId": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				"content": "see attached",
				"type": "comments",
				"createdOn": "2026-08-01 10:00:00",
				"createdBy": "jane.doe@example.com",
				"hasInlineAttachments": true,
				"inlineAttachments": [{
					"id": "cccccccccccccccccccccccccccccccc",
					"fileName": "screenshot.png",
					"contentType": "image/png",
					"downloadUrl": "https://wso2sndev.service-now.com/cccccccccccccccccccccccccccccccc.iix",
					"createdOn": "2026-08-01 10:00:00",
					"createdBy": "jane.doe@example.com"
				}]
			}],
			"offset": 0,
			"limit": 50,
			"totalRecords": 1
		}`))
	}))
	svc := NewServiceNowCommentService(client, nil)

	resp, err := svc.SearchComments(context.Background(), domain.SearchCommentsRequest{
		ReferenceID:   testUUID,
		ReferenceType: domain.ReferenceTypeCase,
		Pagination:    domain.Pagination{Limit: 50},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Comments) != 1 {
		t.Fatalf("got %d comments, want 1", len(resp.Comments))
	}
	c := resp.Comments[0]
	if !c.HasInlineAttachments {
		t.Error("HasInlineAttachments = false, want true")
	}
	if len(c.InlineAttachments) != 1 {
		t.Fatalf("got %d inline attachments, want 1", len(c.InlineAttachments))
	}
	ia := c.InlineAttachments[0]
	const wantID = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	if ia.ID != wantID {
		t.Errorf("InlineAttachments[0].ID = %q, want %q", ia.ID, wantID)
	}
	if ia.FileName != "screenshot.png" {
		t.Errorf("InlineAttachments[0].FileName = %q, want %q", ia.FileName, "screenshot.png")
	}
}
