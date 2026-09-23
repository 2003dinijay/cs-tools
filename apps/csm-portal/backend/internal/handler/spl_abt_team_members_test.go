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
)

func TestSplGetABTTeamMembers(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewSplABTTeamMembersHandler(&mockABTTeamMembersServiceNowClient{}, &mockEmployeeInfoClient{}, []string{"csm-agents"})
		r := httptest.NewRequest(http.MethodGet, "/spl/abt-team-members?teamId=team-1", nil)
		w := httptest.NewRecorder()
		h.GetABTTeamMembers(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("rejects missing teamId", func(t *testing.T) {
		h := NewSplABTTeamMembersHandler(&mockABTTeamMembersServiceNowClient{}, &mockEmployeeInfoClient{}, []string{"csm-agents"})
		r := withUser(httptest.NewRequest(http.MethodGet, "/spl/abt-team-members", nil))
		w := httptest.NewRecorder()
		h.GetABTTeamMembers(w, r)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("rejects a teamId that would inject a query clause", func(t *testing.T) {
		called := false
		sn := &mockABTTeamMembersServiceNowClient{
			tableQueryFn: func(ctx context.Context, table string, params url.Values) ([]byte, error) {
				called = true
				return []byte(`{"result":[]}`), nil
			},
		}
		h := NewSplABTTeamMembersHandler(sn, &mockEmployeeInfoClient{}, []string{"csm-agents"})
		r := withUser(httptest.NewRequest(http.MethodGet, "/spl/abt-team-members?teamId="+url.QueryEscape("team-1^OR active=true"), nil))
		w := httptest.NewRecorder()
		h.GetABTTeamMembers(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		if called {
			t.Error("upstream should not have been called for an unsafe teamId")
		}
	})

	t.Run("returns 404 when no members found", func(t *testing.T) {
		sn := &mockABTTeamMembersServiceNowClient{
			tableQueryFn: func(ctx context.Context, table string, params url.Values) ([]byte, error) {
				return []byte(`{"result":[]}`), nil
			},
		}
		h := NewSplABTTeamMembersHandler(sn, &mockEmployeeInfoClient{}, []string{"csm-agents"})
		r := withUser(httptest.NewRequest(http.MethodGet, "/spl/abt-team-members?teamId=team-1", nil))
		w := httptest.NewRecorder()
		h.GetABTTeamMembers(w, r)
		assertStatus(t, w, http.StatusNotFound)
	})

	t.Run("merges member, role, and thumbnail data", func(t *testing.T) {
		var capturedQueries []string
		sn := &mockABTTeamMembersServiceNowClient{
			tableQueryFn: func(ctx context.Context, table string, params url.Values) ([]byte, error) {
				capturedQueries = append(capturedQueries, table+":"+params.Get("sysparm_query"))
				switch table {
				case "sys_user_grmember":
					return []byte(`{"result":[{"user.name":"Jane Doe","user.email":"jane@example.com"}]}`), nil
				case "u_team_member_role":
					return []byte(`{"result":[{"u_role":"Lead"}]}`), nil
				}
				return []byte(`{"result":[]}`), nil
			},
		}
		thumb := "https://example.com/thumb.png"
		ei := &mockEmployeeInfoClient{
			getEmployeeDataFn: func(ctx context.Context, workEmail string) (*employeeinfo.Employee, error) {
				return &employeeinfo.Employee{FirstName: "Jane", LastName: "Doe", EmployeeThumbnail: &thumb}, nil
			},
		}
		h := NewSplABTTeamMembersHandler(sn, ei, []string{"csm-agents"})
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
		if len(capturedQueries) != 2 || capturedQueries[0] != "sys_user_grmember:group=team-1" {
			t.Errorf("captured queries = %v, unexpected", capturedQueries)
		}
	})

	t.Run("member row survives an employee-info or role lookup failure", func(t *testing.T) {
		sn := &mockABTTeamMembersServiceNowClient{
			tableQueryFn: func(ctx context.Context, table string, params url.Values) ([]byte, error) {
				if table == "sys_user_grmember" {
					return []byte(`{"result":[{"user.name":"Jane Doe","user.email":"jane@example.com"}]}`), nil
				}
				return nil, context.DeadlineExceeded
			},
		}
		ei := &mockEmployeeInfoClient{
			getEmployeeDataFn: func(ctx context.Context, workEmail string) (*employeeinfo.Employee, error) {
				return nil, context.DeadlineExceeded
			},
		}
		h := NewSplABTTeamMembersHandler(sn, ei, []string{"csm-agents"})
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
