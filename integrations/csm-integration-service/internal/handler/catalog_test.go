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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type mockCatalogClient struct {
	products, versions, deployments, deployedProducts, projects func(body []byte) ([]byte, error)
}

func (m *mockCatalogClient) SearchProducts(_ context.Context, b []byte) ([]byte, error) {
	return m.products(b)
}
func (m *mockCatalogClient) SearchProductVersions(_ context.Context, _ string, b []byte) ([]byte, error) {
	return m.versions(b)
}
func (m *mockCatalogClient) SearchDeployments(_ context.Context, b []byte) ([]byte, error) {
	return m.deployments(b)
}
func (m *mockCatalogClient) SearchDeployedProducts(_ context.Context, b []byte) ([]byte, error) {
	return m.deployedProducts(b)
}
func (m *mockCatalogClient) SearchProjects(_ context.Context, b []byte) ([]byte, error) {
	return m.projects(b)
}

const catalogUUID = "11111111-1111-1111-1111-111111111111"

func serveCatalog(t *testing.T, m *mockCatalogClient, pattern, target string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	h := NewCatalogHandler(m)
	mux.HandleFunc("GET /products/search", h.SearchProducts)
	mux.HandleFunc("GET /products/{id}/versions/search", h.SearchProductVersions)
	mux.HandleFunc("GET /deployments/search", h.SearchDeployments)
	mux.HandleFunc("GET /deployed-products/search", h.SearchDeployedProducts)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))
	return w
}

// projectsJSON stands in for entity-service's sfIds filter: SF001 is the only known id.
func projectsJSON(t *testing.T) func([]byte) ([]byte, error) {
	return func(b []byte) ([]byte, error) {
		var req struct {
			SfIDs []string `json:"sfIds"`
		}
		_ = json.Unmarshal(b, &req)
		for _, id := range req.SfIDs {
			if id == "SF001" {
				return []byte(`{"projects":[{"id":"p1"}],"total":1,"hasMore":false}`), nil
			}
		}
		return []byte(`{"projects":[],"total":0,"hasMore":false}`), nil
	}
}

func TestCatalog_InvalidParams(t *testing.T) {
	m := &mockCatalogClient{}
	for _, tc := range []struct{ target, msg string }{
		{"/products/search?limit=0", errMsgInvalidLimit},
		{"/products/search?limit=101", errMsgInvalidLimit},
		{"/deployments/search?limit=abc", errMsgInvalidLimit},
		{"/deployments/search?offset=-1", errMsgInvalidOffset},
		{"/products/search?class=bogus", errMsgInvalidClass},
		{"/products/not-a-uuid/versions/search", ErrMsgInvalidUUID},
	} {
		w := serveCatalog(t, m, "", tc.target)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, tc.msg)
	}
}

func TestCatalog_SearchProducts(t *testing.T) {
	var got map[string]any
	m := &mockCatalogClient{products: func(b []byte) ([]byte, error) {
		_ = json.Unmarshal(b, &got)
		return []byte(`{"products":[{"id":"a","name":"APIM","class":"Software Model","createdOn":"x"}],"total":1,"limit":10,"offset":0,"hasMore":false}`), nil
	}}
	w := serveCatalog(t, m, "", "/products/search?class=software_model&limit=5&offset=2")
	assertStatus(t, w, http.StatusOK)
	if got["class"] != "software_model" {
		t.Errorf("class not forwarded: %v", got)
	}
	resp := decodeJSON[map[string]any](t, w)
	data := resp["data"].(map[string]any)
	if resp["success"] != true || data["total"].(float64) != 1 || data["limit"].(float64) != 5 || data["offset"].(float64) != 2 {
		t.Errorf("unexpected envelope: %v", resp)
	}
	p := data["products"].([]any)[0].(map[string]any)
	if _, has := p["createdOn"]; has || p["name"] != "APIM" {
		t.Errorf("product not reshaped: %v", p)
	}
	if w, ok := data["warnings"].([]any); !ok || len(w) != 0 {
		t.Errorf("warnings = %v, want []", data["warnings"])
	}
}

func TestCatalog_LimitAbovePageCapUsesSeveralCalls(t *testing.T) {
	var calls []int
	m := &mockCatalogClient{products: func(b []byte) ([]byte, error) {
		var req struct {
			Pagination struct{ Limit, Offset int } `json:"pagination"`
		}
		_ = json.Unmarshal(b, &req)
		calls = append(calls, req.Pagination.Limit)
		items := make([]string, req.Pagination.Limit)
		for i := range items {
			items[i] = `{"id":"a","name":"n","class":"c"}`
		}
		return []byte(`{"products":[` + strings.Join(items, ",") + `],"total":500,"hasMore":true}`), nil
	}}
	w := serveCatalog(t, m, "", "/products/search?limit=100")
	assertStatus(t, w, http.StatusOK)
	if len(calls) != 2 || calls[0] != 50 || calls[1] != 50 {
		t.Errorf("upstream page sizes = %v, want [50 50]", calls)
	}
	data := decodeJSON[map[string]any](t, w)["data"].(map[string]any)
	if n := len(data["products"].([]any)); n != 100 {
		t.Errorf("got %d products, want 100", n)
	}
}

func TestCatalog_SearchProductVersions(t *testing.T) {
	m := &mockCatalogClient{versions: func([]byte) ([]byte, error) {
		return []byte(`{"productVersions":[{"id":"v1","productId":"x","version":"4.0.0","currentSupportStatus":"available","releaseDate":"2024-01-01","supportEolDate":null,"createdOn":"c"}],"total":1,"hasMore":false}`), nil
	}}
	w := serveCatalog(t, m, "", "/products/"+catalogUUID+"/versions/search")
	assertStatus(t, w, http.StatusOK)
	data := decodeJSON[map[string]any](t, w)["data"].(map[string]any)
	v := data["versions"].([]any)[0].(map[string]any)
	prod := v["product"].(map[string]any)
	if prod["id"] != catalogUUID || prod["name"] != nil || v["abbreviation"] != nil || v["supportEolDate"] != nil || v["version"] != "4.0.0" {
		t.Errorf("unexpected version: %v", v)
	}
	if _, has := v["createdOn"]; has {
		t.Errorf("createdOn leaked: %v", v)
	}
}

func TestCatalog_SearchDeployments(t *testing.T) {
	var got map[string]any
	m := &mockCatalogClient{
		projects: projectsJSON(t),
		deployments: func(b []byte) ([]byte, error) {
			_ = json.Unmarshal(b, &got)
			return []byte(`{"deployments":[{"id":"d1","number":"DEP1","name":"Prod","type":"primary_production","description":null,"url":null,"project":{"id":"p1","name":"Proj"},"createdOn":"c","updatedOn":"u","deployedProductCount":2}],"total":1,"hasMore":false}`), nil
		},
	}
	w := serveCatalog(t, m, "", "/deployments/search?sfIds=SF001,SF001,UNKNOWN")
	assertStatus(t, w, http.StatusOK)
	if ids, _ := got["projectIds"].([]any); len(ids) != 1 || ids[0] != "p1" {
		t.Errorf("projectIds = %v, want [p1]", got["projectIds"])
	}
	data := decodeJSON[map[string]any](t, w)["data"].(map[string]any)
	if data["totalRecords"].(float64) != 1 {
		t.Errorf("totalRecords = %v", data["totalRecords"])
	}
	d := data["deployments"].([]any)[0].(map[string]any)
	typ := d["type"].(map[string]any)
	if typ["id"].(float64) != 6 || typ["label"] != "Primary Production" || d["version"] != nil || d["project"].(map[string]any)["name"] != "Proj" {
		t.Errorf("unexpected deployment: %v", d)
	}
	if _, has := d["deployedProductCount"]; has {
		t.Errorf("deployedProductCount leaked: %v", d)
	}
}

func TestCatalog_UnknownSfIDsGiveEmptyNotUnfiltered(t *testing.T) {
	m := &mockCatalogClient{projects: projectsJSON(t)} // deployments func nil: must not be called
	for _, path := range []string{"/deployments/search", "/deployed-products/search"} {
		w := serveCatalog(t, m, "", path+"?sfIds=NOPE")
		assertStatus(t, w, http.StatusOK)
		data := decodeJSON[map[string]any](t, w)["data"].(map[string]any)
		if data["totalRecords"].(float64) != 0 {
			t.Errorf("%s: totalRecords = %v", path, data["totalRecords"])
		}
	}
}

func TestCatalog_SearchDeployedProducts(t *testing.T) {
	var got map[string]any
	m := &mockCatalogClient{
		projects: projectsJSON(t),
		deployments: func([]byte) ([]byte, error) {
			return []byte(`{"deployments":[{"id":"d1","type":"qa"},{"id":"d2","type":"uat"}],"total":2,"hasMore":false}`), nil
		},
		deployedProducts: func(b []byte) ([]byte, error) {
			_ = json.Unmarshal(b, &got)
			return []byte(`{"deployedProducts":[{"id":"dp1","deployment":{"id":"d1","name":"QA"},"product":{"id":"pr","name":"APIM","abbreviation":"wso2am"},"version":{"id":"v","name":"4.0.0","releasedDate":"2024-01-01","supportEoLDate":"2027-01-01"},"cores":4,"tps":12.5,"category":"pdp","description":null,"updates":[{"updateLevel":12,"date":"2025-01-01","details":null}],"createdOn":"c","updatedOn":"u"}],"total":1,"hasMore":false}`), nil
		},
	}
	w := serveCatalog(t, m, "", "/deployed-products/search?sfIds=SF001")
	assertStatus(t, w, http.StatusOK)
	if ids, _ := got["deploymentIds"].([]any); len(ids) != 2 {
		t.Errorf("deploymentIds = %v", got["deploymentIds"])
	}
	data := decodeJSON[map[string]any](t, w)["data"].(map[string]any)
	d := data["deployedProducts"].([]any)[0].(map[string]any)
	up := d["updates"].([]any)[0].(map[string]any)
	if d["releasedOn"] != "2024-01-01" || d["endOfLifeOn"] != "2027-01-01" || d["cores"].(float64) != 4 || d["tps"].(float64) != 12.5 ||
		d["category"] != "pdp" || up["updateLevel"] != "12" || d["version"].(map[string]any)["name"] != "4.0.0" ||
		d["product"].(map[string]any)["abbreviation"] != "wso2am" || d["deployment"].(map[string]any)["name"] != "QA" {
		t.Errorf("unexpected deployed product: %v", d)
	}
	if _, has := d["version"].(map[string]any)["releasedDate"]; has {
		t.Errorf("version not reduced to {id,name}: %v", d["version"])
	}
}

func TestCatalog_UpstreamErrors(t *testing.T) {
	for _, tc := range upstreamErrors("Failed to search products.") {
		t.Run(tc.name, func(t *testing.T) {
			m := &mockCatalogClient{products: func([]byte) ([]byte, error) { return nil, tc.err }}
			w := serveCatalog(t, m, "", "/products/search")
			assertStatus(t, w, tc.wantCode)
			assertErrorMessage(t, w, tc.wantMsg)
		})
	}
	m := &mockCatalogClient{products: func([]byte) ([]byte, error) { return []byte(`not json`), nil }}
	assertStatus(t, serveCatalog(t, m, "", "/products/search"), http.StatusInternalServerError)
}
