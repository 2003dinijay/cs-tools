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
// Package unresolvedproducts is the daily digest of products that deployment nodes report
// but the product catalogue does not know. It replaces ServiceNow's email to one fixed
// address on every such miss: entity-service keeps what each node reported and lists the
// unresolved ones (GET /instances/unresolved-products), and this task sends the list once a
// day to the recipients configured for it, and nothing when there is nothing to report.
package unresolvedproducts

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/httpsec"
)

var tokenFetchTimeout = 10 * time.Second

// Config holds this client's configuration.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// Client calls entity-service's unresolved products listing. It is internal-only: this
// component's client id must be in entity-service's M2M_CLIENT_IDS.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient constructs a Client authenticated via the client credentials grant.
func NewClient(cfg Config) (*Client, error) {
	if err := httpsec.RequireSecureURL(cfg.TokenURL); err != nil {
		return nil, fmt.Errorf("unresolvedproducts: token URL: %w", err)
	}
	if err := httpsec.RequireSecureURL(cfg.BaseURL); err != nil {
		return nil, fmt.Errorf("unresolvedproducts: base URL: %w", err)
	}
	cc := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
		Scopes:       cfg.Scopes,
	}
	tokenHTTP := &http.Client{Timeout: tokenFetchTimeout}
	httpsec.RejectInsecureRedirects(tokenHTTP)
	httpClient := cc.Client(context.WithValue(context.Background(), oauth2.HTTPClient, tokenHTTP))
	httpClient.Timeout = 60 * time.Second
	httpsec.RejectInsecureRedirects(httpClient)

	return &Client{http: httpClient, baseURL: strings.TrimRight(cfg.BaseURL, "/")}, nil
}

// Node is one unresolved node, in entity-service's wire shape.
type Node struct {
	NodeID           string  `json:"nodeId"`
	ProjectKey       *string `json:"projectKey"`
	DeploymentNumber *string `json:"deploymentNumber"`
	ReportedProduct  string  `json:"reportedProduct"`
	ReportedVersion  *string `json:"reportedVersion"`
	// UnresolvedSince is when the node's reports stopped matching the catalogue.
	UnresolvedSince time.Time `json:"unresolvedSince"`
	LastSeenOn      time.Time `json:"lastSeenOn"`
}

// Listing is one page of unresolved nodes. Truncated is true when entity-service had
// more than it returned.
type Listing struct {
	Nodes     []Node
	Truncated bool
}

// maxNodes is the most rows one digest asks for: entity-service's own cap.
const maxNodes = 5000

// ListSince returns the nodes last seen since the given time whose reported product is not
// in the catalogue, grouped by what they reported, and whether the list was cut off.
func (c *Client) ListSince(ctx context.Context, since time.Time) (Listing, error) {
	q := url.Values{}
	q.Set("since", since.UTC().Format(time.RFC3339))
	q.Set("limit", fmt.Sprint(maxNodes))
	path := "/instances/unresolved-products?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return Listing{}, fmt.Errorf("unresolvedproducts: build request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return Listing{}, fmt.Errorf("unresolvedproducts: GET /instances/unresolved-products: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Listing{}, fmt.Errorf("unresolvedproducts: read response body: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return Listing{}, &apierror.Error{StatusCode: resp.StatusCode, Body: string(body)}
	}
	var parsed struct {
		Nodes     []Node `json:"nodes"`
		Truncated bool   `json:"truncated"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return Listing{}, fmt.Errorf("unresolvedproducts: decode response: %w", err)
	}
	return Listing{Nodes: parsed.Nodes, Truncated: parsed.Truncated}, nil
}
