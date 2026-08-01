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
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/dashboard"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// dashboardWidgetView is a single widget's filter criteria and display
// metadata, returned as part of GET /dashboards/{dashboardId}. The caller
// resolves each widget's own data by issuing its own POST /cases/search
// request with Filters.
type dashboardWidgetView struct {
	WidgetID    string                      `json:"widgetId"`
	DisplayName string                      `json:"displayName"`
	DisplayType dashboard.DisplayType       `json:"displayType"`
	Filters     dashboard.CaseSearchFilters `json:"filters"`
}

// dashboardListItemView is a dashboard's list-level metadata, returned by
// GET /dashboards.
type dashboardListItemView struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	IsDefault   bool   `json:"isDefault"`
}

// dashboardDetailView is a dashboard's full metadata plus its resolved
// widgets, returned by GET /dashboards/{dashboardId}.
type dashboardDetailView struct {
	ID          string                `json:"id"`
	DisplayName string                `json:"displayName"`
	IsDefault   bool                  `json:"isDefault"`
	Widgets     []dashboardWidgetView `json:"widgets"`
}

// DashboardHandler handles HTTP requests for the config-driven dashboard
// widget pilot.
type DashboardHandler struct{}

// NewDashboardHandler creates a DashboardHandler.
func NewDashboardHandler() *DashboardHandler {
	return &DashboardHandler{}
}

// GetDashboards handles GET /dashboards.
func (h *DashboardHandler) GetDashboards(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	views := make([]dashboardListItemView, 0, len(dashboard.Dashboards))
	for _, d := range dashboard.Dashboards {
		views = append(views, dashboardListItemView{
			ID:          d.ID,
			DisplayName: d.DisplayName,
			IsDefault:   d.IsDefault,
		})
	}

	writeJSONValue(w, http.StatusOK, views)
}

// GetDashboardDetail handles GET /dashboards/{dashboardId}.
func (h *DashboardHandler) GetDashboardDetail(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	dashboardID := r.PathValue("dashboardId")
	d, ok := dashboard.DashboardByID(dashboardID)
	if !ok {
		writeError(w, http.StatusNotFound, ErrMsgNotFound)
		return
	}

	widgets := make([]dashboardWidgetView, 0, len(d.Widgets))
	for _, tpl := range d.Widgets {
		widgets = append(widgets, dashboardWidgetView{
			WidgetID:    tpl.ID,
			DisplayName: tpl.DisplayName,
			DisplayType: tpl.DisplayType,
			Filters:     dashboard.ResolveFilters(tpl, user.UserID),
		})
	}

	writeJSONValue(w, http.StatusOK, dashboardDetailView{
		ID:          d.ID,
		DisplayName: d.DisplayName,
		IsDefault:   d.IsDefault,
		Widgets:     widgets,
	})
}
