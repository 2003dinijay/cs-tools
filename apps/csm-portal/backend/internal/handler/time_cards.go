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
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// entityTimeCardClient abstracts the entity service time-card operations.
type entityTimeCardClient interface {
	SearchTimeCards(ctx context.Context, body []byte) ([]byte, error)
}

// TimeCardHandler handles HTTP requests for time-card operations.
type TimeCardHandler struct {
	entity entityTimeCardClient
}

// NewTimeCardHandler creates a TimeCardHandler backed by the given entity client.
func NewTimeCardHandler(entity entityTimeCardClient) *TimeCardHandler {
	return &TimeCardHandler{entity: entity}
}

// SearchTimeCards handles POST /time-cards/search.
func (h *TimeCardHandler) SearchTimeCards(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	if len(body) > 0 && !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.SearchTimeCards(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchTimeCards failed", "userID", user.UserID, "err", err)
		mapUpstreamError(w, err, "Failed to search time cards.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}
