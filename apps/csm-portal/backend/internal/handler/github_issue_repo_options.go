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
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/githubissue"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// githubIssueRepoOptionView is one entry of the "repository" catalogue the
// "Open Git issue" dialog offers — see githubissue.RepoOption.
type githubIssueRepoOptionView struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
}

// GithubIssueRepoOptionsHandler handles HTTP requests for the config-driven
// "Open Git issue" dialog repository catalogue.
//
// It has no upstream dependency: the option list is resolved once at process
// startup from GITHUB_ISSUE_REPO_OPTIONS (see githubissue.ParseRepoOptions,
// wired up in cmd/server/main.go) and served straight from memory.
type GithubIssueRepoOptionsHandler struct{}

// NewGithubIssueRepoOptionsHandler creates a GithubIssueRepoOptionsHandler.
func NewGithubIssueRepoOptionsHandler() *GithubIssueRepoOptionsHandler {
	return &GithubIssueRepoOptionsHandler{}
}

// GetOptions handles GET /github-issue-repo-options.
//
// A deployment with GITHUB_ISSUE_REPO_OPTIONS unset returns an empty array,
// not an error — same "must still start" contract as the dashboard registry.
func (h *GithubIssueRepoOptionsHandler) GetOptions(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	options := githubissue.Active()
	views := make([]githubIssueRepoOptionView, 0, len(options))
	for _, o := range options {
		views = append(views, githubIssueRepoOptionView{
			Value: o.Value,
			Label: o.Label,
			Owner: o.Owner,
			Repo:  o.Repo,
		})
	}

	writeJSONValue(w, http.StatusOK, views)
}
