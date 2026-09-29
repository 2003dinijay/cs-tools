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
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/servicenow"
)

// entityProjectClient abstracts the entity service project operations used by ProjectHandler.
type entityProjectClient interface {
	GetProject(ctx context.Context, id string) ([]byte, error)
	GetProjectMetadata(ctx context.Context, id string) ([]byte, error)
	SearchProjects(ctx context.Context, body []byte) ([]byte, error)
	SearchProjectContacts(ctx context.Context, projectID string, body []byte) ([]byte, error)
	GetProjectContact(ctx context.Context, projectID, contactID string) ([]byte, error)
	UpdateProject(ctx context.Context, id string, body []byte) ([]byte, error)
}

// ProjectHandler handles HTTP requests for project operations, delegating to the
// entity service for data access.
type ProjectHandler struct {
	entity entityProjectClient
}

// NewProjectHandler creates a ProjectHandler backed by the given entity client.
func NewProjectHandler(entity entityProjectClient) *ProjectHandler {
	return &ProjectHandler{entity: entity}
}

// GetProject handles GET /projects/{id}.
func (h *ProjectHandler) GetProject(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.GetProject(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetProject failed", "userID", user.UserID, "projectID", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve project.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetProjectMetadata handles GET /projects/{id}/metadata.
func (h *ProjectHandler) GetProjectMetadata(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.GetProjectMetadata(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetProjectMetadata failed", "userID", user.UserID, "projectID", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve project metadata.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// SearchProjects handles POST /projects/search.
func (h *ProjectHandler) SearchProjects(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := err.(*http.MaxBytesError); ok {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	// TODO: Decode into a typed SearchProjectsRequest and validate fields before forwarding.

	result, err := h.entity.SearchProjects(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchProjects failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to search projects.")
		return
	}

	// TODO: Unmarshal result and filter to only the fields required by the frontend.
	writeJSON(w, http.StatusOK, result)
}

// SearchProjectContacts handles POST /projects/{id}/contacts/search.
// The endpoint is path-scoped, so the request body is capped and forwarded to the
// entity service as-is (no fields are injected) and the response is returned verbatim.
func (h *ProjectHandler) SearchProjectContacts(w http.ResponseWriter, r *http.Request) {
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

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := err.(*http.MaxBytesError); ok {
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

	result, err := h.entity.SearchProjectContacts(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchProjectContacts failed", "userID", user.UserID, "projectID", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to search project contacts.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetProjectContact handles GET /projects/{id}/contacts/{contactId}.
//
// Returns one contact's attributes for a single project: their roles on it, their
// registration state and their notification preference.
func (h *ProjectHandler) GetProjectContact(w http.ResponseWriter, r *http.Request) {
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

	contactID := r.PathValue("contactId")
	if contactID == "" || !uuidRe.MatchString(contactID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	result, err := h.entity.GetProjectContact(r.Context(), id, contactID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetProjectContact failed",
			"userID", user.UserID, "projectID", id, "contactID", contactID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to fetch the project contact.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// UpdateProject handles PATCH /projects/{id}.
// The endpoint is path-scoped, so the request body is capped and forwarded to the
// entity service as-is (no fields are injected) and the response is returned verbatim.
// The entity service is the source of truth for field-level validation (e.g. at
// least one field must be provided); the backend has no role-based access control
// layer yet, so any authenticated user may invoke this today, matching the
// existing convention on other PATCH endpoints in this codebase.
func (h *ProjectHandler) UpdateProject(w http.ResponseWriter, r *http.Request) {
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

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := err.(*http.MaxBytesError); ok {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.UpdateProject(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity UpdateProject failed", "userID", user.UserID, "projectID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to update project.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// splProjectClient abstracts the ServiceNow operations used by
// SplProjectHandler.
type splProjectClient interface {
	GetProjects(ctx context.Context, phrase *string, offset, limit int) ([]servicenow.ProjectDetails, error)
	GetProjectByID(ctx context.Context, projectID string) (servicenow.ProjectDetails, error)
	GetProjectContacts(ctx context.Context, projectID string, offset, limit int) ([]servicenow.Contact, error)
	GetCasesByProject(ctx context.Context, projectID string, stateFilters, caseTypeFilters []string, offset, limit int) ([]servicenow.CaseDetails, error)
}

// SplProjectHandler handles HTTP requests for SupportPortalLite's
// ServiceNow-backed project endpoints.
type SplProjectHandler struct {
	sn            splProjectClient
	allowedGroups []string
}

// NewSplProjectHandler creates a SplProjectHandler.
func NewSplProjectHandler(sn splProjectClient, allowedGroups []string) *SplProjectHandler {
	return &SplProjectHandler{sn: sn, allowedGroups: allowedGroups}
}

// GetProjects handles GET /spl/projects.
func (h *SplProjectHandler) GetProjects(w http.ResponseWriter, r *http.Request) {
	user, ok := requireSPLGroups(w, r, h.allowedGroups)
	if !ok {
		return
	}
	offset, limit, ok := parsePaginationParams(w, r)
	if !ok {
		return
	}

	result, err := h.sn.GetProjects(r.Context(), optionalQueryParam(r, "phrase"), offset, limit)
	if err != nil {
		if isUnsafeQueryValue(err) {
			writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
			return
		}
		slog.ErrorContext(r.Context(), "servicenow GetProjects failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve projects.")
		return
	}
	writeJSONValue(w, http.StatusOK, result)
}

// GetProjectByID handles GET /spl/projects/{projectId}.
func (h *SplProjectHandler) GetProjectByID(w http.ResponseWriter, r *http.Request) {
	user, ok := requireSPLGroups(w, r, h.allowedGroups)
	if !ok {
		return
	}
	projectID := r.PathValue("projectId")
	if projectID == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.sn.GetProjectByID(r.Context(), projectID)
	if err != nil {
		if errors.Is(err, servicenow.ErrProjectByIDNotFound) {
			writeError(w, http.StatusNotFound, ErrMsgNotFound)
			return
		}
		if isUnsafeQueryValue(err) {
			writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
			return
		}
		slog.ErrorContext(r.Context(), "servicenow GetProjectByID failed", "userID", user.UserID, "projectID", projectID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve project.")
		return
	}
	writeJSONValue(w, http.StatusOK, result)
}

// GetProjectContacts handles GET /spl/projects/{projectId}/contacts.
func (h *SplProjectHandler) GetProjectContacts(w http.ResponseWriter, r *http.Request) {
	user, ok := requireSPLGroups(w, r, h.allowedGroups)
	if !ok {
		return
	}
	projectID := r.PathValue("projectId")
	if projectID == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	offset, limit, ok := parsePaginationParams(w, r)
	if !ok {
		return
	}

	result, err := h.sn.GetProjectContacts(r.Context(), projectID, offset, limit)
	if err != nil {
		if errors.Is(err, servicenow.ErrProjectByIDNotFound) {
			writeError(w, http.StatusNotFound, ErrMsgNotFound)
			return
		}
		if isUnsafeQueryValue(err) {
			writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
			return
		}
		slog.ErrorContext(r.Context(), "servicenow GetProjectContacts failed", "userID", user.UserID, "projectID", projectID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve project contacts.")
		return
	}
	writeJSONValue(w, http.StatusOK, result)
}

// GetProjectCases handles GET /spl/projects/{projectId}/cases.
func (h *SplProjectHandler) GetProjectCases(w http.ResponseWriter, r *http.Request) {
	user, ok := requireSPLGroups(w, r, h.allowedGroups)
	if !ok {
		return
	}
	projectID := r.PathValue("projectId")
	if projectID == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	offset, limit, ok := parsePaginationParams(w, r)
	if !ok {
		return
	}

	q := r.URL.Query()
	stateFilters := splitNonEmpty(q["stateFilters"])
	caseTypeFilters := splitNonEmpty(q["caseTypeFilters"])

	result, err := h.sn.GetCasesByProject(r.Context(), projectID, stateFilters, caseTypeFilters, offset, limit)
	if err != nil {
		if errors.Is(err, servicenow.ErrProjectByIDNotFound) {
			writeError(w, http.StatusNotFound, ErrMsgNotFound)
			return
		}
		if isUnsafeQueryValue(err) {
			writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
			return
		}
		slog.ErrorContext(r.Context(), "servicenow GetCasesByProject failed", "userID", user.UserID, "projectID", projectID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve project cases.")
		return
	}
	writeJSONValue(w, http.StatusOK, result)
}

// splitNonEmpty flattens repeated query params (?stateFilters=a&stateFilters=b)
// and comma-separated values (?stateFilters=a,b) into a single slice,
// dropping empty entries. Supports both call shapes since the Ballerina
// framework's string[]? query param binding accepts either.
func splitNonEmpty(values []string) []string {
	var results []string
	for _, v := range values {
		for _, part := range strings.Split(v, ",") {
			if part != "" {
				results = append(results, part)
			}
		}
	}
	return results
}
