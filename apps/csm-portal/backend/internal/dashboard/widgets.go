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

// Package dashboard holds the pilot's config-driven dashboard widget
// templates. Each widget resolves to a search against that ResourceType's own
// /search endpoint (every resource's search payload shape is
// {filters: {...}, pagination: {...}}) — there is no generic filter DSL and
// no database backing this; the registry itself is loaded from the
// DASHBOARDS_CONFIG environment variable at process startup (see
// ParseDashboardsConfig and cmd/server/main.go).
package dashboard

import (
	"encoding/json"
	"log/slog"
)

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
	ShapePie   Shape = "pie"   // one search per Slices entry, each resolved via its own total — see PieSlice
	ShapeBar   Shape = "bar"   // same resolution as ShapePie (one search per Slices entry); differs only in how the frontend renders the resolved data
)

// PieSlice is one wedge of a Shape "pie" widget. The caller resolves its
// value by issuing that widget's own ResourceType's /search with Filters
// merged under the widget's own base Filters (this slice's keys win on
// conflict) and pagination.limit=1, reading total off the response — the
// exact same mechanism Shape "count" uses, just once per slice.
type PieSlice struct {
	Label string `json:"label"`
	// Color is a palette key ("primary", "secondary", "success", "error",
	// "info", "warning") the frontend already uses elsewhere in this system
	// (see WidgetTemplate's own icon color convention on the frontend) — not
	// validated here, forwarded verbatim. Falls back to a fixed rotation over
	// the same palette on the frontend if omitted.
	Color   string         `json:"color,omitempty"`
	Filters map[string]any `json:"filters"`
}

// WidgetTemplate is resource-agnostic: Filters is opaque JSON, forwarded
// verbatim (after __current_user__ substitution) as the filters object of
// that ResourceType's own /search payload (every resource's search payload
// shape is {filters: {...}, pagination: {...}}). The BE never interprets
// filter contents beyond substituting the current-user placeholder.
type WidgetTemplate struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	// Description is an explanatory subtitle shown under DisplayName —
	// config-owned text, not hardcoded per ResourceType/Shape on the
	// frontend.
	Description  string         `json:"description,omitempty"`
	ResourceType ResourceType   `json:"resourceType"`
	Shape        Shape          `json:"shape"`
	GridWidth    int            `json:"gridWidth"` // 1-12, CSS grid columns out of 12
	Filters      map[string]any `json:"filters"`
	GroupBy      string         `json:"groupBy,omitempty"`   // unused
	ListLimit    int            `json:"listLimit,omitempty"` // only meaningful for Shape list; how many records to show
	Slices       []PieSlice     `json:"slices,omitempty"`    // only meaningful for Shape pie/bar; one search per slice
	// Section groups widgets sharing the same (non-empty) value under a
	// titled sub-section within the dashboard, in the order that value
	// first appears among the dashboard's widgets — e.g. a handful of
	// "count" widgets all set to Section: "SLA Violation" render together
	// under that heading, separately from the dashboard's other widgets.
	// Widgets with no Section (the common case) render in one untitled
	// group, exactly as before this field existed.
	Section string `json:"section,omitempty"`
}

// Dashboard is a single dashboard's metadata plus its widget templates.
type Dashboard struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	IsDefault   bool   `json:"isDefault"`
	// TargetTeam is purely descriptive metadata (e.g. for a future FE team
	// picker); it is not enforced anywhere. GET /dashboards still returns
	// every dashboard to every caller regardless of team membership.
	TargetTeam string `json:"targetTeam"`
	// IsTeamBased marks a dashboard whose FE view should offer a team
	// selector (populated from POST /teams/search) alongside the dashboard
	// switcher. This is currently UI skeleton only: selecting a team does
	// not yet scope any widget's data. Wiring a selected team into widget
	// filters (e.g. resolving its member user IDs into a case widget's
	// assignedUserIds) is deliberately deferred to a later increment.
	IsTeamBased bool             `json:"isTeamBased"`
	Widgets     []WidgetTemplate `json:"widgets"`
}

// Dashboards is the ordered registry of dashboards, populated once at process
// startup from the DASHBOARDS_CONFIG environment variable (see
// ParseDashboardsConfig, called from cmd/server/main.go). It is empty (nil)
// until main() populates it; there is no file-watching or hot-reload — a
// config change requires restarting the process. Order is deterministic and
// is what the frontend's dashboard picker displays.
var Dashboards []Dashboard

// ParseDashboardsConfig decodes DASHBOARDS_CONFIG, a JSON array of Dashboard
// objects (see the Dashboard and WidgetTemplate json tags for the expected
// shape). A missing or malformed value logs an error and yields no
// dashboards rather than failing startup, since callers always check
// dashboard.Dashboards for emptiness (GET /dashboards simply returns an empty
// list; GET /dashboards/{id} 404s) instead of crashing the process.
func ParseDashboardsConfig(raw string) []Dashboard {
	if raw == "" {
		return nil
	}
	var dashboards []Dashboard
	if err := json.Unmarshal([]byte(raw), &dashboards); err != nil {
		slog.Error("failed to parse DASHBOARDS_CONFIG; no dashboards will be available", "err", err)
		return nil
	}
	return dashboards
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

// ResolveSliceFilters is ResolveFilters' counterpart for one Shape "pie"
// slice: substitutes CurrentUserPlaceholder in slice.Filters only. It
// deliberately does NOT merge in the widget's own base Filters — the caller
// (frontend) merges this slice's filters under the widget's own resolved
// Filters itself (this slice's keys win on conflict), the same way it
// already merges any other per-slice override. Sending the two separately,
// rather than one pre-merged object per slice, avoids repeating the base
// filters' JSON in every slice over the wire. Does not mutate slice.Filters.
func ResolveSliceFilters(slice PieSlice, currentUserID string) map[string]any {
	return substituteCurrentUser(slice.Filters, currentUserID).(map[string]any)
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
