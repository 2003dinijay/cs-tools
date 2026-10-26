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
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// entityBannerClient is the entity-service calls the Banners tab and the CSM shell need.
type entityBannerClient interface {
	ListCSMBanners(ctx context.Context) ([]byte, error)
	ListManagedBanners(ctx context.Context) ([]byte, error)
	CreatePortalBanner(ctx context.Context, body []byte) ([]byte, error)
	UpdatePortalBanner(ctx context.Context, id string, body []byte) ([]byte, error)
}

// BannerHandler proxies portal banners. Entity-service decides which rows a
// portal may see and that only an admin may change them.
type BannerHandler struct {
	entity entityBannerClient
}

// NewBannerHandler creates a BannerHandler.
func NewBannerHandler(entity entityBannerClient) *BannerHandler {
	return &BannerHandler{entity: entity}
}

type bannerSave struct {
	Status                  string     `json:"status"`
	SortOrder               int        `json:"sortOrder"`
	VisibleInCustomerPortal bool       `json:"visibleInCustomerPortal"`
	VisibleInCsmPortal      bool       `json:"visibleInCsmPortal"`
	HTML                    string     `json:"html"`
	StartsAt                *time.Time `json:"startsAt"`
	ExpiresAt               *time.Time `json:"expiresAt"`
}

// ListCSMBanners handles GET /portal-banners.
func (h *BannerHandler) ListCSMBanners(w http.ResponseWriter, r *http.Request) {
	h.forward(w, r, "ListCSMBanners", "Failed to load banners.", http.StatusOK, h.entity.ListCSMBanners)
}

// ListManagedBanners handles GET /portal-banners/manage.
func (h *BannerHandler) ListManagedBanners(w http.ResponseWriter, r *http.Request) {
	h.forward(w, r, "ListManagedBanners", "Failed to load banners.", http.StatusOK, h.entity.ListManagedBanners)
}

// CreateBanner handles POST /portal-banners.
func (h *BannerHandler) CreateBanner(w http.ResponseWriter, r *http.Request) {
	body, ok := readBannerSave(w, r)
	if !ok {
		return
	}
	h.forward(w, r, "CreateBanner", "Failed to save the banner.", http.StatusCreated,
		func(ctx context.Context) ([]byte, error) { return h.entity.CreatePortalBanner(ctx, body) })
}

// UpdateBanner handles PUT /portal-banners/{id}.
func (h *BannerHandler) UpdateBanner(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	body, ok := readBannerSave(w, r)
	if !ok {
		return
	}
	h.forward(w, r, "UpdateBanner", "Failed to save the banner.", http.StatusOK,
		func(ctx context.Context) ([]byte, error) { return h.entity.UpdatePortalBanner(ctx, id, body) })
}

func readBannerSave(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return nil, false
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return nil, false
	}
	var req bannerSave
	if err := json.Unmarshal(raw, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return nil, false
	}
	switch strings.TrimSpace(req.Status) {
	case "draft", "published", "unpublished":
	default:
		writeError(w, http.StatusBadRequest, "Status must be draft, published, or unpublished.")
		return nil, false
	}
	if strings.TrimSpace(req.HTML) == "" || req.SortOrder < 0 {
		writeError(w, http.StatusBadRequest, "A banner needs HTML and an order of 0 or more.")
		return nil, false
	}
	req.Status = strings.TrimSpace(req.Status)
	req.HTML = strings.TrimSpace(req.HTML)
	return mustJSON(req), true
}

func (h *BannerHandler) forward(w http.ResponseWriter, r *http.Request, op, fallback string, okStatus int, call func(context.Context) ([]byte, error)) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	result, err := call(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "entity "+op+" failed", "err", err)
		var apiErr *apierror.Error
		if errors.As(err, &apiErr) && (apiErr.StatusCode == http.StatusBadRequest || apiErr.StatusCode == http.StatusNotFound || apiErr.StatusCode == http.StatusForbidden) {
			writeError(w, apiErr.StatusCode, upstreamErrorMessageStrict(apiErr.Body, fallback))
			return
		}
		mapUpstreamErrorGeneric(w, err, fallback)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, okStatus, result)
}
