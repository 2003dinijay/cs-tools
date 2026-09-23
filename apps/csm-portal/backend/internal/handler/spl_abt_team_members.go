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
	"net/url"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/servicenow"
)

// abtTeamMembersServiceNowClient abstracts the ServiceNow Table API
// operations used by SplABTTeamMembersHandler.
type abtTeamMembersServiceNowClient interface {
	TableQuery(ctx context.Context, table string, params url.Values) ([]byte, error)
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

type snABTTeamMemberRow struct {
	UserName  string `json:"user.name"`
	UserEmail string `json:"user.email"`
}

type snABTTeamMembersResult struct {
	Result []snABTTeamMemberRow `json:"result"`
}

type snTeamMemberRoleRow struct {
	Role string `json:"u_role"`
}

type snTeamMemberRolesResult struct {
	Result []snTeamMemberRoleRow `json:"result"`
}

// SplABTTeamMembersHandler handles HTTP requests for an ABT team's member
// roster, cross-referencing ServiceNow (team membership, role) and the
// employee-info service (thumbnail).
type SplABTTeamMembersHandler struct {
	serviceNow    abtTeamMembersServiceNowClient
	employeeInfo  employeeInfoClient
	allowedGroups []string
}

// NewSplABTTeamMembersHandler creates a SplABTTeamMembersHandler backed by
// the given ServiceNow and employee-info clients. allowedGroups is
// SupportPortalLite's blanket access-gate group list (SPL_ALLOWED_GROUPS).
func NewSplABTTeamMembersHandler(serviceNow abtTeamMembersServiceNowClient, employeeInfo employeeInfoClient, allowedGroups []string) *SplABTTeamMembersHandler {
	return &SplABTTeamMembersHandler{serviceNow: serviceNow, employeeInfo: employeeInfo, allowedGroups: allowedGroups}
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

	membersRaw, err := h.serviceNow.TableQuery(ctx, "sys_user_grmember", url.Values{
		"sysparm_query":  {servicenow.BuildEncodedQuery("group=" + teamID)},
		"sysparm_fields": {"user.name, user.email"},
	})
	if err != nil {
		slog.ErrorContext(ctx, "servicenow TableQuery sys_user_grmember failed", "userID", user.UserID, "teamID", teamID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve ABT team members.")
		return
	}

	var members snABTTeamMembersResult
	if err := json.Unmarshal(membersRaw, &members); err != nil {
		slog.ErrorContext(ctx, "decode sys_user_grmember response failed", "userID", user.UserID, "err", err)
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	if len(members.Result) == 0 {
		writeError(w, http.StatusNotFound, ErrMsgNotFound)
		return
	}

	views := make([]SplABTTeamMemberView, 0, len(members.Result))
	for _, m := range members.Result {
		view := SplABTTeamMemberView{Name: m.UserName, Email: m.UserEmail}

		if employee, err := h.employeeInfo.GetEmployeeData(ctx, m.UserEmail); err != nil {
			slog.WarnContext(ctx, "employeeinfo GetEmployeeData failed for ABT team member; leaving thumbnail blank",
				"userID", user.UserID, "memberEmail", m.UserEmail, "err", err)
		} else {
			view.EmployeeThumbnail = employee.EmployeeThumbnail
		}

		// user.name is ServiceNow's own returned value for this row, not
		// caller-supplied input, so it is not run through SanitizeQueryValue —
		// mirroring the Ballerina source, which also concatenates it unescaped.
		roleRaw, err := h.serviceNow.TableQuery(ctx, "u_team_member_role", url.Values{
			"sysparm_query":  {servicenow.BuildEncodedQuery("u_member.name=" + m.UserName)},
			"sysparm_fields": {"u_role"},
		})
		if err != nil {
			slog.WarnContext(ctx, "servicenow TableQuery u_team_member_role failed; leaving role blank",
				"userID", user.UserID, "memberName", m.UserName, "err", err)
		} else {
			var roles snTeamMemberRolesResult
			if err := json.Unmarshal(roleRaw, &roles); err != nil {
				slog.WarnContext(ctx, "decode u_team_member_role response failed; leaving role blank",
					"userID", user.UserID, "memberName", m.UserName, "err", err)
			} else if len(roles.Result) > 0 {
				view.Role = roles.Result[0].Role
			}
		}

		views = append(views, view)
	}

	writeJSONValue(w, http.StatusOK, views)
}
