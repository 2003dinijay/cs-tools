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
	"io"
	"log/slog"
	"net/http"
)

// entityCatalogClient abstracts the entity service operations used by CatalogHandler.
type entityCatalogClient interface {
	SearchProducts(ctx context.Context, body []byte) ([]byte, error)
	SearchProductVersions(ctx context.Context, productID string, body []byte) ([]byte, error)
	SearchDeployments(ctx context.Context, body []byte) ([]byte, error)
	SearchDeployedProducts(ctx context.Context, body []byte) ([]byte, error)
}

// CatalogHandler handles the product, product-version, deployment and
// deployed-product searches. Each request body is forwarded to the entity
// service as-is and its response is returned verbatim.
type CatalogHandler struct {
	entity entityCatalogClient
}

// NewCatalogHandler creates a CatalogHandler backed by the given entity client.
func NewCatalogHandler(entity entityCatalogClient) *CatalogHandler {
	return &CatalogHandler{entity: entity}
}

// readSearchBody caps and reads the request body and checks it is JSON. An empty
// body is allowed and forwarded empty. On failure it writes the error response
// and returns false.
func readSearchBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := err.(*http.MaxBytesError); ok {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return nil, false
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return nil, false
	}
	if len(body) > 0 && !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return nil, false
	}
	return body, true
}

// SearchProducts handles POST /products/search.
func (h *CatalogHandler) SearchProducts(w http.ResponseWriter, r *http.Request) {
	body, ok := readSearchBody(w, r)
	if !ok {
		return
	}
	result, err := h.entity.SearchProducts(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchProducts failed", "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search products.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// SearchProductVersions handles POST /products/{id}/versions/search.
func (h *CatalogHandler) SearchProductVersions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	body, ok := readSearchBody(w, r)
	if !ok {
		return
	}
	result, err := h.entity.SearchProductVersions(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchProductVersions failed", "productID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search product versions.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// SearchDeployments handles POST /deployments/search.
func (h *CatalogHandler) SearchDeployments(w http.ResponseWriter, r *http.Request) {
	body, ok := readSearchBody(w, r)
	if !ok {
		return
	}
	result, err := h.entity.SearchDeployments(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchDeployments failed", "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search deployments.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// SearchDeployedProducts handles POST /deployed-products/search.
func (h *CatalogHandler) SearchDeployedProducts(w http.ResponseWriter, r *http.Request) {
	body, ok := readSearchBody(w, r)
	if !ok {
		return
	}
	result, err := h.entity.SearchDeployedProducts(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchDeployedProducts failed", "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search deployed products.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
