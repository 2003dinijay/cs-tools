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

// Package registry is the HTTP client for the container/robot-account
// registry service — a separate microservice (not entity-service) that
// issues registry access tokens ("robot accounts") scoped to a project, and
// looks up a project's integration users.
package registry

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/apierror"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

var tokenFetchTimeout = 10 * time.Second

// maxResponseBodyBytes bounds how much of a registry-service response this
// client will read into memory.
const maxResponseBodyBytes = 10 << 20 // 10 MiB

func noRedirect(_ *http.Request, _ []*http.Request) error {
	return http.ErrUseLastResponse
}

// Config holds the configuration for the registry service client.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// Client is an HTTP client for the registry service, authenticated via the
// OAuth2 client credentials grant. Tokens are acquired and refreshed
// automatically; callers need not manage them.
type Client struct {
	http    *http.Client
	baseURL string
}

// validateBaseURL rejects a base URL that isn't https with a host. Every
// request through this client carries an OAuth2 bearer token in the
// Authorization header — an http:// base URL would send that token in
// cleartext, including to anything else that can observe loopback traffic, so
// http is rejected unconditionally rather than exempting localhost.
func validateBaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("registry: invalid base URL %q: %w", raw, err)
	}
	if u.Scheme != "https" || u.Hostname() == "" {
		return fmt.Errorf("registry: base URL %q must be an https URL with a host", raw)
	}
	return nil
}

// NewClient constructs a Client that authenticates against the registry
// service using the OAuth2 client credentials grant type.
func NewClient(cfg Config) (*Client, error) {
	if err := validateBaseURL(cfg.BaseURL); err != nil {
		return nil, err
	}

	cc := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
		Scopes:       cfg.Scopes,
	}

	tokenCtx := context.WithValue(context.Background(), oauth2.HTTPClient,
		&http.Client{Timeout: tokenFetchTimeout, CheckRedirect: noRedirect})

	oauthClient := cc.Client(tokenCtx)
	httpClient := &http.Client{
		Transport:     oauthClient.Transport,
		Timeout:       25 * time.Second,
		CheckRedirect: noRedirect,
	}

	return &Client{
		http:    httpClient,
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
	}, nil
}

// do executes an authenticated HTTP request against the registry service and
// returns the raw JSON response body. The caller owns the returned slice.
// Returns (nil, nil) for a 204/empty-body success response.
func (c *Client) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	var reqBody io.Reader
	if len(body) > 0 {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("registry: build request %s %s: %w", method, path, err)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("registry: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, maxResponseBodyBytes+1)
	respBody, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("registry: read response body: %w", err)
	}
	if len(respBody) > maxResponseBodyBytes {
		return nil, fmt.Errorf("registry: %s %s: response body exceeds %d bytes", method, path, maxResponseBodyBytes)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, apierror.NewUpstreamError(resp.StatusCode, respBody)
	}

	return respBody, nil
}
