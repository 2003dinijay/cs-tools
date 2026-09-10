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

// entityGlobalClient abstracts the entity-service global metadata/search
// operations used by GlobalHandler.
type entityGlobalClient interface {
	GetSystemMetadata(ctx context.Context) (entity.SystemMetadataResponse, error)
	GlobalSearch(ctx context.Context, req entity.GlobalSearchRequest) (entity.GlobalSearchResponse, error)
	GetVulnerabilityMeta(ctx context.Context) (entity.VulnerabilityMetaResponse, error)
}

// GlobalHandler handles HTTP requests for system-wide metadata and cross-entity search.
type GlobalHandler struct {
	entity entityGlobalClient
}

// NewGlobalHandler creates a GlobalHandler backed by the given entity client.
func NewGlobalHandler(entity entityGlobalClient) *GlobalHandler {
	return &GlobalHandler{entity: entity}
}

// GetMetadata handles GET /metadata.
func (h *GlobalHandler) GetMetadata(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	result, err := h.entity.GetSystemMetadata(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetSystemMetadata failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve metadata information.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapMetadataResponse(result))
}

// GlobalSearch handles POST /search.
func (h *GlobalHandler) GlobalSearch(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.GlobalSearchRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.GlobalSearch(r.Context(), dto.BuildEntityGlobalSearchRequest(req))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GlobalSearch failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to perform global search.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapGlobalSearchResponse(result))
}

// GetVulnerabilityMeta handles GET /products/vulnerabilities/meta.
func (h *GlobalHandler) GetVulnerabilityMeta(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	result, err := h.entity.GetVulnerabilityMeta(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetVulnerabilityMeta failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve product vulnerability metadata.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapVulnerabilityMeta(result))
}
