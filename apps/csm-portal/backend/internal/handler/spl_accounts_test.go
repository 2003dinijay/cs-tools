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
	getAccountsFn             func(ctx context.Context, email, userType, phrase *string, offset, limit int, active bool) ([]servicenow.AccountDetails, error)
	getAccountByIDFn          func(ctx context.Context, accountNumber string) (servicenow.AccountDetails, error)
	getProjectsByAccountFn    func(ctx context.Context, accountNumber string, offset, limit int) ([]servicenow.ProjectDetails, error)
	getEscalationsByAccountFn func(ctx context.Context, accountNumber string, offset, limit int) ([]servicenow.EscalationDetail, error)
	escalateCaseFn            func(ctx context.Context, accountNumber, caseNumber string, request servicenow.EscalationRequest, submittedByEmail string) (servicenow.EscalationResponse, error)
}

func (m *mockSplAccountClient) GetAccounts(ctx context.Context, email, userType, phrase *string, offset, limit int, active bool) ([]servicenow.AccountDetails, error) {
	return m.getAccountsFn(ctx, email, userType, phrase, offset, limit, active)
}
func (m *mockSplAccountClient) GetAccountByID(ctx context.Context, accountNumber string) (servicenow.AccountDetails, error) {
	return m.getAccountByIDFn(ctx, accountNumber)
}
func (m *mockSplAccountClient) GetProjectsByAccount(ctx context.Context, accountNumber string, offset, limit int) ([]servicenow.ProjectDetails, error) {
	return m.getProjectsByAccountFn(ctx, accountNumber, offset, limit)
}
func (m *mockSplAccountClient) GetEscalationsByAccount(ctx context.Context, accountNumber string, offset, limit int) ([]servicenow.EscalationDetail, error) {
	return m.getEscalationsByAccountFn(ctx, accountNumber, offset, limit)
}
func (m *mockSplAccountClient) EscalateCase(ctx context.Context, accountNumber, caseNumber string, request servicenow.EscalationRequest, submittedByEmail string) (servicenow.EscalationResponse, error) {
	return m.escalateCaseFn(ctx, accountNumber, caseNumber, request, submittedByEmail)
}

func TestSplGetAccounts_RequiresGroupMembership(t *testing.T) {
	h := NewSplAccountHandler(&mockSplAccountClient{}, []string{"sales-team"}, nil)

	t.Run("unauthenticated", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/spl/accounts?offset=0&limit=10", nil)
		w := httptest.NewRecorder()
		h.GetAccounts(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("authenticated but not in allowed group", func(t *testing.T) {
		r := withUser(httptest.NewRequest(http.MethodGet, "/spl/accounts?offset=0&limit=10", nil))
		w := httptest.NewRecorder()
		h.GetAccounts(w, r)
		assertStatus(t, w, http.StatusForbidden)
	})
}

func TestSplGetAccounts_ValidatesPagination(t *testing.T) {
	h := NewSplAccountHandler(&mockSplAccountClient{}, splAllowedGroups, nil)

	tests := []string{"/spl/accounts", "/spl/accounts?offset=-1&limit=10", "/spl/accounts?offset=0&limit=0", "/spl/accounts?offset=abc&limit=10"}
	for _, target := range tests {
		r := withUser(httptest.NewRequest(http.MethodGet, target, nil))
		w := httptest.NewRecorder()
		h.GetAccounts(w, r)
		assertStatus(t, w, http.StatusBadRequest)
	}
}

func TestSplGetAccounts_ReturnsUpstreamResult(t *testing.T) {
	client := &mockSplAccountClient{
		getAccountsFn: func(_ context.Context, email, userType, phrase *string, offset, limit int, active bool) ([]servicenow.AccountDetails, error) {
			if offset != 5 || limit != 20 {
				t.Errorf("offset/limit = %d/%d, want 5/20", offset, limit)
			}
			return []servicenow.AccountDetails{{Number: "ACC1", Name: "Acme"}}, nil
		},
	}
	h := NewSplAccountHandler(client, splAllowedGroups, nil)

	r := withUser(httptest.NewRequest(http.MethodGet, "/spl/accounts?offset=5&limit=20", nil))
	w := httptest.NewRecorder()
	h.GetAccounts(w, r)

	assertStatus(t, w, http.StatusOK)
	result := decodeJSON[[]servicenow.AccountDetails](t, w)
	if len(result) != 1 || result[0].Number != "ACC1" {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestSplGetAccountByID_NotFound(t *testing.T) {
	client := &mockSplAccountClient{
		getAccountByIDFn: func(_ context.Context, _ string) (servicenow.AccountDetails, error) {
			return servicenow.AccountDetails{}, servicenow.ErrAccountNotFound
		},
	}
	h := NewSplAccountHandler(client, splAllowedGroups, nil)

	r := withUser(httptest.NewRequest(http.MethodGet, "/spl/accounts/ACC404", nil))
	r.SetPathValue("accountId", "ACC404")
	w := httptest.NewRecorder()
	h.GetAccountByID(w, r)
	assertStatus(t, w, http.StatusNotFound)
	assertErrorMessage(t, w, ErrMsgNotFound)
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
