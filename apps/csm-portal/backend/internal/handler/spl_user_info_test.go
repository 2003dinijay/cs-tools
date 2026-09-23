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

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/employeeinfo"
)

func TestSplGetUserInfo(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewSplUserInfoHandler(&mockEmployeeInfoClient{}, []string{"csm-agents"})
		r := httptest.NewRequest(http.MethodGet, "/spl/user-info", nil)
		w := httptest.NewRecorder()
		h.GetUserInfo(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
	})

	t.Run("rejects user outside allowedGroups", func(t *testing.T) {
		h := NewSplUserInfoHandler(&mockEmployeeInfoClient{}, []string{"some-other-group"})
		r := withUser(httptest.NewRequest(http.MethodGet, "/spl/user-info", nil))
		w := httptest.NewRecorder()
		h.GetUserInfo(w, r)
		assertStatus(t, w, http.StatusForbidden)
		assertErrorMessage(t, w, ErrMsgForbidden)
	})

	t.Run("resolves employee data from the caller's own email", func(t *testing.T) {
		var capturedEmail string
		thumbnail := "https://example.com/thumb.png"
		h := NewSplUserInfoHandler(&mockEmployeeInfoClient{
			getEmployeeDataFn: func(ctx context.Context, workEmail string) (*employeeinfo.Employee, error) {
				capturedEmail = workEmail
				return &employeeinfo.Employee{FirstName: "Agent", LastName: "Example", EmployeeThumbnail: &thumbnail}, nil
			},
		}, []string{"csm-agents"})
		r := withUser(httptest.NewRequest(http.MethodGet, "/spl/user-info", nil))
		w := httptest.NewRecorder()
		h.GetUserInfo(w, r)
		assertStatus(t, w, http.StatusOK)
		if capturedEmail != testUser.Email {
			t.Errorf("captured email = %q, want %q", capturedEmail, testUser.Email)
		}
		view := decodeJSON[SplUserInfoView](t, w)
		if view.FirstName != "Agent" || view.LastName != "Example" {
			t.Errorf("view = %+v, want FirstName=Agent LastName=Example", view)
		}
		if view.EmployeeThumbnail == nil || *view.EmployeeThumbnail != thumbnail {
			t.Errorf("EmployeeThumbnail = %v, want %q", view.EmployeeThumbnail, thumbnail)
		}
	})

	t.Run("maps upstream failure to a generic 500", func(t *testing.T) {
		h := NewSplUserInfoHandler(&mockEmployeeInfoClient{
			getEmployeeDataFn: func(ctx context.Context, workEmail string) (*employeeinfo.Employee, error) {
				return nil, context.DeadlineExceeded
			},
		}, []string{"csm-agents"})
		r := withUser(httptest.NewRequest(http.MethodGet, "/spl/user-info", nil))
		w := httptest.NewRecorder()
		h.GetUserInfo(w, r)
		assertStatus(t, w, http.StatusInternalServerError)
		assertErrorMessage(t, w, "Failed to retrieve user info.")
	})
}
