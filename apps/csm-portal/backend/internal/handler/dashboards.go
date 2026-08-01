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
// metadata, returned by GET /dashboards/{dashboardId}/widgets. The caller
// resolves each widget's own data by issuing its own POST /cases/search
// request with Filters.
type dashboardWidgetView struct {
	WidgetID    string                      `json:"widgetId"`
	DisplayName string                      `json:"displayName"`
	DisplayType dashboard.DisplayType       `json:"displayType"`
	Filters     dashboard.CaseSearchFilters `json:"filters"`
}

// DashboardHandler handles HTTP requests for the config-driven dashboard
// widget pilot.
type DashboardHandler struct{}

// NewDashboardHandler creates a DashboardHandler.
func NewDashboardHandler() *DashboardHandler {
	return &DashboardHandler{}
}

// GetDashboardWidgets handles GET /dashboards/{dashboardId}/widgets.
func (h *DashboardHandler) GetDashboardWidgets(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	dashboardID := r.PathValue("dashboardId")
	templates, ok := dashboard.Dashboards[dashboardID]
	if !ok {
		writeError(w, http.StatusNotFound, ErrMsgNotFound)
		return
	}

	views := make([]dashboardWidgetView, 0, len(templates))
	for _, tpl := range templates {
		resolved := dashboard.ResolveFilters(tpl, user.UserID)
		views = append(views, dashboardWidgetView{
			WidgetID:    tpl.ID,
			DisplayName: tpl.DisplayName,
			DisplayType: tpl.DisplayType,
			Filters:     resolved.Filters,
		})
	}

	writeJSONValue(w, http.StatusOK, views)
}
