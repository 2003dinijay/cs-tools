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

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/employeeinfo"
)

// employeeInfoClient abstracts the employee-info service operations used by
// SplUserInfoHandler and SplABTTeamMembersHandler.
type employeeInfoClient interface {
	GetEmployeeData(ctx context.Context, workEmail string) (*employeeinfo.Employee, error)
}

// SplUserInfoView is the portal response for GET /spl/user-info — mirrors
// Ballerina modules/userinfo/types.bal's Employee.
type SplUserInfoView struct {
	FirstName         string  `json:"firstName"`
	LastName          string  `json:"lastName"`
	EmployeeThumbnail *string `json:"employeeThumbnail"`
}

// SplUserInfoHandler handles HTTP requests for the caller's own employee
// info, delegating to the employee-info service.
type SplUserInfoHandler struct {
	employeeInfo  employeeInfoClient
	allowedGroups []string
}

// NewSplUserInfoHandler creates a SplUserInfoHandler backed by the given
// employee-info client. allowedGroups is SupportPortalLite's blanket
// access-gate group list (SPL_ALLOWED_GROUPS).
func NewSplUserInfoHandler(employeeInfo employeeInfoClient, allowedGroups []string) *SplUserInfoHandler {
	return &SplUserInfoHandler{employeeInfo: employeeInfo, allowedGroups: allowedGroups}
}

// GetUserInfo handles GET /spl/user-info: returns the caller's own employee
// info, resolved from their JWT email — mirrors Ballerina service.bal's
// `get user\-info` resource function (userinfo:getEmployeeData(authUserCtx.email)).
func (h *SplUserInfoHandler) GetUserInfo(w http.ResponseWriter, r *http.Request) {
	user, ok := requireSPLGroups(w, r, h.allowedGroups)
	if !ok {
		return
	}

	employee, err := h.employeeInfo.GetEmployeeData(r.Context(), user.Email)
	if err != nil {
		slog.ErrorContext(r.Context(), "employeeinfo GetEmployeeData failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve user info.")
		return
	}

	writeJSONValue(w, http.StatusOK, SplUserInfoView{
		FirstName:         employee.FirstName,
		LastName:          employee.LastName,
		EmployeeThumbnail: employee.EmployeeThumbnail,
	})
}
