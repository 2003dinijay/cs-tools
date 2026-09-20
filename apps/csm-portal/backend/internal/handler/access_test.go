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
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// serveWithRoles runs one request through a guard-wrapped handler as a caller
// whose token carries roles, and reports the status plus whether the wrapped
// handler ran.
func serveWithRoles(g *AccessGuard, perm Permission, roles []string) (status int, reached bool) {
	h := g.Require(perm, func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusNoContent)
	})
	user := &middleware.UserInfo{Email: "staff@example.com", UserID: "user-1", Roles: roles}
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()
	h(w, req.WithContext(middleware.WithUserInfo(req.Context(), user)))
	return w.Code, reached
}

func TestAccessGuard_PermissionMatrix(t *testing.T) {
	all := []Permission{PermView, PermViewOperations, PermComment, PermEscalate, PermDownloadAttachment, PermWrite}
	tests := []struct {
		name  string
		roles []string
		allow []Permission
	}{
		{"viewer reads only", []string{"app-csm-viewer-role"}, []Permission{PermView}},
		{"commenter can view and comment", []string{"app-csm-commenter-role"}, []Permission{PermView, PermComment}},
		{"escalator can view and escalate", []string{"app-csm-escalator-role"}, []Permission{PermView, PermEscalate}},
		{"downloader can view and download", []string{"app-csm-attachment-downloader-role"}, []Permission{PermView, PermDownloadAttachment}},
		{"support engineer can do every route permission", []string{"app-csm-support-engineer-role"}, all},
		{"admin can do every route permission", []string{"app-csm-admin-role"}, all},
		{"usage metrics viewer can view only", []string{"app-csm-usage-metrics-viewer-role"}, []Permission{PermView}},
		{"timecard approver can view only", []string{"app-csm-timecard-approver-role"}, []Permission{PermView}},
		{"dashboard designer can view only", []string{"app-csm-dashboard-designer-role"}, []Permission{PermView}},
		{"roles combine", []string{"app-csm-viewer-role", "app-csm-commenter-role", "app-csm-escalator-role"}, []Permission{PermView, PermComment, PermEscalate}},
		{"unrelated roles grant nothing", []string{"wso2-everyone", "admin", "agent", "customer"}, nil},
		{"no roles", nil, nil},
		{"role names are case sensitive", []string{"APP-CSM-ADMIN-ROLE"}, nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewAccessGuard(DefaultAccessConfig())
			for _, perm := range all {
				status, reached := serveWithRoles(g, perm, tc.roles)
				want := slices.Contains(tc.allow, perm)
				wantStatus := http.StatusForbidden
				if want {
					wantStatus = http.StatusNoContent
				}
				if status != wantStatus || reached != want {
					t.Errorf("permission %d: status = %d reached = %v, want status %d reached %v", perm, status, reached, wantStatus, want)
				}
			}
		})
	}
}

func TestAccessGuard_UsesConfiguredRoleNames(t *testing.T) {
	cfg := DefaultAccessConfig()
	cfg.Commenter = []string{"corp-support-notes", "corp-interns"}
	g := NewAccessGuard(cfg)

	for _, role := range []string{"corp-support-notes", "corp-interns"} {
		if status, _ := serveWithRoles(g, PermComment, []string{role}); status != http.StatusNoContent {
			t.Errorf("configured role %q: status = %d, want 204", role, status)
		}
	}
	if status, _ := serveWithRoles(g, PermComment, []string{"app-csm-commenter-role"}); status != http.StatusForbidden {
		t.Errorf("default name after override: status = %d, want 403 (the configured name replaces the default)", status)
	}
}

func TestAccessGuard_AuthenticatedNeedsNoRole(t *testing.T) {
	status, reached := serveWithRoles(NewAccessGuard(DefaultAccessConfig()), PermAuthenticated, nil)
	if status != http.StatusNoContent || !reached {
		t.Errorf("status = %d reached = %v, want 204 and reached", status, reached)
	}
}

func TestAccessGuard_NoUserIs401(t *testing.T) {
	reached := false
	h := NewAccessGuard(DefaultAccessConfig()).Require(PermAuthenticated, func(http.ResponseWriter, *http.Request) { reached = true })
	w := httptest.NewRecorder()
	h(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	assertStatus(t, w, http.StatusUnauthorized)
	if reached {
		t.Error("handler ran for a request with no authenticated user")
	}
}

func TestAccessGuard_UnknownPermissionIsDenied(t *testing.T) {
	status, reached := serveWithRoles(NewAccessGuard(DefaultAccessConfig()), Permission(999), []string{"app-csm-admin-role"})
	if status != http.StatusForbidden || reached {
		t.Errorf("status = %d reached = %v, want 403 and not reached", status, reached)
	}
}

func TestAccessGuard_RolesFor(t *testing.T) {
	g := NewAccessGuard(DefaultAccessConfig())
	tests := []struct {
		name string
		held []string
		want []string
	}{
		{"none", nil, []string{}},
		{"one role", []string{"app-csm-viewer-role"}, []string{"viewer"}},
		{"several roles come back in a fixed order", []string{"app-csm-admin-role", "app-csm-commenter-role", "app-csm-viewer-role"}, []string{"viewer", "commenter", "admin"}},
		{"support engineer", []string{"app-csm-support-engineer-role"}, []string{"support_engineer"}},
		{"every role", []string{
			"app-csm-viewer-role", "app-csm-commenter-role", "app-csm-escalator-role", "app-csm-attachment-downloader-role",
			"app-csm-support-engineer-role", "app-csm-usage-metrics-viewer-role", "app-csm-timecard-approver-role",
			"app-csm-dashboard-designer-role", "app-csm-admin-role",
		}, []string{"viewer", "commenter", "escalator", "attachment_downloader", "support_engineer", "usage_metrics_viewer", "timecard_approver", "dashboard_designer", "admin"}},
		{"unrelated roles are ignored", []string{"wso2-everyone", "agent"}, []string{}},
		{"a duplicated held role is reported once", []string{"app-csm-viewer-role", "app-csm-viewer-role"}, []string{"viewer"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := g.RolesFor(tc.held)
			if got == nil || !slices.Equal(got, tc.want) {
				t.Errorf("RolesFor = %#v, want %#v (non-nil)", got, tc.want)
			}
		})
	}
}

func TestAccessGuard_RolesForUsesConfiguredNames(t *testing.T) {
	cfg := DefaultAccessConfig()
	cfg.SupportEngineer = []string{"corp-se-a", "corp-se-b"}
	g := NewAccessGuard(cfg)
	if got := g.RolesFor([]string{"corp-se-b"}); !slices.Equal(got, []string{"support_engineer"}) {
		t.Errorf("RolesFor = %v, want [support_engineer]", got)
	}
	if got := g.RolesFor([]string{"app-csm-support-engineer-role"}); len(got) != 0 {
		t.Errorf("RolesFor = %v, want none: the configured names replace the default", got)
	}
}

func TestAccessGuard_OperationsAreForSupportEngineersAndAdmins(t *testing.T) {
	g := NewAccessGuard(DefaultAccessConfig())
	for _, role := range []string{
		"app-csm-viewer-role", "app-csm-commenter-role", "app-csm-escalator-role",
		"app-csm-attachment-downloader-role", "app-csm-usage-metrics-viewer-role",
		"app-csm-timecard-approver-role", "app-csm-dashboard-designer-role",
	} {
		if status, _ := serveWithRoles(g, PermViewOperations, []string{role}); status != http.StatusForbidden {
			t.Errorf("%s reading operations: status = %d, want 403", role, status)
		}
		if status, _ := serveWithRoles(g, PermView, []string{role}); status != http.StatusNoContent {
			t.Errorf("%s reading cases and customers: status = %d, want 204", role, status)
		}
	}
	for _, role := range []string{"app-csm-support-engineer-role", "app-csm-admin-role"} {
		if status, _ := serveWithRoles(g, PermViewOperations, []string{role}); status != http.StatusNoContent {
			t.Errorf("%s reading operations: status = %d, want 204", role, status)
		}
	}
}
