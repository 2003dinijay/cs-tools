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

// Package entity is an HTTP client for csm-integration-service — the M2M
// gateway this component calls instead of entity-service directly (per
// Sajith's guidance and the "used by the Account Closure Process automation"
// comment already present in csm-integration-service's own client). See
// integrations/csm-integration-service/openapi.yaml for the API contract.
package entity

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/acp-closure-service/internal/apierror"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// RequiredScopes is the exact space-separated scope set csm-integration-service's
// token endpoint requires — confirmed empirically via Postman against the real
// staging token endpoint; a bare grant_type=client_credentials with no scope is
// not sufficient. Pass this as Config.Scopes when constructing a Client.
var RequiredScopes = []string{
	"csm_integration:project:read",
	"csm_integration:account:read",
	"csm_integration:accounts:read",
	"csm_integration:project:update",
	"csm_integration:accounts:contacts:read",
	"csm_integration:projects:contacts:read",
	"csm_integration:projects:read",
}

// tokenFetchTimeout is the HTTP client timeout for token-endpoint requests.
// Overridden in tests to keep them fast.
var tokenFetchTimeout = 10 * time.Second

type ctxKey string

const correlationIDKey ctxKey = "x-csm-correlation-id" // #nosec G101 -- context map key, not a credential

// WithCorrelationID returns a copy of ctx carrying the correlation ID to be
// forwarded as X-CSM-Correlation-ID on every outgoing request.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationIDKey, id)
}

func correlationIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(correlationIDKey).(string)
	return v
}

// Config holds the configuration for the csm-integration-service client.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	// Scopes should be RequiredScopes — see its doc comment.
	Scopes []string
}

// Client is an HTTP client authenticated to csm-integration-service via the
// OAuth2 client credentials grant. Tokens are acquired and refreshed
// automatically; callers need not manage them.
//
// Unlike csm-integration-service's own entity client, this Client never
// forwards an x-user-id-token: this component is a headless scheduled job
// with no end-user session, ever, so that pass-through has no caller here.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient constructs a Client that authenticates against
// csm-integration-service using the OAuth2 client credentials grant type,
// mirroring csm-integration-service's own internal/entity/client.go.
func NewClient(cfg Config) *Client {
	cc := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
		Scopes:       cfg.Scopes,
	}

	tokenCtx := context.WithValue(context.Background(), oauth2.HTTPClient,
		&http.Client{Timeout: tokenFetchTimeout})
	httpClient := cc.Client(tokenCtx)
	httpClient.Timeout = 25 * time.Second

	return &Client{
		http:    httpClient,
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
	}
}

// do executes an authenticated HTTP request against csm-integration-service
// and returns the raw JSON response body. The caller owns the returned slice.
func (c *Client) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	var reqBody io.Reader
	if len(body) > 0 {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("entity: build request %s %s: %w", method, path, err)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	if id := correlationIDFromContext(ctx); id != "" {
		req.Header.Set("X-CSM-Correlation-ID", id)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("entity: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		const maxErrBody = 256
		excerpt, err := io.ReadAll(io.LimitReader(resp.Body, maxErrBody))
		if err != nil {
			return nil, fmt.Errorf("entity: read error response body: %w", err)
		}
		return nil, &apierror.Error{StatusCode: resp.StatusCode, Body: string(excerpt)}
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("entity: read response body: %w", err)
	}

	return respBody, nil
}
