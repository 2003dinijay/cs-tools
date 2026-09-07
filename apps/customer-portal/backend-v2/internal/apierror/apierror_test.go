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

package apierror

import (
	"net/http"
	"testing"
)

func TestNewUpstreamError_ExtractsMessageField(t *testing.T) {
	raw := []byte(`{"code":400,"message":"caseTypes must be valid UUIDs"}`)

	err := NewUpstreamError(http.StatusBadRequest, raw)

	if err.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", err.StatusCode)
	}
	if err.Body != "caseTypes must be valid UUIDs" {
		t.Fatalf("expected extracted message, got %q", err.Body)
	}
}

// TestNewUpstreamError_LeavesBodyEmptyWhenNotJSON guards against ever
// logging or returning to the frontend a raw, unbounded upstream response
// body (e.g. a gateway error page) — Body must stay empty so callers'
// existing "empty Body means no specific message" fallback kicks in,
// instead of surfacing arbitrary upstream content.
func TestNewUpstreamError_LeavesBodyEmptyWhenNotJSON(t *testing.T) {
	raw := []byte("<html>502 Bad Gateway</html>")

	err := NewUpstreamError(http.StatusBadGateway, raw)

	if err.Body != "" {
		t.Fatalf("expected empty Body for a non-JSON response, got %q", err.Body)
	}
}

func TestNewUpstreamError_LeavesBodyEmptyWhenMessageFieldMissing(t *testing.T) {
	raw := []byte(`{"code":500}`)

	err := NewUpstreamError(http.StatusInternalServerError, raw)

	if err.Body != "" {
		t.Fatalf("expected empty Body when message field is absent, got %q", err.Body)
	}
}
