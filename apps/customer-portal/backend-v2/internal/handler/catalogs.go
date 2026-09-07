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

// entityCatalogClient abstracts the entity-service catalog operations used by CatalogHandler.
type entityCatalogClient interface {
	SearchCatalogs(ctx context.Context, req entity.SearchCatalogsRequest) (entity.SearchCatalogsResponse, error)
	GetCatalogItemVariables(ctx context.Context, catalogID, catalogItemID string) (entity.GetCatalogItemVariablesResponse, error)
}

// CatalogHandler handles HTTP requests for service catalog operations.
type CatalogHandler struct {
	entity entityCatalogClient
}

// NewCatalogHandler creates a CatalogHandler backed by the given entity client.
func NewCatalogHandler(entity entityCatalogClient) *CatalogHandler {
	return &CatalogHandler{entity: entity}
}

// SearchCatalogs handles
// POST /deployments/products/{deployedProductId}/catalogs/search.
func (h *CatalogHandler) SearchCatalogs(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	deployedProductID := r.PathValue("deployedProductId")
	if !uuidRe.MatchString(deployedProductID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.SearchCatalogsRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.SearchCatalogs(r.Context(), dto.BuildEntitySearchCatalogsRequest(deployedProductID, req))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchCatalogs failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search catalogs.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapSearchCatalogs(result))
}

// GetCatalogItemVariables handles GET /catalogs/{catalogId}/items/{itemId}.
func (h *CatalogHandler) GetCatalogItemVariables(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	catalogID := r.PathValue("catalogId")
	catalogItemID := r.PathValue("itemId")
	if !uuidRe.MatchString(catalogID) || !uuidRe.MatchString(catalogItemID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	result, err := h.entity.GetCatalogItemVariables(r.Context(), catalogID, catalogItemID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetCatalogItemVariables failed", "userID", user.UserID, "catalogID", catalogID, "catalogItemID", catalogItemID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve catalog item variables.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapCatalogItemVariables(result))
}
