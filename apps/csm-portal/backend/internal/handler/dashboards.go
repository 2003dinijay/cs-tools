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
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/dashboard"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// dashboardEntityClient is the subset of entityCaseClient DashboardHandler
// needs: it resolves each widget's filters through the same /cases/search
// path CaseHandler.SearchCases uses.
type dashboardEntityClient interface {
	SearchCases(ctx context.Context, body []byte) ([]byte, error)
}

// widgetResult is a single resolved widget's data, returned by
// GET /dashboards/{dashboardId}/widgets.
type widgetResult struct {
	WidgetID    string                `json:"widgetId"`
	DisplayName string                `json:"displayName"`
	DisplayType dashboard.DisplayType `json:"displayType"`
	Count       int                   `json:"count"`
}

// DashboardHandler handles HTTP requests for the config-driven dashboard
// widget pilot, delegating each widget's data resolution to the entity
// service's case search.
type DashboardHandler struct {
	entity dashboardEntityClient
}

// NewDashboardHandler creates a DashboardHandler backed by the given entity client.
func NewDashboardHandler(entity dashboardEntityClient) *DashboardHandler {
	return &DashboardHandler{entity: entity}
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

	results := make([]widgetResult, 0, len(templates))
	for _, tpl := range templates {
		payload := dashboard.ResolveFilters(tpl, user.UserID)
		body, err := json.Marshal(payload)
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to marshal widget search payload", "userID", user.UserID, "widgetID", tpl.ID, "err", err)
			writeError(w, http.StatusInternalServerError, ErrMsgInternal)
			return
		}

		searchResult, err := h.entity.SearchCases(r.Context(), body)
		if err != nil {
			slog.ErrorContext(r.Context(), "entity SearchCases failed for widget", "userID", user.UserID, "widgetID", tpl.ID, "err", err)
			mapUpstreamError(w, err, "Failed to resolve dashboard widgets.")
			return
		}

		var parsed struct {
			Total int `json:"total"`
		}
		if err := json.Unmarshal(searchResult, &parsed); err != nil {
			slog.ErrorContext(r.Context(), "failed to parse widget search response", "userID", user.UserID, "widgetID", tpl.ID, "err", err)
			writeError(w, http.StatusInternalServerError, ErrMsgInternal)
			return
		}

		results = append(results, widgetResult{
			WidgetID:    tpl.ID,
			DisplayName: tpl.DisplayName,
			DisplayType: tpl.DisplayType,
			Count:       parsed.Total,
		})
	}

	writeJSONValue(w, http.StatusOK, results)
}
