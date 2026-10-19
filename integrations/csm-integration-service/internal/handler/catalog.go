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
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
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
// service as-is (an empty one as {}) and its response is returned as received,
// except that the enum-valued fields listed in each handler are upper-cased.
type CatalogHandler struct {
	entity entityCatalogClient
}

// NewCatalogHandler creates a CatalogHandler backed by the given entity client.
func NewCatalogHandler(entity entityCatalogClient) *CatalogHandler {
	return &CatalogHandler{entity: entity}
}

// readSearchBody caps and reads the request body and checks it is JSON. An empty
// (or whitespace-only) body becomes "{}", since entity-service rejects a body
// with no JSON object. On failure it writes the error response and returns false.
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
	if len(bytes.TrimSpace(body)) == 0 {
		return []byte("{}"), true
	}
	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return nil, false
	}
	return body, true
}

// upperEnumFields upper-cases the string value of each named field on every
// object in the response's list under listKey (e.g. "products"), leaving
// everything else as received: numbers keep their exact text, nulls and
// non-string values are untouched, and a response without that list passes
// through unchanged. It errors only when body is not a JSON object.
func upperEnumFields(body []byte, listKey string, fields ...string) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		return nil, err
	}
	items, _ := doc[listKey].([]any)
	for _, it := range items {
		obj, ok := it.(map[string]any)
		if !ok {
			continue
		}
		for _, f := range fields {
			if v, ok := obj[f].(string); ok {
				obj[f] = strings.ToUpper(v)
			}
		}
	}
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	return bytes.TrimRight(out.Bytes(), "\n"), nil
}

// writeUpperedJSON upper-cases the given enum fields and writes the result,
// or a 500 if the upstream body cannot be processed.
func writeUpperedJSON(w http.ResponseWriter, r *http.Request, result []byte, listKey string, fields ...string) {
	out, err := upperEnumFields(result, listKey, fields...)
	if err != nil {
		slog.ErrorContext(r.Context(), "catalog response is not a JSON object", "list", listKey, "err", err)
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// SearchProducts handles POST /products/search; the product class is upper-cased.
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
	writeUpperedJSON(w, r, result, "products", "class")
}

// SearchProductVersions handles POST /products/{id}/versions/search; the
// support status is upper-cased.
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
	writeUpperedJSON(w, r, result, "productVersions", "currentSupportStatus")
}

// SearchDeployments handles POST /deployments/search; the deployment type is upper-cased.
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
	writeUpperedJSON(w, r, result, "deployments", "type")
}

// SearchDeployedProducts handles POST /deployed-products/search; the product
// category is upper-cased.
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
	writeUpperedJSON(w, r, result, "deployedProducts", "category")
}
