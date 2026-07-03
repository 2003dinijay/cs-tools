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
	"strconv"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// ConversationHandler handles HTTP requests for the conversations resource.
type ConversationHandler struct {
	svc service.ConversationService
}

// NewConversationHandler constructs a ConversationHandler with the given service.
func NewConversationHandler(svc service.ConversationService) *ConversationHandler {
	return &ConversationHandler{svc: svc}
}

// GetConversationMessages handles GET /conversations/{id}/messages.
// Optional query parameters: limit (positive integer), offset (non-negative integer).
func (h *ConversationHandler) GetConversationMessages(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	req := domain.GetConversationMessagesRequest{
		ConversationID: id,
	}

	q := r.URL.Query()
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeServiceError(w, r, &apierror.ValidationError{Msg: "limit must be a positive integer"})
			return
		}
		req.Pagination.Limit = n
	}
	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeServiceError(w, r, &apierror.ValidationError{Msg: "offset must be a non-negative integer"})
			return
		}
		req.Pagination.Offset = n
	}

	resp, err := h.svc.GetConversationMessages(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
