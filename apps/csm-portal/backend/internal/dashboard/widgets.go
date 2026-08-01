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
// templates. Each widget resolves to a search against that ResourceType's own
// /search endpoint (every resource's search payload shape is
// {filters: {...}, pagination: {...}}) — there is no generic filter DSL and
// no database backing this; new widgets are added by extending the
// Dashboards registry below.
package dashboard

// CurrentUserPlaceholder marks a filter value that must be resolved to the
// requesting user's id before the filters are sent upstream. It never
// reaches the entity service: ResolveFilters always substitutes it.
const CurrentUserPlaceholder = "__current_user__"

// ResourceType identifies which resource a widget's filters search against.
type ResourceType string

const (
	ResourceCase                 ResourceType = "case"
	ResourceIncident             ResourceType = "incident"
	ResourceChangeRequest        ResourceType = "change_request"
	ResourceAccount              ResourceType = "account"
	ResourceProject              ResourceType = "project"
	ResourceUser                 ResourceType = "user"
	ResourceTimeCard             ResourceType = "time_card"
	ResourceProblem              ResourceType = "problem"
	ResourceProductVulnerability ResourceType = "product_vulnerability"
)

// Shape is how a widget's resolved data should be rendered.
type Shape string

const (
	ShapeCount Shape = "count" // single resolved number
	ShapeList  Shape = "list"  // top-N matching records
	ShapePie   Shape = "pie"   // grouped counts — NOT resolvable by any /search endpoint today (no aggregate endpoint exists anywhere in the stack); keep the const so a future dashboard doesn't need a schema migration, but do not wire any rendering logic for it beyond accepting the value
	ShapeBar   Shape = "bar"   // same caveat as ShapePie
)

// WidgetTemplate is resource-agnostic: Filters is opaque JSON, forwarded
// verbatim (after __current_user__ substitution) as the filters object of
// that ResourceType's own /search payload (every resource's search payload
// shape is {filters: {...}, pagination: {...}}). The BE never interprets
// filter contents beyond substituting the current-user placeholder.
type WidgetTemplate struct {
	ID           string
	DisplayName  string
	ResourceType ResourceType
	Shape        Shape
	GridWidth    int // 1-12, CSS grid columns out of 12
	Filters      map[string]any
	GroupBy      string `json:",omitempty"` // only meaningful for Shape pie/bar — see the caveat on those consts; unused by every widget below
	ListLimit    int    `json:",omitempty"` // only meaningful for Shape list; how many records to show
}

// Dashboard is a single dashboard's metadata plus its static widget
// templates.
type Dashboard struct {
	ID          string
	DisplayName string
	IsDefault   bool
	// TargetTeam is purely descriptive metadata (e.g. for a future FE team
	// picker); it is not enforced anywhere. GET /dashboards still returns
	// every dashboard to every caller regardless of team membership.
	TargetTeam string
	Widgets    []WidgetTemplate
}

// Dashboards is the ordered, static registry of dashboards. Order is
// deterministic and is what the frontend's dashboard picker displays.
var Dashboards = []Dashboard{
	{
		ID: "agents_pilot", DisplayName: "Engineer overview", IsDefault: true, TargetTeam: "cs_engineers",
		Widgets: []WidgetTemplate{
			{
				ID: "my_patches", DisplayName: "My Patches", ResourceType: ResourceCase, Shape: ShapeCount, GridWidth: 3,
				Filters: map[string]any{
					"assignedUserIds": []string{CurrentUserPlaceholder},
					"tags":            []string{"patch"},
					"states":          []string{"open", "work_in_progress", "waiting_on_wso2", "reopened", "awaiting_info"},
				},
			},
			{
				ID: "my_reminders", DisplayName: "My Reminders", ResourceType: ResourceCase, Shape: ShapeCount, GridWidth: 3,
				Filters: map[string]any{
					"assignedUserIds": []string{CurrentUserPlaceholder},
					"states":          []string{"awaiting_info", "solution_proposed"},
				},
			},
			{
				ID: "open_incident_team", DisplayName: "Open Incident (Team)", ResourceType: ResourceCase, Shape: ShapeCount, GridWidth: 3,
				Filters: map[string]any{
					"tags":   []string{"s_dip"},
					"states": []string{"work_in_progress", "open", "waiting_on_wso2", "reopened"},
				},
			},
			{
				ID: "my_critical_open", DisplayName: "My Critical & High Cases", ResourceType: ResourceCase, Shape: ShapeList, GridWidth: 3, ListLimit: 5,
				Filters: map[string]any{
					"assignedUserIds": []string{CurrentUserPlaceholder},
					"severities":      []string{"catastrophic", "critical"},
					"states":          []string{"open", "work_in_progress"},
				},
			},
		},
	},
	{
		ID: "operations", DisplayName: "Operations", TargetTeam: "cs_operations",
		Widgets: []WidgetTemplate{
			{
				ID: "p0_p1_open", DisplayName: "P0/P1 Open", ResourceType: ResourceCase, Shape: ShapeCount, GridWidth: 4,
				Filters: map[string]any{
					"severities": []string{"catastrophic", "critical"},
					"states":     []string{"open", "work_in_progress"},
				},
			},
			{
				ID: "open_critical_incidents", DisplayName: "Open Critical Incidents", ResourceType: ResourceIncident, Shape: ShapeCount, GridWidth: 4,
				Filters: map[string]any{"priorities": []string{"CRITICAL", "HIGH"}},
			},
			{
				ID: "crs_awaiting_approval", DisplayName: "CRs Awaiting Approval", ResourceType: ResourceChangeRequest, Shape: ShapeCount, GridWidth: 4,
				Filters: map[string]any{"states": []string{"customer_approval"}},
			},
		},
	},
	{
		ID: "iam", DisplayName: "IAM CS", TargetTeam: "iam_cs",
		Widgets: []WidgetTemplate{
			{
				ID: "iam_open_cases", DisplayName: "IAM Open Cases", ResourceType: ResourceCase, Shape: ShapeCount, GridWidth: 6,
				Filters: map[string]any{
					"tags":   []string{"iam"},
					"states": []string{"open", "work_in_progress", "awaiting_info"},
				},
			},
			{
				ID: "asgardeo_open_cases", DisplayName: "Asgardeo Open Cases", ResourceType: ResourceCase, Shape: ShapeCount, GridWidth: 6,
				Filters: map[string]any{
					"tags":   []string{"asgardeo"},
					"states": []string{"open", "work_in_progress", "awaiting_info"},
				},
			},
		},
	},
	{
		ID: "security", DisplayName: "Security center", TargetTeam: "security",
		Widgets: []WidgetTemplate{
			{
				ID: "critical_vulns", DisplayName: "Critical Vulnerabilities", ResourceType: ResourceProductVulnerability, Shape: ShapeCount, GridWidth: 4,
				Filters: map[string]any{"priority": "critical"},
			},
			{
				ID: "high_vulns", DisplayName: "High Vulnerabilities", ResourceType: ResourceProductVulnerability, Shape: ShapeCount, GridWidth: 4,
				Filters: map[string]any{"priority": "high"},
			},
			{
				ID: "sra_cases_open", DisplayName: "Open SRAs", ResourceType: ResourceCase, Shape: ShapeCount, GridWidth: 4,
				Filters: map[string]any{
					"types":  []string{"security_report_analysis"},
					"states": []string{"open", "work_in_progress", "awaiting_info"},
				},
			},
		},
	},
	{
		ID: "team_performance", DisplayName: "Team performance", TargetTeam: "cs_team_leads",
		Widgets: []WidgetTemplate{
			{
				ID: "time_cards_pending_approval", DisplayName: "Time Cards Pending Approval", ResourceType: ResourceTimeCard, Shape: ShapeCount, GridWidth: 6,
				Filters: map[string]any{"states": []string{"pending"}},
			},
			{
				ID: "team_open_cases", DisplayName: "Team Open P0/P1", ResourceType: ResourceCase, Shape: ShapeCount, GridWidth: 6,
				Filters: map[string]any{
					"severities": []string{"catastrophic", "critical"},
					"states":     []string{"open", "work_in_progress"},
				},
			},
		},
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

// ResolveFilters returns tpl's filters with CurrentUserPlaceholder substituted
// by currentUserID wherever it appears as a string inside a []any (the only
// place a per-user value belongs in a filters object — e.g. assignedUserIds,
// userIds). It does not mutate tpl.Filters.
func ResolveFilters(tpl WidgetTemplate, currentUserID string) map[string]any {
	return substituteCurrentUser(tpl.Filters, currentUserID).(map[string]any)
}

func substituteCurrentUser(v any, currentUserID string) any {
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, sub := range val {
			out[k] = substituteCurrentUser(sub, currentUserID)
		}
		return out
	case []string:
		out := make([]string, len(val))
		for i, s := range val {
			if s == CurrentUserPlaceholder {
				s = currentUserID
			}
			out[i] = s
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, sub := range val {
			out[i] = substituteCurrentUser(sub, currentUserID)
		}
		return out
	case string:
		if val == CurrentUserPlaceholder {
			return currentUserID
		}
		return val
	default:
		return val
	}
}
