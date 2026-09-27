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

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/servicenow"
)

// splWorknotesClient abstracts the ServiceNow work-note operation used by
// SplWorknotesHandler.
type splWorknotesClient interface {
	PostWorkNote(ctx context.Context, caseNumber, worknote, submitterEmail string) (servicenow.WorkNoteResponse, error)
}

// SplWorknotesHandler handles HTTP requests for posting work notes on a
// case, delegating to the ServiceNow service.
type SplWorknotesHandler struct {
	servicenow        splWorknotesClient
	allowedGroups     []string
	addWorknoteGroups []string
}

// NewSplWorknotesHandler creates a SplWorknotesHandler backed by the given
// ServiceNow client. allowedGroups is SupportPortalLite's blanket
// access-gate group list (SPL_ALLOWED_GROUPS); addWorknoteGroups is the
// additional group list required to post a work note (SPL_ADD_WORKNOTE_GROUPS).
func NewSplWorknotesHandler(sn splWorknotesClient, allowedGroups, addWorknoteGroups []string) *SplWorknotesHandler {
	return &SplWorknotesHandler{servicenow: sn, allowedGroups: allowedGroups, addWorknoteGroups: addWorknoteGroups}
}

type splWorkNoteRequest struct {
	Worknote string `json:"worknote"`
}

// PostWorkNote handles POST /cases/{caseId}/worknote.
func (h *SplWorknotesHandler) PostWorkNote(w http.ResponseWriter, r *http.Request) {
	user, ok := requireSPLGroups(w, r, h.allowedGroups)
	if !ok {
		return
	}
	if !requireSPLSubGroups(w, user, h.addWorknoteGroups) {
		return
	}

	caseID := r.PathValue("caseId")
	if caseID == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if err := servicenow.SanitizeQueryValue(caseID); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
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

	var payload splWorkNoteRequest
	if err := json.Unmarshal(body, &payload); err != nil || payload.Worknote == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.servicenow.PostWorkNote(r.Context(), caseID, payload.Worknote, user.Email)
	if err != nil {
		switch err {
		case servicenow.ErrCaseSysIDNotFound:
			writeError(w, http.StatusNotFound, ErrMsgNotFound)
		case servicenow.ErrCaseClosed:
			writeError(w, http.StatusBadRequest, "Case is closed. Cannot add work notes.")
		default:
			slog.ErrorContext(r.Context(), "servicenow PostWorkNote failed", "userID", user.UserID, "caseID", caseID, "err", err)
			mapUpstreamErrorGeneric(w, err, "Failed to add work note.")
		}
		return
	}

	writeJSONValue(w, http.StatusOK, result)
}
