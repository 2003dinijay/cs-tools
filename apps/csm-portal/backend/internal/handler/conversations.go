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
	"context"
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// entityConversationClient abstracts the entity service conversation operations.
type entityConversationClient interface {
	GetConversationMessages(ctx context.Context, conversationID string, rawQuery string) ([]byte, error)
}

// ConversationHandler handles HTTP requests for conversation operations.
type ConversationHandler struct {
	entity entityConversationClient
}

// NewConversationHandler creates a ConversationHandler backed by the given entity client.
func NewConversationHandler(entity entityConversationClient) *ConversationHandler {
	return &ConversationHandler{entity: entity}
}

// GetConversationMessages handles GET /conversations/{id}/messages.
func (h *ConversationHandler) GetConversationMessages(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	result, err := h.entity.GetConversationMessages(r.Context(), id, r.URL.RawQuery)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetConversationMessages failed", "userID", user.UserID, "conversationID", id, "err", err)
		mapUpstreamError(w, err, "Failed to retrieve conversation messages.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}
