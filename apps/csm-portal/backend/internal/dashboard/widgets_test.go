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

package dashboard

import "testing"

func TestParseDashboardsConfig_Empty(t *testing.T) {
	got := ParseDashboardsConfig("")
	if got != nil {
		t.Errorf("ParseDashboardsConfig(\"\") = %v, want nil", got)
	}
}

func TestParseDashboardsConfig_Malformed(t *testing.T) {
	got := ParseDashboardsConfig("{not valid json")
	if got != nil {
		t.Errorf("ParseDashboardsConfig(malformed) = %v, want nil", got)
	}
}

func TestParseDashboardsConfig_MalformedShape(t *testing.T) {
	// Valid JSON, but not an array of Dashboard objects — must not panic and
	// must return nil, not a zero-value slice with garbage entries.
	got := ParseDashboardsConfig(`{"id":"not-an-array"}`)
	if got != nil {
		t.Errorf("ParseDashboardsConfig(wrong shape) = %v, want nil", got)
	}
}

func TestParseDashboardsConfig_ValidRoundTrip(t *testing.T) {
	const raw = `[
		{
			"id": "agents_pilot",
			"displayName": "Engineer overview",
			"isDefault": true,
			"targetTeam": "cs_engineers",
			"widgets": [
				{
					"id": "my_patches",
					"displayName": "My Patches",
					"resourceType": "case",
					"shape": "count",
					"gridWidth": 3,
					"filters": {
						"filters": [
							{"field": "assignedUserId", "op": "in", "values": ["__current_user__"]},
							{"field": "tag", "op": "in", "values": ["patch"]},
							{"field": "state", "op": "in", "values": ["open", "work_in_progress"]}
						]
					}
				}
			]
		}
	]`

	got := ParseDashboardsConfig(raw)
	if len(got) != 1 {
		t.Fatalf("len(ParseDashboardsConfig(raw)) = %d, want 1", len(got))
	}

	d := got[0]
	if d.ID != "agents_pilot" {
		t.Errorf("Dashboard.ID = %q, want %q", d.ID, "agents_pilot")
	}
	if d.DisplayName != "Engineer overview" {
		t.Errorf("Dashboard.DisplayName = %q, want %q", d.DisplayName, "Engineer overview")
	}
	if !d.IsDefault {
		t.Errorf("Dashboard.IsDefault = false, want true")
	}
	if d.TargetTeam != "cs_engineers" {
		t.Errorf("Dashboard.TargetTeam = %q, want %q", d.TargetTeam, "cs_engineers")
	}
	if len(d.Widgets) != 1 {
		t.Fatalf("len(Dashboard.Widgets) = %d, want 1", len(d.Widgets))
	}

	w := d.Widgets[0]
	if w.ID != "my_patches" {
		t.Errorf("WidgetTemplate.ID = %q, want %q", w.ID, "my_patches")
	}
	if w.ResourceType != ResourceCase {
		t.Errorf("WidgetTemplate.ResourceType = %q, want %q", w.ResourceType, ResourceCase)
	}
	if w.Shape != ShapeCount {
		t.Errorf("WidgetTemplate.Shape = %q, want %q", w.Shape, ShapeCount)
	}
	if w.GridWidth != 3 {
		t.Errorf("WidgetTemplate.GridWidth = %d, want 3", w.GridWidth)
	}

	// The detail that matters for ResolveFilters' substitution logic
	// downstream: a JSON array value unmarshals into map[string]any as
	// []any, not []string — assert the actual runtime type, not just
	// presence, since substituteCurrentUser's []any and []string cases
	// behave identically but are reached via different type switches.
	//
	// Filters is opaque to this package (see widgets.go's WidgetTemplate doc
	// comment), so the specific case-search filter DSL shape used here
	// ({"filters":[{"field","op","values"}, ...]}, see .env.example's
	// DASHBOARDS_CONFIG) is just realistic example data for this generic
	// substitution test, not something ResolveFilters interprets.
	assignedEntryValues := func(filters map[string]any) ([]any, bool) {
		arr, ok := filters["filters"].([]any)
		if !ok || len(arr) == 0 {
			return nil, false
		}
		entry, ok := arr[0].(map[string]any)
		if !ok {
			return nil, false
		}
		values, ok := entry["values"].([]any)
		return values, ok
	}

	assigned, ok := assignedEntryValues(w.Filters)
	if !ok {
		t.Fatalf("Filters has no filters[0].values entry")
	}
	if len(assigned) != 1 || assigned[0] != CurrentUserPlaceholder {
		t.Errorf("Filters[filters][0][values] = %v, want [%q]", assigned, CurrentUserPlaceholder)
	}

	// End-to-end: resolving through the real substitution path yields a
	// concrete user id in place of the placeholder.
	resolved := ResolveFilters(w, "user-123")
	resolvedAssigned, ok := assignedEntryValues(resolved)
	if !ok || len(resolvedAssigned) != 1 || resolvedAssigned[0] != "user-123" {
		t.Errorf("ResolveFilters(...)[filters][0][values] = %v, want [\"user-123\"]", resolvedAssigned)
	}
}

func TestParseDashboardsConfig_PieWidgetSlicesAndDescription(t *testing.T) {
	const raw = `[
		{
			"id": "team_performance",
			"displayName": "Team performance",
			"widgets": [
				{
					"id": "cases_by_severity",
					"displayName": "Cases by severity",
					"description": "Share of active cases at each severity level.",
					"resourceType": "case",
					"shape": "pie",
					"gridWidth": 6,
					"filters": {"states": ["open"]},
					"slices": [
						{"label": "Critical", "color": "error", "filters": {"severities": ["critical"]}},
						{"label": "Mine", "filters": {"assignedUserIds": ["__current_user__"]}}
					]
				}
			]
		}
	]`

	got := ParseDashboardsConfig(raw)
	if len(got) != 1 || len(got[0].Widgets) != 1 {
		t.Fatalf("ParseDashboardsConfig(raw) = %+v, want 1 dashboard with 1 widget", got)
	}

	w := got[0].Widgets[0]
	if w.Description != "Share of active cases at each severity level." {
		t.Errorf("WidgetTemplate.Description = %q, want the configured subtitle", w.Description)
	}
	if len(w.Slices) != 2 {
		t.Fatalf("len(WidgetTemplate.Slices) = %d, want 2", len(w.Slices))
	}
	if w.Slices[0].Label != "Critical" || w.Slices[0].Color != "error" {
		t.Errorf("Slices[0] = %+v, want {Label: Critical, Color: error}", w.Slices[0])
	}

	// A slice's own filters resolve the current-user placeholder
	// independently of the widget's own base Filters — ResolveSliceFilters
	// must not require (or merge in) the base at all.
	resolved := ResolveSliceFilters(w.Slices[1], "user-123")
	assigned, ok := resolved["assignedUserIds"].([]any)
	if !ok || len(assigned) != 1 || assigned[0] != "user-123" {
		t.Errorf("ResolveSliceFilters(Slices[1], ...)[assignedUserIds] = %v, want [\"user-123\"]", resolved["assignedUserIds"])
	}
	if _, present := resolved["states"]; present {
		t.Errorf("ResolveSliceFilters must not merge in the widget's own base filters, got %v", resolved)
	}
}

func TestParseDashboardsConfig_WidgetSection(t *testing.T) {
	const raw = `[
		{
			"id": "team_performance",
			"displayName": "Team performance",
			"widgets": [
				{"id": "team_open_cases", "displayName": "Team Open P0/P1", "resourceType": "case", "shape": "count", "gridWidth": 6, "filters": {}},
				{"id": "incident_wow", "displayName": "Incident WOW", "section": "SLA Violation", "resourceType": "incident", "shape": "count", "gridWidth": 6, "filters": {}}
			]
		}
	]`

	got := ParseDashboardsConfig(raw)
	if len(got) != 1 || len(got[0].Widgets) != 2 {
		t.Fatalf("ParseDashboardsConfig(raw) = %+v, want 1 dashboard with 2 widgets", got)
	}

	if section := got[0].Widgets[0].Section; section != "" {
		t.Errorf("team_open_cases.Section = %q, want empty (no section configured)", section)
	}
	if section := got[0].Widgets[1].Section; section != "SLA Violation" {
		t.Errorf("incident_wow.Section = %q, want %q", section, "SLA Violation")
	}
}
