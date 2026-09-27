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
	"log/slog"
	"net/http"
	"strings"
)

// splAttachmentsClient abstracts the ServiceNow attachment-download
// operation used by SplAttachmentsHandler.
type splAttachmentsClient interface {
	DownloadAttachment(ctx context.Context, attachmentSysID string) (body []byte, contentType string, contentDisposition string, err error)
}

// SplAttachmentsHandler handles HTTP requests for downloading a case
// attachment, delegating to the ServiceNow service.
type SplAttachmentsHandler struct {
	servicenow               splAttachmentsClient
	allowedGroups            []string
	downloadAttachmentGroups []string
}

// NewSplAttachmentsHandler creates a SplAttachmentsHandler backed by the
// given ServiceNow client. allowedGroups is SupportPortalLite's blanket
// access-gate group list (SPL_ALLOWED_GROUPS); downloadAttachmentGroups is
// the additional group list required to download an attachment
// (SPL_DOWNLOAD_ATTACHMENT_GROUPS).
func NewSplAttachmentsHandler(sn splAttachmentsClient, allowedGroups, downloadAttachmentGroups []string) *SplAttachmentsHandler {
	return &SplAttachmentsHandler{servicenow: sn, allowedGroups: allowedGroups, downloadAttachmentGroups: downloadAttachmentGroups}
}

// DownloadAttachment handles GET /attachments/{attachmentId}/download. Only
// an allowlisted set of Content-Type values (safeAttachmentTypes, defined
// in cases.go) are honored, and the response always forces
// Content-Disposition: attachment regardless of what ServiceNow sent —
// mirroring this backend's existing GetCaseAttachmentContent convention,
// which never trusts an upstream Content-Type/Content-Disposition for
// inline rendering.
func (h *SplAttachmentsHandler) DownloadAttachment(w http.ResponseWriter, r *http.Request) {
	user, ok := requireSPLGroups(w, r, h.allowedGroups)
	if !ok {
		return
	}
	if !requireSPLSubGroups(w, user, h.downloadAttachmentGroups) {
		return
	}

	attachmentID := r.PathValue("attachmentId")
	if attachmentID == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	content, contentType, _, err := h.servicenow.DownloadAttachment(r.Context(), attachmentID)
	if err != nil {
		slog.ErrorContext(r.Context(), "servicenow DownloadAttachment failed", "userID", user.UserID, "attachmentID", attachmentID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve attachment content.")
		return
	}

	ct := strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	if !safeAttachmentTypes[ct] {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", "attachment")
	_, _ = w.Write(content) // #nosec G705 -- Content-Type is allowlisted above; Content-Disposition: attachment prevents inline rendering
}
