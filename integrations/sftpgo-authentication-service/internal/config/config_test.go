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
	"os"
	"strings"
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
	t.Setenv("HOOK_API_KEY", "test_hook_key")
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

// Variable sets per flow, mirroring the README's per-flow table.
var (
	alwaysVars = map[string]string{
		"HOOK_API_KEY": "test_hook_key",
		"DIR_PATH":     "/tmp/data",
	}
	webVars = map[string]string{
		"AUTH_JWKS_ENDPOINT":   "https://idp.example.com/jwks",
		"AUTH_ISSUER":          "https://idp.example.com/oauth2/token",
		"ATTACHMENTS_DIR_PATH": "/tmp/attachments",
	}
	// webAudience is required in strict web-attachment mode only.
	webAudience = map[string]string{"AUTH_AUDIENCE": "aud-1"}
	sftpVars    = map[string]string{
		"BASIC_AUTHENTICATOR_ID": "QmFzaWNBdXRoZW50aWNhdG9yOkxPQ0FM",
		"INTERNAL_CLIENT_ID":     "test_client_id",
		"INTERNAL_CLIENT_SECRET": "test_client_secret",
		"OAUTH_CALLBACK_URL":     "http://localhost/callback",
		"INTERNAL_IDP_BASE_PATH": "http://idp.example.com",
		"SCIM_SCOPE":             "scope",
		"CHECK_ROLE":             "internal",
		"SFTPGO_API_BASE":        "https://sftpgo.example.com",
		"ADMIN_USER":             "admin",
		"ADMIN_KEY":              "secret",
		"FOLDER_PATH":            "/tmp/sftpgo",
		"SUBSCRIPTION_API":       "http://sub.example.com",
		"PROJECT_API":            "http://proj.example.com",
		"DB_CONN_STRING":         "db_conn",
	}
)

// clearFlowEnv unsets every variable the flow tests may set, so a test starts
// from an empty environment regardless of the host.
func clearFlowEnv(t *testing.T) {
	t.Helper()
	for _, m := range []map[string]string{alwaysVars, webVars, sftpVars} {
		for k := range m {
			t.Setenv(k, "")
			os.Unsetenv(k)
		}
	}
	t.Setenv("ENABLED_FLOWS", "")
	os.Unsetenv("ENABLED_FLOWS")
	t.Setenv("AUTH_AUDIENCE", "")
	os.Unsetenv("AUTH_AUDIENCE")
}

func setVars(t *testing.T, sets ...map[string]string) {
	t.Helper()
	for _, m := range sets {
		for k, v := range m {
			t.Setenv(k, v)
		}
	}
}

func unset(t *testing.T, name string) {
	t.Helper()
	t.Setenv(name, "")
	os.Unsetenv(name)
}

func TestLoad_EnabledFlows_UnsetEnablesBoth(t *testing.T) {
	clearFlowEnv(t)
	setVars(t, alwaysVars, sftpVars)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !cfg.WebAttachmentEnabled || !cfg.SFTPChannelEnabled || cfg.FlowsExplicit {
		t.Errorf("expected both flows enabled and not explicit, got web=%t sftp=%t explicit=%t",
			cfg.WebAttachmentEnabled, cfg.SFTPChannelEnabled, cfg.FlowsExplicit)
	}
}

func TestLoad_EnabledFlows_EmptyStringEnablesBoth(t *testing.T) {
	clearFlowEnv(t)
	setVars(t, alwaysVars, sftpVars)
	t.Setenv("ENABLED_FLOWS", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !cfg.WebAttachmentEnabled || !cfg.SFTPChannelEnabled || cfg.FlowsExplicit {
		t.Errorf("expected both flows enabled and not explicit, got web=%t sftp=%t explicit=%t",
			cfg.WebAttachmentEnabled, cfg.SFTPChannelEnabled, cfg.FlowsExplicit)
	}
}

// Backward compatibility: with ENABLED_FLOWS unset, the web-attachment
// variables stay optional.
func TestLoad_EnabledFlows_UnsetKeepsWebVarsOptional(t *testing.T) {
	clearFlowEnv(t)
	setVars(t, alwaysVars, sftpVars) // no JWKS, issuer or attachments dir

	if _, err := Load(); err != nil {
		t.Fatalf("expected Load to succeed without the web-attachment variables when ENABLED_FLOWS is unset, got %v", err)
	}
}

// Backward compatibility: with ENABLED_FLOWS unset, the sftp-channel variables
// stay required.
func TestLoad_EnabledFlows_UnsetStillRequiresSFTPVars(t *testing.T) {
	clearFlowEnv(t)
	setVars(t, alwaysVars)

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing sftp-channel variables, got nil")
	}
	if !strings.Contains(err.Error(), "DB_CONN_STRING") {
		t.Errorf("expected error to name DB_CONN_STRING, got %v", err)
	}
}

func TestLoad_EnabledFlows_WebOnlyNeedsOnlyItsVars(t *testing.T) {
	clearFlowEnv(t)
	setVars(t, alwaysVars, webVars, webAudience) // no DB, IdP, admin or subscription variables
	t.Setenv("ENABLED_FLOWS", "web-attachment")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !cfg.WebAttachmentEnabled || cfg.SFTPChannelEnabled || !cfg.FlowsExplicit {
		t.Errorf("unexpected flags: web=%t sftp=%t explicit=%t", cfg.WebAttachmentEnabled, cfg.SFTPChannelEnabled, cfg.FlowsExplicit)
	}
}

func TestLoad_EnabledFlows_WebOnlyRequiresEachWebVar(t *testing.T) {
	for name := range webVars {
		t.Run(name, func(t *testing.T) {
			clearFlowEnv(t)
			setVars(t, alwaysVars, webVars, webAudience)
			t.Setenv("ENABLED_FLOWS", "web-attachment")
			unset(t, name)

			_, err := Load()
			if err == nil {
				t.Fatalf("expected error when %s is missing, got nil", name)
			}
			if !strings.Contains(err.Error(), "missing critical environment variables") ||
				!strings.Contains(err.Error(), name) || !strings.Contains(err.Error(), "web-attachment") {
				t.Errorf("expected error naming %s and the web-attachment flow, got %v", name, err)
			}
		})
	}
}

// Strict case: naming the web-attachment flow explicitly makes its variables required.
func TestLoad_EnabledFlows_ExplicitWebWithoutJWKSFails(t *testing.T) {
	clearFlowEnv(t)
	setVars(t, alwaysVars, sftpVars)
	t.Setenv("ENABLED_FLOWS", "web-attachment,sftp-channel")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing AUTH_JWKS_ENDPOINT, got nil")
	}
	if !strings.Contains(err.Error(), "AUTH_JWKS_ENDPOINT") {
		t.Errorf("expected error to name AUTH_JWKS_ENDPOINT, got %v", err)
	}
}

func TestLoad_EnabledFlows_SFTPOnlyNeedsOnlyItsVars(t *testing.T) {
	clearFlowEnv(t)
	setVars(t, alwaysVars, sftpVars) // no JWKS, issuer or attachments dir
	t.Setenv("ENABLED_FLOWS", "sftp-channel")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.WebAttachmentEnabled || !cfg.SFTPChannelEnabled {
		t.Errorf("unexpected flags: web=%t sftp=%t", cfg.WebAttachmentEnabled, cfg.SFTPChannelEnabled)
	}
}

func TestLoad_EnabledFlows_SFTPOnlyRequiresEachSFTPVar(t *testing.T) {
	for name := range sftpVars {
		t.Run(name, func(t *testing.T) {
			clearFlowEnv(t)
			setVars(t, alwaysVars, sftpVars)
			t.Setenv("ENABLED_FLOWS", "sftp-channel")
			unset(t, name)

			_, err := Load()
			if err == nil {
				t.Fatalf("expected error when %s is missing, got nil", name)
			}
			if !strings.Contains(err.Error(), name) || !strings.Contains(err.Error(), "sftp-channel") {
				t.Errorf("expected error naming %s and the sftp-channel flow, got %v", name, err)
			}
		})
	}
}

func TestLoad_EnabledFlows_BothExplicit(t *testing.T) {
	clearFlowEnv(t)
	setVars(t, alwaysVars, webVars, sftpVars, webAudience)
	t.Setenv("ENABLED_FLOWS", "web-attachment,sftp-channel")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !cfg.WebAttachmentEnabled || !cfg.SFTPChannelEnabled || !cfg.FlowsExplicit {
		t.Errorf("unexpected flags: web=%t sftp=%t explicit=%t", cfg.WebAttachmentEnabled, cfg.SFTPChannelEnabled, cfg.FlowsExplicit)
	}
}

func TestLoad_EnabledFlows_WhitespaceAndDuplicates(t *testing.T) {
	clearFlowEnv(t)
	setVars(t, alwaysVars, webVars, webAudience)
	t.Setenv("ENABLED_FLOWS", " web-attachment , ,web-attachment,")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !cfg.WebAttachmentEnabled || cfg.SFTPChannelEnabled {
		t.Errorf("unexpected flags: web=%t sftp=%t", cfg.WebAttachmentEnabled, cfg.SFTPChannelEnabled)
	}
}

func TestLoad_EnabledFlows_RejectsBadValues(t *testing.T) {
	cases := map[string]string{
		"unknown name":         "bogus",
		"unknown among valid":  "web-attachment,bogus",
		"case sensitive":       "Web-Attachment",
		"only separators":      ",",
		"only whitespace":      "   ",
		"separators and space": " , ,",
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			clearFlowEnv(t)
			setVars(t, alwaysVars, webVars, sftpVars, webAudience)
			t.Setenv("ENABLED_FLOWS", value)

			_, err := Load()
			if err == nil {
				t.Fatalf("expected error for ENABLED_FLOWS=%q, got nil", value)
			}
			if !strings.Contains(err.Error(), "ENABLED_FLOWS") ||
				!strings.Contains(err.Error(), "web-attachment") || !strings.Contains(err.Error(), "sftp-channel") {
				t.Errorf("expected error naming ENABLED_FLOWS and the allowed values, got %v", err)
			}
		})
	}
}

func TestLoad_EnabledFlows_UnknownNameIsNamedInError(t *testing.T) {
	clearFlowEnv(t)
	setVars(t, alwaysVars, webVars, sftpVars, webAudience)
	t.Setenv("ENABLED_FLOWS", "web-attachment, bogus")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), `"bogus"`) {
		t.Fatalf("expected error naming the bad value, got %v", err)
	}
}

// HOOK_API_KEY and DIR_PATH are required in every configuration.
func TestLoad_AlwaysRequiredVarsFailEveryConfiguration(t *testing.T) {
	configs := map[string]string{
		"unset":          "",
		"web-attachment": "web-attachment",
		"sftp-channel":   "sftp-channel",
		"both":           "web-attachment,sftp-channel",
	}
	for cfgName, flows := range configs {
		for varName := range alwaysVars {
			t.Run(cfgName+"/"+varName, func(t *testing.T) {
				clearFlowEnv(t)
				setVars(t, alwaysVars, webVars, sftpVars, webAudience)
				if flows != "" {
					t.Setenv("ENABLED_FLOWS", flows)
				}
				unset(t, varName)

				_, err := Load()
				if err == nil {
					t.Fatalf("expected error when %s is missing, got nil", varName)
				}
				if !strings.Contains(err.Error(), varName) {
					t.Errorf("expected error naming %s, got %v", varName, err)
				}
			})
		}
	}
}

func TestLoad_EnabledFlows_WebOnlyEnforcesHTTPSJWKS(t *testing.T) {
	clearFlowEnv(t)
	setVars(t, alwaysVars, webVars, webAudience)
	t.Setenv("ENABLED_FLOWS", "web-attachment")
	t.Setenv("AUTH_JWKS_ENDPOINT", "http://idp.example.com/jwks")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for non-HTTPS AUTH_JWKS_ENDPOINT, got nil")
	}
}

func TestLoad_EnabledFlows_SFTPOnlyEnforcesHTTPSAdminAPI(t *testing.T) {
	clearFlowEnv(t)
	setVars(t, alwaysVars, sftpVars)
	t.Setenv("ENABLED_FLOWS", "sftp-channel")
	t.Setenv("SFTPGO_API_BASE", "http://sftpgo.example.com")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for non-HTTPS SFTPGO_API_BASE, got nil")
	}
}

func TestLoad_EnabledFlows_ExplicitWebRequiresAudience(t *testing.T) {
	for _, flows := range []string{"web-attachment", "web-attachment,sftp-channel"} {
		t.Run(flows, func(t *testing.T) {
			clearFlowEnv(t)
			setVars(t, alwaysVars, webVars, sftpVars)
			t.Setenv("ENABLED_FLOWS", flows)

			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), "AUTH_AUDIENCE (required by the web-attachment flow)") {
				t.Fatalf("expected error naming AUTH_AUDIENCE, got %v", err)
			}

			t.Setenv("AUTH_AUDIENCE", " , ")
			if _, err := Load(); err == nil {
				t.Fatal("expected error for an AUTH_AUDIENCE with no value, got nil")
			}
		})
	}
}

func TestLoad_EnabledFlows_SFTPOnlyDoesNotRequireAudience(t *testing.T) {
	clearFlowEnv(t)
	setVars(t, alwaysVars, sftpVars)
	t.Setenv("ENABLED_FLOWS", "sftp-channel")

	if _, err := Load(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

// Backward compatibility: with ENABLED_FLOWS unset, AUTH_AUDIENCE stays optional
// even when the JWKS and issuer are set.
func TestLoad_EnabledFlows_UnsetKeepsAudienceOptional(t *testing.T) {
	clearFlowEnv(t)
	setVars(t, alwaysVars, webVars, sftpVars)

	if _, err := Load(); err != nil {
		t.Fatalf("expected no error without AUTH_AUDIENCE when ENABLED_FLOWS is unset, got %v", err)
	}
}
