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

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/splauth"
)

// requireSPLGroups is the first call in every /spl/* handler: it replaces
// the plain UserInfoFromContext nil-check every other handler in this
// package starts with, additionally enforcing SupportPortalLite's blanket
// allowedGroups gate (mirrors Ballerina authJWT.imposeGlobalRules, which
// runs before every SupportPortalLite request). Returns the authenticated
// user and true on success; on failure it has already written the HTTP
// response (401 if unauthenticated, 403 if authenticated but not in
// allowedGroups) and the caller must return immediately.
func requireSPLGroups(w http.ResponseWriter, r *http.Request, allowedGroups []string) (*middleware.UserInfo, bool) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return nil, false
	}
	if !splauth.IsAuthorized(user.Groups, allowedGroups) {
		writeError(w, http.StatusForbidden, ErrMsgForbidden)
		return nil, false
	}
	return user, true
}

// requireSPLSubGroups performs one of SupportPortalLite's additional,
// narrower group checks (addWorknoteGroups, addEscalationGroups,
// downloadAttachmentGroups, usageMetricsGroups) layered on top of the
// blanket allowedGroups gate requireSPLGroups already enforced. Call this
// after requireSPLGroups, only for the handful of endpoints Ballerina's
// operations.bal gates a second time. Returns false (and has already
// written a 403) when user is not in requiredGroups.
func requireSPLSubGroups(w http.ResponseWriter, user *middleware.UserInfo, requiredGroups []string) bool {
	if !splauth.IsAuthorized(user.Groups, requiredGroups) {
		writeError(w, http.StatusForbidden, ErrMsgForbidden)
		return false
	}
	return true
}
