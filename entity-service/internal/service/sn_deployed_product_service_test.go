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

package service

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

var (
	testDeployedProductSysid       = sysid32('1')
	testDeployedProductDeploySysid = sysid32('2')
	testDeployedProductProdSysid   = sysid32('3')
	testDeployedProductCatSysid    = sysid32('4')
)

// TestSNDeployedProductService_SearchDeployedProducts_MapsCategoryFromReferenceObject
// guards against reintroducing a plain-string decode for the SN response's
// "category" field. It is a ReferenceTableItem ({id, name, ...}), not a
// string -- decoding it into *string broke every deployed-products search
// with "json: cannot unmarshal object into Go struct field ...category of
// type string".
func TestSNDeployedProductService_SearchDeployedProducts_MapsCategoryFromReferenceObject(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/deployed-products/search", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"deployedProducts": []map[string]any{
				{
					"id":         testDeployedProductSysid,
					"deployment": map[string]any{"id": testDeployedProductDeploySysid, "name": "Production"},
					"product":    map[string]any{"id": testDeployedProductProdSysid, "name": "API Manager"},
					"category":   map[string]any{"id": testDeployedProductCatSysid, "name": "Middleware"},
					"createdOn":  "2026-01-01 00:00:00",
					"updatedOn":  "2026-01-02 00:00:00",
				},
			},
			"totalRecords": 1, "offset": 0, "limit": 20,
		})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowDeployedProductService(client)

	resp, err := svc.SearchDeployedProducts(contextWithUserIDToken("token"), domain.SearchDeployedProductsRequest{
		Pagination: domain.Pagination{Limit: 20, Offset: 0},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.DeployedProducts) != 1 {
		t.Fatalf("expected 1 deployed product, got %d", len(resp.DeployedProducts))
	}
	got := resp.DeployedProducts[0]
	if got.Category == nil || *got.Category != "Middleware" {
		t.Fatalf("expected category %q, got %v", "Middleware", got.Category)
	}
}

func TestSNDeployedProductService_SearchDeployedProducts_NilCategoryStaysNil(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/deployed-products/search", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"deployedProducts": []map[string]any{
				{
					"id":         testDeployedProductSysid,
					"deployment": map[string]any{"id": testDeployedProductDeploySysid, "name": "Production"},
					"product":    map[string]any{"id": testDeployedProductProdSysid, "name": "API Manager"},
					"category":   nil,
					"createdOn":  "2026-01-01 00:00:00",
					"updatedOn":  "2026-01-02 00:00:00",
				},
			},
			"totalRecords": 1, "offset": 0, "limit": 20,
		})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowDeployedProductService(client)

	resp, err := svc.SearchDeployedProducts(contextWithUserIDToken("token"), domain.SearchDeployedProductsRequest{
		Pagination: domain.Pagination{Limit: 20, Offset: 0},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.DeployedProducts) != 1 {
		t.Fatalf("expected 1 deployed product, got %d", len(resp.DeployedProducts))
	}
	if resp.DeployedProducts[0].Category != nil {
		t.Fatalf("expected nil category, got %v", *resp.DeployedProducts[0].Category)
	}
}
