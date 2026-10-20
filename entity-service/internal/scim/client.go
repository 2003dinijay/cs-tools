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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

// Package scim is an HTTP client for the WSO2 internal-org SCIM search API
// (POST /organizations/internal/users/search), the same operations service
// apps/csm-portal/backend and apps/customer-portal/backend-v2 already call
// from their own internal/scim packages. This client only asks for the one
// thing neither of those two existing clients requests: a user's given/family
// name, needed to auto-provision a CSM "user" row for a Salesforce account
// person reference (owner, technical owner, etc.) that has no match yet.
package scim

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

const (
	// org is fixed to "internal": every email this client resolves is a
	// WSO2 staff member's own (an account manager, technical owner, etc.),
	// reached through the same org csm-portal and customer-portal use for
	// their own employee-facing SCIM lookups -- never the "external" org,
	// which holds customer/partner contacts.
	org = "internal"

	searchPath = "/organizations/" + org + "/users/search"

	domainDefault  = "DEFAULT"
	attrGivenName  = "name.givenName"
	attrFamilyName = "name.familyName"
	attrUserName   = "userName"

	defaultTimeout   = 15 * time.Second
	tokenExpirySlack = 30 * time.Second
)

// ClientCredentialsConfig holds the OAuth2 client credentials used to obtain
// a bearer token for calls to the SCIM operations service.
type ClientCredentialsConfig struct {
	TokenURL     string
	ClientID     string
	ClientSecret string
	// Scopes is a space-separated list of OAuth2 scopes.
	Scopes string
}

// Person is the subset of a SCIM User resource this client asks for: the
// given/family name of a WSO2 staff member, looked up by email.
type Person struct {
	GivenName  string
	FamilyName string
}

type searchRequest struct {
	Domain     string   `json:"domain"`
	Attributes []string `json:"attributes"`
	Filter     string   `json:"filter"`
	StartIndex int      `json:"startIndex"`
}

type searchResponse struct {
	TotalResults int    `json:"totalResults"`
	Resources    []user `json:"Resources"`
}

// user mirrors the one piece of the SCIM User resource this client reads.
// RFC 7643 ยง4.1.1 nests given/family name under "name"; a search response
// only returns whichever attributes were requested (see searchRequest.Attributes),
// so a resource with no name on file at all decodes with Name nil. UserName
// is requested alongside Name purely so searchPerson can confirm a result
// actually matches the email queried (see its own doc comment) rather than
// trusting the server's filter match unconditionally.
type user struct {
	UserName string    `json:"userName,omitempty"`
	Name     *userName `json:"name,omitempty"`
}

type userName struct {
	GivenName  string `json:"givenName,omitempty"`
	FamilyName string `json:"familyName,omitempty"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

var errUnauthorized = errors.New("scim: request unauthorized")

// Client searches the WSO2 internal-org SCIM directory using a
// client-credentials grant.
type Client struct {
	baseURL      string
	tokenURL     string
	clientID     string
	clientSecret string
	scopes       string
	httpClient   *http.Client

	mu          sync.Mutex
	cachedToken string
	tokenExpiry time.Time
}

// New constructs a Client with a default 15-second timeout.
func New(baseURL string, creds ClientCredentialsConfig) *Client {
	return &Client{
		baseURL:      strings.TrimRight(baseURL, "/"),
		tokenURL:     creds.TokenURL,
		clientID:     creds.ClientID,
		clientSecret: creds.ClientSecret,
		scopes:       creds.Scopes,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// SearchPerson looks up a WSO2 staff member's given/family name by email.
// Returns (nil, nil) when the directory has no matching user -- that is a
// normal outcome for the caller (the account person reference stays
// unresolved), not an error. On 401 the token is refreshed and the request
// is retried once, mirroring internal/salesentity.Client's own policy.
func (c *Client) SearchPerson(ctx context.Context, email string) (*Person, error) {
	p, err := c.searchPerson(ctx, email)
	if errors.Is(err, errUnauthorized) {
		c.invalidateToken()
		p, err = c.searchPerson(ctx, email)
		if errors.Is(err, errUnauthorized) {
			return nil, &apierror.UnauthorizedError{Msg: "scim: users/search unauthorized"}
		}
	}
	return p, err
}

func (c *Client) searchPerson(ctx context.Context, email string) (*Person, error) {
	// email is expected to already be validated by the caller (service.
	// validateEmail's regex excludes every character checked here), but this
	// client has no way to enforce that on a future caller, and %q alone is
	// not a SCIM-safe escape (it is Go's own quoting, not RFC 7644's) -- so
	// a value this permissive would still reach the filter unescaped. Refuse
	// it outright rather than risk the filter matching something broader
	// than the one address asked for.
	if strings.ContainsAny(email, `"\`) || hasControlByte(email) {
		return nil, &apierror.ValidationError{Msg: "scim: email contains a character that cannot be used in a search filter"}
	}
	token, err := c.accessToken(ctx)
	if err != nil {
		return nil, err
	}

	// The filter literal must be a quoted SCIM string per RFC 7644 ยง3.4.2.2
	// ("userName eq \"jane@wso2.com\""), not a bare token.
	body, err := json.Marshal(searchRequest{
		Domain:     domainDefault,
		Attributes: []string{attrUserName, attrGivenName, attrFamilyName},
		Filter:     fmt.Sprintf("userName eq %q", email),
		StartIndex: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("scim: marshal search request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+searchPath, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("scim: build search request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, err
		}
		return nil, &apierror.ServiceUnavailableError{Msg: fmt.Sprintf("scim: search request: %v", err)}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("scim: read search response: %w", err)
	}

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		var result searchResponse
		if err := json.Unmarshal(raw, &result); err != nil {
			return nil, fmt.Errorf("scim: parse search response: %w", err)
		}
		// Resources[0] is not trusted on filter match alone: a permissive or
		// buggy server-side filter implementation could return a broader set
		// than the exact address asked for (RFC 7644 ยง3.4.2.2 "eq" is
		// supposed to be exact, but this client does not rely on the server
		// getting that right for a security-relevant decision). userName is
		// WSO2's own convention for a staff member's email (the same
		// assumption the search filter itself already makes), so requiring
		// it to match case-insensitively is the direct check, not a proxy.
		for _, u := range result.Resources {
			if !strings.EqualFold(u.UserName, email) || u.Name == nil {
				continue
			}
			return &Person{GivenName: u.Name.GivenName, FamilyName: u.Name.FamilyName}, nil
		}
		return nil, nil
	case resp.StatusCode == http.StatusUnauthorized:
		return nil, errUnauthorized
	case resp.StatusCode >= 500:
		return nil, &apierror.ServiceUnavailableError{Msg: fmt.Sprintf("scim: search returned %d", resp.StatusCode)}
	default:
		return nil, &apierror.DownstreamError{Msg: fmt.Sprintf("scim rejected the search (status %d)", resp.StatusCode)}
	}
}

func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cachedToken != "" && time.Now().Add(tokenExpirySlack).Before(c.tokenExpiry) {
		return c.cachedToken, nil
	}

	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", c.clientID)
	data.Set("client_secret", c.clientSecret)
	if c.scopes != "" {
		data.Set("scope", c.scopes)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("scim: build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return "", err
		}
		return "", &apierror.ServiceUnavailableError{Msg: fmt.Sprintf("scim: token endpoint unavailable: %v", err)}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("scim: read token response: %w", err)
	}
	switch resp.StatusCode {
	case http.StatusOK:
		// ok
	case http.StatusUnauthorized, http.StatusForbidden:
		return "", &apierror.UnauthorizedError{Msg: fmt.Sprintf("scim: token endpoint rejected credentials (status %d)", resp.StatusCode)}
	default:
		return "", &apierror.ServiceUnavailableError{Msg: fmt.Sprintf("scim: token endpoint returned %d", resp.StatusCode)}
	}

	var tr tokenResponse
	if err := json.Unmarshal(raw, &tr); err != nil {
		return "", fmt.Errorf("scim: parse token response: %w", err)
	}
	if tr.AccessToken == "" {
		return "", fmt.Errorf("scim: token endpoint returned empty access_token")
	}

	c.cachedToken = tr.AccessToken
	if tr.ExpiresIn > 0 {
		c.tokenExpiry = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	} else {
		c.tokenExpiry = time.Now().Add(3600 * time.Second)
	}
	return c.cachedToken, nil
}

func (c *Client) invalidateToken() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cachedToken = ""
	c.tokenExpiry = time.Time{}
}

// hasControlByte reports whether s contains a C0/C1 control character or
// DEL, none of which belong in an email address and some of which (CR, LF)
// could otherwise be used to smuggle extra content into the request body.
func hasControlByte(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] == 0x7f {
			return true
		}
	}
	return false
}
