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

package plg

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	csmhandler "github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/handler"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
	plghandler "github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/handler"
)

// The four routes that author a playbook template. Everything else PLG serves
// is PermUsePlg. Kept as data rather than derived from the path, because
// "starts with /plg/playbooks" is exactly the wrong rule: GET /plg/playbooks
// and GET /plg/playbooks/{id} are reads a CS engineer must keep.
var playbookManagementRoutes = []struct{ method, path string }{
	{http.MethodPost, "/plg/products/choreo/playbooks"},
	{http.MethodPatch, "/plg/playbooks/pb-1"},
	{http.MethodPut, "/plg/playbooks/pb-1/tasks"},
	{http.MethodDelete, "/plg/playbooks/pb-1"},
}

// Every other PLG route, one per registered pattern.
var everydayRoutes = []struct{ method, path string }{
	{http.MethodGet, "/plg/me"},
	{http.MethodGet, "/plg/products"},
	{http.MethodGet, "/plg/cs-users"},
	{http.MethodGet, "/plg/lifecycle"},
	{http.MethodPost, "/plg/organizations/search"},
	{http.MethodGet, "/plg/organizations/org-1"},
	{http.MethodPatch, "/plg/organizations/org-1"},
	{http.MethodGet, "/plg/organizations/org-1/products/choreo"},
	{http.MethodPatch, "/plg/organizations/org-1/products/choreo"},
	{http.MethodPost, "/plg/organizations/org-1/products/choreo/playbook-runs"},
	{http.MethodPost, "/plg/organizations/org-1/products/choreo/notes"},
	{http.MethodDelete, "/plg/playbook-runs/run-1"},
	{http.MethodPatch, "/plg/playbook-run-tasks/task-1"},
	{http.MethodPatch, "/plg/notes/note-1"},
	{http.MethodPost, "/plg/registrations/search"},
	{http.MethodPost, "/plg/registrations/op-1/acknowledge"},
	{http.MethodGet, "/plg/playbooks"},
	{http.MethodGet, "/plg/playbooks/pb-1"},
	{http.MethodGet, "/plg/analytics/dashboard"},
	{http.MethodGet, "/plg/work-queue"},
}

// testAccessConfig mirrors the csm-portal handler package's own fixture: dummy
// role names, never the real ones, since those are organisation vocabulary that
// must not be committed.
func testAccessConfig() csmhandler.AccessConfig {
	return csmhandler.AccessConfig{
		Viewer:               []string{"test-viewer"},
		Escalator:            []string{"test-escalator"},
		AttachmentDownloader: []string{"test-attachment-downloader"},
		UsageMetricsViewer:   []string{"test-usage-metrics-viewer"},
		CsEngineer:           []string{"test-cs-engineer"},
		Admin:                []string{"test-admin"},
		TimecardApprover:     []string{"test-timecard-approver"},
		DashboardDesigner:    []string{"test-dashboard-designer"},
	}
}

// plgMux registers PLG's real route table against a stub identity middleware.
//
// The stub answers 204 INSTEAD OF calling the handler, which is what makes this
// test possible without a running entity-service: a request that reaches it has
// passed the guard, and no PLG handler ever runs. It also pins the middleware
// ORDER — the guard is outside identity, so a caller whose roles are wrong is
// rejected before the (upstream-calling) identity resolver is entered. If those
// two were ever swapped, every denied case below would come back 204.
func plgMux(t *testing.T) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	stubIdentity := func(http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})
	}
	// Handlers is never invoked — the stub above stands in for it — so the zero
	// value is enough, and avoids building five services with no upstream.
	register(mux, &plghandler.Handlers{}, stubIdentity, csmhandler.NewAccessGuard(testAccessConfig()))
	return mux
}

// statusAs issues one request as a caller whose token carries roles, and
// returns the status. The body is "{}" so a handler that reads one is not the
// thing that fails — though with the stub identity in plgMux, none ever runs.
func statusAs(t *testing.T, mux *http.ServeMux, method, path string, roles []string) int {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader("{}"))
	user := &middleware.UserInfo{Email: "staff@example.com", UserID: "sub-1", Roles: roles}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req.WithContext(middleware.WithUserInfo(req.Context(), user)))
	return w.Code
}

// The main split: a CS engineer reaches all 20 everyday routes and is refused
// on the four that author a playbook template.
func TestRoutes_CsEngineerWorksTheQueueButCannotAuthorPlaybooks(t *testing.T) {
	mux := plgMux(t)
	engineer := []string{"test-cs-engineer"}

	for _, r := range everydayRoutes {
		if got := statusAs(t, mux, r.method, r.path, engineer); got != http.StatusNoContent {
			t.Errorf("%s %s as cs engineer: status = %d, want 204", r.method, r.path, got)
		}
	}
	for _, r := range playbookManagementRoutes {
		if got := statusAs(t, mux, r.method, r.path, engineer); got != http.StatusForbidden {
			t.Errorf("%s %s as cs engineer: status = %d, want 403", r.method, r.path, got)
		}
	}
}

// Admin holds both permissions, so no PLG route is closed to it.
func TestRoutes_AdminReachesEverything(t *testing.T) {
	mux := plgMux(t)
	admin := []string{"test-admin"}

	for _, r := range append(append([]struct{ method, path string }{}, everydayRoutes...), playbookManagementRoutes...) {
		if got := statusAs(t, mux, r.method, r.path, admin); got != http.StatusNoContent {
			t.Errorf("%s %s as admin: status = %d, want 204", r.method, r.path, got)
		}
	}
}

// PLG is narrower than PermView. Every role below holds PermView and so can read
// cases and customers, but none of them may open PLG: a section where every
// control 403s is worse than one that is not offered.
func TestRoutes_ViewOnlyRolesAreShutOutEntirely(t *testing.T) {
	mux := plgMux(t)
	all := append(append([]struct{ method, path string }{}, everydayRoutes...), playbookManagementRoutes...)

	for _, role := range []string{
		"test-viewer", "test-escalator", "test-attachment-downloader",
		"test-usage-metrics-viewer", "test-timecard-approver", "test-dashboard-designer",
	} {
		for _, r := range all {
			if got := statusAs(t, mux, r.method, r.path, []string{role}); got != http.StatusForbidden {
				t.Errorf("%s %s as %s: status = %d, want 403", r.method, r.path, role, got)
			}
		}
	}
}

// A token carrying no roles at all reaches nothing — the guard denies before
// identity would have had a chance to resolve the caller.
func TestRoutes_NoRolesReachesNothing(t *testing.T) {
	mux := plgMux(t)
	all := append(append([]struct{ method, path string }{}, everydayRoutes...), playbookManagementRoutes...)

	for _, r := range all {
		if got := statusAs(t, mux, r.method, r.path, nil); got != http.StatusForbidden {
			t.Errorf("%s %s with no roles: status = %d, want 403", r.method, r.path, got)
		}
	}
}

// Reading a playbook is not managing one. Pinned separately because the obvious
// wrong implementation — gating on the /plg/playbooks path prefix — passes every
// other test in this file and breaks exactly this: an engineer who cannot list
// templates cannot choose one to run.
func TestRoutes_EngineerCanStillReadPlaybooks(t *testing.T) {
	mux := plgMux(t)
	engineer := []string{"test-cs-engineer"}

	for _, path := range []string{"/plg/playbooks", "/plg/playbooks/pb-1"} {
		if got := statusAs(t, mux, http.MethodGet, path, engineer); got != http.StatusNoContent {
			t.Errorf("GET %s as cs engineer: status = %d, want 204", path, got)
		}
	}
	// ...and assigning one to a pairing is the engineer's job, not admin's.
	if got := statusAs(t, mux, http.MethodPost, "/plg/organizations/org-1/products/choreo/playbook-runs", engineer); got != http.StatusNoContent {
		t.Errorf("attaching a playbook as cs engineer: status = %d, want 204", got)
	}
}

// The route table and this file's two lists must not drift apart. There is no
// way to enumerate a ServeMux's patterns, so this asserts the count instead —
// a new route added without a test here trips it.
func TestRoutes_EveryRouteIsCovered(t *testing.T) {
	const registered = 24
	if got := len(everydayRoutes) + len(playbookManagementRoutes); got != registered {
		t.Errorf("this file covers %d routes, register() mounts %d — add the new route to one of the two lists", got, registered)
	}
}
