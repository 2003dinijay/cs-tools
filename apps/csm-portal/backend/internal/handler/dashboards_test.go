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
	"reflect"
	"sort"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/dashboard"
)

// dashboardWidgetJSONKeys are the top-level JSON keys openapi.yaml's
// DashboardWidget schema declares. Kept in sync with that schema by hand;
// TestGetDashboardWidgets fails if the handler's actual response keys ever
// diverge from this set, catching an unannounced field rename/add/remove
// that a struct-only decode (which silently ignores unknown keys and
// zero-values missing ones) would miss.
var dashboardWidgetJSONKeys = []string{"widgetId", "displayName", "displayType", "filters"}

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

		body := w.Body.Bytes()

		// Decode into the real production type (dashboardWidgetView, defined
		// in dashboards.go), not a duplicate ad hoc struct — a JSON tag
		// rename on the real type breaks this decode/assertions directly,
		// instead of silently zero-valuing a field in a copy that has
		// already drifted from what's actually returned.
		var results []dashboardWidgetView
		if err := json.Unmarshal(body, &results); err != nil {
			t.Fatalf("decode response body: %v; raw: %s", err, body)
		}
		if len(results) != 3 {
			t.Fatalf("len(results) = %d, want 3", len(results))
		}

		// Confirm the actual JSON keys match openapi.yaml's declared
		// DashboardWidget schema exactly — catches an added/removed field
		// that the struct decode above wouldn't (json.Unmarshal ignores
		// unknown keys and zero-values missing ones).
		var raw []map[string]json.RawMessage
		if err := json.Unmarshal(body, &raw); err != nil {
			t.Fatalf("decode response body as raw keys: %v; raw: %s", err, body)
		}
		wantKeys := append([]string(nil), dashboardWidgetJSONKeys...)
		sort.Strings(wantKeys)
		for i, obj := range raw {
			gotKeys := make([]string, 0, len(obj))
			for k := range obj {
				gotKeys = append(gotKeys, k)
			}
			sort.Strings(gotKeys)
			if !reflect.DeepEqual(gotKeys, wantKeys) {
				t.Errorf("result[%d] JSON keys = %v, want %v (keep dashboardWidgetJSONKeys in sync with openapi.yaml's DashboardWidget schema)", i, gotKeys, wantKeys)
			}
		}

		byID := make(map[string]int)
		for i, res := range results {
			byID[res.WidgetID] = i
			if res.DisplayType != dashboard.DisplayTypeSingleScore {
				t.Errorf("widget %s displayType = %q, want %q", res.WidgetID, res.DisplayType, dashboard.DisplayTypeSingleScore)
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
