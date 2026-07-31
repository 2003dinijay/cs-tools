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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReferenceHandler_SearchRoles(t *testing.T) {
	t.Run("rejects an unauthenticated caller", func(t *testing.T) {
		h := NewReferenceHandler(&mockEntityReferenceClient{})
		w := httptest.NewRecorder()
		h.SearchRoles(w, httptest.NewRequest(http.MethodPost, "/roles/search", strings.NewReader(`{}`)))
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
	})

	t.Run("forwards the body verbatim", func(t *testing.T) {
		var got string
		h := NewReferenceHandler(&mockEntityReferenceClient{
			searchRolesFn: func(_ context.Context, body []byte) ([]byte, error) {
				got = string(body)
				return []byte(`{"roles":[{"id":"agent","name":"Agent"}],"total":1,"offset":0,"limit":50}`), nil
			},
		})
		body := `{"filters":{"searchQuery":"age"}}`
		w := httptest.NewRecorder()
		h.SearchRoles(w, withUser(httptest.NewRequest(http.MethodPost, "/roles/search", strings.NewReader(body))))

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if got != body {
			t.Errorf("forwarded body = %q, want %q", got, body)
		}
	})

	// An absent body means "no filters, default page" — it must not be a 400.
	t.Run("accepts an empty body", func(t *testing.T) {
		h := NewReferenceHandler(&mockEntityReferenceClient{})
		w := httptest.NewRecorder()
		h.SearchRoles(w, withUser(httptest.NewRequest(http.MethodPost, "/roles/search", nil)))
		assertStatus(t, w, http.StatusOK)
	})

	t.Run("rejects a malformed body", func(t *testing.T) {
		h := NewReferenceHandler(&mockEntityReferenceClient{})
		w := httptest.NewRecorder()
		h.SearchRoles(w, withUser(httptest.NewRequest(http.MethodPost, "/roles/search", strings.NewReader(`{`))))
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("maps an upstream failure", func(t *testing.T) {
		h := NewReferenceHandler(&mockEntityReferenceClient{
			searchRolesFn: func(_ context.Context, _ []byte) ([]byte, error) {
				return nil, errors.New("entity down")
			},
		})
		w := httptest.NewRecorder()
		h.SearchRoles(w, withUser(httptest.NewRequest(http.MethodPost, "/roles/search", strings.NewReader(`{}`))))
		if w.Code == http.StatusOK {
			t.Fatal("status = 200, want an error status")
		}
	})
}

func TestReferenceHandler_SearchTeams(t *testing.T) {
	t.Run("forwards to the teams call, not the roles call", func(t *testing.T) {
		rolesCalled := false
		teamsCalled := false
		h := NewReferenceHandler(&mockEntityReferenceClient{
			searchRolesFn: func(_ context.Context, _ []byte) ([]byte, error) {
				rolesCalled = true
				return []byte(`{}`), nil
			},
			searchTeamsFn: func(_ context.Context, _ []byte) ([]byte, error) {
				teamsCalled = true
				return []byte(`{"teams":[],"total":0,"offset":0,"limit":50}`), nil
			},
		})
		w := httptest.NewRecorder()
		h.SearchTeams(w, withUser(httptest.NewRequest(http.MethodPost, "/teams/search", strings.NewReader(`{}`))))

		assertStatus(t, w, http.StatusOK)
		if rolesCalled || !teamsCalled {
			t.Fatalf("rolesCalled=%v teamsCalled=%v, want false/true", rolesCalled, teamsCalled)
		}
	})
}

func TestUsersHandler_GetUser(t *testing.T) {
	t.Run("rejects an unauthenticated caller", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{})
		w := httptest.NewRecorder()
		h.GetUser(w, httptest.NewRequest(http.MethodGet, "/users/abc", nil))
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("passes the path id through", func(t *testing.T) {
		var gotID string
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{
			getUserFn: func(_ context.Context, id string) ([]byte, error) {
				gotID = id
				return []byte(`{"id":"` + id + `","userType":"internal","groups":[],"teams":[]}`), nil
			},
		})
		r := withUser(httptest.NewRequest(http.MethodGet, "/users/11111111-1111-1111-1111-111111111111", nil))
		r.SetPathValue("id", "11111111-1111-1111-1111-111111111111")
		w := httptest.NewRecorder()
		h.GetUser(w, r)

		assertStatus(t, w, http.StatusOK)
		if gotID != "11111111-1111-1111-1111-111111111111" {
			t.Errorf("id = %q, want the path value", gotID)
		}
	})

	t.Run("rejects a missing id", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{})
		w := httptest.NewRecorder()
		h.GetUser(w, withUser(httptest.NewRequest(http.MethodGet, "/users/", nil)))
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("maps an upstream not-found", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{
			getUserFn: func(_ context.Context, _ string) ([]byte, error) {
				return nil, errors.New("not found")
			},
		})
		r := withUser(httptest.NewRequest(http.MethodGet, "/users/x", nil))
		r.SetPathValue("id", "x")
		w := httptest.NewRecorder()
		h.GetUser(w, r)
		if w.Code == http.StatusOK {
			t.Fatal("status = 200, want an error status")
		}
	})
}

func TestProjectHandler_GetProjectContact(t *testing.T) {
	t.Run("rejects an unauthenticated caller", func(t *testing.T) {
		h := NewProjectHandler(&mockEntityProjectClient{})
		w := httptest.NewRecorder()
		h.GetProjectContact(w, httptest.NewRequest(http.MethodGet, "/projects/p/contacts/c", nil))
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("passes both path ids through", func(t *testing.T) {
		var gotProject, gotContact string
		h := NewProjectHandler(&mockEntityProjectClient{
			getProjectContactFn: func(_ context.Context, projectID, contactID string) ([]byte, error) {
				gotProject, gotContact = projectID, contactID
				return []byte(`{"id":"` + contactID + `","registrationState":"REGISTERED"}`), nil
			},
		})
		r := withUser(httptest.NewRequest(http.MethodGet, "/projects/pid/contacts/cid", nil))
		r.SetPathValue("id", "pid")
		r.SetPathValue("contactId", "cid")
		w := httptest.NewRecorder()
		h.GetProjectContact(w, r)

		assertStatus(t, w, http.StatusOK)
		if gotProject != "pid" || gotContact != "cid" {
			t.Fatalf("ids = %q/%q, want pid/cid", gotProject, gotContact)
		}
	})

	t.Run("rejects a missing contact id", func(t *testing.T) {
		h := NewProjectHandler(&mockEntityProjectClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/projects/pid/contacts/", nil))
		r.SetPathValue("id", "pid")
		w := httptest.NewRecorder()
		h.GetProjectContact(w, r)
		assertStatus(t, w, http.StatusBadRequest)
	})
}
