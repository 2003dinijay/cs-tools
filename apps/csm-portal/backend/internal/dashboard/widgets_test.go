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
						"assignedUserIds": ["__current_user__"],
						"tags": ["patch"],
						"states": ["open", "work_in_progress"]
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
	assignedRaw, present := w.Filters["assignedUserIds"]
	if !present {
		t.Fatalf("Filters has no assignedUserIds key")
	}
	assigned, ok := assignedRaw.([]any)
	if !ok {
		t.Fatalf("Filters[assignedUserIds] is %T, want []any", assignedRaw)
	}
	if len(assigned) != 1 || assigned[0] != CurrentUserPlaceholder {
		t.Errorf("Filters[assignedUserIds] = %v, want [%q]", assigned, CurrentUserPlaceholder)
	}

	// End-to-end: resolving through the real substitution path yields a
	// concrete user id in place of the placeholder.
	resolved := ResolveFilters(w, "user-123")
	resolvedAssigned, ok := resolved["assignedUserIds"].([]any)
	if !ok || len(resolvedAssigned) != 1 || resolvedAssigned[0] != "user-123" {
		t.Errorf("ResolveFilters(...)[assignedUserIds] = %v, want [\"user-123\"]", resolved["assignedUserIds"])
	}
}
