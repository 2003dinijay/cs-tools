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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/customer-portal-activity-stream-service/internal/middleware"
)

// The service has a single listener, so liveness probing hits the same
// auth-wrapped mux as the stream: /health must answer without a token while
// every other path still demands one.
func TestStreamMux_HealthIsUnauthenticatedOthersAreNot(t *testing.T) {
	streamCalled := false
	mux := newStreamMux(func(w http.ResponseWriter, r *http.Request) {
		streamCalled = true
		w.WriteHeader(http.StatusOK)
	})
	h := middleware.Auth(middleware.Config{
		JWKSEndpoint:          "http://unused.example.com",
		Issuer:                "test-issuer",
		Audiences:             []string{"test-audience"},
		TokenValidatorEnabled: false,
	})(mux)

	t.Run("health without token returns 200", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if got := strings.TrimSpace(rec.Body.String()); got != `{"status":"ok"}` {
			t.Errorf("body = %q, want {\"status\":\"ok\"}", got)
		}
	})

	t.Run("stream without token is rejected before the handler", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cases/abc/activities/stream", nil))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
		if streamCalled {
			t.Error("stream handler ran without a token")
		}
	})

	t.Run("unknown path without token is rejected", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/other", nil))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("health is GET only", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/health", nil))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
	})
}
