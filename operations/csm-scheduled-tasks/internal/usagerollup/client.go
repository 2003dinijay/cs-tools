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
// Package usagerollup triggers entity-service's usage rollups: the daily usage
// rows rebuilt from the hourly ones, and the monthly rows rebuilt from the daily
// ones. They are the Go replacement for ServiceNow's UsageAggregator
// (runAggregationForDate / runAggregationForMonth).
//
// *** A TRIGGER, NOT A CALCULATOR. *** The rollups are set-based SQL inside the
// database (entity-service migration 0245), deliberately: summing every node's
// hourly rows over HTTP and writing the summaries back would drag the whole
// working set across the wire each night, and this component holds no database
// credentials anyway. Same split as internal/availability.
package usagerollup

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/httpsec"
)

var tokenFetchTimeout = 10 * time.Second

// rollupTimeout bounds one call. A day's rollup touches every node that
// reported that day, so it is given more room than a typical lookup.
const rollupTimeout = 5 * time.Minute

// Config holds this client's configuration.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// Client calls entity-service's usage rollup endpoints. They are internal-only:
// this component's client id must be in entity-service's M2M_CLIENT_IDS.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient constructs a Client authenticated via the client credentials grant.
func NewClient(cfg Config) (*Client, error) {
	if err := httpsec.RequireSecureURL(cfg.TokenURL); err != nil {
		return nil, fmt.Errorf("usagerollup: token URL: %w", err)
	}
	if err := httpsec.RequireSecureURL(cfg.BaseURL); err != nil {
		return nil, fmt.Errorf("usagerollup: base URL: %w", err)
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
	httpClient.Timeout = rollupTimeout
	httpsec.RejectInsecureRedirects(httpClient)

	return &Client{http: httpClient, baseURL: strings.TrimRight(cfg.BaseURL, "/")}, nil
}

// result is entity-service's wire shape for both rollups.
type result struct {
	RowsWritten int    `json:"rowsWritten"`
	Status      string `json:"status"`
}

// RollupDaily asks entity-service to rebuild one UTC day's daily usage rows
// (day is YYYY-MM-DD) and returns how many rows it wrote.
//
// Idempotent: a rerun over unchanged hourly rows writes nothing, so a missed or
// repeated night is safe to run again.
func (c *Client) RollupDaily(ctx context.Context, day string) (int, error) {
	return c.post(ctx, "/instances/usage-rollups/daily", map[string]any{"date": day})
}

// RollupMonthly asks entity-service to rebuild one calendar month's monthly
// usage rows and returns how many rows it wrote. Idempotent like RollupDaily.
func (c *Client) RollupMonthly(ctx context.Context, year, month int) (int, error) {
	return c.post(ctx, "/instances/usage-rollups/monthly", map[string]any{"year": year, "month": month})
}

func (c *Client) post(ctx context.Context, path string, payload map[string]any) (int, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("usagerollup: encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("usagerollup: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("usagerollup: POST %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("usagerollup: read response body: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return 0, &apierror.Error{StatusCode: resp.StatusCode, Body: string(respBody)}
	}
	var parsed result
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return 0, fmt.Errorf("usagerollup: decode response: %w", err)
	}
	return parsed.RowsWritten, nil
}
