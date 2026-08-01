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
//
// groupBy and listLimit are omitempty on the wire and are not included here;
// widgets that set them are checked individually where relevant.
var dashboardWidgetJSONKeys = []string{"widgetId", "displayName", "resourceType", "shape", "gridWidth", "filters"}

// dashboardListItemJSONKeys are the top-level JSON keys openapi.yaml's
// DashboardListItem schema declares.
var dashboardListItemJSONKeys = []string{"id", "displayName", "isDefault"}

// dashboardDetailJSONKeys are the top-level JSON keys openapi.yaml's
// Dashboard schema declares.
var dashboardDetailJSONKeys = []string{"id", "displayName", "isDefault", "targetTeam", "widgets"}

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

// assertJSONKeysSubset is like assertJSONKeys but only requires want to be
// present; used for widgets that additionally carry an omitempty field
// (groupBy/listLimit) beyond the base set.
func assertJSONKeysSuperset(t *testing.T, obj map[string]json.RawMessage, want []string, context string) {
	t.Helper()
	for _, k := range want {
		if _, ok := obj[k]; !ok {
			t.Errorf("%s missing expected key %q; got keys %v", context, k, keysOf(obj))
		}
	}
}

func keysOf(obj map[string]json.RawMessage) []string {
	out := make([]string, 0, len(obj))
	for k := range obj {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
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

// TestAllDashboardsHaveWidgets is the "no more mock/empty placeholders"
// guarantee: every dashboard in the registry now has real widgets.
func TestAllDashboardsHaveWidgets(t *testing.T) {
	if len(dashboard.Dashboards) != 5 {
		t.Fatalf("len(dashboard.Dashboards) = %d, want 5", len(dashboard.Dashboards))
	}
	for _, d := range dashboard.Dashboards {
		if len(d.Widgets) == 0 {
			t.Errorf("dashboard %q has no widgets, want at least 1", d.ID)
		}
	}
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

	t.Run("agents_pilot returns metadata and its four widgets", func(t *testing.T) {
		h := NewDashboardHandler()
		r := withUser(withDashboardID(httptest.NewRequest(http.MethodGet, "/dashboards/agents_pilot", nil), "agents_pilot"))
		w := httptest.NewRecorder()
		h.GetDashboardDetail(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		body := w.Body.Bytes()
		t.Logf("GET /dashboards/agents_pilot response: %s", body)

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
		if result.TargetTeam != "cs_engineers" {
			t.Errorf("TargetTeam = %q, want %q", result.TargetTeam, "cs_engineers")
		}
		if len(result.Widgets) != 4 {
			t.Fatalf("len(result.Widgets) = %d, want 4", len(result.Widgets))
		}

		// Confirm the actual top-level JSON keys match openapi.yaml's
		// declared Dashboard schema exactly.
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(body, &raw); err != nil {
			t.Fatalf("decode response body as raw keys: %v; raw: %s", err, body)
		}
		assertJSONKeys(t, raw, dashboardDetailJSONKeys, "response")

		// Confirm each widget's JSON keys match openapi.yaml's declared
		// DashboardWidget schema exactly (allowing the omitempty
		// groupBy/listLimit extras) — catches an added/removed field that
		// the struct decode above wouldn't (json.Unmarshal ignores unknown
		// keys and zero-values missing ones).
		var rawWidgets []map[string]json.RawMessage
		if err := json.Unmarshal(raw["widgets"], &rawWidgets); err != nil {
			t.Fatalf("decode widgets as raw keys: %v; raw: %s", err, raw["widgets"])
		}
		for i, obj := range rawWidgets {
			assertJSONKeysSuperset(t, obj, dashboardWidgetJSONKeys, fmt.Sprintf("widgets[%d]", i))
		}

		byID := make(map[string]int)
		for i, res := range result.Widgets {
			byID[res.WidgetID] = i
			if res.DisplayName == "" {
				t.Errorf("widget %s has empty displayName", res.WidgetID)
			}
		}

		wantResourceShape := map[string]struct {
			resourceType dashboard.ResourceType
			shape        dashboard.Shape
			gridWidth    int
		}{
			"my_patches":         {dashboard.ResourceCase, dashboard.ShapeCount, 3},
			"my_reminders":       {dashboard.ResourceCase, dashboard.ShapeCount, 3},
			"open_incident_team": {dashboard.ResourceCase, dashboard.ShapeCount, 3},
			"my_critical_open":   {dashboard.ResourceCase, dashboard.ShapeList, 3},
		}
		for id, want := range wantResourceShape {
			idx, ok := byID[id]
			if !ok {
				t.Fatalf("missing widget %q in response", id)
			}
			got := result.Widgets[idx]
			if got.ResourceType != want.resourceType {
				t.Errorf("widget %s resourceType = %q, want %q", id, got.ResourceType, want.resourceType)
			}
			if got.Shape != want.shape {
				t.Errorf("widget %s shape = %q, want %q", id, got.Shape, want.shape)
			}
			if got.GridWidth != want.gridWidth {
				t.Errorf("widget %s gridWidth = %d, want %d", id, got.GridWidth, want.gridWidth)
			}
		}

		if idx := byID["my_critical_open"]; result.Widgets[idx].ListLimit != 5 {
			t.Errorf("widget my_critical_open listLimit = %d, want 5", result.Widgets[idx].ListLimit)
		}

		for _, id := range []string{"my_patches", "my_reminders"} {
			idx, ok := byID[id]
			if !ok {
				t.Fatalf("missing widget %q in response", id)
			}
			filters := result.Widgets[idx].Filters
			assignedRaw, present := filters["assignedUserIds"]
			if !present {
				t.Fatalf("widget %s filters has no assignedUserIds key", id)
			}
			assigned, ok := assignedRaw.([]any)
			if !ok {
				t.Fatalf("widget %s assignedUserIds is %T, want []any", id, assignedRaw)
			}
			if len(assigned) != 1 || assigned[0] != testUser.UserID {
				t.Errorf("widget %s assignedUserIds = %v, want [%q]", id, assigned, testUser.UserID)
			}
			for _, uid := range assigned {
				if uid == "__current_user__" {
					t.Errorf("widget %s assignedUserIds leaked the unresolved placeholder", id)
				}
			}
		}

		// Widgets with no assignedUserIds field in their template must not
		// gain one during substitution: substituteCurrentUser only rewrites
		// values already present, it never adds keys.
		for _, id := range []string{"open_incident_team", "my_critical_open"} {
			idx, ok := byID[id]
			if !ok {
				t.Fatalf("missing widget %q in response", id)
			}
			if id == "my_critical_open" {
				// my_critical_open DOES carry assignedUserIds (the current
				// user's critical/high cases) — verify it resolved cleanly
				// instead of asserting absence.
				filters := result.Widgets[idx].Filters
				assignedRaw, present := filters["assignedUserIds"]
				if !present {
					t.Fatalf("widget %s filters has no assignedUserIds key", id)
				}
				assigned, ok := assignedRaw.([]any)
				if !ok || len(assigned) != 1 || assigned[0] != testUser.UserID {
					t.Errorf("widget %s assignedUserIds = %v, want [%q]", id, assignedRaw, testUser.UserID)
				}
				continue
			}
			filters := result.Widgets[idx].Filters
			if _, present := filters["assignedUserIds"]; present {
				t.Errorf("widget %s filters unexpectedly has an assignedUserIds key: %v", id, filters["assignedUserIds"])
			}
		}
	})

	t.Run("operations dashboard has three resource-type-diverse widgets", func(t *testing.T) {
		h := NewDashboardHandler()
		r := withUser(withDashboardID(httptest.NewRequest(http.MethodGet, "/dashboards/operations", nil), "operations"))
		w := httptest.NewRecorder()
		h.GetDashboardDetail(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		body := w.Body.Bytes()

		var result dashboardDetailView
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("decode response body: %v; raw: %s", err, body)
		}
		if result.ID != "operations" {
			t.Errorf("ID = %q, want %q", result.ID, "operations")
		}
		if result.TargetTeam != "cs_operations" {
			t.Errorf("TargetTeam = %q, want %q", result.TargetTeam, "cs_operations")
		}
		if len(result.Widgets) != 3 {
			t.Fatalf("len(result.Widgets) = %d, want 3", len(result.Widgets))
		}

		byID := make(map[string]dashboardWidgetView)
		for _, w := range result.Widgets {
			byID[w.WidgetID] = w
		}

		wantTypes := map[string]dashboard.ResourceType{
			"p0_p1_open":              dashboard.ResourceCase,
			"open_critical_incidents": dashboard.ResourceIncident,
			"crs_awaiting_approval":   dashboard.ResourceChangeRequest,
		}
		for id, wantType := range wantTypes {
			got, ok := byID[id]
			if !ok {
				t.Fatalf("missing widget %q in response", id)
			}
			if got.ResourceType != wantType {
				t.Errorf("widget %s resourceType = %q, want %q", id, got.ResourceType, wantType)
			}
		}

		incident, ok := byID["open_critical_incidents"]
		if !ok {
			t.Fatalf("missing widget %q in response", "open_critical_incidents")
		}
		prioritiesRaw, present := incident.Filters["priorities"]
		if !present {
			t.Fatalf("open_critical_incidents filters has no priorities key: %v", incident.Filters)
		}
		priorities, ok := prioritiesRaw.([]any)
		if !ok || len(priorities) != 2 || priorities[0] != "CRITICAL" || priorities[1] != "HIGH" {
			t.Errorf("open_critical_incidents filters.priorities = %v, want [CRITICAL HIGH] unmodified", prioritiesRaw)
		}
	})

	t.Run("security dashboard's product_vulnerability widget has a scalar string filter", func(t *testing.T) {
		h := NewDashboardHandler()
		r := withUser(withDashboardID(httptest.NewRequest(http.MethodGet, "/dashboards/security", nil), "security"))
		w := httptest.NewRecorder()
		h.GetDashboardDetail(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		body := w.Body.Bytes()
		t.Logf("GET /dashboards/security response: %s", body)

		var result dashboardDetailView
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("decode response body: %v; raw: %s", err, body)
		}
		if len(result.Widgets) != 3 {
			t.Fatalf("len(result.Widgets) = %d, want 3", len(result.Widgets))
		}

		byID := make(map[string]dashboardWidgetView)
		for _, w := range result.Widgets {
			byID[w.WidgetID] = w
		}

		critical, ok := byID["critical_vulns"]
		if !ok {
			t.Fatalf("missing widget %q in response", "critical_vulns")
		}
		if critical.ResourceType != dashboard.ResourceProductVulnerability {
			t.Errorf("critical_vulns resourceType = %q, want %q", critical.ResourceType, dashboard.ResourceProductVulnerability)
		}
		priority, present := critical.Filters["priority"]
		if !present {
			t.Fatalf("critical_vulns filters has no priority key: %v", critical.Filters)
		}
		if s, ok := priority.(string); !ok || s != "critical" {
			t.Errorf("critical_vulns filters.priority = %v (%T), want string %q", priority, priority, "critical")
		}
	})

	t.Run("every dashboard in the registry now has at least one widget", func(t *testing.T) {
		h := NewDashboardHandler()
		for _, d := range dashboard.Dashboards {
			r := withUser(withDashboardID(httptest.NewRequest(http.MethodGet, "/dashboards/"+d.ID, nil), d.ID))
			w := httptest.NewRecorder()
			h.GetDashboardDetail(w, r)
			assertStatus(t, w, http.StatusOK)

			var result dashboardDetailView
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Fatalf("dashboard %s: decode response body: %v; raw: %s", d.ID, err, w.Body.Bytes())
			}
			if len(result.Widgets) == 0 {
				t.Errorf("dashboard %s has 0 widgets in the response, want > 0", d.ID)
			}
		}
	})
}
