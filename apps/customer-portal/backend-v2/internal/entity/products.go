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

// SearchProducts calls POST /products/search.
func (c *Client) SearchProducts(ctx context.Context, req SearchProductsRequest) (SearchProductsResponse, error) {
	var out SearchProductsResponse
	err := c.postJSON(ctx, "/products/search", req, &out)
	return out, err
}

// SearchProductVersions calls POST /products/{id}/versions/search.
func (c *Client) SearchProductVersions(ctx context.Context, productID string, req SearchProductVersionsRequest) (SearchProductVersionsResponse, error) {
	req.ProductID = productID // never serialized (json:"-"); set for consistency with the struct's doc comment
	var out SearchProductVersionsResponse
	err := c.postJSON(ctx, fmt.Sprintf("/products/%s/versions/search", url.PathEscape(productID)), req, &out)
	return out, err
}
