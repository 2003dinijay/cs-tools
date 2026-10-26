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
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/sftpgo-authentication-service/internal/config"
	"github.com/wso2-open-operations/cs-tools/integrations/sftpgo-authentication-service/internal/constants"
	"github.com/wso2-open-operations/cs-tools/integrations/sftpgo-authentication-service/internal/handler"
	applog "github.com/wso2-open-operations/cs-tools/integrations/sftpgo-authentication-service/internal/log"
	"github.com/wso2-open-operations/cs-tools/integrations/sftpgo-authentication-service/internal/service"
)

// TestBuildMux_RegistersOnlyEnabledFlows proves a disabled flow's paths answer
// 404 while an enabled flow's paths reach their handler. The handler has no
// backing services, so a reached route answers 401 (wrong key) rather than 404.
func TestBuildMux_RegistersOnlyEnabledFlows(t *testing.T) {
	tests := []struct {
		name           string
		web, sftp      bool
		wantRegistered map[string]bool
	}{
		{"web-attachment only", true, false, map[string]bool{"/external-auth-hook": true, "/prelogin-hook": false, "/auth-hook": false}},
		{"sftp-channel only", false, true, map[string]bool{"/external-auth-hook": false, "/prelogin-hook": true, "/auth-hook": true}},
		{"both", true, true, map[string]bool{"/external-auth-hook": true, "/prelogin-hook": true, "/auth-hook": true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{HookAPIKey: "secret-key", WebAttachmentEnabled: tt.web, SFTPChannelEnabled: tt.sftp}
			h := handler.NewHandler(cfg, applog.NewAppLogger("ERROR"), nil, nil, nil, nil, nil)
			mux := buildMux(cfg, h)

			for path, registered := range tt.wantRegistered {
				req := httptest.NewRequest(http.MethodPost, path, nil)
				req.Header.Set(constants.HeaderAPIKey, "wrong-key")
				w := httptest.NewRecorder()
				mux.ServeHTTP(w, req)

				if registered && w.Code != http.StatusUnauthorized {
					t.Errorf("%s: expected the route to be registered (401 for a wrong key), got %d", path, w.Code)
				}
				if !registered && w.Code != http.StatusNotFound {
					t.Errorf("%s: expected 404 for a disabled flow, got %d", path, w.Code)
				}
			}
		})
	}
}

// TestInitJWTAuth proves a validator construction error is fatal only when
// ENABLED_FLOWS is explicit, and warn-and-continue (nil validator, no error)
// when it is unset.
func TestInitJWTAuth(t *testing.T) {
	failing := func(*config.Config, *applog.AppLogger) (*service.JWTAuthService, error) {
		return nil, errors.New("boom")
	}
	working := func(*config.Config, *applog.AppLogger) (*service.JWTAuthService, error) {
		return &service.JWTAuthService{}, nil
	}
	logger := applog.NewAppLogger("ERROR")

	if _, err := initJWTAuth(&config.Config{FlowsExplicit: true, WebAttachmentEnabled: true}, logger, failing); err == nil {
		t.Error("explicit ENABLED_FLOWS: expected the construction error to be returned")
	}
	svc, err := initJWTAuth(&config.Config{WebAttachmentEnabled: true}, logger, failing)
	if err != nil || svc != nil {
		t.Errorf("unset ENABLED_FLOWS: expected nil validator and nil error, got %v, %v", svc, err)
	}
	svc, err = initJWTAuth(&config.Config{FlowsExplicit: true, WebAttachmentEnabled: true}, logger, working)
	if err != nil || svc == nil {
		t.Errorf("expected the validator on success, got %v, %v", svc, err)
	}
}
