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
	"reflect"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// httpHandlerMethods returns every exported method of h with the plain
// http handler signature, by name. Reading them by reflection means a handler
// added to the Customer Health domain later is covered without touching the
// tests below.
func httpHandlerMethods(h any) map[string]http.HandlerFunc {
	out := map[string]http.HandlerFunc{}
	v := reflect.ValueOf(h)
	for i := 0; i < v.NumMethod(); i++ {
		if fn, ok := v.Method(i).Interface().(func(http.ResponseWriter, *http.Request)); ok {
			out[v.Type().Method(i).Name] = fn
		}
	}
	return out
}

// deniedForRoles runs fn as a caller whose token carries roles and reports
// whether the handler's own access check rejected it (401/403). A handler that
// gets past the check may then touch the empty fakes these tests use and
// panic; that still means access was granted, so the panic is swallowed.
func deniedForRoles(fn http.HandlerFunc, roles []string) bool {
	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader("{}"))
	req = req.WithContext(middleware.WithUserInfo(req.Context(),
		&middleware.UserInfo{Email: "staff@example.com", UserID: "u-staff", Roles: roles}))
	w := httptest.NewRecorder()
	func() {
		defer func() { _ = recover() }()
		fn(w, req)
	}()
	return w.Code == http.StatusForbidden || w.Code == http.StatusUnauthorized
}

// TestCustomerHealthHandlers_EveryHandlerChecksCustomerHealthAccess pins the
// in-handler check of EVERY Customer Health handler and of the two filter lists
// (the route registration is pinned separately in cmd/server): Viewer,
// cs_engineer and admin get past it, every other configured role is rejected.
// A handler reverted to the Viewer-only check would fail the cs_engineer case.
func TestCustomerHealthHandlers_EveryHandlerChecksCustomerHealthAccess(t *testing.T) {
	health := httpHandlerMethods(NewCustomerHealthHandler(&fakeRiskClient{}, &fakeSNCustomerHealthClient{}, &fakeHealthAccountSearch{}, viewerAccessGuard))
	lookups := httpHandlerMethods(NewLookupsHandler(&mockLookupsClient{}, viewerAccessGuard))

	// 17 routes today; a lower count means the reflection above stopped finding
	// the handlers, not that the domain shrank.
	if len(health) < 17 {
		t.Fatalf("found %d Customer Health handlers, want at least 17", len(health))
	}
	if len(lookups) != 2 {
		t.Fatalf("found %d lookup handlers, want 2 (GetProducts, GetABTTeams)", len(lookups))
	}

	all := map[string]http.HandlerFunc{}
	for name, fn := range health {
		all["CustomerHealthHandler."+name] = fn
	}
	for name, fn := range lookups {
		all["LookupsHandler."+name] = fn
	}

	for name, fn := range all {
		for _, role := range []string{"test-viewer", "test-cs-engineer", "test-admin"} {
			if deniedForRoles(fn, []string{role}) {
				t.Errorf("%s rejected %s, want access", name, role)
			}
		}
		for _, role := range rolesOutsideCustomerHealthAudience() {
			if !deniedForRoles(fn, []string{role}) {
				t.Errorf("%s let %s through, want 403", name, role)
			}
		}
		if !deniedForRoles(fn, nil) {
			t.Errorf("%s let a caller with no roles through, want 403", name)
		}
	}
}
