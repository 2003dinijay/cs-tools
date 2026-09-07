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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/sftpgo-authentication-service/internal/config"
	"github.com/wso2-open-operations/cs-tools/integrations/sftpgo-authentication-service/internal/httpclient"
	"github.com/wso2-open-operations/cs-tools/integrations/sftpgo-authentication-service/internal/constants"
	"github.com/wso2-open-operations/cs-tools/integrations/sftpgo-authentication-service/internal/log"
	"github.com/wso2-open-operations/cs-tools/integrations/sftpgo-authentication-service/internal/models"
)

// SubscriptionService handles interactions with the external subscription/folder API.
type SubscriptionService struct {
	// cfg is the application configuration.
	cfg *config.Config
	// logger is the application-wide logger.
	logger *log.AppLogger
	// client is the HTTP client used for subscription API calls.
	client *http.Client
}

// NewSubscriptionService creates a new SubscriptionService.
func NewSubscriptionService(cfg *config.Config, logger *log.AppLogger) *SubscriptionService {
	return &SubscriptionService{
		cfg:    cfg,
		logger: logger,
		client: httpclient.NewLoggingClient(time.Duration(cfg.HTTPTimeout)*time.Second, logger),
	}
}

// FolderLookupStatus distinguishes why GetUserFolderList returned the folder
// list it did, since an empty/nil list is ambiguous on its own: it can mean
// "this is a legitimate customer with no project-specific folders yet",
// "this user was explicitly denied", or "the lookup itself failed". Callers
// must only apply a broader fallback (e.g. SFTPFolders) for
// FolderLookupAuthorized; an explicit denial or a failed lookup must never be
// treated the same as "nothing case-specific to show".
type FolderLookupStatus int

const (
	// FolderLookupAuthorized means the subscription API confirmed the user is
	// a valid customer. Folders may still be empty (a legitimate customer
	// with no project-specific folders provisioned yet) -- that is the only
	// state in which a broader fallback folder list is safe to apply.
	FolderLookupAuthorized FolderLookupStatus = iota
	// FolderLookupDenied means the subscription API explicitly reported the
	// user is not a valid customer. Must not receive any fallback folders.
	FolderLookupDenied
	// FolderLookupFailed means the lookup itself could not be completed
	// (request construction error, network/timeout error, non-2xx response,
	// or an undecodable body) -- this says nothing about the user's actual
	// entitlement and must not receive any fallback folders either.
	FolderLookupFailed
)

// GetUserFolderList retrieves a custom folder list for a user, along with a
// FolderLookupStatus explaining why. See FolderLookupStatus for why the
// distinction matters: a subscription-lookup failure and an explicit denial
// must never grant the same access as "nothing project-specific to show".
func (s *SubscriptionService) GetUserFolderList(username string) ([]string, FolderLookupStatus) {
	s.logger.Debug("Attempting to retrieve custom folder list for user: %s", username)
	apiURL := fmt.Sprintf(s.cfg.SubscriptionAPI, url.QueryEscape(username))

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		s.logger.Error("Folder list request creation error for user %s: %v", username, err)
		return nil, FolderLookupFailed
	}
	req.Header.Set(constants.HeaderAccept, constants.MIMEApplicationJSON)

	resp, err := s.client.Do(req)
	if err != nil {
		s.logger.Error("Failed to send folder list request for user %s: %v", username, err)
		return nil, FolderLookupFailed
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		s.logger.Warn("Folder list API for user %s returned status %d, body: %s", username, resp.StatusCode, body)
		return nil, FolderLookupFailed
	}

	var folderResp models.FolderResponse
	if err := json.NewDecoder(resp.Body).Decode(&folderResp); err != nil {
		s.logger.Error("Failed to decode custom folder list for user %s: %v", username, err)
		return nil, FolderLookupFailed
	}

	if !folderResp.IsValidCustomer {
		s.logger.Debug("User %s is not a valid customer. No project keys returned.", username)
		return nil, FolderLookupDenied
	}

	var lowercaseKeys []string
	for _, key := range folderResp.ProjectKeys {
		lowercaseKeys = append(lowercaseKeys, strings.ToLower(key))
	}

	s.logger.Debug("Successfully retrieved %d custom folders for user %s.", len(lowercaseKeys), username)
	return lowercaseKeys, FolderLookupAuthorized
}

// IsValidProjectKey checks if the provided project key is valid.
func (s *SubscriptionService) IsValidProjectKey(projectKey string) bool {
	s.logger.Debug("Attempting to validate the project key: %s", projectKey)
	apiURL := fmt.Sprintf(s.cfg.ProjectAPI, url.QueryEscape(projectKey))

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		s.logger.Error("Project key validation request creation error for %s: %v", projectKey, err)
		return false
	}
	req.Header.Set(constants.HeaderAccept, constants.MIMEApplicationJSON)

	resp, err := s.client.Do(req)
	if err != nil {
		s.logger.Error("Failed to send project key validation request for %s: %v", projectKey, err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.logger.Debug("Project key validation API for %s returned status %d", projectKey, resp.StatusCode)
		return false
	}

	s.logger.Debug("Project key %s is valid.", projectKey)
	return true
}
