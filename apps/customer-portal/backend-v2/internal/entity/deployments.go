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

// SearchDeployments calls POST /deployments/search.
func (c *Client) SearchDeployments(ctx context.Context, req SearchDeploymentsRequest) (SearchDeploymentsResponse, error) {
	var out SearchDeploymentsResponse
	err := c.postJSON(ctx, "/deployments/search", req, &out)
	return out, err
}

// CreateDeployment calls POST /deployments.
//
// NOTE: only entity-service's ServiceNow data source supports this route —
// see CreateDeploymentRequest's doc comment.
func (c *Client) CreateDeployment(ctx context.Context, req CreateDeploymentRequest) (CreateDeploymentResponse, error) {
	var out CreateDeploymentResponse
	err := c.postJSON(ctx, "/deployments", req, &out)
	return out, err
}

// UpdateDeployment calls PATCH /deployments/{id}.
func (c *Client) UpdateDeployment(ctx context.Context, id string, req UpdateDeploymentRequest) (UpdateDeploymentResponse, error) {
	req.ID = id // never serialized (json:"-"); set for consistency with the struct's doc comment
	var out UpdateDeploymentResponse
	err := c.patchJSON(ctx, fmt.Sprintf("/deployments/%s", url.PathEscape(id)), req, &out)
	return out, err
}
