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
	"encoding/json"
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/servicenow"
)

type mockEntityCasesClient struct {
	lastSearchBody        []byte
	searchCasesFn         func(ctx context.Context, body []byte) ([]byte, error)
	lastCommentsCaseID    string
	lastCommentsBody      []byte
	searchCaseCommentsFn  func(ctx context.Context, caseID string, body []byte) ([]byte, error)
	lastCreateCommentID   string
	lastCreateCommentBody []byte
	createCaseCommentFn   func(ctx context.Context, caseID string, body []byte) ([]byte, error)
}

func (m *mockEntityCasesClient) SearchCases(ctx context.Context, body []byte) ([]byte, error) {
	m.lastSearchBody = body
	return m.searchCasesFn(ctx, body)
}

func (m *mockEntityCasesClient) SearchCaseComments(ctx context.Context, caseID string, body []byte) ([]byte, error) {
	m.lastCommentsCaseID = caseID
	m.lastCommentsBody = body
	return m.searchCaseCommentsFn(ctx, caseID, body)
}

func (m *mockEntityCasesClient) CreateCaseComment(ctx context.Context, caseID string, body []byte) ([]byte, error) {
	m.lastCreateCommentID = caseID
	m.lastCreateCommentBody = body
	return m.createCaseCommentFn(ctx, caseID, body)
}

type mockAttachmentsInfoClient struct {
	called bool
	fn     func(ctx context.Context, caseNumber string, offset, limit int) ([]servicenow.AttachmentInfo, error)
}

func (m *mockAttachmentsInfoClient) GetAttachmentsInfo(ctx context.Context, caseNumber string, offset, limit int) ([]servicenow.AttachmentInfo, error) {
	m.called = true
	return m.fn(ctx, caseNumber, offset, limit)
}

func jsonBytes(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	return b
}

func TestPostgresSplCaseClient_GetCases_MapsStateAndFieldsBothDirections(t *testing.T) {
	entity := &mockEntityCasesClient{
		searchCasesFn: func(_ context.Context, body []byte) ([]byte, error) {
			var req entitySearchCasesRequest
			if err := json.Unmarshal(body, &req); err != nil {
				t.Fatalf("unmarshal request sent to entity-service: %v", err)
			}
			if len(req.Filters.Filters) != 1 || req.Filters.Filters[0].Field != "state" ||
				req.Filters.Filters[0].Op != "in" || req.Filters.Filters[0].Values[0] != "work_in_progress" {
				t.Errorf("state filter sent to entity-service = %+v, want field=state op=in values=[work_in_progress]", req.Filters.Filters)
			}
			subject := "Deployment failing"
			state := "work_in_progress"
			return jsonBytes(t, entitySearchCasesResponse{
				Total: 1,
				Cases: []entitySearchCaseView{
					{
						ID: "uuid-1", InternalID: "WSO2-1", Number: "CS0000001",
						CreatedOn: "2026-01-01T00:00:00Z", Subject: &subject, State: &state,
						CreatedBy: &entityUserReference{Name: "Jane Doe"},
					},
				},
			}), nil
		},
	}

	c := NewPostgresSplCaseClient(entity, &mockAttachmentsInfoClient{})
	stateFilter := "Work In Progress"
	result, err := c.GetCases(context.Background(), nil, &stateFilter, 0, 10)
	if err != nil {
		t.Fatalf("GetCases: %v", err)
	}
	if result.Count != 1 || len(result.Cases) != 1 {
		t.Fatalf("result = %+v, want 1 case", result)
	}
	got := result.Cases[0]
	if got.Number != "CS0000001" {
		t.Errorf("Number = %q, want CS0000001", got.Number)
	}
	if got.State != "Work In Progress" {
		t.Errorf("State = %q, want the display label, not the wire value", got.State)
	}
	if got.ShortDescription != "Deployment failing" {
		t.Errorf("ShortDescription = %q, want the mapped Subject", got.ShortDescription)
	}
	if got.OpenedBy != "Jane Doe" {
		t.Errorf("OpenedBy = %q, want the mapped CreatedBy name", got.OpenedBy)
	}
	// The known, real entity-service gap (see postgresSplCaseClient's own
	// doc comment) -- must come back empty, not panic on the nil pointers.
	if got.AccountNumber != "" || got.AccountName != "" || got.ProjectKey != "" {
		t.Errorf("AccountNumber/AccountName/ProjectKey = %q/%q/%q, want all empty for a Postgres-sourced case with no account/project linked",
			got.AccountNumber, got.AccountName, got.ProjectKey)
	}
}

func TestPostgresSplCaseClient_GetCases_UnknownStateFilterReturnsNoResultsNotEverything(t *testing.T) {
	entity := &mockEntityCasesClient{
		searchCasesFn: func(context.Context, []byte) ([]byte, error) {
			t.Fatal("entity-service should not be called for an unrecognized state filter")
			return nil, nil
		},
	}
	c := NewPostgresSplCaseClient(entity, &mockAttachmentsInfoClient{})
	badState := "Some Made Up State"
	result, err := c.GetCases(context.Background(), nil, &badState, 0, 10)
	if err != nil {
		t.Fatalf("GetCases: %v", err)
	}
	if result.Count != 0 || len(result.Cases) != 0 {
		t.Errorf("result = %+v, want an empty result for an unknown state label", result)
	}
}

func TestPostgresSplCaseClient_GetCaseByNumber_NotFound(t *testing.T) {
	entity := &mockEntityCasesClient{
		searchCasesFn: func(context.Context, []byte) ([]byte, error) {
			return jsonBytes(t, entitySearchCasesResponse{Cases: []entitySearchCaseView{}, Total: 0}), nil
		},
	}
	c := NewPostgresSplCaseClient(entity, &mockAttachmentsInfoClient{})
	_, err := c.GetCaseByNumber(context.Background(), "CS9999999")
	if !errors.Is(err, servicenow.ErrCaseNotFound) {
		t.Errorf("err = %v, want servicenow.ErrCaseNotFound", err)
	}
}

func TestPostgresSplCaseClient_GetCaseByNumber_SendsExactNumberFilter(t *testing.T) {
	entity := &mockEntityCasesClient{
		searchCasesFn: func(_ context.Context, body []byte) ([]byte, error) {
			var req entitySearchCasesRequest
			_ = json.Unmarshal(body, &req)
			if len(req.Filters.Filters) != 1 || req.Filters.Filters[0].Field != "number" ||
				req.Filters.Filters[0].Op != "eq" || req.Filters.Filters[0].Values[0] != "CS0000042" {
				t.Errorf("filter sent = %+v, want field=number op=eq values=[CS0000042]", req.Filters.Filters)
			}
			return jsonBytes(t, entitySearchCasesResponse{
				Total: 1,
				Cases: []entitySearchCaseView{{ID: "uuid-42", Number: "CS0000042"}},
			}), nil
		},
	}
	c := NewPostgresSplCaseClient(entity, &mockAttachmentsInfoClient{})
	got, err := c.GetCaseByNumber(context.Background(), "CS0000042")
	if err != nil {
		t.Fatalf("GetCaseByNumber: %v", err)
	}
	if got.Number != "CS0000042" {
		t.Errorf("Number = %q, want CS0000042", got.Number)
	}
}

func TestPostgresSplCaseClient_GetCommentsAndWorknotes_ResolvesNumberToIDAndMapsTypes(t *testing.T) {
	entity := &mockEntityCasesClient{
		searchCasesFn: func(context.Context, []byte) ([]byte, error) {
			return jsonBytes(t, entitySearchCasesResponse{
				Total: 1,
				Cases: []entitySearchCaseView{{ID: "uuid-7", Number: "CS0000007"}},
			}), nil
		},
		searchCaseCommentsFn: func(_ context.Context, caseID string, _ []byte) ([]byte, error) {
			if caseID != "uuid-7" {
				t.Errorf("SearchCaseComments called with caseID = %q, want the resolved UUID uuid-7, not the case number", caseID)
			}
			return jsonBytes(t, entitySearchCaseCommentsResponse{
				Total: 2,
				Comments: []entityCaseComment{
					{Type: "work_note", Content: "internal note", CreatedOn: "2026-01-02T00:00:00Z", CreatedBy: &entityUserReference{Name: "Support Eng"}},
					{Type: "comment", Content: "customer reply", CreatedOn: "2026-01-03T00:00:00Z"},
				},
			}), nil
		},
	}
	c := NewPostgresSplCaseClient(entity, &mockAttachmentsInfoClient{})
	got, err := c.GetCommentsAndWorknotes(context.Background(), "CS0000007", 0, 10)
	if err != nil {
		t.Fatalf("GetCommentsAndWorknotes: %v", err)
	}
	if got.Total != 2 || len(got.Comments) != 2 {
		t.Fatalf("got = %+v, want 2 comments", got)
	}
	if got.Comments[0].Type != "Work note" || got.Comments[0].CreatedBy != "Support Eng" {
		t.Errorf("Comments[0] = %+v, want Type=Work note CreatedBy=Support Eng", got.Comments[0])
	}
	if got.Comments[1].Type != "Comment" {
		t.Errorf("Comments[1].Type = %q, want Comment", got.Comments[1].Type)
	}
}

func TestPostgresSplCaseClient_PostWorkNote_ResolvesNumberAndCreatesWorkNoteComment(t *testing.T) {
	entity := &mockEntityCasesClient{
		searchCasesFn: func(context.Context, []byte) ([]byte, error) {
			return jsonBytes(t, entitySearchCasesResponse{
				Total: 1,
				Cases: []entitySearchCaseView{{ID: "uuid-9", Number: "CS0000009"}},
			}), nil
		},
		createCaseCommentFn: func(_ context.Context, caseID string, body []byte) ([]byte, error) {
			if caseID != "uuid-9" {
				t.Errorf("CreateCaseComment called with caseID = %q, want the resolved UUID uuid-9", caseID)
			}
			var req entityCreateCommentRequest
			if err := json.Unmarshal(body, &req); err != nil {
				t.Fatalf("unmarshal request sent to entity-service: %v", err)
			}
			if req.Type != "work_note" || req.Content != "checking on this" {
				t.Errorf("request sent = %+v, want type=work_note content=%q", req, "checking on this")
			}
			return jsonBytes(t, entityCreateCommentResponse{
				Comment: entityCaseCommentDetail{ID: "comment-1", CreatedOn: "2026-01-04T12:00:00Z", CreatedBy: "jane.doe@example.com"},
			}), nil
		},
	}
	c := NewPostgresSplCaseClient(entity, &mockAttachmentsInfoClient{})
	got, err := c.PostWorkNote(context.Background(), "CS0000009", "checking on this", "jane.doe@example.com")
	if err != nil {
		t.Fatalf("PostWorkNote: %v", err)
	}
	if got.Number != "CS0000009" {
		t.Errorf("Number = %q, want the input case number CS0000009", got.Number)
	}
	if got.UpdatedOn == "" {
		t.Error("UpdatedOn is empty, want the mapped comment createdOn timestamp")
	}
}

func TestPostgresSplCaseClient_GetAttachmentsInfo_DelegatesToServiceNow(t *testing.T) {
	sn := &mockAttachmentsInfoClient{
		fn: func(_ context.Context, caseNumber string, _, _ int) ([]servicenow.AttachmentInfo, error) {
			if caseNumber != "CS0000001" {
				t.Errorf("caseNumber = %q, want CS0000001", caseNumber)
			}
			return []servicenow.AttachmentInfo{{FileName: "log.txt"}}, nil
		},
	}
	c := NewPostgresSplCaseClient(&mockEntityCasesClient{}, sn)
	got, err := c.GetAttachmentsInfo(context.Background(), "CS0000001", 0, 10)
	if err != nil {
		t.Fatalf("GetAttachmentsInfo: %v", err)
	}
	if !sn.called {
		t.Error("GetAttachmentsInfo did not delegate to the wrapped ServiceNow client")
	}
	if len(got) != 1 || got[0].FileName != "log.txt" {
		t.Errorf("got = %+v, want the ServiceNow client's own result passed through unchanged", got)
	}
}
