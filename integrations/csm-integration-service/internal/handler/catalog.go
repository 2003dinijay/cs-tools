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
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

// Limits of the ServiceNow-shaped API this file reproduces, versus entity-service's.
const (
	catalogDefaultLimit = 10
	catalogMaxLimit     = 100
	// entityMaxPageSize is entity-service's page cap; a larger caller limit is
	// served by several upstream calls.
	entityMaxPageSize = 50
	maxSfIDs          = 100
	// maxSfScopedDeployments bounds the deployments gathered to scope a deployed-products search by sfIds.
	maxSfScopedDeployments = 5000
)

const (
	errMsgInvalidLimit  = "Invalid limit. It must be a positive integer."
	errMsgInvalidOffset = "Invalid offset. It must be a non-negative integer."
	errMsgInvalidClass  = "Invalid class. It must be one of product_model, software_model, service_model."
	errMsgTooManySfIDs  = "Too many sfIds."
)

// entityCatalogClient abstracts the entity service operations used by CatalogHandler.
type entityCatalogClient interface {
	SearchProducts(ctx context.Context, body []byte) ([]byte, error)
	SearchProductVersions(ctx context.Context, productID string, body []byte) ([]byte, error)
	SearchDeployments(ctx context.Context, body []byte) ([]byte, error)
	SearchDeployedProducts(ctx context.Context, body []byte) ([]byte, error)
	SearchProjects(ctx context.Context, body []byte) ([]byte, error)
}

// CatalogHandler serves the product, product-version, deployment and
// deployed-product reads in the response shape of the ServiceNow scripted REST
// API ("SF WSO2 Product & Deployment API") that third-party consumers already
// integrate against, built from entity-service's responses.
//
// Unlike most handlers here this one reshapes upstream bodies, on purpose: it
// is a compatibility facade (see CloudStatusHandler for the other one), so
// entity-service stays a clean internal API. Fields entity-service cannot supply
// are returned as null (deployment.version, productVersion.abbreviation,
// productVersion.product.name) or empty (products.warnings).
type CatalogHandler struct {
	entity entityCatalogClient
}

// NewCatalogHandler creates a CatalogHandler backed by the given entity client.
func NewCatalogHandler(entity entityCatalogClient) *CatalogHandler {
	return &CatalogHandler{entity: entity}
}

// ----- shared helpers -----

// parseLimitOffset reads ?limit (default 10, 1..100) and ?offset (default 0, >= 0).
func parseLimitOffset(r *http.Request) (limit, offset int, msg string) {
	limit, offset = catalogDefaultLimit, 0
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > catalogMaxLimit {
			return 0, 0, errMsgInvalidLimit
		}
		limit = n
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return 0, 0, errMsgInvalidOffset
		}
		offset = n
	}
	return limit, offset, ""
}

// parseSfIDs splits ?sfIds into a de-duplicated list. present is false when the
// parameter was absent or blank, meaning "no project filter".
func parseSfIDs(r *http.Request) (ids []string, present bool, msg string) {
	raw := r.URL.Query().Get("sfIds")
	seen := map[string]struct{}{}
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, dup := seen[p]; dup {
			continue
		}
		seen[p] = struct{}{}
		ids = append(ids, p)
	}
	if len(ids) > maxSfIDs {
		return nil, false, errMsgTooManySfIDs
	}
	return ids, len(ids) > 0, ""
}

// page is one upstream result page.
type page[T any] struct {
	items   []T
	total   int
	hasMore bool
}

// fetchWindow returns limit items starting at offset, issuing as many
// entity-service calls as its page cap requires. total is the upstream total.
func fetchWindow[T any](limit, offset int, fetch func(limit, offset int) (page[T], error)) ([]T, int, error) {
	items := []T{}
	total := 0
	for len(items) < limit {
		n := min(entityMaxPageSize, limit-len(items))
		p, err := fetch(n, offset+len(items))
		if err != nil {
			return nil, 0, err
		}
		total = p.total
		items = append(items, p.items...)
		if !p.hasMore || len(p.items) == 0 {
			break
		}
	}
	return items, total, nil
}

func writeEnvelope(w http.ResponseWriter, data any) {
	body, err := json.Marshal(map[string]any{"success": true, "data": data})
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}
	writeJSON(w, http.StatusOK, body)
}

func (h *CatalogHandler) call(ctx context.Context, fn func() ([]byte, error), out any) error {
	raw, err := fn()
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

// ----- sfIds -> project ids -----

type entityProjectsPage struct {
	Projects []struct {
		ID string `json:"id"`
	} `json:"projects"`
	HasMore bool `json:"hasMore"`
}

// resolveProjectIDs maps Salesforce ids to project UUIDs using entity-service's
// sfIds project filter; ids matching no project are simply absent from the result.
func (h *CatalogHandler) resolveProjectIDs(ctx context.Context, sfIDs []string) ([]string, error) {
	ids, _, err := fetchWindow(len(sfIDs), 0, func(n, o int) (page[string], error) {
		body, _ := json.Marshal(map[string]any{
			"sfIds":      sfIDs,
			"pagination": map[string]int{"limit": n, "offset": o},
		})
		var p entityProjectsPage
		if err := h.call(ctx, func() ([]byte, error) { return h.entity.SearchProjects(ctx, body) }, &p); err != nil {
			return page[string]{}, err
		}
		out := make([]string, 0, len(p.Projects))
		for _, pr := range p.Projects {
			out = append(out, pr.ID)
		}
		return page[string]{items: out, hasMore: p.HasMore}, nil
	})
	return ids, err
}

// ----- GET /products/search -----

type entityProductsPage struct {
	Products []struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Class string `json:"class"`
	} `json:"products"`
	Total   int  `json:"total"`
	HasMore bool `json:"hasMore"`
}

type productView struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Class string `json:"class"`
}

var validProductClasses = map[string]struct{}{"product_model": {}, "software_model": {}, "service_model": {}}

// SearchProducts handles GET /products/search.
func (h *CatalogHandler) SearchProducts(w http.ResponseWriter, r *http.Request) {
	limit, offset, msg := parseLimitOffset(r)
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	class := r.URL.Query().Get("class")
	if class != "" {
		if _, ok := validProductClasses[class]; !ok {
			writeError(w, http.StatusBadRequest, errMsgInvalidClass)
			return
		}
	}

	items, total, err := fetchWindow(limit, offset, func(n, o int) (page[productView], error) {
		req := map[string]any{"pagination": map[string]int{"limit": n, "offset": o}}
		if class != "" {
			req["class"] = class
		}
		body, _ := json.Marshal(req)
		var p entityProductsPage
		if err := h.call(r.Context(), func() ([]byte, error) { return h.entity.SearchProducts(r.Context(), body) }, &p); err != nil {
			return page[productView]{}, err
		}
		out := make([]productView, 0, len(p.Products))
		for _, x := range p.Products {
			out = append(out, productView{ID: x.ID, Name: x.Name, Class: x.Class})
		}
		return page[productView]{items: out, total: p.Total, hasMore: p.HasMore}, nil
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchProducts failed", "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search products.")
		return
	}

	writeEnvelope(w, map[string]any{
		"products": items, "total": total, "offset": offset, "limit": limit,
		"warnings": []string{},
	})
}

// ----- GET /products/{id}/versions/search -----

type entityVersionsPage struct {
	ProductVersions []struct {
		ID                             string  `json:"id"`
		Version                        *string `json:"version"`
		CurrentSupportStatus           *string `json:"currentSupportStatus"`
		ReleaseDate                    *string `json:"releaseDate"`
		SupportEOLDate                 *string `json:"supportEolDate"`
		EarliestPossibleSupportEOLDate *string `json:"earliestPossibleSupportEolDate"`
	} `json:"productVersions"`
	Total   int  `json:"total"`
	HasMore bool `json:"hasMore"`
}

type productRefView struct {
	ID   string  `json:"id"`
	Name *string `json:"name"`
}

type productVersionView struct {
	ID                             string         `json:"id"`
	Version                        *string        `json:"version"`
	CurrentSupportStatus           *string        `json:"currentSupportStatus"`
	ReleaseDate                    *string        `json:"releaseDate"`
	SupportEOLDate                 *string        `json:"supportEolDate"`
	EarliestPossibleSupportEOLDate *string        `json:"earliestPossibleSupportEolDate"`
	Abbreviation                   *string        `json:"abbreviation"`
	Product                        productRefView `json:"product"`
}

// SearchProductVersions handles GET /products/{id}/versions/search.
func (h *CatalogHandler) SearchProductVersions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	limit, offset, msg := parseLimitOffset(r)
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	items, total, err := fetchWindow(limit, offset, func(n, o int) (page[productVersionView], error) {
		body, _ := json.Marshal(map[string]any{"pagination": map[string]int{"limit": n, "offset": o}})
		var p entityVersionsPage
		if err := h.call(r.Context(), func() ([]byte, error) { return h.entity.SearchProductVersions(r.Context(), id, body) }, &p); err != nil {
			return page[productVersionView]{}, err
		}
		out := make([]productVersionView, 0, len(p.ProductVersions))
		for _, v := range p.ProductVersions {
			out = append(out, productVersionView{
				ID: v.ID, Version: v.Version, CurrentSupportStatus: v.CurrentSupportStatus,
				ReleaseDate: v.ReleaseDate, SupportEOLDate: v.SupportEOLDate,
				EarliestPossibleSupportEOLDate: v.EarliestPossibleSupportEOLDate,
				Product:                        productRefView{ID: id},
			})
		}
		return page[productVersionView]{items: out, total: p.Total, hasMore: p.HasMore}, nil
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchProductVersions failed", "productID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search product versions.")
		return
	}

	writeEnvelope(w, map[string]any{"versions": items, "total": total, "offset": offset, "limit": limit})
}

// ----- GET /deployments/search -----

type entityDeploymentsPage struct {
	Deployments []struct {
		ID          string  `json:"id"`
		Number      *string `json:"number"`
		Name        *string `json:"name"`
		Type        string  `json:"type"`
		Description *string `json:"description"`
		URL         *string `json:"url"`
		CreatedOn   *string `json:"createdOn"`
		UpdatedOn   *string `json:"updatedOn"`
		Project     *struct {
			ID   string  `json:"id"`
			Name *string `json:"name"`
		} `json:"project"`
	} `json:"deployments"`
	Total   int  `json:"total"`
	HasMore bool `json:"hasMore"`
}

type deploymentTypeView struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
}

type deploymentView struct {
	ID          string              `json:"id"`
	Number      *string             `json:"number"`
	Name        *string             `json:"name"`
	Description *string             `json:"description"`
	URL         *string             `json:"url"`
	Version     *string             `json:"version"`
	CreatedOn   *string             `json:"createdOn"`
	UpdatedOn   *string             `json:"updatedOn"`
	Project     *productRefView     `json:"project"`
	Type        *deploymentTypeView `json:"type"`
}

// deploymentTypes maps entity-service's type enum to ServiceNow's type key and label.
var deploymentTypes = map[string]deploymentTypeView{
	"development":        {1, "Development"},
	"qa":                 {2, "QA"},
	"staging":            {3, "Staging"},
	"stress":             {4, "Stress"},
	"uat":                {5, "UAT"},
	"primary_production": {6, "Primary Production"},
}

func (h *CatalogHandler) fetchDeployments(ctx context.Context, projectIDs []string, limit, offset int) ([]deploymentView, int, error) {
	return fetchWindow(limit, offset, func(n, o int) (page[deploymentView], error) {
		req := map[string]any{"pagination": map[string]int{"limit": n, "offset": o}}
		if projectIDs != nil {
			req["projectIds"] = projectIDs
		}
		body, _ := json.Marshal(req)
		var p entityDeploymentsPage
		if err := h.call(ctx, func() ([]byte, error) { return h.entity.SearchDeployments(ctx, body) }, &p); err != nil {
			return page[deploymentView]{}, err
		}
		out := make([]deploymentView, 0, len(p.Deployments))
		for _, d := range p.Deployments {
			v := deploymentView{
				ID: d.ID, Number: d.Number, Name: d.Name, Description: d.Description, URL: d.URL,
				CreatedOn: d.CreatedOn, UpdatedOn: d.UpdatedOn,
			}
			if d.Project != nil {
				v.Project = &productRefView{ID: d.Project.ID, Name: d.Project.Name}
			}
			if t, ok := deploymentTypes[d.Type]; ok {
				v.Type = &t
			}
			out = append(out, v)
		}
		return page[deploymentView]{items: out, total: p.Total, hasMore: p.HasMore}, nil
	})
}

func writeEmptyList(w http.ResponseWriter, key string, totalKey string, limit, offset int) {
	writeEnvelope(w, map[string]any{key: []any{}, totalKey: 0, "offset": offset, "limit": limit})
}

// SearchDeployments handles GET /deployments/search.
func (h *CatalogHandler) SearchDeployments(w http.ResponseWriter, r *http.Request) {
	limit, offset, msg := parseLimitOffset(r)
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	sfIDs, filtered, msg := parseSfIDs(r)
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	var projectIDs []string
	if filtered {
		var err error
		if projectIDs, err = h.resolveProjectIDs(r.Context(), sfIDs); err != nil {
			slog.ErrorContext(r.Context(), "entity SearchProjects failed", "err", summarizeErr(err))
			mapUpstreamError(w, err, "Failed to search deployments.")
			return
		}
		if len(projectIDs) == 0 {
			writeEmptyList(w, "deployments", "totalRecords", limit, offset)
			return
		}
	}

	items, total, err := h.fetchDeployments(r.Context(), projectIDs, limit, offset)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchDeployments failed", "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search deployments.")
		return
	}
	writeEnvelope(w, map[string]any{"deployments": items, "totalRecords": total, "offset": offset, "limit": limit})
}

// ----- GET /deployed-products/search -----

type entityDeployedProductsPage struct {
	DeployedProducts []struct {
		ID         string `json:"id"`
		Deployment *struct {
			ID   string  `json:"id"`
			Name *string `json:"name"`
		} `json:"deployment"`
		Product *struct {
			ID           string  `json:"id"`
			Name         *string `json:"name"`
			Abbreviation *string `json:"abbreviation"`
		} `json:"product"`
		Version *struct {
			ID           string  `json:"id"`
			Name         *string `json:"name"`
			ReleasedDate *string `json:"releasedDate"`
			SupportEoL   *string `json:"supportEoLDate"`
		} `json:"version"`
		Cores       json.RawMessage `json:"cores"`
		TPS         json.RawMessage `json:"tps"`
		Category    json.RawMessage `json:"category"`
		Description *string         `json:"description"`
		Updates     []struct {
			UpdateLevel json.Number `json:"updateLevel"`
			Date        *string     `json:"date"`
			Details     *string     `json:"details"`
		} `json:"updates"`
		CreatedOn *string `json:"createdOn"`
		UpdatedOn *string `json:"updatedOn"`
	} `json:"deployedProducts"`
	Total   int  `json:"total"`
	HasMore bool `json:"hasMore"`
}

type productUpdateView struct {
	UpdateLevel *string `json:"updateLevel"`
	Date        *string `json:"date"`
	Details     *string `json:"details"`
}

type deployedProductView struct {
	ID          string  `json:"id"`
	Description *string `json:"description"`
	Version     *struct {
		ID   string  `json:"id"`
		Name *string `json:"name"`
	} `json:"version"`
	Cores       json.RawMessage     `json:"cores"`
	TPS         json.RawMessage     `json:"tps"`
	ReleasedOn  *string             `json:"releasedOn"`
	EndOfLifeOn *string             `json:"endOfLifeOn"`
	Updates     []productUpdateView `json:"updates"`
	CreatedOn   *string             `json:"createdOn"`
	UpdatedOn   *string             `json:"updatedOn"`
	Deployment  *productRefView     `json:"deployment"`
	Product     *struct {
		ID           string  `json:"id"`
		Name         *string `json:"name"`
		Abbreviation *string `json:"abbreviation"`
	} `json:"product"`
	Category json.RawMessage `json:"category"`
}

func nullIfAbsent(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("null")
	}
	return raw
}

// SearchDeployedProducts handles GET /deployed-products/search.
func (h *CatalogHandler) SearchDeployedProducts(w http.ResponseWriter, r *http.Request) {
	limit, offset, msg := parseLimitOffset(r)
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	sfIDs, filtered, msg := parseSfIDs(r)
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	// entity-service filters deployed products by deployment, so sfIds become
	// projects, then those projects' deployments.
	var deploymentIDs []string
	if filtered {
		projectIDs, err := h.resolveProjectIDs(r.Context(), sfIDs)
		if err != nil {
			slog.ErrorContext(r.Context(), "entity SearchProjects failed", "err", summarizeErr(err))
			mapUpstreamError(w, err, "Failed to search deployed products.")
			return
		}
		if len(projectIDs) > 0 {
			deps, _, err := h.fetchDeployments(r.Context(), projectIDs, maxSfScopedDeployments, 0)
			if err != nil {
				slog.ErrorContext(r.Context(), "entity SearchDeployments failed", "err", summarizeErr(err))
				mapUpstreamError(w, err, "Failed to search deployed products.")
				return
			}
			for _, d := range deps {
				deploymentIDs = append(deploymentIDs, d.ID)
			}
		}
		if len(deploymentIDs) == 0 {
			writeEmptyList(w, "deployedProducts", "totalRecords", limit, offset)
			return
		}
	}

	items, total, err := fetchWindow(limit, offset, func(n, o int) (page[deployedProductView], error) {
		req := map[string]any{"pagination": map[string]int{"limit": n, "offset": o}}
		if deploymentIDs != nil {
			req["deploymentIds"] = deploymentIDs
		}
		body, _ := json.Marshal(req)
		var p entityDeployedProductsPage
		if err := h.call(r.Context(), func() ([]byte, error) { return h.entity.SearchDeployedProducts(r.Context(), body) }, &p); err != nil {
			return page[deployedProductView]{}, err
		}
		out := make([]deployedProductView, 0, len(p.DeployedProducts))
		for _, d := range p.DeployedProducts {
			v := deployedProductView{
				ID: d.ID, Description: d.Description, Cores: nullIfAbsent(d.Cores), TPS: nullIfAbsent(d.TPS),
				Category: nullIfAbsent(d.Category), CreatedOn: d.CreatedOn, UpdatedOn: d.UpdatedOn,
				Product: d.Product,
			}
			if d.Deployment != nil {
				v.Deployment = &productRefView{ID: d.Deployment.ID, Name: d.Deployment.Name}
			}
			if d.Version != nil {
				v.Version = &struct {
					ID   string  `json:"id"`
					Name *string `json:"name"`
				}{d.Version.ID, d.Version.Name}
				v.ReleasedOn, v.EndOfLifeOn = d.Version.ReleasedDate, d.Version.SupportEoL
			}
			if d.Updates != nil {
				v.Updates = make([]productUpdateView, 0, len(d.Updates))
				for _, u := range d.Updates {
					pu := productUpdateView{Date: u.Date, Details: u.Details}
					if u.UpdateLevel != "" {
						lvl := fmt.Sprint(u.UpdateLevel)
						pu.UpdateLevel = &lvl
					}
					v.Updates = append(v.Updates, pu)
				}
			}
			out = append(out, v)
		}
		return page[deployedProductView]{items: out, total: p.Total, hasMore: p.HasMore}, nil
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchDeployedProducts failed", "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search deployed products.")
		return
	}
	writeEnvelope(w, map[string]any{"deployedProducts": items, "totalRecords": total, "offset": offset, "limit": limit})
}
