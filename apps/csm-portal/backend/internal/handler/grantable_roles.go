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
)

// GrantableRole is one portal role an admin may grant to a new user via
// POST /users, resolved to a specific, real role ID on the identity
// provider.
type GrantableRole struct {
	// Key is the stable portal role key -- the same vocabulary AccessGuard's
	// own portalRoles list uses ("cs_engineer", "escalator", ...). This is
	// what GET /roles/grantable reports and what POST /users' own grantRoles
	// field accepts; the real role name/ID behind it is never exposed to a
	// caller.
	Key string
	// RoleID is the real role's ID on the identity provider, resolved by
	// matching AUTH_<ROLE>_ROLES' configured real role name(s) for this
	// portal role against the configured role-ID mapping.
	RoleID string
}

// ResolveGrantableRoles cross-references cfg (AUTH_<ROLE>_ROLES' parsed real
// role names, one list per portal role) against idsByRoleName (the
// configured role-ID mapping, keyed by that same real role name -- see
// directory.ParseRoleIDs) to find which portal roles have at least one real
// role name with a known ID. Only these can actually be granted via SCIM; a
// portal role with no configured real-name match is simply absent from the
// result, the same "no entry means not wired up here" posture the role-ID
// mapping itself documents.
//
// A portal role whose AUTH_<ROLE>_ROLES lists several real names uses
// whichever is found first in cfg's own field order -- an edge case worth a
// deployment keeping to one real role per permission in practice, since
// granting "the escalator role" can only ever mean one specific role, not a
// choice made per call.
func ResolveGrantableRoles(cfg AccessConfig, idsByRoleName map[string]string) []GrantableRole {
	candidates := []struct {
		key   string
		names []string
	}{
		{"viewer", cfg.Viewer},
		{"escalator", cfg.Escalator},
		{"attachment_downloader", cfg.AttachmentDownloader},
		{"usage_metrics_viewer", cfg.UsageMetricsViewer},
		{"cs_engineer", cfg.CsEngineer},
		{"admin", cfg.Admin},
		{"timecard_approver", cfg.TimecardApprover},
		{"dashboard_designer", cfg.DashboardDesigner},
		{"sales_solutions", cfg.SalesSolutions},
		{"worknote_creator", cfg.WorknoteCreator},
	}

	resolved := make([]GrantableRole, 0, len(candidates))
	for _, c := range candidates {
		for _, name := range c.names {
			if id, ok := idsByRoleName[name]; ok {
				resolved = append(resolved, GrantableRole{Key: c.key, RoleID: id})
				break
			}
		}
	}
	return resolved
}

// GrantableRolesHandler serves GET /roles/grantable: the portal role keys an
// admin may grant to a new user via POST /users' own grantRoles field,
// resolved once at startup (see ResolveGrantableRoles) and never recomputed
// per request. The real role name/ID behind each key is deployment
// configuration and is never exposed to a caller.
type GrantableRolesHandler struct {
	roles []GrantableRole
}

// NewGrantableRolesHandler creates a GrantableRolesHandler serving exactly
// the resolved roles given -- computed once in cmd/server/main.go via
// ResolveGrantableRoles.
func NewGrantableRolesHandler(roles []GrantableRole) *GrantableRolesHandler {
	return &GrantableRolesHandler{roles: roles}
}

type grantableRoleRef struct {
	Key string `json:"key"`
}

type grantableRolesResponse struct {
	Roles []grantableRoleRef `json:"roles"`
}

// GetGrantableRoles handles GET /roles/grantable. Restricted to admin via
// the route's PermAdmin permission (cmd/server/main.go), the same gate
// POST /users itself already sits behind — this handler only checks that
// the caller is authenticated at all, it does not re-check the caller's
// role. Keeping both endpoints behind the identical permission is
// deliberate: the role list this reports is meaningful only to the same
// admin-only Add User flow that can actually grant one.
func (h *GrantableRolesHandler) GetGrantableRoles(w http.ResponseWriter, r *http.Request) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	refs := make([]grantableRoleRef, 0, len(h.roles))
	for _, role := range h.roles {
		refs = append(refs, grantableRoleRef{Key: role.Key})
	}
	writeJSONValue(w, http.StatusOK, grantableRolesResponse{Roles: refs})
}

// RoleIDForKey returns the real role ID for the given portal role key, and
// whether one was configured -- used by UsersHandler.CreateUser to resolve
// each requested grantRoles entry, and by cmd/server/main.go to derive
// single-role config (e.g. GetTimeCardApprovers' own role ID) from the same
// resolved list rather than keeping a second, parallel lookup.
func RoleIDForKey(roles []GrantableRole, key string) (string, bool) {
	for _, role := range roles {
		if role.Key == key {
			return role.RoleID, true
		}
	}
	return "", false
}
