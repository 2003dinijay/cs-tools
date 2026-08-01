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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func withDashboardID(r *http.Request, dashboardID string) *http.Request {
	r.SetPathValue("dashboardId", dashboardID)
	return r
}

func TestGetDashboardWidgets(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewDashboardHandler()
		r := withDashboardID(httptest.NewRequest(http.MethodGet, "/dashboards/agents_pilot/widgets", nil), "agents_pilot")
		w := httptest.NewRecorder()
		h.GetDashboardWidgets(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("unknown dashboard id returns 404", func(t *testing.T) {
		h := NewDashboardHandler()
		r := withUser(withDashboardID(httptest.NewRequest(http.MethodGet, "/dashboards/bogus/widgets", nil), "bogus"))
		w := httptest.NewRecorder()
		h.GetDashboardWidgets(w, r)
		assertStatus(t, w, http.StatusNotFound)
		assertErrorMessage(t, w, ErrMsgNotFound)
	})

	t.Run("returns filter criteria and display metadata for all three pilot widgets", func(t *testing.T) {
		h := NewDashboardHandler()
		r := withUser(withDashboardID(httptest.NewRequest(http.MethodGet, "/dashboards/agents_pilot/widgets", nil), "agents_pilot"))
		w := httptest.NewRecorder()
		h.GetDashboardWidgets(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		var results []struct {
			WidgetID    string `json:"widgetId"`
			DisplayName string `json:"displayName"`
			DisplayType string `json:"displayType"`
			Filters     struct {
				AssignedUserIDs []string `json:"assignedUserIds"`
				Tags            []string `json:"tags"`
				States          []string `json:"states"`
			} `json:"filters"`
		}
		if err := json.NewDecoder(w.Body).Decode(&results); err != nil {
			t.Fatalf("decode response body: %v; raw: %s", err, w.Body.String())
		}
		if len(results) != 3 {
			t.Fatalf("len(results) = %d, want 3", len(results))
		}

		byID := make(map[string]int)
		for i, res := range results {
			byID[res.WidgetID] = i
			if res.DisplayType != "single_score" {
				t.Errorf("widget %s displayType = %q, want single_score", res.WidgetID, res.DisplayType)
			}
			if res.DisplayName == "" {
				t.Errorf("widget %s has empty displayName", res.WidgetID)
			}
		}

		for _, id := range []string{"my_patches", "my_reminders"} {
			idx, ok := byID[id]
			if !ok {
				t.Fatalf("missing widget %q in response", id)
			}
			filters := results[idx].Filters
			if len(filters.AssignedUserIDs) != 1 || filters.AssignedUserIDs[0] != testUser.UserID {
				t.Errorf("widget %s assignedUserIds = %v, want [%q]", id, filters.AssignedUserIDs, testUser.UserID)
			}
			for _, uid := range filters.AssignedUserIDs {
				if uid == "__current_user__" {
					t.Errorf("widget %s assignedUserIds leaked the unresolved placeholder", id)
				}
			}
		}

		teamIdx, ok := byID["open_incident_team"]
		if !ok {
			t.Fatalf("missing widget %q in response", "open_incident_team")
		}
		if len(results[teamIdx].Filters.AssignedUserIDs) != 0 {
			t.Errorf("widget open_incident_team assignedUserIds = %v, want none", results[teamIdx].Filters.AssignedUserIDs)
		}
	})
}
