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
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/servicenow"
)

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
