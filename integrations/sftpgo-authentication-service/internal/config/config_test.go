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

package config

import (
	"testing"
)

// setAllCriticalEnvVars sets every critical environment variable Load()
// requires to succeed, using t.Setenv so each is automatically restored when
// the test (or subtest) ends. Individual tests override or unset specific
// variables afterwards to exercise a single missing/invalid case at a time.
func setAllCriticalEnvVars(t *testing.T) {
	t.Helper()

	t.Setenv("INTERNAL_CLIENT_ID", "test_client_id")
	t.Setenv("INTERNAL_CLIENT_SECRET", "test_client_secret")
	t.Setenv("OAUTH_CALLBACK_URL", "http://localhost/callback")
	t.Setenv("INTERNAL_IDP_BASE_PATH", "http://idp.example.com")
	t.Setenv("SUBSCRIPTION_API", "http://sub.example.com")
	t.Setenv("PROJECT_API", "http://proj.example.com")
	// SFTPGO_API_BASE must be HTTPS (fix #7): the admin API bearer token is
	// sent on every request to it, so an HTTP endpoint would leak it.
	t.Setenv("SFTPGO_API_BASE", "https://sftpgo.example.com")
	t.Setenv("ADMIN_USER", "admin")
	t.Setenv("ADMIN_KEY", "secret")
	t.Setenv("FOLDER_PATH", "/tmp/sftpgo")
	t.Setenv("DIR_PATH", "/tmp/data")
	t.Setenv("CHECK_ROLE", "internal")
	t.Setenv("SCIM_SCOPE", "scope")
	t.Setenv("DB_CONN_STRING", "db_conn")
	// Not a secret: base64("BasicAuthenticator:LOCAL"), the standard username/password
	// authenticator identifier in the WSO2 identity platform.
	t.Setenv("BASIC_AUTHENTICATOR_ID", "QmFzaWNBdXRoZW50aWNhdG9yOkxPQ0FM")
}

func TestLoad_Success(t *testing.T) {
	setAllCriticalEnvVars(t)
	t.Setenv("HTTP_TIMEOUT", "20")
	t.Setenv("HOOK_API_KEY", "test_hook_key")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if cfg.HookAPIKey != "test_hook_key" {
		t.Errorf("Expected HookAPIKey 'test_hook_key', got '%s'", cfg.HookAPIKey)
	}

	if cfg.InternalClientID != "test_client_id" {
		t.Errorf("Expected InternalClientID 'test_client_id', got '%s'", cfg.InternalClientID)
	}
	if cfg.HTTPTimeout != 20 {
		t.Errorf("Expected HTTPTimeout 20, got %d", cfg.HTTPTimeout)
	}
	if cfg.AdminTokenEndPoint != "https://sftpgo.example.com/token" {
		t.Errorf("Expected AdminTokenEP 'https://sftpgo.example.com/token', got '%s'", cfg.AdminTokenEndPoint)
	}
}

func TestLoad_MissingCritical(t *testing.T) {
	// Only INTERNAL_CLIENT_ID is set; every other critical variable is
	// missing, so Load() must fail. t.Setenv restores the environment
	// automatically at the end of the test.
	t.Setenv("INTERNAL_CLIENT_ID", "test_client_id")

	_, err := Load()
	if err == nil {
		t.Fatal("Expected error due to missing critical variables, got nil")
	}
}

func TestLoad_Defaults(t *testing.T) {
	setAllCriticalEnvVars(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if cfg.Port != "9090" {
		t.Errorf("Expected default Port '9090', got '%s'", cfg.Port)
	}
	if cfg.HTTPTimeout != 15 {
		t.Errorf("Expected default HTTPTimeout 15, got %d", cfg.HTTPTimeout)
	}
}

func TestLoad_InvalidLogLevel(t *testing.T) {
	setAllCriticalEnvVars(t)
	t.Setenv("LOG_LEVEL", "INVALID")

	_, err := Load()
	if err == nil {
		t.Fatal("Expected error for invalid LOG_LEVEL, got nil")
	}
}

// TestLoad_RejectsNonHTTPSJWKSEndpoint proves fix #1: AUTH_JWKS_ENDPOINT must
// be HTTPS when configured (the default HTTP client follows redirects, and an
// HTTP JWKS endpoint is a man-in-the-middle risk for the keys that establish
// trust for the external-auth-hook path).
func TestLoad_RejectsNonHTTPSJWKSEndpoint(t *testing.T) {
	setAllCriticalEnvVars(t)
	t.Setenv("AUTH_JWKS_ENDPOINT", "http://idp.example.com/jwks")
	t.Setenv("AUTH_ISSUER", "https://idp.example.com/oauth2/token")
	t.Setenv("AUTH_AUDIENCE", "test-audience")

	_, err := Load()
	if err == nil {
		t.Fatal("Expected error for non-HTTPS AUTH_JWKS_ENDPOINT, got nil")
	}
}

// TestLoad_AllowsHTTPSJWKSEndpoint proves the HTTPS check does not reject a
// correctly-configured HTTPS endpoint.
func TestLoad_AllowsHTTPSJWKSEndpoint(t *testing.T) {
	setAllCriticalEnvVars(t)
	t.Setenv("AUTH_JWKS_ENDPOINT", "https://idp.example.com/jwks")
	t.Setenv("AUTH_ISSUER", "https://idp.example.com/oauth2/token")
	t.Setenv("AUTH_AUDIENCE", "test-audience")

	if _, err := Load(); err != nil {
		t.Fatalf("Expected no error for HTTPS AUTH_JWKS_ENDPOINT, got %v", err)
	}
}

// TestLoad_RejectsNonHTTPSSFTPGoAPIBase proves fix #7: SFTPGO_API_BASE must be
// HTTPS, since the admin API bearer token is sent on every request to it.
func TestLoad_RejectsNonHTTPSSFTPGoAPIBase(t *testing.T) {
	setAllCriticalEnvVars(t)
	t.Setenv("SFTPGO_API_BASE", "http://sftpgo.example.com")

	_, err := Load()
	if err == nil {
		t.Fatal("Expected error for non-HTTPS SFTPGO_API_BASE, got nil")
	}
}

// TestLoad_RejectsOpaqueHTTPSURL proves requireHTTPS rejects a host-less,
// opaque URL such as "https:sftpgo.example.com" (missing the "//" prefix).
// Go's net/url parses this as scheme "https" with no host, which would
// otherwise pass the scheme check and fail much later, and more confusingly,
// deep inside http.Client with "no Host in request URL".
func TestLoad_RejectsOpaqueHTTPSURL(t *testing.T) {
	setAllCriticalEnvVars(t)
	t.Setenv("SFTPGO_API_BASE", "https:sftpgo.example.com")

	_, err := Load()
	if err == nil {
		t.Fatal("Expected error for opaque/host-less SFTPGO_API_BASE, got nil")
	}
}

// Fix #11 (empty AUTH_AUDIENCE must not silently accept any audience) is
// enforced in service.NewJWTAuthService rather than here: like a JWKS-fetch
// failure, a misconfigured external-auth-hook must disable that hook and keep
// the pre-login/keyboard-interactive hooks running, not crash the whole
// service. See internal/service/jwtauth_test.go for that coverage.
