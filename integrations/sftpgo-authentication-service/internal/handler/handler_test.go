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
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/sftpgo-authentication-service/internal/config"
	"github.com/wso2-open-operations/cs-tools/integrations/sftpgo-authentication-service/internal/constants"
	"github.com/wso2-open-operations/cs-tools/integrations/sftpgo-authentication-service/internal/log"
	"github.com/wso2-open-operations/cs-tools/integrations/sftpgo-authentication-service/internal/models"
	"github.com/wso2-open-operations/cs-tools/integrations/sftpgo-authentication-service/internal/service"
)

func TestHandler_Authenticate(t *testing.T) {
	tests := []struct {
		name           string
		cfgKey         string
		requestKey     string
		expectedStatus int
		expectedResult bool
	}{
		{
			name:           "No key configured",
			cfgKey:         "",
			requestKey:     "",
			expectedStatus: http.StatusOK,
			expectedResult: true,
		},
		{
			name:           "Valid key provided",
			cfgKey:         "secret-key",
			requestKey:     "secret-key",
			expectedStatus: http.StatusOK,
			expectedResult: true,
		},
		{
			name:           "Invalid key provided",
			cfgKey:         "secret-key",
			requestKey:     "wrong-key",
			expectedStatus: http.StatusUnauthorized,
			expectedResult: false,
		},
		{
			name:           "Missing key provided",
			cfgKey:         "secret-key",
			requestKey:     "",
			expectedStatus: http.StatusUnauthorized,
			expectedResult: false,
		},
	}

	logger := log.NewAppLogger("INFO")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{HookAPIKey: tt.cfgKey}
			h := &Handler{cfg: cfg, logger: logger}

			req := httptest.NewRequest(http.MethodPost, "/test", nil)
			if tt.requestKey != "" {
				req.Header.Set(constants.HeaderAPIKey, tt.requestKey)
			}
			w := httptest.NewRecorder()

			got := h.authenticate(req, w)

			if got != tt.expectedResult {
				t.Errorf("authenticate() = %v, want %v", got, tt.expectedResult)
			}

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// TestPreLoginHook_ExternalUser_FolderLookupStates proves fix #8 end-to-end
// through PreLoginHook: an explicit subscription denial and a subscription
// lookup failure must both be denied outright (no SFTPFolders fallback),
// while a legitimate customer with no project-specific folders yet must
// still receive the fallback exactly as before.
func TestPreLoginHook_ExternalUser_FolderLookupStates(t *testing.T) {
	const (
		deniedUser   = "denied@example.com"
		failedUser   = "failed@example.com"
		fallbackUser = "fallback@example.com"
		fallbackDir  = "fallback-folder"
	)

	idpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "test-token"})
		case "/scim2/Users":
			filter := r.URL.Query().Get("filter")
			user := models.AsgardeoUser{
				CustomUserExtension: models.CustomUserExtension{SFTPFolders: fallbackDir},
			}
			if strings.Contains(filter, fallbackUser) {
				user.Emails = []string{fallbackUser}
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"Resources": []models.AsgardeoUser{user}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer idpServer.Close()

	subscriptionServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("customerEmail") {
		case deniedUser:
			_ = json.NewEncoder(w).Encode(models.FolderResponse{IsValidCustomer: false})
		case failedUser:
			w.WriteHeader(http.StatusInternalServerError)
		case fallbackUser:
			// Valid customer, but no project-specific folders yet -- the one
			// state where the caller's SFTPFolders fallback is safe to apply.
			_ = json.NewEncoder(w).Encode(models.FolderResponse{IsValidCustomer: true, ProjectKeys: nil})
		default:
			http.NotFound(w, r)
		}
	}))
	defer subscriptionServer.Close()

	sftpgoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/token":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "admin-token"})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/folders/"):
			w.WriteHeader(http.StatusOK) // folder already exists
		default:
			http.NotFound(w, r)
		}
	}))
	defer sftpgoServer.Close()

	cfg := &config.Config{
		InternalUserSuffix:           "@wso2.com", // none of the test users match this, so all are external
		ExternalIdPTokenEndPoint:     idpServer.URL + "/token",
		ExternalIdPSCIMUsersEndPoint: idpServer.URL + "/scim2/Users",
		SubscriptionAPI:              subscriptionServer.URL + "?customerEmail=%s",
		SFTPGoBasePath:               sftpgoServer.URL,
		DIRPath:                      "/data",
		FolderPath:                   "/folders",
	}
	cfg.AdminTokenEndPoint = cfg.SFTPGoBasePath + "/token"
	cfg.SFTPGoFoldersEndPoint = cfg.SFTPGoBasePath + "/folders"
	cfg.SFTPGoUsersEndPoint = cfg.SFTPGoBasePath + "/users"

	logger := log.NewAppLogger("ERROR")
	h := &Handler{
		cfg:          cfg,
		logger:       logger,
		idp:          service.NewIdPService(cfg, logger),
		sftpgo:       service.NewSFTPGoService(cfg, logger),
		subscription: service.NewSubscriptionService(cfg, logger),
	}

	postPreLogin := func(t *testing.T, username string) *httptest.ResponseRecorder {
		t.Helper()
		body, err := json.Marshal(models.SFTPGoUser{Username: username})
		if err != nil {
			t.Fatalf("failed to marshal request: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, "/prelogin-hook", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.PreLoginHook(w, req)
		return w
	}

	// (a) explicit denial -> no folders, not the fallback.
	t.Run("Explicit denial denies outright", func(t *testing.T) {
		w := postPreLogin(t, deniedUser)
		if w.Code != http.StatusNoContent {
			t.Fatalf("expected 204 No Content for an explicitly denied user, got %d: %s", w.Code, w.Body.String())
		}
	})

	// (b) lookup failure -> no folders, not the fallback.
	t.Run("Lookup failure denies outright", func(t *testing.T) {
		w := postPreLogin(t, failedUser)
		if w.Code != http.StatusNoContent {
			t.Fatalf("expected 204 No Content when the subscription lookup fails, got %d: %s", w.Code, w.Body.String())
		}
	})

	// (c) legitimate customer, no project-specific folders yet -> the
	// SFTPFolders fallback still works exactly as before.
	t.Run("Legitimate no-projects-yet customer still gets the fallback", func(t *testing.T) {
		w := postPreLogin(t, fallbackUser)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for a legitimate customer via the SFTPFolders fallback, got %d: %s", w.Code, w.Body.String())
		}
		var res models.MinimalSFTPGoUser
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if _, ok := res.Permissions["/"+fallbackDir]; !ok {
			t.Errorf("expected the fallback folder %q to be granted, got permissions: %v", fallbackDir, res.Permissions)
		}
	})
}
