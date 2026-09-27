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

// testUser (helpers_test.go) is in group "csm-agents" — reused here as the
// SPL allowed-groups membership for tests that should succeed.
var splAllowedGroups = []string{"csm-agents"}

type mockSplAccountClient struct {
	getEscalationsByAccountFn func(ctx context.Context, accountNumber string, offset, limit int) ([]servicenow.EscalationDetail, error)
	escalateCaseFn            func(ctx context.Context, accountNumber, caseNumber string, request servicenow.EscalationRequest, submittedByEmail string) (servicenow.EscalationResponse, error)
}

func (m *mockSplAccountClient) GetEscalationsByAccount(ctx context.Context, accountNumber string, offset, limit int) ([]servicenow.EscalationDetail, error) {
	return m.getEscalationsByAccountFn(ctx, accountNumber, offset, limit)
}
func (m *mockSplAccountClient) EscalateCase(ctx context.Context, accountNumber, caseNumber string, request servicenow.EscalationRequest, submittedByEmail string) (servicenow.EscalationResponse, error) {
	return m.escalateCaseFn(ctx, accountNumber, caseNumber, request, submittedByEmail)
}

func TestSplEscalateCase_RequiresEscalationGroup(t *testing.T) {
	h := NewSplAccountHandler(&mockSplAccountClient{}, splAllowedGroups, []string{"escalation-team"})

	body := `{"justification":"urgent","requestSource":"Customer","reason":"Inactivity","severity":"High Severity"}`
	r := withUser(httptest.NewRequest(http.MethodPost, "/spl/accounts/ACC1/cases/CS1/escalate", strings.NewReader(body)))
	r.SetPathValue("accountId", "ACC1")
	r.SetPathValue("caseId", "CS1")
	w := httptest.NewRecorder()
	h.EscalateCase(w, r)
	assertStatus(t, w, http.StatusForbidden)
}

func TestSplEscalateCase_RejectsInvalidPayload(t *testing.T) {
	h := NewSplAccountHandler(&mockSplAccountClient{}, splAllowedGroups, splAllowedGroups)

	tests := []string{
		`{"justification":"","requestSource":"Customer","reason":"Inactivity","severity":"High Severity"}`,
		`{"justification":"x","requestSource":"Bogus","reason":"Inactivity","severity":"High Severity"}`,
		`{"justification":"x","requestSource":"Customer","reason":"Bogus","severity":"High Severity"}`,
		`{"justification":"x","requestSource":"Customer","reason":"Inactivity","severity":"Bogus"}`,
		`not-json`,
	}
	for _, body := range tests {
		r := withUser(httptest.NewRequest(http.MethodPost, "/spl/accounts/ACC1/cases/CS1/escalate", strings.NewReader(body)))
		r.SetPathValue("accountId", "ACC1")
		r.SetPathValue("caseId", "CS1")
		w := httptest.NewRecorder()
		h.EscalateCase(w, r)
		assertStatus(t, w, http.StatusBadRequest)
	}
}

func TestSplEscalateCase_Conflict(t *testing.T) {
	client := &mockSplAccountClient{
		escalateCaseFn: func(_ context.Context, _, _ string, _ servicenow.EscalationRequest, _ string) (servicenow.EscalationResponse, error) {
			return servicenow.EscalationResponse{}, servicenow.ErrEscalationConflict
		},
	}
	h := NewSplAccountHandler(client, splAllowedGroups, splAllowedGroups)

	body := `{"justification":"urgent","requestSource":"Customer","reason":"Inactivity","severity":"High Severity"}`
	r := withUser(httptest.NewRequest(http.MethodPost, "/spl/accounts/ACC1/cases/CS1/escalate", strings.NewReader(body)))
	r.SetPathValue("accountId", "ACC1")
	r.SetPathValue("caseId", "CS1")
	w := httptest.NewRecorder()
	h.EscalateCase(w, r)
	assertStatus(t, w, http.StatusConflict)
}
