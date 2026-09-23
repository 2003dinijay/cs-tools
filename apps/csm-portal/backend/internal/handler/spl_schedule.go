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
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/servicenow"
)

// splScheduleClient abstracts the ServiceNow ABT team schedule operation
// used by SplScheduleHandler.
type splScheduleClient interface {
	GetABTTeamSchedule(ctx context.Context, from, duration, teamID, eventType, teamScheduleURL string) (servicenow.ABTTeamScheduleData, error)
}

// SplScheduleHandler handles HTTP requests for the ABT team schedule,
// delegating to the ServiceNow service.
type SplScheduleHandler struct {
	servicenow      splScheduleClient
	allowedGroups   []string
	teamScheduleURL string
}

// NewSplScheduleHandler creates a SplScheduleHandler backed by the given
// ServiceNow client. allowedGroups is SupportPortalLite's blanket
// access-gate group list (SPL_ALLOWED_GROUPS); teamScheduleURL is the
// static URL echoed back in every response (SPL_TEAM_SCHEDULE_URL).
func NewSplScheduleHandler(sn splScheduleClient, allowedGroups []string, teamScheduleURL string) *SplScheduleHandler {
	return &SplScheduleHandler{servicenow: sn, allowedGroups: allowedGroups, teamScheduleURL: teamScheduleURL}
}

// GetABTTeamSchedule handles GET /spl/abt-team-schedule. All query
// parameters are optional, mirroring the Ballerina resource function's
// `string?` parameters.
func (h *SplScheduleHandler) GetABTTeamSchedule(w http.ResponseWriter, r *http.Request) {
	user, ok := requireSPLGroups(w, r, h.allowedGroups)
	if !ok {
		return
	}

	q := r.URL.Query()
	from, duration, teamID, eventType := q.Get("from"), q.Get("duration"), q.Get("teamId"), q.Get("eventType")
	if teamID != "" {
		if err := servicenow.SanitizeQueryValue(teamID); err != nil {
			writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
			return
		}
	}

	schedule, err := h.servicenow.GetABTTeamSchedule(r.Context(), from, duration, teamID, eventType, h.teamScheduleURL)
	if err != nil {
		slog.ErrorContext(r.Context(), "servicenow GetABTTeamSchedule failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve team schedule.")
		return
	}

	writeJSONValue(w, http.StatusOK, schedule)
}
