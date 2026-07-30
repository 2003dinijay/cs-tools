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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package server

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestEveryRegisteredRouteIsPublished guards against a failure mode that is
// invisible locally: a route registered here but absent from the published
// openapi.yaml works in every local and in-cluster test, then 404s in any
// deployment where a gateway routes only the operations the contract declares.
// The gateway rejects the request before this service ever sees it, so no
// amount of service-side testing catches it.
//
// Comparison is on the method plus the path shape with parameter names
// normalized away, because the contract and the router are free to name a path
// parameter differently ({id} vs {caseId}) without being in disagreement.
func TestEveryRegisteredRouteIsPublished(t *testing.T) {
	registered := registeredRoutes(t)
	if len(registered) == 0 {
		t.Fatal("parsed no routes from routes.go; the parser is broken, not the routes")
	}
	published := publishedOperations(t)
	if len(published) == 0 {
		t.Fatal("parsed no operations from openapi.yaml; the parser is broken, not the contract")
	}

	// Not part of the public contract: the health probe is consumed by the
	// platform, not by API callers, and is deliberately unpublished.
	skip := map[string]bool{"GET /health": true}

	var missing []string
	for _, r := range registered {
		if skip[r] {
			continue
		}
		if !published[r] {
			missing = append(missing, r)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("routes registered in routes.go but not declared in openapi.yaml:\n  %s\n\n"+
			"An undeclared operation is unreachable behind an API gateway even though it works locally. "+
			"Add it to openapi.yaml (and set its resource scopes when publishing).",
			strings.Join(missing, "\n  "))
	}
}

var (
	handleFuncRE = regexp.MustCompile(`mux\.HandleFunc\("([A-Z]+) (/[^"]*)"`)
	pathParamRE  = regexp.MustCompile(`\{[^}]*\}`)
)

// normalizeRoute renders a method and path as a comparable key with path
// parameter names collapsed, e.g. `POST /cases/{id}/tags` -> `POST /cases/{}/tags`.
func normalizeRoute(method, path string) string {
	return strings.ToUpper(method) + " " + pathParamRE.ReplaceAllString(strings.TrimSuffix(path, "/"), "{}")
}

// registeredRoutes extracts every route literal registered in routes.go.
// Reading the source rather than the built mux avoids having to construct the
// full handler dependency graph, and every registration in this package is a
// string literal (a non-literal pattern would simply not be seen, which the
// empty-result guard in the test catches if it ever becomes the norm).
func registeredRoutes(t *testing.T) []string {
	t.Helper()
	src, err := os.ReadFile("routes.go")
	if err != nil {
		t.Fatalf("read routes.go: %v", err)
	}
	seen := map[string]bool{}
	var out []string
	for _, m := range handleFuncRE.FindAllStringSubmatch(string(src), -1) {
		key := normalizeRoute(m[1], m[2])
		if !seen[key] {
			seen[key] = true
			out = append(out, key)
		}
	}
	return out
}

var (
	// A path key under `paths:`, at two-space indentation, e.g. `  /cases/{id}:`.
	specPathRE = regexp.MustCompile(`^  (/\S*):\s*$`)
	// An HTTP method under a path, at four-space indentation.
	specMethodRE = regexp.MustCompile(`^    (get|put|post|delete|patch|options|head|trace):\s*$`)
)

// publishedOperations extracts every method+path declared in the published
// contract. The spec is parsed line-wise on indentation rather than with a YAML
// library so that this check adds no dependency; the test's empty-result guard
// fails loudly if the spec's formatting ever moves out from under it.
func publishedOperations(t *testing.T) map[string]bool {
	t.Helper()
	specPath := filepath.Join("..", "..", "openapi.yaml")
	src, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read %s: %v", specPath, err)
	}

	ops := map[string]bool{}
	inPaths := false
	current := ""
	for _, line := range strings.Split(string(src), "\n") {
		if strings.HasPrefix(line, "paths:") {
			inPaths = true
			continue
		}
		if !inPaths {
			continue
		}
		// A new top-level key ends the paths section.
		if len(line) > 0 && line[0] != ' ' && line[0] != '#' {
			if !strings.HasPrefix(line, "paths:") {
				inPaths = false
			}
			continue
		}
		if m := specPathRE.FindStringSubmatch(line); m != nil {
			current = m[1]
			continue
		}
		if current == "" {
			continue
		}
		if m := specMethodRE.FindStringSubmatch(line); m != nil {
			ops[normalizeRoute(m[1], current)] = true
		}
	}
	return ops
}
