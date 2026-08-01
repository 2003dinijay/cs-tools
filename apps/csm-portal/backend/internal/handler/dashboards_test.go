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
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/dashboard"
)

// dashboardWidgetJSONKeys are the top-level JSON keys openapi.yaml's
// DashboardWidget schema declares. Kept in sync with that schema by hand;
// the tests below fail if the handler's actual response keys ever diverge
// from this set, catching an unannounced field rename/add/remove that a
// struct-only decode (which silently ignores unknown keys and zero-values
// missing ones) would miss.
var dashboardWidgetJSONKeys = []string{"widgetId", "displayName", "displayType", "filters"}

// dashboardListItemJSONKeys are the top-level JSON keys openapi.yaml's
// DashboardListItem schema declares.
var dashboardListItemJSONKeys = []string{"id", "displayName", "isDefault"}

// dashboardDetailJSONKeys are the top-level JSON keys openapi.yaml's
// Dashboard schema declares.
var dashboardDetailJSONKeys = []string{"id", "displayName", "isDefault", "widgets"}

func assertJSONKeys(t *testing.T, obj map[string]json.RawMessage, want []string, context string) {
	t.Helper()
	wantKeys := append([]string(nil), want...)
	sort.Strings(wantKeys)
	gotKeys := make([]string, 0, len(obj))
	for k := range obj {
		gotKeys = append(gotKeys, k)
	}
	sort.Strings(gotKeys)
	if !reflect.DeepEqual(gotKeys, wantKeys) {
		t.Errorf("%s JSON keys = %v, want %v", context, gotKeys, wantKeys)
	}
}

func withDashboardID(r *http.Request, dashboardID string) *http.Request {
	r.SetPathValue("dashboardId", dashboardID)
	return r
}

func TestGetDashboards(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewDashboardHandler()
		r := httptest.NewRequest(http.MethodGet, "/dashboards", nil)
		w := httptest.NewRecorder()
		h.GetDashboards(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("returns all dashboards in registry order with correct isDefault", func(t *testing.T) {
		h := NewDashboardHandler()
		r := withUser(httptest.NewRequest(http.MethodGet, "/dashboards", nil))
		w := httptest.NewRecorder()
		h.GetDashboards(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		body := w.Body.Bytes()

		var results []dashboardListItemView
		if err := json.Unmarshal(body, &results); err != nil {
			t.Fatalf("decode response body: %v; raw: %s", err, body)
		}
		if len(results) != len(dashboard.Dashboards) {
			t.Fatalf("len(results) = %d, want %d", len(results), len(dashboard.Dashboards))
		}

		var raw []map[string]json.RawMessage
		if err := json.Unmarshal(body, &raw); err != nil {
			t.Fatalf("decode response body as raw keys: %v; raw: %s", err, body)
		}
		for i, obj := range raw {
			assertJSONKeys(t, obj, dashboardListItemJSONKeys, fmt.Sprintf("result[%d]", i))
		}

		for i, want := range dashboard.Dashboards {
			got := results[i]
			if got.ID != want.ID {
				t.Errorf("result[%d].ID = %q, want %q (registry order must be preserved)", i, got.ID, want.ID)
			}
			if got.DisplayName != want.DisplayName {
				t.Errorf("result[%d].DisplayName = %q, want %q", i, got.DisplayName, want.DisplayName)
			}
			if got.IsDefault != want.IsDefault {
				t.Errorf("result[%d].IsDefault = %v, want %v", i, got.IsDefault, want.IsDefault)
			}
		}

		defaultCount := 0
		for _, res := range results {
			if res.IsDefault {
				defaultCount++
				if res.ID != "agents_pilot" {
					t.Errorf("unexpected default dashboard %q, want agents_pilot", res.ID)
				}
			}
		}
		if defaultCount != 1 {
			t.Errorf("default dashboard count = %d, want 1", defaultCount)
		}
	})
}

func TestGetDashboardDetail(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewDashboardHandler()
		r := withDashboardID(httptest.NewRequest(http.MethodGet, "/dashboards/agents_pilot", nil), "agents_pilot")
		w := httptest.NewRecorder()
		h.GetDashboardDetail(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("unknown dashboard id returns 404", func(t *testing.T) {
		h := NewDashboardHandler()
		r := withUser(withDashboardID(httptest.NewRequest(http.MethodGet, "/dashboards/bogus", nil), "bogus"))
		w := httptest.NewRecorder()
		h.GetDashboardDetail(w, r)
		assertStatus(t, w, http.StatusNotFound)
		assertErrorMessage(t, w, ErrMsgNotFound)
	})

	t.Run("agents_pilot returns metadata and its three widgets", func(t *testing.T) {
		h := NewDashboardHandler()
		r := withUser(withDashboardID(httptest.NewRequest(http.MethodGet, "/dashboards/agents_pilot", nil), "agents_pilot"))
		w := httptest.NewRecorder()
		h.GetDashboardDetail(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		body := w.Body.Bytes()

		// Decode into the real production type (dashboardDetailView, defined
		// in dashboards.go), not a duplicate ad hoc struct — a JSON tag
		// rename on the real type breaks this decode/assertions directly,
		// instead of silently zero-valuing a field in a copy that has
		// already drifted from what's actually returned.
		var result dashboardDetailView
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("decode response body: %v; raw: %s", err, body)
		}

		if result.ID != "agents_pilot" {
			t.Errorf("ID = %q, want %q", result.ID, "agents_pilot")
		}
		if result.DisplayName != "Engineer overview" {
			t.Errorf("DisplayName = %q, want %q", result.DisplayName, "Engineer overview")
		}
		if !result.IsDefault {
			t.Errorf("IsDefault = %v, want true", result.IsDefault)
		}
		if len(result.Widgets) != 3 {
			t.Fatalf("len(result.Widgets) = %d, want 3", len(result.Widgets))
		}

		// Confirm the actual top-level JSON keys match openapi.yaml's
		// declared Dashboard schema exactly.
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(body, &raw); err != nil {
			t.Fatalf("decode response body as raw keys: %v; raw: %s", err, body)
		}
		assertJSONKeys(t, raw, dashboardDetailJSONKeys, "response")

		// Confirm each widget's JSON keys match openapi.yaml's declared
		// DashboardWidget schema exactly — catches an added/removed field
		// that the struct decode above wouldn't (json.Unmarshal ignores
		// unknown keys and zero-values missing ones).
		var rawWidgets []map[string]json.RawMessage
		if err := json.Unmarshal(raw["widgets"], &rawWidgets); err != nil {
			t.Fatalf("decode widgets as raw keys: %v; raw: %s", err, raw["widgets"])
		}
		for i, obj := range rawWidgets {
			assertJSONKeys(t, obj, dashboardWidgetJSONKeys, fmt.Sprintf("widgets[%d]", i))
		}

		byID := make(map[string]int)
		for i, res := range result.Widgets {
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
			filters := result.Widgets[idx].Filters
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
		if len(result.Widgets[teamIdx].Filters.AssignedUserIDs) != 0 {
			t.Errorf("widget open_incident_team assignedUserIds = %v, want none", result.Widgets[teamIdx].Filters.AssignedUserIDs)
		}
	})

	t.Run("mock dashboard with no widgets returns an empty widgets array, not null", func(t *testing.T) {
		h := NewDashboardHandler()
		r := withUser(withDashboardID(httptest.NewRequest(http.MethodGet, "/dashboards/operations", nil), "operations"))
		w := httptest.NewRecorder()
		h.GetDashboardDetail(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		body := w.Body.Bytes()

		var raw map[string]json.RawMessage
		if err := json.Unmarshal(body, &raw); err != nil {
			t.Fatalf("decode response body as raw keys: %v; raw: %s", err, body)
		}
		assertJSONKeys(t, raw, dashboardDetailJSONKeys, "response")

		widgetsRaw, present := raw["widgets"]
		if !present {
			t.Fatalf("response has no \"widgets\" key at all: %s", body)
		}
		if string(widgetsRaw) != "[]" {
			t.Errorf("widgets raw JSON = %s, want literal \"[]\" (never null)", widgetsRaw)
		}

		var result dashboardDetailView
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("decode response body: %v; raw: %s", err, body)
		}
		if result.ID != "operations" {
			t.Errorf("ID = %q, want %q", result.ID, "operations")
		}
		if result.DisplayName != "Operations" {
			t.Errorf("DisplayName = %q, want %q", result.DisplayName, "Operations")
		}
		if result.IsDefault {
			t.Errorf("IsDefault = true, want false")
		}
		if len(result.Widgets) != 0 {
			t.Errorf("len(result.Widgets) = %d, want 0", len(result.Widgets))
		}
	})
}
