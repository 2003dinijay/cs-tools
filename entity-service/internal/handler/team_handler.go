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

// TeamHandler handles HTTP requests for the team resource.
type TeamHandler struct {
	svc service.TeamService
}

// NewTeamHandler constructs a TeamHandler with the given service.
func NewTeamHandler(svc service.TeamService) *TeamHandler {
	return &TeamHandler{svc: svc}
}

// GetTeamMembers handles GET /teams/{id}/members.
func (h *TeamHandler) GetTeamMembers(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	resp, err := h.svc.GetTeamMembers(r.Context(), id)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// SearchTeams handles POST /teams/search.
func (h *TeamHandler) SearchTeams(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchTeamsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchTeams(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// ListChatWebhooks handles GET /teams/chat-webhooks -- a plain GET with no
// query params or pagination, like GET /sla-duration-policy: internal
// callers only (TeamService.ListChatWebhooks enforces this), and the result
// set is bounded by how many teams exist, not large enough to need paging.
func (h *TeamHandler) ListChatWebhooks(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.ListChatWebhooks(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// GetChatWebhook handles GET /teams/{id}/chat-webhook -- internal callers
// only (TeamService.GetChatWebhook enforces this).
func (h *TeamHandler) GetChatWebhook(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	resp, err := h.svc.GetChatWebhook(r.Context(), id)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// UpdateChatWebhook handles PATCH /teams/{id}/chat-webhook -- internal
// callers only (TeamService.SetChatWebhook enforces this); fine-grained
// per-team authorization (admin, or that team's own lead) is this route's
// caller's job -- see apps/csm-portal/backend's own CLAUDE.md.
func (h *TeamHandler) UpdateChatWebhook(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req domain.UpdateTeamChatWebhookRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SetChatWebhook(r.Context(), id, req.ChatWebhookURL)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// AssignLead handles POST /teams/{id}/lead -- internal callers only
// (TeamService.AssignTeamLead enforces this); fine-grained authorization
// (Asgardeo admin only) is this route's caller's job -- see
// apps/csm-portal/backend's own CLAUDE.md.
func (h *TeamHandler) AssignLead(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req domain.AssignTeamLeadRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	if err := h.svc.AssignTeamLead(r.Context(), id, req.UserID); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
