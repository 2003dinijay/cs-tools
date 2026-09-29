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

// splCaseClient abstracts the ServiceNow operations used by SplCaseHandler.
// GetCases/GetCaseByNumber/GetCommentsAndWorknotes used to live here too,
// backed first by ServiceNow and later by a Postgres translation layer --
// both removed in favor of calling CS Portal's own POST /cases/search,
// GET /cases/{id}, and POST /cases/{id}/comments/search directly (worknote
// creation similarly merged onto POST /cases/{id}/comments, using the same
// entity-service CommentType distinction CS Portal's own comment handler
// already exposes -- see splWorknotesHandler's removal). Attachments have no
// entity-service equivalent at all yet (no Postgres storage/backfill path),
// so that one stays here, ServiceNow-backed, unmerged.
type splCaseClient interface {
	GetAttachmentsInfo(ctx context.Context, caseNumber string, offset, limit int) ([]servicenow.AttachmentInfo, error)
}

// SplCaseHandler handles HTTP requests for SupportPortalLite's case-
// attachments endpoint -- the one piece of the case domain with no
// Postgres/entity-service equivalent to merge onto (see splCaseClient's own
// doc comment). Reading, searching, and commenting on cases now goes
// through CS Portal's own /cases routes directly.
type SplCaseHandler struct {
	sn          splCaseClient
	accessGuard *AccessGuard
}

// NewSplCaseHandler creates a SplCaseHandler.
func NewSplCaseHandler(sn splCaseClient, accessGuard *AccessGuard) *SplCaseHandler {
	return &SplCaseHandler{sn: sn, accessGuard: accessGuard}
}

// GetAttachmentsInfo handles GET /cases/{caseId}/attachments-info.
func (h *SplCaseHandler) GetAttachmentsInfo(w http.ResponseWriter, r *http.Request) {
	user, ok := requireSPLAccess(w, r, h.accessGuard)
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
