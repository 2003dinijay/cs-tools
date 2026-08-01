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

// Package dashboard holds the pilot's static, config-driven dashboard widget
// templates. Each widget resolves to a case search against the existing
// /cases/search filter shape (see CaseSearchFilters in openapi.yaml) — there
// is no generic filter DSL and no database backing this; new widgets are
// added by extending the Dashboards registry below.
package dashboard

// CurrentUserPlaceholder marks an assignedUserIds entry that must be resolved
// to the requesting user's id before the filters are sent upstream. It never
// reaches the entity service: ResolveFilters always substitutes it.
const CurrentUserPlaceholder = "__current_user__"

// DisplayType is how a widget's resolved data should be rendered.
type DisplayType string

// DisplayTypeSingleScore is the only display type this pilot supports: a
// single resolved count.
const DisplayTypeSingleScore DisplayType = "single_score"

// CaseSearchFilters mirrors the subset of the entity service's
// CaseSearchFilters schema (openapi.yaml, component CaseSearchFilters) that
// the pilot widgets need. Field names and JSON tags match that schema exactly
// so the marshaled payload is forwarded to /cases/search unchanged.
type CaseSearchFilters struct {
	States          []string `json:"states,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	AssignedUserIDs []string `json:"assignedUserIds,omitempty"`
}

// WidgetTemplate is a static, config-driven widget definition: which case
// filters it runs and how its resolved data should be displayed.
type WidgetTemplate struct {
	ID          string
	DisplayName string
	DisplayType DisplayType
	Filters     CaseSearchFilters
}

// Dashboard is a single dashboard's metadata plus its static widget
// templates.
type Dashboard struct {
	ID          string
	DisplayName string
	IsDefault   bool
	Widgets     []WidgetTemplate
}

// Dashboards is the ordered, static registry of dashboards. Order is
// deterministic and is what the frontend's dashboard picker displays.
var Dashboards = []Dashboard{
	{
		ID:          "agents_pilot",
		DisplayName: "Engineer overview",
		IsDefault:   true,
		Widgets: []WidgetTemplate{
			{
				ID:          "my_patches",
				DisplayName: "My Patches",
				DisplayType: DisplayTypeSingleScore,
				Filters: CaseSearchFilters{
					AssignedUserIDs: []string{CurrentUserPlaceholder},
					Tags:            []string{"patch"},
					States: []string{
						"open",
						"work_in_progress",
						"waiting_on_wso2",
						"reopened",
						"awaiting_info",
					},
				},
			},
			{
				ID:          "my_reminders",
				DisplayName: "My Reminders",
				DisplayType: DisplayTypeSingleScore,
				Filters: CaseSearchFilters{
					AssignedUserIDs: []string{CurrentUserPlaceholder},
					States: []string{
						"awaiting_info",
						"solution_proposed",
					},
				},
			},
			{
				ID:          "open_incident_team",
				DisplayName: "Open Incident (Team)",
				DisplayType: DisplayTypeSingleScore,
				Filters: CaseSearchFilters{
					Tags: []string{"s_dip"},
					States: []string{
						"work_in_progress",
						"open",
						"waiting_on_wso2",
						"reopened",
					},
				},
			},
		},
	},
	{
		ID:          "operations",
		DisplayName: "Operations",
		IsDefault:   false,
		Widgets:     nil,
	},
	{
		ID:          "iam",
		DisplayName: "IAM CS",
		IsDefault:   false,
		Widgets:     nil,
	},
	{
		ID:          "security",
		DisplayName: "Security center",
		IsDefault:   false,
		Widgets:     nil,
	},
	{
		ID:          "team_performance",
		DisplayName: "Team performance",
		IsDefault:   false,
		Widgets:     nil,
	},
}

// DashboardByID looks up a dashboard by id, returning ok=false if the id
// isn't in the registry.
func DashboardByID(id string) (Dashboard, bool) {
	for _, d := range Dashboards {
		if d.ID == id {
			return d, true
		}
	}
	return Dashboard{}, false
}

// ResolveFilters substitutes CurrentUserPlaceholder in tpl's AssignedUserIDs
// with currentUserID, returning the filters to send to /cases/search. The
// caller adds its own pagination (the frontend hardcodes limit 1, since it
// only needs the response's total count).
func ResolveFilters(tpl WidgetTemplate, currentUserID string) CaseSearchFilters {
	filters := tpl.Filters
	if len(filters.AssignedUserIDs) > 0 {
		resolved := make([]string, len(filters.AssignedUserIDs))
		for i, id := range filters.AssignedUserIDs {
			if id == CurrentUserPlaceholder {
				id = currentUserID
			}
			resolved[i] = id
		}
		filters.AssignedUserIDs = resolved
	}
	return filters
}
