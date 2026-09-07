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

package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/sftpgo-authentication-service/internal/config"
	"github.com/wso2-open-operations/cs-tools/integrations/sftpgo-authentication-service/internal/log"
)

func TestIsInternalUser(t *testing.T) {
	// Create a minimal service instance for testing
	s := &IdPService{
		cfg: &config.Config{InternalUserSuffix: "@wso2.com"},
	}

	tests := []struct {
		name     string
		username string
		want     bool
	}{
		{
			name:     "Internal user with @wso2.com",
			username: "john.doe@wso2.com",
			want:     true,
		},
		{
			name:     "External user with different domain",
			username: "jane.smith@example.com",
			want:     false,
		},
		{
			name:     "Empty string",
			username: "",
			want:     false,
		},
		{
			name:     "No @ symbol",
			username: "username",
			want:     false,
		},
		{
			name:     "Multiple @ symbols",
			username: "user@name@wso2.com",
			want:     true,
		},
		{
			name:     "Case sensitive - lowercase",
			username: "user@wso2.com",
			want:     true,
		},
		{
			name:     "Case sensitive - uppercase domain",
			username: "user@WSO2.COM",
			want:     false,
		},
		{
			name:     "Subdomain",
			username: "user@mail.wso2.com",
			want:     false, // HasSuffix checks exact suffix, not domain matching
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.isInternalUser(tt.username)
			if got != tt.want {
				t.Errorf("isInternalUser(%q) = %v, want %v", tt.username, got, tt.want)
			}
		})
	}
}

// TestGetAsgardeoUser_SCIMFilterEscaping proves fix #4: backslashes are
// escaped before quotes, so a username containing a literal `\"` cannot
// terminate the SCIM filter's string literal early. If the escaping order
// were reversed (quotes first, then backslashes), the attacker-supplied `\"`
// would itself be escaped into `\\"`, which closes the filter's string
// literal one character early -- this test fails loudly if that regresses.
func TestGetAsgardeoUser_SCIMFilterEscaping(t *testing.T) {
	var capturedFilter string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "test-token"})
		case "/scim2/Users":
			capturedFilter = r.URL.Query().Get("filter")
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"Resources": []any{}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := &config.Config{
		InternalUserSuffix:   "@wso2.com",
		IdPTokenEndPoint:     server.URL + "/token",
		IdPSCIMUsersEndPoint: server.URL + "/scim2/Users",
		InternalClientID:     "client",
		InternalClientSecret: "secret",
	}
	s := NewIdPService(cfg, log.NewAppLogger("ERROR"))

	// Malicious username attempting to break out of the SCIM filter's string
	// literal via an unescaped-looking `\"` sequence. Ends in the configured
	// internal suffix so it routes through the internal org endpoints set up
	// above.
	maliciousUsername := `attacker\" or userName eq "admin@wso2.com`

	// GetAsgardeoUser is expected to return an error (no matching resource),
	// but the important assertion is on the filter string it sent upstream.
	_, _ = s.GetAsgardeoUser(maliciousUsername)

	decoded, err := url.QueryUnescape(capturedFilter)
	if err != nil {
		t.Fatalf("failed to decode captured filter: %v", err)
	}

	// Backslashes must be doubled BEFORE quotes are escaped, so the filter's
	// string literal spans the entire username unbroken.
	wantUsername := `attacker\\\" or userName eq \"admin@wso2.com`
	want := `userName eq "DEFAULT/` + wantUsername + `"`
	if decoded != want {
		t.Errorf("SCIM filter = %q, want %q", decoded, want)
	}
}
