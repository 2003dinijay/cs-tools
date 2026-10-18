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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type mockCatalogClient struct {
	products         func(body []byte) ([]byte, error)
	versions         func(productID string, body []byte) ([]byte, error)
	deployments      func(body []byte) ([]byte, error)
	deployedProducts func(body []byte) ([]byte, error)
}

func (m *mockCatalogClient) SearchProducts(_ context.Context, b []byte) ([]byte, error) {
	return m.products(b)
}
func (m *mockCatalogClient) SearchProductVersions(_ context.Context, id string, b []byte) ([]byte, error) {
	return m.versions(id, b)
}
func (m *mockCatalogClient) SearchDeployments(_ context.Context, b []byte) ([]byte, error) {
	return m.deployments(b)
}
func (m *mockCatalogClient) SearchDeployedProducts(_ context.Context, b []byte) ([]byte, error) {
	return m.deployedProducts(b)
}

const catalogUUID = "11111111-1111-1111-1111-111111111111"

func serveCatalog(m *mockCatalogClient, target, body string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	h := NewCatalogHandler(m)
	mux.HandleFunc("POST /products/search", h.SearchProducts)
	mux.HandleFunc("POST /products/{id}/versions/search", h.SearchProductVersions)
	mux.HandleFunc("POST /deployments/search", h.SearchDeployments)
	mux.HandleFunc("POST /deployed-products/search", h.SearchDeployedProducts)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodPost, target, strings.NewReader(body)))
	return w
}

// catalogCases lists each search endpoint with the mock wiring that records its upstream body.
func catalogCases(got *[]byte, resp string, err error) []struct {
	name, target, failMsg string
	mock                  *mockCatalogClient
} {
	rec := func(b []byte) ([]byte, error) { *got = b; return []byte(resp), err }
	return []struct {
		name, target, failMsg string
		mock                  *mockCatalogClient
	}{
		{"products", "/products/search", "Failed to search products.", &mockCatalogClient{products: rec}},
		{"versions", "/products/" + catalogUUID + "/versions/search", "Failed to search product versions.",
			&mockCatalogClient{versions: func(_ string, b []byte) ([]byte, error) { return rec(b) }}},
		{"deployments", "/deployments/search", "Failed to search deployments.", &mockCatalogClient{deployments: rec}},
		{"deployed-products", "/deployed-products/search", "Failed to search deployed products.", &mockCatalogClient{deployedProducts: rec}},
	}
}

func TestCatalog_ForwardsBodyAndReturnsResponseVerbatim(t *testing.T) {
	const payload = `{"pagination":{"limit":5,"offset":0},"searchQuery":"x"}`
	const upstream = `{"items":[{"id":"a"}],"total":1,"hasMore":false,"extra":{"kept":true}}`
	var got []byte
	for _, tc := range catalogCases(&got, upstream, nil) {
		t.Run(tc.name, func(t *testing.T) {
			got = nil
			w := serveCatalog(tc.mock, tc.target, payload)
			assertStatus(t, w, http.StatusOK)
			assertContentType(t, w, "application/json")
			if string(got) != payload {
				t.Errorf("upstream body = %q, want %q", got, payload)
			}
			if w.Body.String() != upstream {
				t.Errorf("response = %q, want upstream bytes %q", w.Body.String(), upstream)
			}
		})
	}
}

func TestCatalog_EmptyBodyIsForwarded(t *testing.T) {
	var got []byte
	for _, tc := range catalogCases(&got, `{}`, nil) {
		t.Run(tc.name, func(t *testing.T) {
			assertStatus(t, serveCatalog(tc.mock, tc.target, ""), http.StatusOK)
		})
	}
}

func TestCatalog_RejectsBadBodies(t *testing.T) {
	var got []byte
	for _, tc := range catalogCases(&got, `{}`, nil) {
		t.Run(tc.name+"/invalid json", func(t *testing.T) {
			w := serveCatalog(tc.mock, tc.target, "not-json")
			assertStatus(t, w, http.StatusBadRequest)
			assertErrorMessage(t, w, ErrMsgBadRequest)
		})
		t.Run(tc.name+"/too large", func(t *testing.T) {
			w := serveCatalog(tc.mock, tc.target, strings.Repeat("x", maxRequestBodyBytes+1))
			assertStatus(t, w, http.StatusRequestEntityTooLarge)
			assertErrorMessage(t, w, ErrMsgTooLarge)
		})
	}
}

func TestCatalog_VersionsRejectsNonUUIDProductID(t *testing.T) {
	w := serveCatalog(&mockCatalogClient{}, "/products/not-a-uuid/versions/search", `{}`)
	assertStatus(t, w, http.StatusBadRequest)
	assertErrorMessage(t, w, ErrMsgInvalidUUID)
}

func TestCatalog_VersionsForwardsProductID(t *testing.T) {
	var gotID string
	m := &mockCatalogClient{versions: func(id string, _ []byte) ([]byte, error) {
		gotID = id
		return []byte(`{}`), nil
	}}
	assertStatus(t, serveCatalog(m, "/products/"+catalogUUID+"/versions/search", `{}`), http.StatusOK)
	if gotID != catalogUUID {
		t.Errorf("product id = %q, want %q", gotID, catalogUUID)
	}
}

func TestCatalog_UpstreamErrors(t *testing.T) {
	var sink []byte
	for i, base := range catalogCases(&sink, ``, nil) {
		for _, tc := range upstreamErrors(base.failMsg) {
			t.Run(base.name+"/"+tc.name, func(t *testing.T) {
				c := catalogCases(&sink, ``, tc.err)[i]
				w := serveCatalog(c.mock, c.target, `{}`)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
			})
		}
	}
}
