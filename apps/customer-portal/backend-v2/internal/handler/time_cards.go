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
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/dto"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
)

// entityTimeCardClient abstracts the entity-service time-card operations
// used by TimeCardHandler.
type entityTimeCardClient interface {
	SearchTimeCards(ctx context.Context, req entity.SearchTimeCardsRequest) (entity.SearchTimeCardsResponse, error)
}

// TimeCardHandler handles HTTP requests for time-card search.
//
// NOTE: entity-service only supports time cards on its ServiceNow data
// source — a Postgres-mode deployment 404s on this route. Read-only: this
// backend never exposes time-card creation or updates to the customer
// portal, only search.
type TimeCardHandler struct {
	entity entityTimeCardClient
}

// NewTimeCardHandler creates a TimeCardHandler backed by the given entity client.
func NewTimeCardHandler(entity entityTimeCardClient) *TimeCardHandler {
	return &TimeCardHandler{entity: entity}
}

// SearchTimeCards handles POST /projects/{id}/time-cards/search.
func (h *TimeCardHandler) SearchTimeCards(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	projectID := r.PathValue("id")
	if projectID == "" || !uuidRe.MatchString(projectID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.TimeCardSearchRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	if !validateDateParams(w,
		dateParam{"filters.startDate", derefString(req.Filters.StartDate)},
		dateParam{"filters.endDate", derefString(req.Filters.EndDate)},
	) {
		return
	}

	result, err := h.entity.SearchTimeCards(r.Context(), dto.BuildEntitySearchTimeCardsRequest(projectID, req))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchTimeCards failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search time cards.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapSearchTimeCards(result))
}
