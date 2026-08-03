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
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/directory"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// ReferenceHandler serves the role catalogue and the team registry, both of
// which back the user-directory filters and the dashboard team picker.
//
// Both are deployment configuration this service resolves once at startup (see
// package directory), so neither endpoint makes an upstream call: they are
// memory reads, on the first request and on every request after it.
type ReferenceHandler struct {
	dir *directory.Directory
}

// NewReferenceHandler creates a ReferenceHandler backed by the startup-resolved
// directory.
func NewReferenceHandler(dir *directory.Directory) *ReferenceHandler {
	return &ReferenceHandler{dir: dir}
}

// SearchRoles handles POST /roles/search.
func (h *ReferenceHandler) SearchRoles(w http.ResponseWriter, r *http.Request) {
	req, ok := h.decodeSearch(w, r)
	if !ok {
		return
	}
	writeJSONValue(w, http.StatusOK, h.dir.SearchRoles(req))
}

// SearchTeams handles POST /teams/search.
func (h *ReferenceHandler) SearchTeams(w http.ResponseWriter, r *http.Request) {
	req, ok := h.decodeSearch(w, r)
	if !ok {
		return
	}
	writeJSONValue(w, http.StatusOK, h.dir.SearchTeams(req))
}

// decodeSearch carries the shared auth / read-body / decode sequence for both
// catalogue endpoints. It writes the error response itself and reports false
// when the caller should stop.
func (h *ReferenceHandler) decodeSearch(w http.ResponseWriter, r *http.Request) (directory.SearchRequest, bool) {
	var req directory.SearchRequest

	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return req, false
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return req, false
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return req, false
	}

	// Both endpoints accept an absent body, meaning "no filters, default page".
	// Only a non-empty body has to be valid JSON.
	if len(body) == 0 {
		return req, true
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return directory.SearchRequest{}, false
	}
	return req, true
}
