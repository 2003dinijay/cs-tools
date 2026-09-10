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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// SLAClockHandler handles HTTP requests for the sla_clocks resource — see
// domain.SLAClock's doc comment for what it's for.
type SLAClockHandler struct {
	svc service.SLAClockService
}

// NewSLAClockHandler constructs an SLAClockHandler with the given service.
func NewSLAClockHandler(svc service.SLAClockService) *SLAClockHandler {
	return &SLAClockHandler{svc: svc}
}

// RegisterSLAClock handles POST /cases/{caseId}/sla-clocks.
func (h *SLAClockHandler) RegisterSLAClock(w http.ResponseWriter, r *http.Request) {
	var req domain.RegisterSLAClockRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.CaseID = r.PathValue("caseId")
	resp, err := h.svc.RegisterSLAClock(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// GetSLAClock handles GET /cases/{caseId}/sla-clocks/{clockType}.
func (h *SLAClockHandler) GetSLAClock(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.GetSLAClock(r.Context(), r.PathValue("caseId"), r.PathValue("clockType"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// SetSLAClockTierReached handles
// PATCH /cases/{caseId}/sla-clocks/{clockType}/tiers/{tier}.
func (h *SLAClockHandler) SetSLAClockTierReached(w http.ResponseWriter, r *http.Request) {
	var req domain.SetSLAClockTierRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SetSLAClockTierReached(r.Context(), r.PathValue("caseId"), r.PathValue("clockType"), r.PathValue("tier"), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
