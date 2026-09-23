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

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/servicenow"
)

type mockSplCaseClient struct {
	getCasesFn                func(ctx context.Context, searchString, stateFilter *string, offset, limit int) (servicenow.CaseDetailsWithCount, error)
	getCaseByNumberFn         func(ctx context.Context, caseNumber string) (servicenow.CaseDetails, error)
	getCommentsAndWorknotesFn func(ctx context.Context, caseNumber string, offset, limit int) (servicenow.CommentsResponse, error)
	getAttachmentsInfoFn      func(ctx context.Context, caseNumber string, offset, limit int) ([]servicenow.AttachmentInfo, error)
}

func (m *mockSplCaseClient) GetCases(ctx context.Context, searchString, stateFilter *string, offset, limit int) (servicenow.CaseDetailsWithCount, error) {
	return m.getCasesFn(ctx, searchString, stateFilter, offset, limit)
}
func (m *mockSplCaseClient) GetCaseByNumber(ctx context.Context, caseNumber string) (servicenow.CaseDetails, error) {
	return m.getCaseByNumberFn(ctx, caseNumber)
}
func (m *mockSplCaseClient) GetCommentsAndWorknotes(ctx context.Context, caseNumber string, offset, limit int) (servicenow.CommentsResponse, error) {
	return m.getCommentsAndWorknotesFn(ctx, caseNumber, offset, limit)
}
func (m *mockSplCaseClient) GetAttachmentsInfo(ctx context.Context, caseNumber string, offset, limit int) ([]servicenow.AttachmentInfo, error) {
	return m.getAttachmentsInfoFn(ctx, caseNumber, offset, limit)
}

func TestSplGetCaseByNumber_NotFound(t *testing.T) {
	client := &mockSplCaseClient{
		getCaseByNumberFn: func(_ context.Context, _ string) (servicenow.CaseDetails, error) {
			return servicenow.CaseDetails{}, servicenow.ErrCaseNotFound
		},
	}
	h := NewSplCaseHandler(client, splAllowedGroups)

	r := withUser(httptest.NewRequest(http.MethodGet, "/spl/cases/CS404", nil))
	r.SetPathValue("caseId", "CS404")
	w := httptest.NewRecorder()
	h.GetCaseByNumber(w, r)
	assertStatus(t, w, http.StatusNotFound)
}

func TestSplGetCases_ReturnsUpstreamResult(t *testing.T) {
	client := &mockSplCaseClient{
		getCasesFn: func(_ context.Context, searchString, stateFilter *string, offset, limit int) (servicenow.CaseDetailsWithCount, error) {
			return servicenow.CaseDetailsWithCount{Count: 1, Cases: []servicenow.CaseDetails{{Number: "CS1"}}}, nil
		},
	}
	h := NewSplCaseHandler(client, splAllowedGroups)

	r := withUser(httptest.NewRequest(http.MethodGet, "/spl/cases?offset=0&limit=10", nil))
	w := httptest.NewRecorder()
	h.GetCases(w, r)

	assertStatus(t, w, http.StatusOK)
	result := decodeJSON[servicenow.CaseDetailsWithCount](t, w)
	if result.Count != 1 || len(result.Cases) != 1 {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestSplGetCommentsAndWorknotes_RequiresGroupMembership(t *testing.T) {
	h := NewSplCaseHandler(&mockSplCaseClient{}, []string{"sales-team"})
	r := withUser(httptest.NewRequest(http.MethodGet, "/spl/cases/CS1/comments-and-worknotes?offset=0&limit=10", nil))
	r.SetPathValue("caseId", "CS1")
	w := httptest.NewRecorder()
	h.GetCommentsAndWorknotes(w, r)
	assertStatus(t, w, http.StatusForbidden)
}

func TestSplGetAttachmentsInfo_MissingCaseIDIs400(t *testing.T) {
	h := NewSplCaseHandler(&mockSplCaseClient{}, splAllowedGroups)
	r := withUser(httptest.NewRequest(http.MethodGet, "/spl/cases//attachments-info?offset=0&limit=10", nil))
	w := httptest.NewRecorder()
	h.GetAttachmentsInfo(w, r)
	assertStatus(t, w, http.StatusBadRequest)
}
