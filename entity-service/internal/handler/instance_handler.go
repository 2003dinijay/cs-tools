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

// Package handler is declared in user_handler.go.
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// InstanceHandler handles HTTP requests for the instances resource.
type InstanceHandler struct {
	svc service.InstanceService
}

// NewInstanceHandler constructs an InstanceHandler with the given service.
func NewInstanceHandler(svc service.InstanceService) *InstanceHandler {
	return &InstanceHandler{svc: svc}
}

// SearchInstances handles POST /instances/search.
func (h *InstanceHandler) SearchInstances(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchInstancesRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchInstances(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// SearchInstanceMetrics handles POST /instances/metrics/search.
func (h *InstanceHandler) SearchInstanceMetrics(w http.ResponseWriter, r *http.Request) {
	var req domain.InstanceMetricsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchInstanceMetrics(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// SearchInstanceUsage handles POST /instances/usages/search.
func (h *InstanceHandler) SearchInstanceUsage(w http.ResponseWriter, r *http.Request) {
	var req domain.InstanceUsageRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchInstanceUsage(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// SearchInstanceMetricsStats handles POST /instances/metrics/stats/search.
func (h *InstanceHandler) SearchInstanceMetricsStats(w http.ResponseWriter, r *http.Request) {
	var req domain.InstanceMetricsStatsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchInstanceMetricsStats(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// SearchInstanceUsageStats handles POST /instances/usages/stats/search.
func (h *InstanceHandler) SearchInstanceUsageStats(w http.ResponseWriter, r *http.Request) {
	var req domain.InstanceUsageStatsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchInstanceUsageStats(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
