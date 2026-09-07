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

// SearchTimeCards calls POST /time-cards/search.
//
// NOTE: only entity-service's ServiceNow data source supports this route —
// see cs-tools/entity-service/internal/server/routes.go.
func (c *Client) SearchTimeCards(ctx context.Context, req SearchTimeCardsRequest) (SearchTimeCardsResponse, error) {
	var out SearchTimeCardsResponse
	err := c.postJSON(ctx, "/time-cards/search", req, &out)
	return out, err
}

// SearchCaseTimeCards calls POST /cases/time-cards/search — same request
// shape as SearchTimeCards, but results are grouped by case.
func (c *Client) SearchCaseTimeCards(ctx context.Context, req SearchTimeCardsRequest) (SearchCaseTimeCardsResponse, error) {
	var out SearchCaseTimeCardsResponse
	err := c.postJSON(ctx, "/cases/time-cards/search", req, &out)
	return out, err
}
