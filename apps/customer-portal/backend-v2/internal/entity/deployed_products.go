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

package entity

import (
	"context"
	"fmt"
	"net/url"
)

// SearchDeployedProducts calls POST /deployed-products/search.
func (c *Client) SearchDeployedProducts(ctx context.Context, req SearchDeployedProductsRequest) (SearchDeployedProductsResponse, error) {
	var out SearchDeployedProductsResponse
	err := c.postJSON(ctx, "/deployed-products/search", req, &out)
	return out, err
}

// CreateDeployedProduct calls POST /deployed-products.
//
// NOTE: only entity-service's ServiceNow data source supports this route —
// see CreateDeployedProductRequest's doc comment.
func (c *Client) CreateDeployedProduct(ctx context.Context, req CreateDeployedProductRequest) (CreateDeployedProductResponse, error) {
	var out CreateDeployedProductResponse
	err := c.postJSON(ctx, "/deployed-products", req, &out)
	return out, err
}

// UpdateDeployedProduct calls PATCH /deployed-products/{id}.
//
// NOTE: only entity-service's ServiceNow data source supports this route —
// see CreateDeployedProductRequest's doc comment.
func (c *Client) UpdateDeployedProduct(ctx context.Context, id string, req UpdateDeployedProductRequest) (UpdateDeployedProductResponse, error) {
	req.ID = id // never serialized (json:"-"); set for consistency with the struct's doc comment
	var out UpdateDeployedProductResponse
	err := c.patchJSON(ctx, fmt.Sprintf("/deployed-products/%s", url.PathEscape(id)), req, &out)
	return out, err
}

// SearchDeployedProductMetrics calls POST /deployed-products/{id}/metrics/search.
func (c *Client) SearchDeployedProductMetrics(ctx context.Context, id string, req DeployedProductMetricsRequest) (DeployedProductMetricsResponse, error) {
	var out DeployedProductMetricsResponse
	err := c.postJSON(ctx, fmt.Sprintf("/deployed-products/%s/metrics/search", url.PathEscape(id)), req, &out)
	return out, err
}

// SearchDeployedProductUsageCounts calls
// POST /deployed-products/{id}/metrics/usage-counts/search.
func (c *Client) SearchDeployedProductUsageCounts(ctx context.Context, id string, req DeployedProductUsageCountsRequest) (DeployedProductUsageCountsResponse, error) {
	var out DeployedProductUsageCountsResponse
	err := c.postJSON(ctx, fmt.Sprintf("/deployed-products/%s/metrics/usage-counts/search", url.PathEscape(id)), req, &out)
	return out, err
}
