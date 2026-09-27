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
	"net/url"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/employeeinfo"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/servicenow"
)

// mockAbtTeamMembersClient mocks abtTeamMembersClient — the small domain
// interface both the ServiceNow and Postgres data sources implement (see
// this handler's own doc comment on why it no longer depends on raw
// TableQuery; the ServiceNow-specific two-call merge logic this test used to
// exercise directly now lives in servicenow.Client.GetABTTeamMembers, with
// its own test coverage there).
type mockAbtTeamMembersClient struct {
	fn func(ctx context.Context, teamID string) ([]servicenow.ABTTeamRosterMember, error)
}

func (m *mockAbtTeamMembersClient) GetABTTeamMembers(ctx context.Context, teamID string) ([]servicenow.ABTTeamRosterMember, error) {
	return m.fn(ctx, teamID)
}

func TestSplGetABTTeamMembers(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewSplABTTeamMembersHandler(&mockAbtTeamMembersClient{}, &mockEmployeeInfoClient{}, []string{"csm-agents"})
		r := httptest.NewRequest(http.MethodGet, "/spl/abt-team-members?teamId=team-1", nil)
		w := httptest.NewRecorder()
		h.GetABTTeamMembers(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("rejects missing teamId", func(t *testing.T) {
		h := NewSplABTTeamMembersHandler(&mockAbtTeamMembersClient{}, &mockEmployeeInfoClient{}, []string{"csm-agents"})
		r := withUser(httptest.NewRequest(http.MethodGet, "/spl/abt-team-members", nil))
		w := httptest.NewRecorder()
		h.GetABTTeamMembers(w, r)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("rejects a teamId that would inject a query clause", func(t *testing.T) {
		called := false
		roster := &mockAbtTeamMembersClient{
			fn: func(ctx context.Context, teamID string) ([]servicenow.ABTTeamRosterMember, error) {
				called = true
				return nil, nil
			},
		}
		h := NewSplABTTeamMembersHandler(roster, &mockEmployeeInfoClient{}, []string{"csm-agents"})
		r := withUser(httptest.NewRequest(http.MethodGet, "/spl/abt-team-members?teamId="+url.QueryEscape("team-1^OR active=true"), nil))
		w := httptest.NewRecorder()
		h.GetABTTeamMembers(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		if called {
			t.Error("roster client should not have been called for an unsafe teamId")
		}
	})

	t.Run("returns 404 when no members found", func(t *testing.T) {
		roster := &mockAbtTeamMembersClient{
			fn: func(ctx context.Context, teamID string) ([]servicenow.ABTTeamRosterMember, error) {
				return nil, nil
			},
		}
		h := NewSplABTTeamMembersHandler(roster, &mockEmployeeInfoClient{}, []string{"csm-agents"})
		r := withUser(httptest.NewRequest(http.MethodGet, "/spl/abt-team-members?teamId=team-1", nil))
		w := httptest.NewRecorder()
		h.GetABTTeamMembers(w, r)
		assertStatus(t, w, http.StatusNotFound)
	})

	t.Run("merges member and thumbnail data", func(t *testing.T) {
		roster := &mockAbtTeamMembersClient{
			fn: func(ctx context.Context, teamID string) ([]servicenow.ABTTeamRosterMember, error) {
				if teamID != "team-1" {
					t.Errorf("teamID = %q, want team-1", teamID)
				}
				return []servicenow.ABTTeamRosterMember{{Name: "Jane Doe", Email: "jane@example.com", Role: "Lead"}}, nil
			},
		}
		thumb := "https://example.com/thumb.png"
		ei := &mockEmployeeInfoClient{
			getEmployeeDataFn: func(ctx context.Context, workEmail string) (*employeeinfo.Employee, error) {
				return &employeeinfo.Employee{FirstName: "Jane", LastName: "Doe", EmployeeThumbnail: &thumb}, nil
			},
		}
		h := NewSplABTTeamMembersHandler(roster, ei, []string{"csm-agents"})
		r := withUser(httptest.NewRequest(http.MethodGet, "/spl/abt-team-members?teamId=team-1", nil))
		w := httptest.NewRecorder()
		h.GetABTTeamMembers(w, r)
		assertStatus(t, w, http.StatusOK)
		views := decodeJSON[[]SplABTTeamMemberView](t, w)
		if len(views) != 1 {
			t.Fatalf("got %d members, want 1", len(views))
		}
		v := views[0]
		if v.Name != "Jane Doe" || v.Email != "jane@example.com" || v.Role != "Lead" {
			t.Errorf("view = %+v, unexpected", v)
		}
		if v.EmployeeThumbnail == nil || *v.EmployeeThumbnail != thumb {
			t.Errorf("EmployeeThumbnail = %v, want %q", v.EmployeeThumbnail, thumb)
		}
	})

	t.Run("member row survives an employee-info lookup failure", func(t *testing.T) {
		roster := &mockAbtTeamMembersClient{
			fn: func(ctx context.Context, teamID string) ([]servicenow.ABTTeamRosterMember, error) {
				return []servicenow.ABTTeamRosterMember{{Name: "Jane Doe", Email: "jane@example.com"}}, nil
			},
		}
		ei := &mockEmployeeInfoClient{
			getEmployeeDataFn: func(ctx context.Context, workEmail string) (*employeeinfo.Employee, error) {
				return nil, context.DeadlineExceeded
			},
		}
		h := NewSplABTTeamMembersHandler(roster, ei, []string{"csm-agents"})
		r := withUser(httptest.NewRequest(http.MethodGet, "/spl/abt-team-members?teamId=team-1", nil))
		w := httptest.NewRecorder()
		h.GetABTTeamMembers(w, r)
		assertStatus(t, w, http.StatusOK)
		views := decodeJSON[[]SplABTTeamMemberView](t, w)
		if len(views) != 1 {
			t.Fatalf("got %d members, want 1", len(views))
		}
		if views[0].Role != "" || views[0].EmployeeThumbnail != nil {
			t.Errorf("view = %+v, want blank role/thumbnail rather than an aborted request", views[0])
		}
	})
}
