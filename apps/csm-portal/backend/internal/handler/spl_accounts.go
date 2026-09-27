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
	"strconv"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/servicenow"
)

// splAccountClient abstracts the ServiceNow operations used by
// SplAccountHandler. GetAccounts/GetAccountByID/GetProjectsByAccount used to
// live here too, backed first by ServiceNow and later by a Postgres
// translation layer -- both removed in favor of calling CS Portal's own
// GET /accounts/{id}, POST /accounts/search, and POST /projects/search
// (filtered by accountId) directly, now that SPL's data source for these
// reads is the exact same entity-service Postgres data CS Portal's own
// routes already serve, with no ServiceNow-shape translation left to
// justify a second, parallel /spl/* contract for them. Escalation
// create/read have no entity-service equivalent (CreateEscalation is an
// explicit stub -- see entity-service's escalation_service.go), so those
// two stay here, ServiceNow-backed, unmerged.
type splAccountClient interface {
	GetEscalationsByAccount(ctx context.Context, accountNumber string, offset, limit int) ([]servicenow.EscalationDetail, error)
	EscalateCase(ctx context.Context, accountNumber, caseNumber string, request servicenow.EscalationRequest, submittedByEmail string) (servicenow.EscalationResponse, error)
}

// SplAccountHandler handles HTTP requests for SupportPortalLite's
// account-escalation endpoints -- the one piece of the account domain with
// no Postgres/entity-service equivalent to merge onto (see splAccountClient's
// own doc comment). Reading and listing accounts/projects now goes through
// CS Portal's own /accounts and /projects routes directly.
type SplAccountHandler struct {
	sn                  splAccountClient
	allowedGroups       []string
	addEscalationGroups []string
}

// NewSplAccountHandler creates a SplAccountHandler.
func NewSplAccountHandler(sn splAccountClient, allowedGroups, addEscalationGroups []string) *SplAccountHandler {
	return &SplAccountHandler{sn: sn, allowedGroups: allowedGroups, addEscalationGroups: addEscalationGroups}
}

var escalationRequestSourceValues = map[string]bool{"Customer": true, "Internal": true}
var escalationReasonValues = map[string]bool{"Inactivity": true, "Lack Of Progress": true, "Customer Imposed Deadline": true}
var escalationSeverityValues = map[string]bool{"High Severity": true, "Medium Severity": true}

// parsePaginationParams parses required, non-negative "offset" and
// positive "limit" query params, matching the Ballerina resource
// functions' non-nilable int offset/'limit params (framework-rejected on
// missing/invalid there; validated explicitly here for the same effect).
func parsePaginationParams(w http.ResponseWriter, r *http.Request) (offset, limit int, ok bool) {
	q := r.URL.Query()
	offset, err := strconv.Atoi(q.Get("offset"))
	if err != nil || offset < 0 {
		writeError(w, http.StatusBadRequest, "offset must be a non-negative integer")
		return 0, 0, false
	}
	limit, err = strconv.Atoi(q.Get("limit"))
	if err != nil || limit < 1 {
		writeError(w, http.StatusBadRequest, "limit must be a positive integer")
		return 0, 0, false
	}
	return offset, limit, true
}

func optionalQueryParam(r *http.Request, key string) *string {
	if !r.URL.Query().Has(key) {
		return nil
	}
	v := r.URL.Query().Get(key)
	if v == "" {
		return nil
	}
	return &v
}

// GetAccountEscalations handles GET /spl/accounts/{accountId}/escalations.
func (h *SplAccountHandler) GetAccountEscalations(w http.ResponseWriter, r *http.Request) {
	user, ok := requireSPLGroups(w, r, h.allowedGroups)
	if !ok {
		return
	}

	accountID := r.PathValue("accountId")
	if accountID == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	offset, limit, ok := parsePaginationParams(w, r)
	if !ok {
		return
	}

	result, err := h.sn.GetEscalationsByAccount(r.Context(), accountID, offset, limit)
	if err != nil {
		if errors.Is(err, servicenow.ErrAccountNotFound) {
			writeError(w, http.StatusNotFound, ErrMsgNotFound)
			return
		}
		if isUnsafeQueryValue(err) {
			writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
			return
		}
		slog.ErrorContext(r.Context(), "servicenow GetEscalationsByAccount failed", "userID", user.UserID, "accountID", accountID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve account escalations.")
		return
	}
	writeJSONValue(w, http.StatusOK, result)
}

// EscalateCase handles POST /spl/accounts/{accountId}/cases/{caseId}/escalate.
func (h *SplAccountHandler) EscalateCase(w http.ResponseWriter, r *http.Request) {
	user, ok := requireSPLGroups(w, r, h.allowedGroups)
	if !ok {
		return
	}
	if !requireSPLSubGroups(w, user, h.addEscalationGroups) {
		return
	}

	accountID := r.PathValue("accountId")
	caseID := r.PathValue("caseId")
	if accountID == "" || caseID == "" {
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

	var payload servicenow.EscalationRequest
	if err := json.Unmarshal(body, &payload); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if payload.Justification == "" || !escalationRequestSourceValues[payload.RequestSource] ||
		!escalationReasonValues[payload.Reason] || !escalationSeverityValues[payload.Severity] {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.sn.EscalateCase(r.Context(), accountID, caseID, payload, user.Email)
	if err != nil {
		if errors.Is(err, servicenow.ErrEscalationConflict) {
			writeError(w, http.StatusConflict, "Case has already been escalated.")
			return
		}
		if isUnsafeQueryValue(err) {
			writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
			return
		}
		slog.ErrorContext(r.Context(), "servicenow EscalateCase failed", "userID", user.UserID, "accountID", accountID, "caseID", caseID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to escalate case.")
		return
	}
	writeJSONValue(w, http.StatusOK, result)
}

// isUnsafeQueryValue reports whether err is a
// *servicenow.ErrUnsafeQueryValue, returned when a caller-supplied value
// fails SanitizeQueryValue.
func isUnsafeQueryValue(err error) bool {
	var unsafe *servicenow.ErrUnsafeQueryValue
	return errors.As(err, &unsafe)
}
