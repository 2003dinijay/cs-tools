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

// abtTeamMembersClient abstracts the roster lookup used by
// SplABTTeamMembersHandler — a small domain-shaped interface (not raw
// ServiceNow TableQuery) specifically so both the ServiceNow and Postgres
// data sources can implement it, the same pattern every other SPL domain
// already uses.
type abtTeamMembersClient interface {
	GetABTTeamMembers(ctx context.Context, teamID string) ([]servicenow.ABTTeamRosterMember, error)
}

// SplABTTeamMemberView is one entry of the GET /spl/abt-team-members
// response — mirrors Ballerina modules/operations/types.bal's
// ABTTeamMemberDetails.
type SplABTTeamMemberView struct {
	Name              string  `json:"name"`
	Email             string  `json:"email"`
	Role              string  `json:"role"`
	EmployeeThumbnail *string `json:"employeeThumbnail"`
}

// SplABTTeamMembersHandler handles HTTP requests for an ABT team's member
// roster, cross-referencing the roster client (team membership, role) and
// the employee-info service (thumbnail) — the thumbnail enrichment is
// shared across data sources here rather than duplicated in each client.
type SplABTTeamMembersHandler struct {
	roster        abtTeamMembersClient
	employeeInfo  employeeInfoClient
	allowedGroups []string
}

// NewSplABTTeamMembersHandler creates a SplABTTeamMembersHandler backed by
// the given roster and employee-info clients. allowedGroups is
// SupportPortalLite's blanket access-gate group list (SPL_ALLOWED_GROUPS).
func NewSplABTTeamMembersHandler(roster abtTeamMembersClient, employeeInfo employeeInfoClient, allowedGroups []string) *SplABTTeamMembersHandler {
	return &SplABTTeamMembersHandler{roster: roster, employeeInfo: employeeInfo, allowedGroups: allowedGroups}
}

// GetABTTeamMembers handles GET /spl/abt-team-members?teamId=... — ported
// from Ballerina modules/operations/operations.bal's getABTTeamMembers and
// getABTTeamMemberDetails. Unlike the Ballerina original, a per-member
// employee-info or role lookup failure is logged and treated as best-effort
// (leaving that field blank) rather than aborting the whole request — the
// Ballerina source already does this for the employee lookup (`if employee
// is userinfo:Employee { ... }`, silently skipped otherwise) and for the
// role lookup (`if role is SNTeamMemberRoles { ... }`), so this preserves
// that same best-effort behavior faithfully, it does not add new leniency.
func (h *SplABTTeamMembersHandler) GetABTTeamMembers(w http.ResponseWriter, r *http.Request) {
	user, ok := requireSPLGroups(w, r, h.allowedGroups)
	if !ok {
		return
	}

	teamID := r.URL.Query().Get("teamId")
	if teamID == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if err := servicenow.SanitizeQueryValue(teamID); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	ctx := r.Context()

	roster, err := h.roster.GetABTTeamMembers(ctx, teamID)
	if err != nil {
		slog.ErrorContext(ctx, "GetABTTeamMembers failed", "userID", user.UserID, "teamID", teamID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve ABT team members.")
		return
	}

	if len(roster) == 0 {
		writeError(w, http.StatusNotFound, ErrMsgNotFound)
		return
	}

	views := make([]SplABTTeamMemberView, 0, len(roster))
	for _, m := range roster {
		view := SplABTTeamMemberView{Name: m.Name, Email: m.Email, Role: m.Role}

		if employee, err := h.employeeInfo.GetEmployeeData(ctx, m.Email); err != nil {
			slog.WarnContext(ctx, "employeeinfo GetEmployeeData failed for ABT team member; leaving thumbnail blank",
				"userID", user.UserID, "memberEmail", m.Email, "err", err)
		} else {
			view.EmployeeThumbnail = employee.EmployeeThumbnail
		}

		views = append(views, view)
	}

	writeJSONValue(w, http.StatusOK, views)
}
