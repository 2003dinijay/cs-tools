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

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/servicenow"
)

// splCaseClient abstracts the ServiceNow operations used by
// SplCaseHandler.
type splCaseClient interface {
	GetCases(ctx context.Context, searchString, stateFilter *string, offset, limit int) (servicenow.CaseDetailsWithCount, error)
	GetCaseByNumber(ctx context.Context, caseNumber string) (servicenow.CaseDetails, error)
	GetCommentsAndWorknotes(ctx context.Context, caseNumber string, offset, limit int) (servicenow.CommentsResponse, error)
	GetAttachmentsInfo(ctx context.Context, caseNumber string, offset, limit int) ([]servicenow.AttachmentInfo, error)
}

// SplCaseHandler handles HTTP requests for SupportPortalLite's
// ServiceNow-backed case endpoints.
type SplCaseHandler struct {
	sn            splCaseClient
	allowedGroups []string
}

// NewSplCaseHandler creates a SplCaseHandler.
func NewSplCaseHandler(sn splCaseClient, allowedGroups []string) *SplCaseHandler {
	return &SplCaseHandler{sn: sn, allowedGroups: allowedGroups}
}

// GetCases handles GET /spl/cases.
func (h *SplCaseHandler) GetCases(w http.ResponseWriter, r *http.Request) {
	user, ok := requireSPLGroups(w, r, h.allowedGroups)
	if !ok {
		return
	}
	offset, limit, ok := parsePaginationParams(w, r)
	if !ok {
		return
	}

	result, err := h.sn.GetCases(r.Context(), optionalQueryParam(r, "phrase"), optionalQueryParam(r, "stateFilter"), offset, limit)
	if err != nil {
		if isUnsafeQueryValue(err) {
			writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
			return
		}
		slog.ErrorContext(r.Context(), "servicenow GetCases failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to search cases.")
		return
	}
	writeJSONValue(w, http.StatusOK, result)
}

// GetCaseByNumber handles GET /spl/cases/{caseId}.
func (h *SplCaseHandler) GetCaseByNumber(w http.ResponseWriter, r *http.Request) {
	user, ok := requireSPLGroups(w, r, h.allowedGroups)
	if !ok {
		return
	}
	caseID := r.PathValue("caseId")
	if caseID == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.sn.GetCaseByNumber(r.Context(), caseID)
	if err != nil {
		if errors.Is(err, servicenow.ErrCaseNotFound) {
			writeError(w, http.StatusNotFound, ErrMsgNotFound)
			return
		}
		if isUnsafeQueryValue(err) {
			writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
			return
		}
		slog.ErrorContext(r.Context(), "servicenow GetCaseByNumber failed", "userID", user.UserID, "caseID", caseID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve case.")
		return
	}
	writeJSONValue(w, http.StatusOK, result)
}

// GetCommentsAndWorknotes handles GET /spl/cases/{caseId}/comments-and-worknotes.
func (h *SplCaseHandler) GetCommentsAndWorknotes(w http.ResponseWriter, r *http.Request) {
	user, ok := requireSPLGroups(w, r, h.allowedGroups)
	if !ok {
		return
	}
	caseID := r.PathValue("caseId")
	if caseID == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	offset, limit, ok := parsePaginationParams(w, r)
	if !ok {
		return
	}

	result, err := h.sn.GetCommentsAndWorknotes(r.Context(), caseID, offset, limit)
	if err != nil {
		if errors.Is(err, servicenow.ErrCaseNotFound) {
			writeError(w, http.StatusNotFound, ErrMsgNotFound)
			return
		}
		if isUnsafeQueryValue(err) {
			writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
			return
		}
		slog.ErrorContext(r.Context(), "servicenow GetCommentsAndWorknotes failed", "userID", user.UserID, "caseID", caseID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve comments and work notes.")
		return
	}
	writeJSONValue(w, http.StatusOK, result)
}

// GetAttachmentsInfo handles GET /spl/cases/{caseId}/attachments-info.
func (h *SplCaseHandler) GetAttachmentsInfo(w http.ResponseWriter, r *http.Request) {
	user, ok := requireSPLGroups(w, r, h.allowedGroups)
	if !ok {
		return
	}
	caseID := r.PathValue("caseId")
	if caseID == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	offset, limit, ok := parsePaginationParams(w, r)
	if !ok {
		return
	}

	result, err := h.sn.GetAttachmentsInfo(r.Context(), caseID, offset, limit)
	if err != nil {
		if errors.Is(err, servicenow.ErrCaseNotFound) {
			writeError(w, http.StatusNotFound, ErrMsgNotFound)
			return
		}
		if isUnsafeQueryValue(err) {
			writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
			return
		}
		slog.ErrorContext(r.Context(), "servicenow GetAttachmentsInfo failed", "userID", user.UserID, "caseID", caseID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve case attachments.")
		return
	}
	writeJSONValue(w, http.StatusOK, result)
}
