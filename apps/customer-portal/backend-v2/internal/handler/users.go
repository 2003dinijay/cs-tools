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
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/dto"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/scim"
)

// entityUserClient abstracts the entity-service user operations used by UserHandler.
type entityUserClient interface {
	GetMe(ctx context.Context) (entity.GetUserMeResponse, error)
	PatchMe(ctx context.Context, req entity.PatchUserMeRequest) (entity.PatchUserMeResponse, error)
}

// scimUserClient abstracts the SCIM operations used by UserHandler.
type scimUserClient interface {
	SearchUser(ctx context.Context, email string) (*scim.UserInfo, error)
	UpdateUserPhone(ctx context.Context, userID, mobile string) (*string, error)
}

// UserHandler handles HTTP requests for the logged-in user's own profile.
// Profile fields (name, timezone, roles) are sourced from entity-service;
// phone number is sourced from SCIM.
type UserHandler struct {
	entity entityUserClient
	scim   scimUserClient
}

// NewUserHandler creates a UserHandler backed by the given entity and SCIM clients.
func NewUserHandler(entity entityUserClient, scim scimUserClient) *UserHandler {
	return &UserHandler{entity: entity, scim: scim}
}

// userUpdateRequest is the PATCH /users/me request shape. At least one field
// must be set.
type userUpdateRequest struct {
	PhoneNumber *string `json:"phoneNumber,omitempty"`
	TimeZone    *string `json:"timeZone,omitempty"`
}

// GetMe handles GET /users/me.
func (h *UserHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	result, err := h.entity.GetMe(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetMe failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve user profile.")
		return
	}

	resp := dto.MapUserMe(result)

	// Phone number is sourced from SCIM, not entity-service. A SCIM lookup
	// failure or miss is not fatal to the request — the profile is still
	// useful without a phone number.
	scimInfo, scimErr := h.scim.SearchUser(r.Context(), user.Email)
	if scimErr != nil {
		slog.ErrorContext(r.Context(), "scim SearchUser failed", "userID", user.UserID, "err", summarizeErr(scimErr))
	} else if scimInfo == nil {
		slog.WarnContext(r.Context(), "no SCIM user found", "userID", user.UserID)
	} else {
		resp.PhoneNumber = scimInfo.PhoneNumber
		resp.LastPasswordUpdateTime = scimInfo.LastPasswordUpdateTime
	}

	writeJSONValue(w, http.StatusOK, resp)
}

// PatchMe handles PATCH /users/me. phoneNumber is updated via SCIM; timeZone
// is updated via entity-service. At least one field must be provided.
func (h *UserHandler) PatchMe(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var payload userUpdateRequest
	if err := json.Unmarshal(body, &payload); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if payload.PhoneNumber == nil && payload.TimeZone == nil {
		writeError(w, http.StatusBadRequest, "At least one field must be provided for update.")
		return
	}

	resp := dto.UserUpdateResponse{}

	if payload.PhoneNumber != nil {
		updatedPhone, err := h.scim.UpdateUserPhone(r.Context(), user.UserID, *payload.PhoneNumber)
		if err != nil {
			slog.ErrorContext(r.Context(), "scim UpdateUserPhone failed", "userID", user.UserID, "err", summarizeErr(err))
			mapUpstreamError(w, err, "Failed to update phone number.")
			return
		}
		resp.PhoneNumber = updatedPhone
	}

	if payload.TimeZone != nil {
		if _, err := h.entity.PatchMe(r.Context(), entity.PatchUserMeRequest{TimeZone: *payload.TimeZone}); err != nil {
			slog.ErrorContext(r.Context(), "entity PatchMe failed", "userID", user.UserID, "err", summarizeErr(err))
			mapUpstreamError(w, err, "Failed to update time zone.")
			return
		}
		resp.TimeZone = payload.TimeZone
	}

	writeJSONValue(w, http.StatusOK, resp)
}
