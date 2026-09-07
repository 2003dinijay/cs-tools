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

// EscalationHandler handles HTTP requests for the escalations resource.
type EscalationHandler struct {
	svc service.EscalationService
}

// NewEscalationHandler constructs an EscalationHandler with the given service.
func NewEscalationHandler(svc service.EscalationService) *EscalationHandler {
	return &EscalationHandler{svc: svc}
}

// SearchEscalations handles POST /escalations/search.
func (h *EscalationHandler) SearchEscalations(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchEscalationsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchEscalations(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// CreateEscalation handles POST /escalations.
func (h *EscalationHandler) CreateEscalation(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateEscalationRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.CreateEscalation(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}
