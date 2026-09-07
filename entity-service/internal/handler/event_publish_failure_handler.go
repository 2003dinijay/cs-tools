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

// EventPublishFailureHandler handles HTTP requests for the
// event_publish_failures resource — see domain.EventPublishFailure's doc
// comment for what it's for.
type EventPublishFailureHandler struct {
	svc service.EventPublishFailureService
}

// NewEventPublishFailureHandler constructs an EventPublishFailureHandler
// with the given service.
func NewEventPublishFailureHandler(svc service.EventPublishFailureService) *EventPublishFailureHandler {
	return &EventPublishFailureHandler{svc: svc}
}

// CreateEventPublishFailure handles POST /event-publish-failures.
func (h *EventPublishFailureHandler) CreateEventPublishFailure(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateEventPublishFailureRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.CreateEventPublishFailure(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// SearchEventPublishFailures handles POST /event-publish-failures/search.
func (h *EventPublishFailureHandler) SearchEventPublishFailures(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchEventPublishFailuresRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchEventPublishFailures(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// ResolveEventPublishFailure handles POST /event-publish-failures/{id}/resolve.
func (h *EventPublishFailureHandler) ResolveEventPublishFailure(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	resp, err := h.svc.ResolveEventPublishFailure(r.Context(), id)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
