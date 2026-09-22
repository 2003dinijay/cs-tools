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
	"encoding/json"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// GithubServiceRequestHandler raises a service request from a GitHub issue.
//
// The webhook does this on its own when GitHub can reach us. This is the same
// operation asked for directly, for a repository that would rather call than
// wait -- or one whose workflow already knows the issue is ready and does not
// want to depend on delivery.
type GithubServiceRequestHandler struct {
	svc service.GithubSyncService
}

// NewGithubServiceRequestHandler constructs the endpoint.
func NewGithubServiceRequestHandler(svc service.GithubSyncService) *GithubServiceRequestHandler {
	return &GithubServiceRequestHandler{svc: svc}
}

// Create handles POST /github/service-requests.
func (h *GithubServiceRequestHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateServiceRequestFromIssueRequest
	if !decodeRequest(w, r, &req) {
		return
	}

	resp, err := h.svc.CreateServiceRequestFromIssue(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}

	// 200 rather than 201 when the record already existed: a caller retrying
	// after a timeout has not created anything, and saying so lets it tell the
	// two apart without parsing the message.
	status := http.StatusOK
	if resp.Created {
		status = http.StatusCreated
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

var _ = apierror.WriteJSON
