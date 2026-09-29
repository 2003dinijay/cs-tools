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

func TestGetAccount(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewAccountHandler(&mockEntityAccountClient{})
		r := httptest.NewRequest(http.MethodGet, "/accounts/acc-1", nil)
		r.SetPathValue("id", "acc-1")
		w := httptest.NewRecorder()
		h.GetAccount(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty account ID", func(t *testing.T) {
		h := NewAccountHandler(&mockEntityAccountClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/accounts/", nil))
		w := httptest.NewRecorder()
		h.GetAccount(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects non-UUID account ID", func(t *testing.T) {
		h := NewAccountHandler(&mockEntityAccountClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/accounts/acc-42", nil))
		r.SetPathValue("id", "acc-42")
		w := httptest.NewRecorder()
		h.GetAccount(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("passes ID to upstream and returns 200 with response", func(t *testing.T) {
		const accountID = "11111111-1111-1111-1111-111111111111"
		var capturedID string
		client := &mockEntityAccountClient{
			getAccountFn: func(_ context.Context, id string) ([]byte, error) {
				capturedID = id
				return []byte(`{"id":"` + accountID + `","name":"WSO2"}`), nil
			},
		}
		h := NewAccountHandler(client)
		r := withUser(httptest.NewRequest(http.MethodGet, "/accounts/"+accountID, nil))
		r.SetPathValue("id", accountID)
		w := httptest.NewRecorder()
		h.GetAccount(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if capturedID != accountID {
			t.Errorf("upstream received id %q, want %q", capturedID, accountID)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["id"] != accountID {
			t.Errorf("response id = %v, want %s", resp["id"], accountID)
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		const accountID = "11111111-1111-1111-1111-111111111111"
		for _, tc := range upstreamErrorsGeneric("Failed to retrieve account.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityAccountClient{
					getAccountFn: func(_ context.Context, _ string) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewAccountHandler(client)
				r := withUser(httptest.NewRequest(http.MethodGet, "/accounts/"+accountID, nil))
				r.SetPathValue("id", accountID)
				w := httptest.NewRecorder()
				h.GetAccount(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestSearchAccounts(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewAccountHandler(&mockEntityAccountClient{})
		r := httptest.NewRequest(http.MethodPost, "/accounts/search", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.SearchAccounts(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewAccountHandler(&mockEntityAccountClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/accounts/search", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		w := httptest.NewRecorder()
		h.SearchAccounts(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewAccountHandler(&mockEntityAccountClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/accounts/search", strings.NewReader(`not-json`)))
		w := httptest.NewRecorder()
		h.SearchAccounts(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards body to upstream and returns 200 with response", func(t *testing.T) {
		const reqPayload = `{"name":"WSO2","limit":10}`
		var capturedBody []byte
		client := &mockEntityAccountClient{
			searchAccountsFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"accounts":[{"id":"acc-1","name":"WSO2"}],"total":1}`), nil
			},
		}
		h := NewAccountHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/accounts/search", strings.NewReader(reqPayload)))
		w := httptest.NewRecorder()
		h.SearchAccounts(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if string(capturedBody) != reqPayload {
			t.Errorf("upstream received body %q, want %q", capturedBody, reqPayload)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["total"] != float64(1) {
			t.Errorf("total = %v, want 1", resp["total"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to search accounts.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityAccountClient{
					searchAccountsFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewAccountHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/accounts/search", strings.NewReader(`{}`)))
				w := httptest.NewRecorder()
				h.SearchAccounts(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestSearchAccountContacts(t *testing.T) {
	const accountID = "11111111-1111-1111-1111-111111111111"

	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewAccountHandler(&mockEntityAccountClient{})
		r := httptest.NewRequest(http.MethodPost, "/accounts/"+accountID+"/contacts/search", strings.NewReader(`{}`))
		r.SetPathValue("id", accountID)
		w := httptest.NewRecorder()
		h.SearchAccountContacts(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty account ID", func(t *testing.T) {
		h := NewAccountHandler(&mockEntityAccountClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/accounts//contacts/search", strings.NewReader(`{}`)))
		w := httptest.NewRecorder()
		h.SearchAccountContacts(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects non-UUID account ID", func(t *testing.T) {
		h := NewAccountHandler(&mockEntityAccountClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/accounts/acc-42/contacts/search", strings.NewReader(`{}`)))
		r.SetPathValue("id", "acc-42")
		w := httptest.NewRecorder()
		h.SearchAccountContacts(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewAccountHandler(&mockEntityAccountClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/accounts/"+accountID+"/contacts/search", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		r.SetPathValue("id", accountID)
		w := httptest.NewRecorder()
		h.SearchAccountContacts(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewAccountHandler(&mockEntityAccountClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/accounts/"+accountID+"/contacts/search", strings.NewReader(`not-json`)))
		r.SetPathValue("id", accountID)
		w := httptest.NewRecorder()
		h.SearchAccountContacts(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards body verbatim and returns upstream response", func(t *testing.T) {
		var capturedAccountID string
		var capturedBody []byte
		reqBody := `{"filters":{"searchQuery":"jane"},"pagination":{"limit":20,"offset":0}}`
		client := &mockEntityAccountClient{
			searchAccountContactsFn: func(_ context.Context, id string, body []byte) ([]byte, error) {
				capturedAccountID = id
				capturedBody = body
				return []byte(`{"contacts":[{"name":"Jane Doe","isPrimary":true}],"total":1,"limit":20,"offset":0}`), nil
			},
		}
		h := NewAccountHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/accounts/"+accountID+"/contacts/search", strings.NewReader(reqBody)))
		r.SetPathValue("id", accountID)
		w := httptest.NewRecorder()
		h.SearchAccountContacts(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		if capturedAccountID != accountID {
			t.Errorf("accountID = %q, want %q", capturedAccountID, accountID)
		}
		if string(capturedBody) != reqBody {
			t.Errorf("upstream body = %q, want verbatim %q", string(capturedBody), reqBody)
		}

		resp := decodeJSON[map[string]any](t, w)
		if resp["total"] != float64(1) {
			t.Errorf("total = %v, want 1", resp["total"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to search account contacts.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityAccountClient{
					searchAccountContactsFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewAccountHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/accounts/"+accountID+"/contacts/search", strings.NewReader(`{}`)))
				r.SetPathValue("id", accountID)
				w := httptest.NewRecorder()
				h.SearchAccountContacts(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestUpdateAccountTeams(t *testing.T) {
	const accountID = "11111111-1111-1111-1111-111111111111"

	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewAccountHandler(&mockEntityAccountClient{})
		r := httptest.NewRequest(http.MethodPatch, "/accounts/"+accountID, strings.NewReader(`{"creTeamId":null}`))
		r.SetPathValue("id", accountID)
		w := httptest.NewRecorder()
		h.UpdateAccountTeams(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty account ID", func(t *testing.T) {
		h := NewAccountHandler(&mockEntityAccountClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/accounts/", strings.NewReader(`{"creTeamId":null}`)))
		w := httptest.NewRecorder()
		h.UpdateAccountTeams(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects non-UUID account ID", func(t *testing.T) {
		h := NewAccountHandler(&mockEntityAccountClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/accounts/acc-42", strings.NewReader(`{"creTeamId":null}`)))
		r.SetPathValue("id", "acc-42")
		w := httptest.NewRecorder()
		h.UpdateAccountTeams(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewAccountHandler(&mockEntityAccountClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/accounts/"+accountID, strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		r.SetPathValue("id", accountID)
		w := httptest.NewRecorder()
		h.UpdateAccountTeams(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewAccountHandler(&mockEntityAccountClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/accounts/"+accountID, strings.NewReader(`not-json`)))
		r.SetPathValue("id", accountID)
		w := httptest.NewRecorder()
		h.UpdateAccountTeams(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards body verbatim and returns upstream response", func(t *testing.T) {
		var capturedID string
		var capturedBody []byte
		reqBody := `{"creTeamId":"22222222-2222-2222-2222-222222222222","sreTeamId":null}`
		client := &mockEntityAccountClient{
			updateAccountTeamsFn: func(_ context.Context, id string, body []byte) ([]byte, error) {
				capturedID = id
				capturedBody = body
				return []byte(`{"id":"` + accountID + `","creTeamId":"22222222-2222-2222-2222-222222222222","sreTeamId":null}`), nil
			},
		}
		h := NewAccountHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/accounts/"+accountID, strings.NewReader(reqBody)))
		r.SetPathValue("id", accountID)
		w := httptest.NewRecorder()
		h.UpdateAccountTeams(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		if capturedID != accountID {
			t.Errorf("accountID = %q, want %q", capturedID, accountID)
		}
		if string(capturedBody) != reqBody {
			t.Errorf("upstream body = %q, want verbatim %q", string(capturedBody), reqBody)
		}

		resp := decodeJSON[map[string]any](t, w)
		if resp["id"] != accountID {
			t.Errorf("response id = %v, want %s", resp["id"], accountID)
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to update account teams.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityAccountClient{
					updateAccountTeamsFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewAccountHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPatch, "/accounts/"+accountID, strings.NewReader(`{"creTeamId":null}`)))
				r.SetPathValue("id", accountID)
				w := httptest.NewRecorder()
				h.UpdateAccountTeams(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

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
