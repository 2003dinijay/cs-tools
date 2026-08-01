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

// caseSearchPagination mirrors the Pagination fields /cases/search accepts.
type caseSearchPagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// CaseSearchPayload is the body ResolveFilters produces for /cases/search.
type CaseSearchPayload struct {
	Filters    CaseSearchFilters    `json:"filters"`
	Pagination caseSearchPagination `json:"pagination"`
}

// WidgetTemplate is a static, config-driven widget definition: which case
// filters it runs and how its resolved data should be displayed.
type WidgetTemplate struct {
	ID          string
	DisplayName string
	DisplayType DisplayType
	Filters     CaseSearchFilters
}

// Dashboards is the static registry of widget templates, keyed by dashboard id.
var Dashboards = map[string][]WidgetTemplate{
	"agents_pilot": {
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
}

// ResolveFilters builds the /cases/search payload for tpl, substituting
// CurrentUserPlaceholder in AssignedUserIDs with currentUserID. Pagination is
// fixed at limit 1: callers only need the response's total count, matching
// the existing count-only /cases/search usage pattern (see
// useCaseCountsMatrix.ts on the frontend).
func ResolveFilters(tpl WidgetTemplate, currentUserID string) CaseSearchPayload {
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
	return CaseSearchPayload{
		Filters:    filters,
		Pagination: caseSearchPagination{Limit: 1, Offset: 0},
	}
}
