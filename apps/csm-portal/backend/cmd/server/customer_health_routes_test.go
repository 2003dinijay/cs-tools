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

package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestCustomerHealthRoutePermissions pins how the Customer Health routes (and
// the two filter lists its dashboard reads) are registered in main.go, because
// the registration decides who can reach them: a route put back on
// handler.PermViewerAccess would give a cs_engineer a 403 on that part of the
// dashboard (the handler tests would still pass), and the new permission put on
// some other viewer route would widen it past the Viewer audience.
func TestCustomerHealthRoutePermissions(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}

	// route("METHOD /path", handler.PermX, ...)
	reg := regexp.MustCompile(`(?m)^\s*route\("([A-Z]+) (/[^"]*)",\s*(handler\.Perm\w+),`)
	matches := reg.FindAllStringSubmatch(string(src), -1)
	if len(matches) == 0 {
		t.Fatal("found no routes in main.go; the pattern in this test is stale")
	}

	const want = "handler.PermViewCustomerHealth"
	seen := map[string]bool{}
	for _, m := range matches {
		method, path, perm := m[1], m[2], m[3]
		name := method + " " + path
		seen[name] = true

		inAudience := strings.HasPrefix(path, "/customer-health/") ||
			(method == "GET" && (path == "/products" || path == "/abt-teams"))
		switch {
		case inAudience && perm != want:
			t.Errorf("%s is registered with %s, want %s (it is part of the Customer Health dashboard)", name, perm, want)
		case !inAudience && perm == want:
			t.Errorf("%s is registered with %s, but only the Customer Health routes and the GET /products and GET /abt-teams lists belong to that audience", name, want)
		}
	}

	for _, name := range []string{"POST /customer-health/summary", "GET /products", "GET /abt-teams"} {
		if !seen[name] {
			t.Errorf("route %q is no longer registered with route(...) in main.go; update this test if it moved", name)
		}
	}
}
