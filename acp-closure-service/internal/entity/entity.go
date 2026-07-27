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
	"net/http"
	"net/url"
)

// GetAccount calls GET /accounts/{id}. Response is returned as raw JSON;
// typed response structs are deferred to the caller.
func (c *Client) GetAccount(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/accounts/%s", url.PathEscape(id)), nil)
}

// SearchProjects calls POST /projects/search. Response is returned as raw
// JSON; typed response structs are deferred to the caller.
//
// closureStatus/endDateFrom/endDateTo/sortBy/sortOrder are undocumented in
// csm-integration-service's openapi.yaml (which only lists
// pagination/searchQuery) but are confirmed working via direct Postman
// testing against staging — the handler forwards the request body verbatim
// to entity-service, which does honor them. The response body includes
// closureState, endDateClosureState, and endDate even though those fields
// are similarly undocumented — also confirmed via Postman.
//
// If body sets an explicit empty-string searchQuery, expect a 400 — omit
// the field entirely instead (confirmed quirk, same as calling
// entity-service directly: an empty JSON object is accepted, an explicit
// empty searchQuery is not).
func (c *Client) SearchProjects(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/projects/search", body)
}

// SearchAccountContacts calls POST /accounts/{id}/contacts/search. Response
// is returned as raw JSON; typed response structs are deferred to the caller.
// See SearchProjects's doc comment for the empty-searchQuery quirk.
func (c *Client) SearchAccountContacts(ctx context.Context, accountID string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/accounts/%s/contacts/search", url.PathEscape(accountID)), body)
}

// SearchProjectContacts calls POST /projects/{id}/contacts/search. Response
// is returned as raw JSON; typed response structs are deferred to the caller.
// See SearchProjects's doc comment for the empty-searchQuery quirk.
func (c *Client) SearchProjectContacts(ctx context.Context, projectID string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/projects/%s/contacts/search", url.PathEscape(projectID)), body)
}

// UpdateProject calls PATCH /projects/{id}. This is a
// ServiceNow-data-source-only operation — it requires a forwarded
// x-user-id-token, which this client never has (see Client's doc comment);
// calls will 401 until the M2M-auth open dependency is resolved. Response is
// returned as raw JSON; typed response structs are deferred to the caller.
func (c *Client) UpdateProject(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, fmt.Sprintf("/projects/%s", url.PathEscape(id)), body)
}
