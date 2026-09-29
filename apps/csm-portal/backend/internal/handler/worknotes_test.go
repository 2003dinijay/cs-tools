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

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/servicenow"
)

type mockSplWorknotesClient struct {
	result            servicenow.WorkNoteResponse
	err               error
	gotCaseNumber     string
	gotWorknote       string
	gotSubmitterEmail string
}

func (m *mockSplWorknotesClient) PostWorkNote(ctx context.Context, caseNumber, worknote, submitterEmail string) (servicenow.WorkNoteResponse, error) {
	m.gotCaseNumber, m.gotWorknote, m.gotSubmitterEmail = caseNumber, worknote, submitterEmail
	return m.result, m.err
}

func newWorkNoteRequest(caseID, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/worknote", strings.NewReader(body))
	req.SetPathValue("caseId", caseID)
	return withUser(req)
}

func TestPostWorkNote_Success(t *testing.T) {
	mock := &mockSplWorknotesClient{result: servicenow.WorkNoteResponse{Number: "CS001", UpdatedOn: "2024-01-01"}}
	h := NewSplWorknotesHandler(mock, []string{"csm-agents"}, []string{"csm-agents"})
	w := httptest.NewRecorder()

	h.PostWorkNote(w, newWorkNoteRequest("CS001", `{"worknote":"hello"}`))

	assertStatus(t, w, http.StatusOK)
	if mock.gotCaseNumber != "CS001" || mock.gotWorknote != "hello" || mock.gotSubmitterEmail != testUser.Email {
		t.Errorf("client called with caseNumber=%q worknote=%q email=%q", mock.gotCaseNumber, mock.gotWorknote, mock.gotSubmitterEmail)
	}
}

func TestPostWorkNote_RejectsMissingWorknote(t *testing.T) {
	h := NewSplWorknotesHandler(&mockSplWorknotesClient{}, []string{"csm-agents"}, []string{"csm-agents"})
	w := httptest.NewRecorder()

	h.PostWorkNote(w, newWorkNoteRequest("CS001", `{}`))

	assertStatus(t, w, http.StatusBadRequest)
}

func TestPostWorkNote_RejectsSubGroupMismatch(t *testing.T) {
	h := NewSplWorknotesHandler(&mockSplWorknotesClient{}, []string{"csm-agents"}, []string{"worknote-writers"})
	w := httptest.NewRecorder()

	h.PostWorkNote(w, newWorkNoteRequest("CS001", `{"worknote":"hello"}`))

	assertStatus(t, w, http.StatusForbidden)
}

func TestPostWorkNote_MapsCaseClosedTo400(t *testing.T) {
	mock := &mockSplWorknotesClient{err: servicenow.ErrCaseClosed}
	h := NewSplWorknotesHandler(mock, []string{"csm-agents"}, []string{"csm-agents"})
	w := httptest.NewRecorder()

	h.PostWorkNote(w, newWorkNoteRequest("CS001", `{"worknote":"hello"}`))

	assertStatus(t, w, http.StatusBadRequest)
}

func TestPostWorkNote_MapsCaseNotFoundTo404(t *testing.T) {
	mock := &mockSplWorknotesClient{err: servicenow.ErrCaseSysIDNotFound}
	h := NewSplWorknotesHandler(mock, []string{"csm-agents"}, []string{"csm-agents"})
	w := httptest.NewRecorder()

	h.PostWorkNote(w, newWorkNoteRequest("CS001", `{"worknote":"hello"}`))

	assertStatus(t, w, http.StatusNotFound)
}
