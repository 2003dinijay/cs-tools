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
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/sftpgo-authentication-service/internal/config"
	"github.com/wso2-open-operations/cs-tools/integrations/sftpgo-authentication-service/internal/log"
	"github.com/wso2-open-operations/cs-tools/integrations/sftpgo-authentication-service/internal/models"
)

func TestSubscriptionService_GetUserFolderList(t *testing.T) {
	logger := log.NewAppLogger("INFO")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("customerEmail") {
		case "valid@example.com":
			resp := models.FolderResponse{
				IsValidCustomer: true,
				ProjectKeys:     []string{"PROJ1", "proj2"},
			}
			_ = json.NewEncoder(w).Encode(resp)
		case "novalidprojects@example.com":
			// Fix #8: a legitimate customer with no project-specific folders
			// yet must still come back as FolderLookupAuthorized (with an
			// empty list), not the same status as an explicit denial.
			resp := models.FolderResponse{IsValidCustomer: true, ProjectKeys: nil}
			_ = json.NewEncoder(w).Encode(resp)
		case "invalid@example.com":
			resp := models.FolderResponse{IsValidCustomer: false}
			_ = json.NewEncoder(w).Encode(resp)
		case "servererror@example.com":
			w.WriteHeader(http.StatusInternalServerError)
		case "badbody@example.com":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("not json"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := &config.Config{
		SubscriptionAPI: server.URL + "?customerEmail=%s",
	}
	s := NewSubscriptionService(cfg, logger)

	t.Run("Valid Customer", func(t *testing.T) {
		folders, status := s.GetUserFolderList("valid@example.com")
		if status != FolderLookupAuthorized {
			t.Errorf("Expected FolderLookupAuthorized, got %v", status)
		}
		if len(folders) != 2 {
			t.Errorf("Expected 2 folders, got %d", len(folders))
		}
		if folders[0] != "proj1" || folders[1] != "proj2" {
			t.Errorf("Expected [proj1, proj2], got %v", folders)
		}
	})

	// Fix #8: this is the ONE state where an empty folder list may still
	// receive the caller's SFTPFolders fallback -- a legitimate customer who
	// simply has no project-specific folders yet, as distinct from either of
	// the two cases below.
	t.Run("Valid Customer With No Projects Yet", func(t *testing.T) {
		folders, status := s.GetUserFolderList("novalidprojects@example.com")
		if status != FolderLookupAuthorized {
			t.Errorf("Expected FolderLookupAuthorized, got %v", status)
		}
		if len(folders) != 0 {
			t.Errorf("Expected no folders, got %v", folders)
		}
	})

	// Fix #8: an explicit denial must be distinguishable from a lookup
	// failure, so the caller never grants the same fallback access to both.
	t.Run("Invalid Customer", func(t *testing.T) {
		folders, status := s.GetUserFolderList("invalid@example.com")
		if status != FolderLookupDenied {
			t.Errorf("Expected FolderLookupDenied, got %v", status)
		}
		if folders != nil {
			t.Errorf("Expected nil folders for invalid customer, got %v", folders)
		}
	})

	t.Run("Lookup Failure: Non-2xx Response", func(t *testing.T) {
		folders, status := s.GetUserFolderList("servererror@example.com")
		if status != FolderLookupFailed {
			t.Errorf("Expected FolderLookupFailed, got %v", status)
		}
		if folders != nil {
			t.Errorf("Expected nil folders on lookup failure, got %v", folders)
		}
	})

	t.Run("Lookup Failure: Undecodable Body", func(t *testing.T) {
		folders, status := s.GetUserFolderList("badbody@example.com")
		if status != FolderLookupFailed {
			t.Errorf("Expected FolderLookupFailed, got %v", status)
		}
		if folders != nil {
			t.Errorf("Expected nil folders on lookup failure, got %v", folders)
		}
	})

	t.Run("Lookup Failure: Request Creation Error", func(t *testing.T) {
		badCfg := &config.Config{
			// An invalid control character in the URL makes http.NewRequest fail.
			SubscriptionAPI: "http://example.com/\x7f%s",
		}
		badSvc := NewSubscriptionService(badCfg, logger)
		folders, status := badSvc.GetUserFolderList("someone@example.com")
		if status != FolderLookupFailed {
			t.Errorf("Expected FolderLookupFailed, got %v", status)
		}
		if folders != nil {
			t.Errorf("Expected nil folders on lookup failure, got %v", folders)
		}
	})
}

func TestSubscriptionService_IsValidProjectKey(t *testing.T) {
	logger := log.NewAppLogger("INFO")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("projectKey") == "VALID" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cfg := &config.Config{
		ProjectAPI: server.URL + "?projectKey=%s",
	}
	s := NewSubscriptionService(cfg, logger)

	t.Run("Valid Key", func(t *testing.T) {
		if !s.IsValidProjectKey("VALID") {
			t.Error("Expected IsValidProjectKey to return true for VALID")
		}
	})

	t.Run("Invalid Key", func(t *testing.T) {
		if s.IsValidProjectKey("INVALID") {
			t.Error("Expected IsValidProjectKey to return false for INVALID")
		}
	})
}
