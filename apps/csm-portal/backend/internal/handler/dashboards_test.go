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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

type mockDashboardEntityClient struct {
	searchCasesFn func(ctx context.Context, body []byte) ([]byte, error)
}

func (m *mockDashboardEntityClient) SearchCases(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchCasesFn != nil {
		return m.searchCasesFn(ctx, body)
	}
	return []byte(`{"cases":[],"total":0}`), nil
}

func withDashboardID(r *http.Request, dashboardID string) *http.Request {
	r.SetPathValue("dashboardId", dashboardID)
	return r
}

func TestGetDashboardWidgets(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewDashboardHandler(&mockDashboardEntityClient{})
		r := withDashboardID(httptest.NewRequest(http.MethodGet, "/dashboards/agents_pilot/widgets", nil), "agents_pilot")
		w := httptest.NewRecorder()
		h.GetDashboardWidgets(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("unknown dashboard id returns 404", func(t *testing.T) {
		h := NewDashboardHandler(&mockDashboardEntityClient{})
		r := withUser(withDashboardID(httptest.NewRequest(http.MethodGet, "/dashboards/bogus/widgets", nil), "bogus"))
		w := httptest.NewRecorder()
		h.GetDashboardWidgets(w, r)
		assertStatus(t, w, http.StatusNotFound)
		assertErrorMessage(t, w, ErrMsgNotFound)
	})

	t.Run("resolves all three pilot widgets and substitutes the current user id", func(t *testing.T) {
		var capturedBodies [][]byte
		client := &mockDashboardEntityClient{
			searchCasesFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBodies = append(capturedBodies, body)
				return []byte(`{"cases":[],"total":7}`), nil
			},
		}
		h := NewDashboardHandler(client)
		r := withUser(withDashboardID(httptest.NewRequest(http.MethodGet, "/dashboards/agents_pilot/widgets", nil), "agents_pilot"))
		w := httptest.NewRecorder()
		h.GetDashboardWidgets(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		var results []struct {
			WidgetID    string `json:"widgetId"`
			DisplayName string `json:"displayName"`
			DisplayType string `json:"displayType"`
			Count       *int   `json:"count"`
			Error       string `json:"error"`
		}
		if err := json.NewDecoder(w.Body).Decode(&results); err != nil {
			t.Fatalf("decode response body: %v; raw: %s", err, w.Body.String())
		}
		if len(results) != 3 {
			t.Fatalf("len(results) = %d, want 3", len(results))
		}
		for _, res := range results {
			if res.DisplayType != "single_score" {
				t.Errorf("widget %s displayType = %q, want single_score", res.WidgetID, res.DisplayType)
			}
			if res.Error != "" {
				t.Errorf("widget %s error = %q, want none", res.WidgetID, res.Error)
			}
			if res.Count == nil || *res.Count != 7 {
				t.Errorf("widget %s count = %v, want 7", res.WidgetID, res.Count)
			}
			if res.DisplayName == "" {
				t.Errorf("widget %s has empty displayName", res.WidgetID)
			}
		}

		if len(capturedBodies) != 3 {
			t.Fatalf("len(capturedBodies) = %d, want 3", len(capturedBodies))
		}
		// The two user-scoped widgets ("my_patches", "my_reminders") must carry the
		// resolved user id, never the raw placeholder.
		for _, body := range capturedBodies {
			var sent struct {
				Filters struct {
					AssignedUserIDs []string `json:"assignedUserIds"`
				} `json:"filters"`
				Pagination struct {
					Limit int `json:"limit"`
				} `json:"pagination"`
			}
			if err := json.Unmarshal(body, &sent); err != nil {
				t.Fatalf("upstream body invalid JSON: %v", err)
			}
			if sent.Pagination.Limit != 1 {
				t.Errorf("pagination.limit = %d, want 1", sent.Pagination.Limit)
			}
			for _, id := range sent.Filters.AssignedUserIDs {
				if id == "__current_user__" {
					t.Errorf("assignedUserIds leaked the unresolved placeholder: %v", sent.Filters.AssignedUserIDs)
				}
				if id != testUser.UserID {
					t.Errorf("assignedUserIds = %v, want [%q]", sent.Filters.AssignedUserIDs, testUser.UserID)
				}
			}
		}
	})

	t.Run("upstream error on every widget still returns 200 with each widget carrying its own error", func(t *testing.T) {
		client := &mockDashboardEntityClient{
			searchCasesFn: func(_ context.Context, _ []byte) ([]byte, error) {
				return nil, &apierror.Error{StatusCode: http.StatusServiceUnavailable}
			},
		}
		h := NewDashboardHandler(client)
		r := withUser(withDashboardID(httptest.NewRequest(http.MethodGet, "/dashboards/agents_pilot/widgets", nil), "agents_pilot"))
		w := httptest.NewRecorder()
		h.GetDashboardWidgets(w, r)
		assertStatus(t, w, http.StatusOK)

		var results []struct {
			WidgetID string `json:"widgetId"`
			Count    *int   `json:"count"`
			Error    string `json:"error"`
		}
		if err := json.NewDecoder(w.Body).Decode(&results); err != nil {
			t.Fatalf("decode response body: %v; raw: %s", err, w.Body.String())
		}
		if len(results) != 3 {
			t.Fatalf("len(results) = %d, want 3", len(results))
		}
		for _, res := range results {
			if res.Error == "" {
				t.Errorf("widget %s error = %q, want non-empty", res.WidgetID, res.Error)
			}
			if res.Count != nil {
				t.Errorf("widget %s count = %v, want omitted", res.WidgetID, *res.Count)
			}
		}
	})

	t.Run("non-apierror upstream failure is reported per widget, not as a handler-level 500", func(t *testing.T) {
		client := &mockDashboardEntityClient{
			searchCasesFn: func(_ context.Context, _ []byte) ([]byte, error) {
				return nil, errors.New("connection refused")
			},
		}
		h := NewDashboardHandler(client)
		r := withUser(withDashboardID(httptest.NewRequest(http.MethodGet, "/dashboards/agents_pilot/widgets", nil), "agents_pilot"))
		w := httptest.NewRecorder()
		h.GetDashboardWidgets(w, r)
		assertStatus(t, w, http.StatusOK)

		var results []struct {
			Error string `json:"error"`
		}
		if err := json.NewDecoder(w.Body).Decode(&results); err != nil {
			t.Fatalf("decode response body: %v; raw: %s", err, w.Body.String())
		}
		for _, res := range results {
			if res.Error == "" {
				t.Error("expected every widget to carry an error, got none")
			}
		}
	})

	t.Run("one widget's upstream failure does not take down its siblings", func(t *testing.T) {
		var calls int
		client := &mockDashboardEntityClient{
			searchCasesFn: func(_ context.Context, _ []byte) ([]byte, error) {
				calls++
				if calls == 1 {
					// Only the first-resolved widget ("my_patches") fails, mirroring the
					// live SN DEV finding: an assignedUserIds-based search 400s for one
					// widget while the others resolve normally in the same response.
					return nil, &apierror.Error{StatusCode: http.StatusBadRequest, Body: `{"message":"no active user found for sys_id ..."}`}
				}
				return []byte(`{"cases":[],"total":7}`), nil
			},
		}
		h := NewDashboardHandler(client)
		r := withUser(withDashboardID(httptest.NewRequest(http.MethodGet, "/dashboards/agents_pilot/widgets", nil), "agents_pilot"))
		w := httptest.NewRecorder()
		h.GetDashboardWidgets(w, r)
		assertStatus(t, w, http.StatusOK)

		var results []struct {
			WidgetID string `json:"widgetId"`
			Count    *int   `json:"count"`
			Error    string `json:"error"`
		}
		if err := json.NewDecoder(w.Body).Decode(&results); err != nil {
			t.Fatalf("decode response body: %v; raw: %s", err, w.Body.String())
		}
		if len(results) != 3 {
			t.Fatalf("len(results) = %d, want 3", len(results))
		}

		failed := results[0]
		if failed.WidgetID != "my_patches" {
			t.Fatalf("results[0].WidgetID = %q, want my_patches", failed.WidgetID)
		}
		if failed.Error == "" {
			t.Errorf("widget %s: expected an error, got none", failed.WidgetID)
		}
		if failed.Count != nil {
			t.Errorf("widget %s: count = %v, want omitted", failed.WidgetID, *failed.Count)
		}

		for _, res := range results[1:] {
			if res.Error != "" {
				t.Errorf("widget %s: unexpected error %q, want it unaffected by my_patches' failure", res.WidgetID, res.Error)
			}
			if res.Count == nil || *res.Count != 7 {
				t.Errorf("widget %s: count = %v, want 7", res.WidgetID, res.Count)
			}
		}
	})
}
