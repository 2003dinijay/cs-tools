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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// CloudStatusDashboardHandler serves the public cloud status dashboard's reads.
//
// Consumed by wso2-enterprise/uptime-dashboard through csm-integration-service,
// not by the portal. The response shapes are that dashboard's, not this
// service's -- see the domain types for why they look the way they do.
type CloudStatusDashboardHandler struct {
	svc service.CloudStatusDashboardService
}

// NewCloudStatusDashboardHandler constructs the handler.
func NewCloudStatusDashboardHandler(svc service.CloudStatusDashboardService) *CloudStatusDashboardHandler {
	return &CloudStatusDashboardHandler{svc: svc}
}

// Monitors handles GET /cloud-status/monitors?cloud=…
func (h *CloudStatusDashboardHandler) Monitors(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.Monitors(r.Context(), r.URL.Query().Get("cloud"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// Incidents handles GET /cloud-status/incidents?cloud=…
func (h *CloudStatusDashboardHandler) Incidents(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.Incidents(r.Context(), r.URL.Query().Get("cloud"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
