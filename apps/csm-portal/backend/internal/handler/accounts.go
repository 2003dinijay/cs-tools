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

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/servicenow"
)

// entityAccountClient abstracts the entity service account operations used by AccountHandler.
type entityAccountClient interface {
	GetAccount(ctx context.Context, id string) ([]byte, error)
	SearchAccounts(ctx context.Context, body []byte) ([]byte, error)
	SearchAccountContacts(ctx context.Context, accountID string, body []byte) ([]byte, error)
	UpdateAccountTeams(ctx context.Context, id string, body []byte) ([]byte, error)
}

// AccountHandler handles HTTP requests for account operations, delegating to the
// entity service for data access.
type AccountHandler struct {
	entity entityAccountClient
}

// NewAccountHandler creates an AccountHandler backed by the given entity client.
func NewAccountHandler(entity entityAccountClient) *AccountHandler {
	return &AccountHandler{entity: entity}
}

// GetAccount handles GET /accounts/{id}.
func (h *AccountHandler) GetAccount(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.GetAccount(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetAccount failed", "userID", user.UserID, "accountID", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve account.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// SearchAccounts handles POST /accounts/search.
func (h *AccountHandler) SearchAccounts(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.SearchAccounts(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchAccounts failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to search accounts.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// SearchAccountContacts handles POST /accounts/{id}/contacts/search.
// The endpoint is path-scoped, so the request body is capped and forwarded to the
// entity service as-is (no fields are injected) and the response is returned verbatim.
func (h *AccountHandler) SearchAccountContacts(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.SearchAccountContacts(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchAccountContacts failed", "userID", user.UserID, "accountID", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to search account contacts.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// UpdateAccountTeams handles PATCH /accounts/{id}: updates an account's CRE
// team and/or SRE team assignment. The endpoint is path-scoped, so the
// request body is capped and forwarded to the entity service as-is (no
// fields are injected) and the response is returned verbatim. Restricted to
// callers holding the "admin" role — enforced by the PermAdmin permission
// this route is registered with (see cmd/server/main.go), not by this
// handler; every route's access is decided at registration, not inline.
func (h *AccountHandler) UpdateAccountTeams(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.UpdateAccountTeams(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity UpdateAccountTeams failed", "userID", user.UserID, "accountID", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to update account teams.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// splAccountClient abstracts the ServiceNow operations used by
// SplAccountHandler.
type splAccountClient interface {
	GetAccounts(ctx context.Context, email, userType, phrase *string, offset, limit int, active bool) ([]servicenow.AccountDetails, error)
	GetAccountByID(ctx context.Context, accountNumber string) (servicenow.AccountDetails, error)
	GetProjectsByAccount(ctx context.Context, accountNumber string, offset, limit int) ([]servicenow.ProjectDetails, error)
	GetEscalationsByAccount(ctx context.Context, accountNumber string, offset, limit int) ([]servicenow.EscalationDetail, error)
	EscalateCase(ctx context.Context, accountNumber, caseNumber string, request servicenow.EscalationRequest, submittedByEmail string) (servicenow.EscalationResponse, error)
}

// SplAccountHandler handles HTTP requests for SupportPortalLite's
// ServiceNow-backed account endpoints.
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

// GetAccounts handles GET /spl/accounts.
func (h *SplAccountHandler) GetAccounts(w http.ResponseWriter, r *http.Request) {
	user, ok := requireSPLGroups(w, r, h.allowedGroups)
	if !ok {
		return
	}

	offset, limit, ok := parsePaginationParams(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	active, _ := strconv.ParseBool(q.Get("active"))

	result, err := h.sn.GetAccounts(r.Context(), optionalQueryParam(r, "email"), optionalQueryParam(r, "userType"), optionalQueryParam(r, "phrase"), offset, limit, active)
	if err != nil {
		if isUnsafeQueryValue(err) {
			writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
			return
		}
		slog.ErrorContext(r.Context(), "servicenow GetAccounts failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve accounts.")
		return
	}
	writeJSONValue(w, http.StatusOK, result)
}

// GetAccountByID handles GET /spl/accounts/{accountId}.
func (h *SplAccountHandler) GetAccountByID(w http.ResponseWriter, r *http.Request) {
	user, ok := requireSPLGroups(w, r, h.allowedGroups)
	if !ok {
		return
	}

	accountID := r.PathValue("accountId")
	if accountID == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.sn.GetAccountByID(r.Context(), accountID)
	if err != nil {
		if errors.Is(err, servicenow.ErrAccountNotFound) {
			writeError(w, http.StatusNotFound, ErrMsgNotFound)
			return
		}
		if isUnsafeQueryValue(err) {
			writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
			return
		}
		slog.ErrorContext(r.Context(), "servicenow GetAccountByID failed", "userID", user.UserID, "accountID", accountID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve account.")
		return
	}
	writeJSONValue(w, http.StatusOK, result)
}

// GetAccountProjects handles GET /spl/accounts/{accountId}/projects.
func (h *SplAccountHandler) GetAccountProjects(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.sn.GetProjectsByAccount(r.Context(), accountID, offset, limit)
	if err != nil {
		if errors.Is(err, servicenow.ErrAccountNotFound) {
			writeError(w, http.StatusNotFound, ErrMsgNotFound)
			return
		}
		if isUnsafeQueryValue(err) {
			writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
			return
		}
		slog.ErrorContext(r.Context(), "servicenow GetProjectsByAccount failed", "userID", user.UserID, "accountID", accountID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve account projects.")
		return
	}
	writeJSONValue(w, http.StatusOK, result)
}

// GetAccountEscalations handles GET /accounts/{accountId}/escalations.
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

// EscalateCase handles POST /accounts/{accountId}/cases/{caseId}/escalate.
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
		if errors.Is(err, servicenow.ErrAccountNotFound) || errors.Is(err, servicenow.ErrCaseNotFound) {
			writeError(w, http.StatusNotFound, ErrMsgNotFound)
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
