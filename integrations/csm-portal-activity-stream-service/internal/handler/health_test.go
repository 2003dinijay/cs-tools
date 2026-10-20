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
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-portal-activity-stream-service/internal/middleware"
)

// streamChain mirrors the production wrapping of the stream mux in main.go
// (Auth around the mux), with signature validation off.
func streamChain() http.Handler {
	cfg := middleware.Config{
		JWKSEndpoint:          "http://unused.example.com",
		Issuer:                "test-issuer",
		Audiences:             []string{"test-audience"},
		TokenValidatorEnabled: false,
	}
	// nil hub: stream routes answer 503 once past auth.
	return middleware.Auth(cfg)(NewStreamMux(NewStreamHandler(nil, nil)))
}

func TestStreamMux_HealthNeedsNoToken(t *testing.T) {
	w := httptest.NewRecorder()
	streamChain().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if got := strings.TrimSpace(w.Body.String()); got != `{"status":"ok"}` {
		t.Errorf("body = %q", got)
	}
}

func TestStreamMux_OtherPathsStillRequireAuth(t *testing.T) {
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/cases/activities/stream?ids=x"},
		{http.MethodGet, "/cases/abc/activities/stream"},
		{http.MethodGet, "/health/"},
		{http.MethodPost, "/health"},
	} {
		w := httptest.NewRecorder()
		streamChain().ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: status = %d, want 401", tc.method, tc.path, w.Code)
		}
	}
}
