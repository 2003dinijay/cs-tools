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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/sftpgo-authentication-service/internal/config"
	"github.com/wso2-open-operations/cs-tools/integrations/sftpgo-authentication-service/internal/log"
	"github.com/wso2-open-operations/cs-tools/integrations/sftpgo-authentication-service/internal/util"
)

func TestValidateFolderName(t *testing.T) {
	tests := []struct {
		name       string
		folderName string
		wantError  bool
	}{
		{
			name:       "Valid folder name",
			folderName: "project1",
			wantError:  false,
		},
		{
			name:       "Valid folder with underscore",
			folderName: "project_folder_1",
			wantError:  false,
		},
		{
			name:       "Valid folder with hyphen",
			folderName: "project-folder-1",
			wantError:  false,
		},
		{
			name:       "Empty folder name",
			folderName: "",
			wantError:  true,
		},
		{
			name:       "Folder with parent directory traversal",
			folderName: "../etc",
			wantError:  true,
		},
		{
			name:       "Folder with double dots in middle",
			folderName: "folder..name",
			wantError:  true,
		},
		{
			name:       "Folder with forward slash",
			folderName: "folder/name",
			wantError:  true,
		},
		{
			name:       "Folder with backslash",
			folderName: "folder\\name",
			wantError:  true,
		},
		{
			name:       "Folder with leading slash",
			folderName: "/folder",
			wantError:  true,
		},
		{
			name:       "Folder with trailing slash",
			folderName: "folder/",
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := util.ValidateFolderName(tt.folderName)
			if (err != nil) != tt.wantError {
				t.Errorf("util.ValidateFolderName(%q) error = %v, wantError %v", tt.folderName, err, tt.wantError)
			}
		})
	}
}

// TestCreateFolder_ConcurrentCreation_TreatedAsSuccess proves fix #6: when
// SFTPGo responds 400 to a folder-creation request because a concurrent
// pre-login request already created it, createFolder must treat that as
// success after re-confirming the folder actually exists.
func TestCreateFolder_ConcurrentCreation_TreatedAsSuccess(t *testing.T) {
	var folderCheckCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/folders":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error": "folder already exists"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/folders/proj1":
			folderCheckCalls++
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := &config.Config{
		SFTPGoFoldersEndPoint: server.URL + "/folders",
	}
	s := NewSFTPGoService(cfg, log.NewAppLogger("ERROR"))

	if err := s.createFolder("proj1", "test-token"); err != nil {
		t.Fatalf("expected createFolder to treat a 400-because-it-already-exists as success, got error: %v", err)
	}
	if folderCheckCalls != 1 {
		t.Errorf("expected createFolder to re-check folder existence exactly once, got %d calls", folderCheckCalls)
	}
}

// TestCreateFolder_OtherValidationError_NotTreatedAsSuccess proves the fix
// does not blanket-treat every 400 as success: if the folder still does not
// exist after the re-check, the original validation error must surface.
func TestCreateFolder_OtherValidationError_NotTreatedAsSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/folders":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error": "invalid mapped_path"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/folders/proj1":
			// The folder genuinely does not exist: this 400 was a real
			// validation error, not a concurrent-creation race.
			w.WriteHeader(http.StatusNotFound)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := &config.Config{
		SFTPGoFoldersEndPoint: server.URL + "/folders",
	}
	s := NewSFTPGoService(cfg, log.NewAppLogger("ERROR"))

	if err := s.createFolder("proj1", "test-token"); err == nil {
		t.Fatal("expected createFolder to return an error for a genuine validation failure, got nil")
	}
}
