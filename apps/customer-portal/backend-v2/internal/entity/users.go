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

import "context"

// GetMe calls GET /users/me.
//
// NOTE: this route is only registered by entity-service when it is deployed
// with DATA_SOURCE=servicenow (see cs-tools/entity-service/internal/server/routes.go).
// A Postgres-mode deployment will 404 on this call.
func (c *Client) GetMe(ctx context.Context) (GetUserMeResponse, error) {
	var out GetUserMeResponse
	err := c.getJSON(ctx, "/users/me", &out)
	return out, err
}

// PatchMe calls PATCH /users/me to update the caller's timezone.
//
// NOTE: this route is only registered by entity-service when it is deployed
// with DATA_SOURCE=servicenow (see cs-tools/entity-service/internal/server/routes.go).
// A Postgres-mode deployment will 404 on this call.
func (c *Client) PatchMe(ctx context.Context, req PatchUserMeRequest) (PatchUserMeResponse, error) {
	var out PatchUserMeResponse
	err := c.patchJSON(ctx, "/users/me", req, &out)
	return out, err
}
