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

// CreateChangeRequest calls POST /change-requests.
//
// NOTE: entity-service only supports change requests on its ServiceNow data
// source — a Postgres-mode deployment 404s on every route in this file.
func (c *Client) CreateChangeRequest(ctx context.Context, req CreateChangeRequestRequest) (CreateChangeRequestResponse, error) {
	var out CreateChangeRequestResponse
	err := c.postJSON(ctx, "/change-requests", req, &out)
	return out, err
}

// SearchChangeRequests calls POST /change-requests/search.
func (c *Client) SearchChangeRequests(ctx context.Context, req SearchChangeRequestsRequest) (SearchChangeRequestsResponse, error) {
	var out SearchChangeRequestsResponse
	err := c.postJSON(ctx, "/change-requests/search", req, &out)
	return out, err
}

// GetChangeRequest calls GET /change-requests/{id}.
func (c *Client) GetChangeRequest(ctx context.Context, id string) (ChangeRequest, error) {
	var out ChangeRequest
	err := c.getJSON(ctx, fmt.Sprintf("/change-requests/%s", url.PathEscape(id)), &out)
	return out, err
}

// UpdateChangeRequest calls PATCH /change-requests/{id}.
func (c *Client) UpdateChangeRequest(ctx context.Context, id string, req PatchChangeRequestRequest) (PatchChangeRequestResponse, error) {
	var out PatchChangeRequestResponse
	err := c.patchJSON(ctx, fmt.Sprintf("/change-requests/%s", url.PathEscape(id)), req, &out)
	return out, err
}

// GetChangeRequestApprovals calls GET /change-requests/{id}/approvals.
func (c *Client) GetChangeRequestApprovals(ctx context.Context, id string) (ChangeRequestApprovals, error) {
	var out ChangeRequestApprovals
	err := c.getJSON(ctx, fmt.Sprintf("/change-requests/%s/approvals", url.PathEscape(id)), &out)
	return out, err
}

// DecideChangeRequestApproval calls POST /change-requests/{id}/approvals/decision.
func (c *Client) DecideChangeRequestApproval(ctx context.Context, id string, req ChangeRequestApprovalDecisionRequest) (ChangeRequestApprovalDecisionResponse, error) {
	var out ChangeRequestApprovalDecisionResponse
	err := c.postJSON(ctx, fmt.Sprintf("/change-requests/%s/approvals/decision", url.PathEscape(id)), req, &out)
	return out, err
}
