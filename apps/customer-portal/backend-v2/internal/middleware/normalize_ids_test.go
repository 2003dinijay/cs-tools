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

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalizePathSysIDs(t *testing.T) {
	const (
		sysid  = "0123456789abcdef0123456789abcdef"
		dashed = "01234567-89ab-cdef-0123-456789abcdef"
	)
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"bare sysid segment", "/projects/" + sysid, "/projects/" + dashed},
		{"uppercase sysid is lowercased", "/projects/0123456789ABCDEF0123456789ABCDEF", "/projects/" + dashed},
		{"several sysids", "/projects/" + sysid + "/cases/" + sysid, "/projects/" + dashed + "/cases/" + dashed},
		{"already dashed", "/projects/" + dashed, "/projects/" + dashed},
		{"31 hex chars", "/projects/0123456789abcdef0123456789abcde", "/projects/0123456789abcdef0123456789abcde"},
		{"33 hex chars", "/projects/0123456789abcdef0123456789abcdef0", "/projects/0123456789abcdef0123456789abcdef0"},
		{"32 chars with a non-hex", "/projects/0123456789abcdef0123456789abcdeg", "/projects/0123456789abcdef0123456789abcdeg"},
		{"sysid as part of a longer segment", "/x/" + sysid + ".iix", "/x/" + sysid + ".iix"},
		{"no ids", "/users/me", "/users/me"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizePathSysIDs(tc.in); got != tc.want {
				t.Errorf("normalizePathSysIDs(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizeSysIDsRoutesToDashedPathValue(t *testing.T) {
	mux := http.NewServeMux()
	var gotID string
	mux.HandleFunc("GET /projects/{id}", func(w http.ResponseWriter, r *http.Request) {
		gotID = r.PathValue("id")
	})

	req := httptest.NewRequest(http.MethodGet, "/projects/0123456789abcdef0123456789abcdef?x=1", nil)
	originalPath := req.URL.Path
	NormalizeSysIDs(mux).ServeHTTP(httptest.NewRecorder(), req)

	if want := "01234567-89ab-cdef-0123-456789abcdef"; gotID != want {
		t.Errorf("handler saw id %q, want %q", gotID, want)
	}
	if req.URL.Path != originalPath {
		t.Errorf("caller's request was mutated: path is now %q", req.URL.Path)
	}
}
