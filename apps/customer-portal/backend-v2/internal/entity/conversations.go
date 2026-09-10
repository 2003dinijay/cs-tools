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

// SearchConversations calls POST /conversations/search.
//
// NOTE: only entity-service's ServiceNow data source supports this route —
// see cs-tools/entity-service/internal/server/routes.go.
func (c *Client) SearchConversations(ctx context.Context, req SearchConversationsRequest) (SearchConversationsResponse, error) {
	var out SearchConversationsResponse
	err := c.postJSON(ctx, "/conversations/search", req, &out)
	return out, err
}

// GetConversation calls GET /conversations/{id}.
func (c *Client) GetConversation(ctx context.Context, id string) (ConversationDetails, error) {
	var out ConversationDetails
	err := c.getJSON(ctx, fmt.Sprintf("/conversations/%s", url.PathEscape(id)), &out)
	return out, err
}

// CreateConversation calls POST /conversations.
func (c *Client) CreateConversation(ctx context.Context, req CreateConversationRequest) (CreateConversationResponse, error) {
	var out CreateConversationResponse
	err := c.postJSON(ctx, "/conversations", req, &out)
	return out, err
}

// UpdateConversation calls PATCH /conversations/{id}.
func (c *Client) UpdateConversation(ctx context.Context, id string, req UpdateConversationRequest) (UpdateConversationResponse, error) {
	var out UpdateConversationResponse
	err := c.patchJSON(ctx, fmt.Sprintf("/conversations/%s", url.PathEscape(id)), req, &out)
	return out, err
}
