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

// Package servicenow is a client for SupportPortalLite's ServiceNow
// integration: the Table API (GET /api/now/table/{table}) plus a custom
// scoped-app REST API on the same host (e.g. GET
// /api/wso2/wso2_team_schedule/schedule), both Basic-Auth-protected. Ported
// from the Ballerina backend's modules/operations package (its single
// snClient, shared by every function in that module).
package servicenow

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

// Config holds the configuration for the ServiceNow client.
type Config struct {
	// BaseURL is snHost in the Ballerina config — the ServiceNow instance
	// root, e.g. "https://wso2.service-now.com". Both the Table API and the
	// custom scoped-app API live under this same host.
	BaseURL  string
	Username string
	Password string

	// EscalationTemplateID is escalationTemplateId in the Ballerina config —
	// the "type" field value ServiceNow expects when creating a new
	// escalation record (see Client.EscalateCase).
	EscalationTemplateID string
	// TeamScheduleURL is teamScheduleUrl in the Ballerina config — a base
	// URL an integration-CS-team sys_id is appended to when building
	// AccountDetails.IntegrationCSTeamScheduleURL, and also returned
	// verbatim as ABTTeamScheduleData's snURL.
	TeamScheduleURL string
}

// Client is an HTTP client for ServiceNow's Table API and SupportPortalLite's
// custom scoped-app REST API, authenticated via HTTP Basic Auth.
type Client struct {
	http                 *http.Client
	baseURL              string
	username             string
	password             string
	escalationTemplateID string
	teamScheduleURL      string
}

// NewClient constructs a ServiceNow Client. Requests are retried up to twice
// on 500/502/503/504/408, mirroring the Ballerina snClient's retryConfig
// (count: 2, on the same status codes).
func NewClient(cfg Config) *Client {
	return &Client{
		http: &http.Client{
			Transport: &retryTransport{base: http.DefaultTransport, maxRetries: 2},
			Timeout:   30 * time.Second,
		},
		baseURL:              strings.TrimRight(cfg.BaseURL, "/"),
		username:             cfg.Username,
		password:             cfg.Password,
		escalationTemplateID: cfg.EscalationTemplateID,
		teamScheduleURL:      cfg.TeamScheduleURL,
	}
}

// TableQuery performs a GET against the ServiceNow Table API for the given
// table, e.g. TableQuery(ctx, "customer_account", url.Values{"sysparm_query":
// {"number=123"}, "sysparm_limit": {"1"}}) mirrors the Ballerina
// snClient->/api/now/'table/customer_account(sysparm_query=...,
// sysparm_limit=...) call shape. Returns the raw JSON response body.
func (c *Client) TableQuery(ctx context.Context, table string, params url.Values) ([]byte, error) {
	return c.get(ctx, "/api/now/table/"+url.PathEscape(table), params)
}

// TableQueryWithHeaders is TableQuery, additionally returning the upstream
// response headers — needed for endpoints that read ServiceNow's
// "X-Total-Count" pagination header alongside the body (e.g. case search,
// comment/worknote listing).
func (c *Client) TableQueryWithHeaders(ctx context.Context, table string, params url.Values) ([]byte, http.Header, error) {
	resp, err := c.doRaw(ctx, http.MethodGet, "/api/now/table/"+url.PathEscape(table), params, nil)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("servicenow: read response body: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		const maxErrBody = 256
		excerpt := respBody
		if len(excerpt) > maxErrBody {
			excerpt = excerpt[:maxErrBody]
		}
		return nil, nil, &apierror.Error{StatusCode: resp.StatusCode, Body: string(excerpt)}
	}

	return respBody, resp.Header, nil
}

// TablePatch performs a PATCH against a single ServiceNow Table API record
// identified by its sys_id, e.g. TablePatch(ctx, "sn_customerservice_case",
// caseSysID, body) mirrors the Ballerina
// snClient->/api/now/'table/sn_customerservice_case/[caseSysId].patch(...)
// call shape. Returns the raw JSON response body.
func (c *Client) TablePatch(ctx context.Context, table, sysID string, body []byte) ([]byte, error) {
	path := "/api/now/table/" + url.PathEscape(table) + "/" + url.PathEscape(sysID)
	return c.do(ctx, http.MethodPatch, path, nil, body)
}

// CustomGet performs a GET against SupportPortalLite's custom scoped-app
// REST API, which lives on the same ServiceNow host under its own path
// namespace (e.g. "/api/wso2/wso2_team_schedule/schedule"). path must start
// with "/". Returns the raw JSON response body.
func (c *Client) CustomGet(ctx context.Context, path string, params url.Values) ([]byte, error) {
	return c.get(ctx, path, params)
}

// CustomPost performs a POST with a JSON body against SupportPortalLite's
// custom scoped-app REST API. Returns the raw JSON response body.
func (c *Client) CustomPost(ctx context.Context, path string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, path, nil, body)
}

// CustomPut performs a PUT with a JSON body against SupportPortalLite's
// custom scoped-app REST API. Returns the raw JSON response body.
func (c *Client) CustomPut(ctx context.Context, path string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPut, path, nil, body)
}

// GetBinary performs a GET and returns the raw response body together with
// the upstream Content-Type and Content-Disposition headers, for endpoints
// that return non-JSON binary content (e.g. attachment download).
func (c *Client) GetBinary(ctx context.Context, path string, params url.Values) (body []byte, contentType string, contentDisposition string, err error) {
	resp, err := c.doRaw(ctx, http.MethodGet, path, params, nil)
	if err != nil {
		return nil, "", "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", "", fmt.Errorf("servicenow: read response body: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		const maxErrBody = 256
		excerpt := respBody
		if len(excerpt) > maxErrBody {
			excerpt = excerpt[:maxErrBody]
		}
		return nil, "", "", &apierror.Error{StatusCode: resp.StatusCode, Body: string(excerpt)}
	}

	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "application/octet-stream"
	}
	return respBody, ct, resp.Header.Get("Content-Disposition"), nil
}

func (c *Client) get(ctx context.Context, path string, params url.Values) ([]byte, error) {
	return c.do(ctx, http.MethodGet, path, params, nil)
}

func (c *Client) do(ctx context.Context, method, path string, params url.Values, body []byte) ([]byte, error) {
	resp, err := c.doRaw(ctx, method, path, params, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("servicenow: read response body: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		const maxErrBody = 256
		excerpt := respBody
		if len(excerpt) > maxErrBody {
			excerpt = excerpt[:maxErrBody]
		}
		return nil, &apierror.Error{StatusCode: resp.StatusCode, Body: string(excerpt)}
	}

	return respBody, nil
}

func (c *Client) doRaw(ctx context.Context, method, path string, params url.Values, body []byte) (*http.Response, error) {
	reqURL := c.baseURL + path
	if len(params) > 0 {
		reqURL += "?" + params.Encode()
	}

	var reqBody io.Reader
	if len(body) > 0 {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("servicenow: build request %s %s: %w", method, path, err)
	}
	req.SetBasicAuth(c.username, c.password)
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("servicenow: %s %s: %w", method, path, err)
	}
	return resp, nil
}

// retryTransport retries a request up to maxRetries times when the upstream
// response status is one Ballerina's snClient retryConfig also retries on:
// 500, 502, 503, 504, and 408. Only requests with no body, or a body that
// can be safely re-read (http.Request.GetBody set, which
// http.NewRequestWithContext populates automatically for the []byte-backed
// readers this package uses), are retried.
type retryTransport struct {
	base       http.RoundTripper
	maxRetries int
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	for attempt := 0; attempt < t.maxRetries && shouldRetry(resp, err); attempt++ {
		if resp != nil {
			_ = resp.Body.Close()
		}
		if req.GetBody != nil {
			body, gbErr := req.GetBody()
			if gbErr != nil {
				break
			}
			req.Body = body
		}
		resp, err = t.base.RoundTrip(req)
	}
	return resp, err
}

func shouldRetry(resp *http.Response, err error) bool {
	if err != nil {
		return true
	}
	switch resp.StatusCode {
	case http.StatusRequestTimeout, http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}
